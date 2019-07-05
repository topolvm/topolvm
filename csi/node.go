package csi

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"github.com/cybozu-go/well"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// DeviceDirectory is a directory where TopoLVM Node service creates device files.
	DeviceDirectory = "/dev/topolvm"

	mkfsCmd          = "/sbin/mkfs"
	mountCmd         = "/bin/mount"
	mountpointCmd    = "/bin/mountpoint"
	umountCmd        = "/bin/umount"
	devicePermission = 0600 | unix.S_IFBLK
)

// NewNodeService returns a new NodeServer.
func NewNodeService(nodeName string, conn *grpc.ClientConn) NodeServer {
	return &nodeService{
		nodeName: nodeName,
		client:   proto.NewVGServiceClient(conn),
	}
}

type nodeService struct {
	nodeName string
	client   proto.VGServiceClient
	mu       sync.Mutex
}

func (s *nodeService) NodeStageVolume(context.Context, *NodeStageVolumeRequest) (*NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeStageVolume not implemented")
}

func (s *nodeService) NodeUnstageVolume(context.Context, *NodeUnstageVolumeRequest) (*NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeUnstageVolume not implemented")
}

func (s *nodeService) NodePublishVolume(ctx context.Context, req *NodePublishVolumeRequest) (*NodePublishVolumeResponse, error) {
	log.Info("NodePublishVolume called", map[string]interface{}{
		"volume_id":         req.GetVolumeId(),
		"publish_context":   req.GetPublishContext(),
		"target_path":       req.GetTargetPath(),
		"volume_capability": req.GetVolumeCapability(),
		"read_only":         req.GetReadonly(),
		"num_secrets":       len(req.GetSecrets()),
		"volume_context":    req.GetVolumeContext(),
	})

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume_capability is provided")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	listResp, err := s.client.GetLVList(ctx, &proto.Empty{})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list LV: %v", err)
	}
	lv := s.findVolumeByID(listResp, req.GetVolumeId())
	if lv == nil {
		return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", req.GetVolumeId())
	}

	if req.GetVolumeCapability().GetBlock() != nil {
		return s.nodePublishBlockVolume(ctx, req, lv)
	}
	if req.GetVolumeCapability().GetMount() != nil {
		return s.nodePublishFilesystemVolume(ctx, req, lv)
	}
	return nil, status.Errorf(codes.InvalidArgument, "no supported volume capability: %v", req.GetVolumeCapability())
}

func (s *nodeService) nodePublishFilesystemVolume(ctx context.Context, req *NodePublishVolumeRequest, lv *proto.LogicalVolume) (*NodePublishVolumeResponse, error) {
	// Check request
	mountOption := req.GetVolumeCapability().GetMount()
	if mountOption.FsType == "" {
		mountOption.FsType = "ext4"
	}
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	if accessMode != VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		modeName := VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s", modeName)
	}

	// Find lv and create a block device with it
	device := filepath.Join(DeviceDirectory, req.GetVolumeId())
	stat := new(unix.Stat_t)
	err := unix.Stat(device, stat)
	switch err {
	case nil:
		// a block device already exists, check and validate filesystem
		fsType, err := detectFsType(ctx, device)
		if err != nil {
			return nil, err
		}
		if fsType != "" && fsType != mountOption.FsType {
			return nil, status.Errorf(codes.InvalidArgument, "requested fs type and existing one are different, requested: %s, existing: %s", mountOption.FsType, fsType)
		}

		// Check device
		if stat.Rdev == unix.Mkdev(lv.DevMajor, lv.DevMinor) && (stat.Mode&devicePermission) == devicePermission {
			return &NodePublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "device's permission is invalid. expected: %x, actual: %x", devicePermission, stat.Mode)
	case unix.ENOENT:
		dev, err := mkdev(lv.DevMajor, lv.DevMinor)
		if err != nil {
			return nil, err
		}
		err = unix.Mknod(device, devicePermission, dev)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "mknod failed for %s. major=%d, minor=%d, error=%v",
				device, lv.DevMajor, lv.DevMinor, err)
		}
	default:
		return nil, status.Errorf(codes.Internal, "failed to stat: %v", err)
	}

	// a block device is created, check and validate filesystem
	fsType, err := detectFsType(ctx, device)
	if err != nil {
		return nil, err
	}
	if fsType != "" && fsType != mountOption.FsType {
		return nil, status.Errorf(codes.InvalidArgument, "requested fs type and existing one are different, existing: %s, requested: %s", fsType, mountOption.FsType)
	}

	// Check mountpoint
	out, err := well.CommandContext(ctx, mountpointCmd, "-d", req.GetTargetPath()).Output()
	if err == nil {
		out2, err := well.CommandContext(ctx, mountpointCmd, "-x", device).Output()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "mountpoint failed for %s, error: %v", device, err)
		}
		if !bytes.Equal(out, out2) {
			return nil, status.Errorf(codes.Internal, "device numbers are different, target_path: %s, device: %s", out, out2)
		}

		log.Warn("target_path already used", map[string]interface{}{
			"target_path": req.GetTargetPath(),
			"fstype":      mountOption.FsType,
		})
		return &NodePublishVolumeResponse{}, nil
	}

	err = os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed, target path: %s, error: %v", req.GetTargetPath(), err)
	}

	// Format filesystem
	if fsType == "" {
		out, err := well.CommandContext(ctx, mkfsCmd, "-t", mountOption.FsType, device).CombinedOutput()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to mkfs: %s, error: %v", out, err)
		}
	} else {
		log.Info("skipped mkfs, because file system already exists", map[string]interface{}{
			"device_path": device,
		})
	}

	// Mount filesystem
	out, err = well.CommandContext(ctx, mountCmd, "-t", mountOption.FsType, device, req.GetTargetPath()).CombinedOutput()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount: %s, error: %v", out, err)
	}

	log.Info("NodePublishVolume(fs) is succeeded", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
		"fstype":      mountOption.FsType,
	})

	return &NodePublishVolumeResponse{}, nil
}

