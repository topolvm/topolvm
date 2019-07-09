package driver

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm"
	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/topolvm/filesystem"
	"github.com/cybozu-go/topolvm/lvmd/proto"
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
func NewNodeService(nodeName string, conn *grpc.ClientConn) csi.NodeServer {
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

func (s *nodeService) NodeStageVolume(context.Context, *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeStageVolume not implemented")
}

func (s *nodeService) NodeUnstageVolume(context.Context, *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeUnstageVolume not implemented")
}

func (s *nodeService) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
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

func (s *nodeService) nodePublishFilesystemVolume(ctx context.Context, req *csi.NodePublishVolumeRequest, lv *proto.LogicalVolume) (*csi.NodePublishVolumeResponse, error) {
	// Check request
	mountOption := req.GetVolumeCapability().GetMount()
	if mountOption.FsType == "" {
		mountOption.FsType = "ext4"
	}
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	if accessMode != csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s", modeName)
	}

	// Find lv and create a block device with it
	device := filepath.Join(DeviceDirectory, req.GetVolumeId())
	var stat unix.Stat_t
	err := unix.Stat(device, &stat)
	switch err {
	case nil:
		// a block device already exists, check its attributes
		if stat.Rdev == unix.Mkdev(lv.DevMajor, lv.DevMinor) && (stat.Mode&devicePermission) == devicePermission {
			break
		}

		err := os.Remove(device)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to remove device file %s: error=%v", device, err)
		}
		fallthrough
	case unix.ENOENT:
		devno := unix.Mkdev(lv.DevMajor, lv.DevMinor)
		if err := unix.Mknod(device, devicePermission, int(devno)); err != nil {
			return nil, status.Errorf(codes.Internal, "mknod failed for %s. major=%d, minor=%d, error=%v",
				device, lv.DevMajor, lv.DevMinor, err)
		}
	default:
		return nil, status.Errorf(codes.Internal, "failed to stat %s: error=%v", device, err)
	}

	fs, err := filesystem.New(mountOption.FsType, device)
	if err != nil {
		return nil, err
	}
	if !fs.Exists() {
		if err := fs.Mkfs(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create filesystem: volume=%s, error=%v", req.GetVolumeId(), err)
		}
	}

	err = os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", req.GetTargetPath(), err)
	}
	if err := fs.Mount(req.GetTargetPath(), req.GetReadonly()); err != nil {
		return nil, status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
	}

	log.Info("NodePublishVolume(fs) succeeded", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
		"fstype":      mountOption.FsType,
	})

	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) nodePublishBlockVolume(ctx context.Context, req *csi.NodePublishVolumeRequest, lv *proto.LogicalVolume) (*csi.NodePublishVolumeResponse, error) {
	// Find lv and create a block device with it
	var stat unix.Stat_t
	target := req.GetTargetPath()
	err := unix.Stat(target, &stat)
	switch err {
	case nil:
		if stat.Rdev == unix.Mkdev(lv.DevMajor, lv.DevMinor) && stat.Mode&devicePermission == devicePermission {
			return &csi.NodePublishVolumeResponse{}, nil
		}
		if err := os.Remove(target); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to remove %s", target)
		}
	case unix.ENOENT:
	default:
		return nil, status.Errorf(codes.Internal, "failed to stat: %v", err)
	}

	devno := unix.Mkdev(lv.DevMajor, lv.DevMinor)
	if err := unix.Mknod(target, devicePermission, int(devno)); err != nil {
		return nil, status.Errorf(codes.Internal, "mknod failed for %s: error=%v", req.GetTargetPath(), err)
	}

	log.Info("NodePublishVolume(block) succeeded", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
	})
	return &csi.NodePublishVolumeResponse{}, nil
}

func (s *nodeService) findVolumeByID(listResp *proto.GetLVListResponse, name string) *proto.LogicalVolume {
	for _, v := range listResp.Volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (s *nodeService) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volID := req.GetVolumeId()
	target := req.GetTargetPath()
	log.Info("NodeUnpublishVolume called", map[string]interface{}{
		"volume_id":   volID,
		"target_path": target,
	})

	if len(volID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(target) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	device := filepath.Join(DeviceDirectory, volID)

	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		// target_path does not exist, but device for mount-type PV may still exist.
		_ = os.Remove(device)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", target, err)
	}

	// remove device file if target_path is device, umount target_path otherwise
	if info.IsDir() {
		return s.nodeUnpublishFilesystemVolume(ctx, req, device)
	}
	return s.nodeUnpublishBlockVolume(req)
}

func (s *nodeService) nodeUnpublishFilesystemVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest, device string) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := filesystem.Unmount(device); err != nil {
		return nil, status.Errorf(codes.Internal, "umount failed for %s: error=%v", req.GetTargetPath(), err)
	}
	if err := os.RemoveAll(req.GetTargetPath()); err != nil {
		return nil, status.Errorf(codes.Internal, "remove dir failed for %s: error=%v", req.GetTargetPath(), err)
	}
	if err := os.Remove(device); err != nil {
		return nil, status.Errorf(codes.Internal, "remove device failed for %s: error=%v", device, err)
	}

	log.Info("NodeUnpublishVolume(fs) is succeeded", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
	})
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) nodeUnpublishBlockVolume(req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if err := os.Remove(req.GetTargetPath()); err != nil {
		return nil, status.Errorf(codes.Internal, "remove failed for %s: error=%v", req.GetTargetPath(), err)
	}
	log.Info("NodeUnpublishVolume(block) is succeeded", map[string]interface{}{
		"volume_id":   req.GetVolumeId(),
		"target_path": req.GetTargetPath(),
	})
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeGetVolumeStats not implemented")
}

func (s *nodeService) NodeExpandVolume(context.Context, *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	return nil, status.Error(codes.Unimplemented, "NodeExpandVolume not implemented")
}

func (s *nodeService) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				// TODO: add capabilities when we implement volume expansion
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
		},
	}, nil
}

func (s *nodeService) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.nodeName,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				topolvm.TopologyNodeKey: s.nodeName,
			},
		},
	}, nil
}
