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

	mkfsCmd       = "/sbin/mkfs"
	mountCmd      = "/bin/mount"
	mountpointCmd = "/bin/mountpoint"
	umountCmd     = "/bin/umount"
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
	return nil, status.Errorf(codes.Unimplemented, "NodeStageVolume not implemented")
}

func (s *nodeService) NodeUnstageVolume(context.Context, *NodeUnstageVolumeRequest) (*NodeUnstageVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "NodeUnstageVolume not implemented")
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
		stat := new(unix.Stat_t)
		err = unix.Stat(req.GetTargetPath(), stat)
		if err == nil {
			if stat.Rdev == unix.Mkdev(lv.DevMajor, lv.DevMinor) && stat.Mode&unix.S_IFBLK != 0 {
				return &NodePublishVolumeResponse{}, nil
			}
			return nil, status.Errorf(codes.AlreadyExists, "target_path already used")
		} else if err != unix.ENOENT {
			return nil, status.Errorf(codes.Internal, "failed to stat: %v", err)

		}

		dev, err := mkdev(lv.DevMajor, lv.DevMinor)
		if err != nil {
			return nil, err
		}
		err = unix.Mknod(req.GetTargetPath(), unix.S_IFBLK|0660, dev)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "mknod failed for %s", req.GetTargetPath())
		}
		return &NodePublishVolumeResponse{}, nil
	}

	mountOption := req.GetVolumeCapability().GetMount()
	if mountOption == nil {
		return nil, status.Error(codes.Internal, "failed to GetMount")
	}
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	if accessMode != VolumeCapability_AccessMode_SINGLE_NODE_WRITER {
		modeName := VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s", modeName)
	}

	device := filepath.Join(DeviceDirectory, req.GetVolumeId())
	stat := new(unix.Stat_t)
	err = unix.Stat(device, stat)
	switch err {
	case nil:
		if !(stat.Rdev == unix.Mkdev(lv.DevMajor, lv.DevMinor) && stat.Mode&unix.S_IFBLK != 0) {
			return nil, status.Errorf(codes.Internal, "device %s exists, but it is not expected block device", device)
		}
	case unix.ENOENT:
		dev, err := mkdev(lv.DevMajor, lv.DevMinor)
		if err != nil {
			return nil, err
		}
		err = unix.Mknod(device, unix.S_IFBLK|0660, dev)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to mknod")
		}
	default:
		return nil, status.Errorf(codes.Internal, "failed to stat: %v", err)
	}

	out, err := well.CommandContext(ctx, mountpointCmd, "-d", req.GetTargetPath()).Output()
	if err == nil {
		out2, err := well.CommandContext(ctx, mountpointCmd, "-x", device).Output()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "mountpoint failed for %s", device)
		}
		if bytes.Equal(out, out2) {
			return &NodePublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.AlreadyExists, "target_path already used")
	}

	err = os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mkdir failed, target path: "+req.GetTargetPath())
	}

	existingFsType, err := detectFsType(ctx, device)
	if err != nil {
		return nil, err
	}
	if existingFsType == "" {
		out, err := well.CommandContext(ctx, mkfsCmd, "-t", mountOption.FsType, device).CombinedOutput()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to mkfs: %s", out)
		}
	} else {
		log.Info("skipped mkfs, because file system already exists", map[string]interface{}{
			"device_path": device,
		})
	}

	out, err = well.CommandContext(ctx, mountCmd, "-t", mountOption.FsType, device, req.TargetPath).CombinedOutput()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to mount: %s", out)
	}

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
	if info.Mode()&os.ModeDevice != 0 {
		err := os.Remove(req.GetTargetPath())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "remove failed for %s: %v", req.GetTargetPath(), err)
		}
	} else {

		out, err := well.CommandContext(ctx, umountCmd, req.GetTargetPath()).CombinedOutput()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "umount failed for %s: %s", req.GetTargetPath(), out)
		}
		err = os.Remove(device)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "remove failed for %s", device)
		}
	}

	return &NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeService) NodeGetVolumeStats(ctx context.Context, req *NodeGetVolumeStatsRequest) (*NodeGetVolumeStatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "NodeGetVolumeStats not implemented")
}

func (s *nodeService) NodeExpandVolume(context.Context, *NodeExpandVolumeRequest) (*NodeExpandVolumeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "NodeExpandVolume not implemented")
}

func (s *nodeService) NodeGetCapabilities(context.Context, *NodeGetCapabilitiesRequest) (*NodeGetCapabilitiesResponse, error) {
	return &NodeGetCapabilitiesResponse{
		Capabilities: []*NodeServiceCapability{
			{
				Type: &NodeServiceCapability_Rpc{
					Rpc: &NodeServiceCapability_RPC{
						Type: NodeServiceCapability_RPC_GET_VOLUME_STATS,
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