func detectFsType(ctx context.Context, devicePath string) (string, error) {
	out, err := well.CommandContext(ctx, "file", "-bsL", devicePath).CombinedOutput()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(string(out)) == "data" {
		return "", nil
	}
	out, err = well.CommandContext(ctx, "blkid", "-c", "/dev/null", "-o", "export", devicePath).CombinedOutput()
	if err != nil {
		return "", err
	}
	for _, l := range strings.Split(string(out), "\n") {
		prefix := "TYPE="
		if !strings.HasPrefix(l, prefix) {
			continue
		}
		return l[len(prefix):], nil
	}
	return "", nil
}

func (s *nodeService) nodePublishBlockVolume(ctx context.Context, req *NodePublishVolumeRequest, lv *proto.LogicalVolume) (*NodePublishVolumeResponse, error) {
	// Find lv and create a block device with it
	stat := new(unix.Stat_t)
	err := unix.Stat(req.GetTargetPath(), stat)
	switch err {
	case nil:
		if stat.Rdev == unix.Mkdev(lv.DevMajor, lv.DevMinor) && stat.Mode&devicePermission == devicePermission {
			return &NodePublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.AlreadyExists,
			"device %s exists, but it is not expected block device. expected_mode: %x, actual_mode: %x", req.GetTargetPath(), devicePermission, stat.Mode)
	case unix.ENOENT:
	default:
		return nil, status.Errorf(codes.Internal, "failed to stat: %v", err)
	}

	dev, err := mkdev(lv.DevMajor, lv.DevMinor)
	if err != nil {
		return nil, err
	}
	err = unix.Mknod(req.GetTargetPath(), devicePermission, dev)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mknod failed for %s. error: %v", req.GetTargetPath(), err)
	}

	log.Info("NodePublishVolume(block) is succeeded", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
	})
	return &NodePublishVolumeResponse{}, nil
}

func (s *nodeService) findVolumeByID(listResp *proto.GetLVListResponse, name string) *proto.LogicalVolume {
	for _, v := range listResp.Volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (s *nodeService) NodeUnpublishVolume(ctx context.Context, req *NodeUnpublishVolumeRequest) (*NodeUnpublishVolumeResponse, error) {
	log.Info("NodeUnpublishVolume called", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
	})

	if len(req.GetVolumeId()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	device := filepath.Join(DeviceDirectory, req.GetVolumeId())

	info, err := os.Stat(req.GetTargetPath())
	if os.IsNotExist(err) {
		// target_path does not exist, but device for mount-type PV may still exist.
		_ = os.Remove(device)
		return &NodeUnpublishVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", req.GetTargetPath(), err)
	}

	// remove device file if target_path is device, umount target_path otherwise
	if info.IsDir() {
		return s.nodeUnpublishFilesystemVolume(ctx, req, device)
	}
	return s.nodeUnpublishBlockVolume(req)
}

func (s *nodeService) nodeUnpublishFilesystemVolume(ctx context.Context, req *NodeUnpublishVolumeRequest, device string) (*NodeUnpublishVolumeResponse, error) {
	out, err := well.CommandContext(ctx, umountCmd, req.GetTargetPath()).CombinedOutput()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "umount failed for %s: %s, error: %v", req.GetTargetPath(), out, err)
	}
	err = os.Remove(device)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "remove device failed for %s, error: %v", device, err)
	}
	err = os.RemoveAll(req.GetTargetPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "remove dir failed for %s, error: %v", req.GetTargetPath(), err)
	}

	log.Info("NodeUnpublishVolume(fs) is succeeded", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
	})
	return &NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) nodeUnpublishBlockVolume(req *NodeUnpublishVolumeRequest) (*NodeUnpublishVolumeResponse, error) {
	err := os.Remove(req.GetTargetPath())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "remove failed for %s: %v", req.GetTargetPath(), err)
	}
	log.Info("NodeUnpublishVolume(block) is succeeded", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
	})
	return &NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) NodeGetVolumeStats(ctx context.Context, req *NodeGetVolumeStatsRequest) (*NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeGetVolumeStats not implemented")
}

func (s *nodeService) NodeExpandVolume(context.Context, *NodeExpandVolumeRequest) (*NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeExpandVolume not implemented")
}

func (s *nodeService) NodeGetCapabilities(context.Context, *NodeGetCapabilitiesRequest) (*NodeGetCapabilitiesResponse, error) {
	return &NodeGetCapabilitiesResponse{
		Capabilities: []*NodeServiceCapability{
			{
				// TODO: add capabilities when we implement functions
				Type: &NodeServiceCapability_Rpc{
					Rpc: &NodeServiceCapability_RPC{
						Type: NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
		},
	}, nil
}

func (s *nodeService) NodeGetInfo(ctx context.Context, req *NodeGetInfoRequest) (*NodeGetInfoResponse, error) {
	return &NodeGetInfoResponse{
		NodeId: s.nodeName,
		AccessibleTopology: &Topology{
			Segments: map[string]string{
				topolvm.TopologyNodeKey: s.nodeName,
			},
		},
	}, nil
}

func mkdev(major, minor uint32) (int, error) {
	dev := unix.Mkdev(major, minor)
	devInt := int(dev)
	if dev != uint64(devInt) {
		return 0, status.Errorf(codes.Internal, "failed to convert. dev: %d, devInt: %d", dev, devInt)
	}
	return devInt, nil
}
