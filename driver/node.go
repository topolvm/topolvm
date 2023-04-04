package driver

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/driver/internal/k8s"
	"github.com/topolvm/topolvm/filesystem"
	"github.com/topolvm/topolvm/lvmd/proto"
	"golang.org/x/sys/unix"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mountutil "k8s.io/mount-utils"
	utilexec "k8s.io/utils/exec"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// DeviceDirectory is a directory where TopoLVM Node service creates device files.
	DeviceDirectory = "/dev/topolvm"

	findmntCmd = "/bin/findmnt"

	deviceMode = 0600 | unix.S_IFBLK
)

var nodeLogger = ctrl.Log.WithName("driver").WithName("node")

// NewNodeServer returns a new NodeServer.
func NewNodeServer(nodeName string, conn *grpc.ClientConn, mgr manager.Manager) (csi.NodeServer, error) {
	lvService, err := k8s.NewLogicalVolumeService(mgr)
	if err != nil {
		return nil, err
	}

	return &nodeServer{
		server: &nodeServerNoLocked{
			nodeName:     nodeName,
			client:       proto.NewVGServiceClient(conn),
			lvService:    proto.NewLVServiceClient(conn),
			k8sLVService: lvService,
			mounter: mountutil.SafeFormatAndMount{
				Interface: mountutil.New(""),
				Exec:      utilexec.New(),
			},
		},
	}, nil
}

// This is a wrapper for nodeServerNoLocked to protect concurrent method calls.
type nodeServer struct {
	csi.UnimplementedNodeServer

	// This protects concurrent nodeServerNoLocked method calls.
	// We use a global lock because it assumes that each method does not take a long time,
	// and we scare about wired behaviors from concurrent device or filesystem operations.
	mu     sync.Mutex
	server *nodeServerNoLocked
}

func (s *nodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.server.NodePublishVolume(ctx, req)
}

func (s *nodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.server.NodeUnpublishVolume(ctx, req)
}

func (s *nodeServer) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.server.NodeGetVolumeStats(ctx, req)
}

func (s *nodeServer) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.server.NodeExpandVolume(ctx, req)
}

func (s *nodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	// This returns constant value only, it is unnecessary to take lock.
	return s.server.NodeGetCapabilities(ctx, req)
}

func (s *nodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	// This returns unmodified value only, it is unnecessary to take lock.
	return s.server.NodeGetInfo(ctx, req)
}

// nodeServerNoLocked implements csi.NodeServer.
// It does not take any lock, gRPC calls may be interleaved.
// Therefore, must not use it directly.
type nodeServerNoLocked struct {
	csi.UnimplementedNodeServer

	nodeName     string
	client       proto.VGServiceClient
	lvService    proto.LVServiceClient
	k8sLVService *k8s.LogicalVolumeService
	mounter      mountutil.SafeFormatAndMount
}

func (s *nodeServerNoLocked) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	volumeContext := req.GetVolumeContext()
	volumeID := req.GetVolumeId()

	nodeLogger.Info("NodePublishVolume called",
		"volume_id", volumeID,
		"publish_context", req.GetPublishContext(),
		"target_path", req.GetTargetPath(),
		"volume_capability", req.GetVolumeCapability(),
		"read_only", req.GetReadonly(),
		"num_secrets", len(req.GetSecrets()),
		"volume_context", volumeContext)

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(req.GetTargetPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "no volume_capability is provided")
	}
	isBlockVol := req.GetVolumeCapability().GetBlock() != nil
	isFsVol := req.GetVolumeCapability().GetMount() != nil
	if !(isBlockVol || isFsVol) {
		return nil, status.Errorf(codes.InvalidArgument, "no supported volume capability: %v", req.GetVolumeCapability())
	}
	// we only support SINGLE_NODE_WRITER
	accessMode := req.GetVolumeCapability().GetAccessMode().GetMode()
	switch accessMode {
	case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER:
	default:
		modeName := csi.VolumeCapability_AccessMode_Mode_name[int32(accessMode)]
		return nil, status.Errorf(codes.FailedPrecondition, "unsupported access mode: %s (%d)", modeName, accessMode)
	}

	var lv *proto.LogicalVolume
	var err error

	lvr, err := s.k8sLVService.GetVolume(ctx, volumeID)
	if err != nil {
		return nil, err
	}
	lv, err = s.getLvFromContext(ctx, lvr.Spec.DeviceClass, volumeID)
	if err != nil {
		return nil, err
	}
	if lv == nil {
		return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", volumeID)
	}

	if isBlockVol {
		err = s.nodePublishBlockVolume(req, lv)
	} else if isFsVol {
		err = s.nodePublishFilesystemVolume(req, lv)
	}
	if err != nil {
		return nil, err
	}
	return &csi.NodePublishVolumeResponse{}, nil
}

func makeMountOptions(readOnly bool, mountOption *csi.VolumeCapability_MountVolume) ([]string, error) {
	var mountOptions []string
	if readOnly {
		mountOptions = append(mountOptions, "ro")
	}

	for _, f := range mountOption.MountFlags {
		if f == "rw" && readOnly {
			return nil, status.Error(codes.InvalidArgument, "mount option \"rw\" is specified even though read only mode is specified")
		}
		mountOptions = append(mountOptions, f)
	}

	// avoid duplicate UUIDs
	if mountOption.FsType == "xfs" {
		mountOptions = append(mountOptions, "nouuid")
	}

	return mountOptions, nil
}

func (s *nodeServerNoLocked) nodePublishFilesystemVolume(req *csi.NodePublishVolumeRequest, lv *proto.LogicalVolume) error {
	// Check request
	mountOption := req.GetVolumeCapability().GetMount()
	if mountOption.FsType == "" {
		mountOption.FsType = "ext4"
	}

	// Find lv and create a block device with it
	device := filepath.Join(DeviceDirectory, req.GetVolumeId())
	err := s.createDeviceIfNeeded(device, lv)
	if err != nil {
		return err
	}

	mountOptions, err := makeMountOptions(req.GetReadonly(), mountOption)
	if err != nil {
		return err
	}

	err = os.MkdirAll(req.GetTargetPath(), 0755)
	if err != nil {
		return status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	fsType, err := filesystem.DetectFilesystem(device)
	if err != nil {
		return status.Errorf(codes.Internal, "filesystem check failed: volume=%s, error=%v", req.GetVolumeId(), err)
	}

	if fsType != "" && fsType != mountOption.FsType {
		return status.Errorf(codes.Internal, "target device is already formatted with different filesystem: volume=%s, current=%s, new:%s", req.GetVolumeId(), fsType, mountOption.FsType)
	}

	mounted, err := filesystem.IsMounted(device, req.GetTargetPath())
	if err != nil {
		return status.Errorf(codes.Internal, "mount check failed: target=%s, error=%v", req.GetTargetPath(), err)
	}

	if !mounted {
		if err := s.mounter.FormatAndMount(device, req.GetTargetPath(), mountOption.FsType, mountOptions); err != nil {
			return status.Errorf(codes.Internal, "mount failed: volume=%s, error=%v", req.GetVolumeId(), err)
		}
		if err := os.Chmod(req.GetTargetPath(), 0777|os.ModeSetgid); err != nil {
			return status.Errorf(codes.Internal, "chmod 2777 failed: target=%s, error=%v", req.GetTargetPath(), err)
		}
	}

	nodeLogger.Info("NodePublishVolume(fs) succeeded",
		"volume_id", req.GetVolumeId(),
		"target_path", req.GetTargetPath(),
		"fstype", mountOption.FsType)

	return nil
}

func (s *nodeServerNoLocked) createDeviceIfNeeded(device string, lv *proto.LogicalVolume) error {
	var stat unix.Stat_t
	err := filesystem.Stat(device, &stat)
	switch err {
	case nil:
		// a block device already exists, check its attributes
		if stat.Rdev == unix.Mkdev(lv.DevMajor, lv.DevMinor) && stat.Uid == uint32(os.Getuid()) && stat.Mode == deviceMode {
			return nil
		}
		err := os.Remove(device)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to remove device file %s: error=%v", device, err)
		}
		fallthrough
	case unix.ENOENT:
		err = os.MkdirAll(path.Dir(device), 0755)
		if err != nil {
			return status.Errorf(codes.Internal, "mkdir failed: target=%s, error=%v", path.Dir(device), err)
		}

		devno := unix.Mkdev(lv.DevMajor, lv.DevMinor)
		if err := filesystem.Mknod(device, deviceMode, int(devno)); err != nil {
			return status.Errorf(codes.Internal, "mknod failed for %s. major=%d, minor=%d, error=%v",
				device, lv.DevMajor, lv.DevMinor, err)
		}
	default:
		return status.Errorf(codes.Internal, "failed to stat %s: error=%v", device, err)
	}
	return nil
}

func (s *nodeServerNoLocked) nodePublishBlockVolume(req *csi.NodePublishVolumeRequest, lv *proto.LogicalVolume) error {
	// Find lv and create a block device with it
	targetPath := req.GetTargetPath()
	err := s.createDeviceIfNeeded(targetPath, lv)
	if err != nil {
		return err
	}

	nodeLogger.Info("NodePublishVolume(block) succeeded",
		"volume_id", req.GetVolumeId(),
		"target_path", targetPath)
	return nil
}

func (s *nodeServerNoLocked) findVolumeByID(listResp *proto.GetLVListResponse, name string) *proto.LogicalVolume {
	for _, v := range listResp.Volumes {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (s *nodeServerNoLocked) getLvFromContext(ctx context.Context, deviceClass, volumeID string) (*proto.LogicalVolume, error) {
	listResp, err := s.client.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: deviceClass})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list LV: %v", err)
	}
	return s.findVolumeByID(listResp, volumeID), nil
}

func (s *nodeServerNoLocked) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	targetPath := req.GetTargetPath()
	nodeLogger.Info("NodeUnpublishVolume called",
		"volume_id", volumeID,
		"target_path", targetPath)

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no target_path is provided")
	}

	device := filepath.Join(DeviceDirectory, volumeID)

	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		// target_path does not exist, but device for mount-type PV may still exist.
		_ = os.Remove(device)
		return &csi.NodeUnpublishVolumeResponse{}, nil
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", targetPath, err)
	}

	// remove device file if target_path is device, unmount target_path otherwise
	if info.IsDir() {
		err = s.nodeUnpublishFilesystemVolume(req, device)
	} else {
		err = s.nodeUnpublishBlockVolume(req)
	}
	if err != nil {
		return nil, err
	}
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (s *nodeServerNoLocked) nodeUnpublishFilesystemVolume(req *csi.NodeUnpublishVolumeRequest, device string) error {
	targetPath := req.GetTargetPath()

	mounted, err := filesystem.IsMounted(device, targetPath)
	if err != nil {
		return status.Errorf(codes.Internal, "mount check failed: target=%s, error=%v", targetPath, err)
	}
	if mounted {
		if err := s.mounter.Unmount(targetPath); err != nil {
			return status.Errorf(codes.Internal, "unmount failed for %s: error=%v", targetPath, err)
		}
	}

	if err := os.RemoveAll(targetPath); err != nil {
		return status.Errorf(codes.Internal, "remove dir failed for %s: error=%v", targetPath, err)
	}

	err = os.Remove(device)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "remove device failed for %s: error=%v", device, err)
	}

	nodeLogger.Info("NodeUnpublishVolume(fs) is succeeded",
		"volume_id", req.GetVolumeId(),
		"target_path", targetPath)
	return nil
}

func (s *nodeServerNoLocked) nodeUnpublishBlockVolume(req *csi.NodeUnpublishVolumeRequest) error {
	if err := os.Remove(req.GetTargetPath()); err != nil {
		return status.Errorf(codes.Internal, "remove failed for %s: error=%v", req.GetTargetPath(), err)
	}
	nodeLogger.Info("NodeUnpublishVolume(block) is succeeded",
		"volume_id", req.GetVolumeId(),
		"target_path", req.GetTargetPath())
	return nil
}

func (s *nodeServerNoLocked) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	volumeID := req.GetVolumeId()
	volumePath := req.GetVolumePath()
	nodeLogger.Info("NodeGetVolumeStats is called", "volume_id", volumeID, "volume_path", volumePath)
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_path is provided")
	}

	var st unix.Stat_t
	switch err := filesystem.Stat(volumePath, &st); err {
	case unix.ENOENT:
		return nil, status.Error(codes.NotFound, "Volume is not found at "+volumePath)
	case nil:
	default:
		return nil, status.Errorf(codes.Internal, "stat on %s was failed: %v", volumePath, err)
	}

	if (st.Mode & unix.S_IFMT) == unix.S_IFBLK {
		f, err := os.Open(volumePath)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "open on %s was failed: %v", volumePath, err)
		}
		defer f.Close()
		pos, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "seek on %s was failed: %v", volumePath, err)
		}
		return &csi.NodeGetVolumeStatsResponse{
			Usage: []*csi.VolumeUsage{{Total: pos, Unit: csi.VolumeUsage_BYTES}},
		}, nil
	}

	if st.Mode&unix.S_IFDIR == 0 {
		return nil, status.Errorf(codes.Internal, "invalid mode bits for %s: %d", volumePath, st.Mode)
	}

	var sfs unix.Statfs_t
	if err := filesystem.Statfs(volumePath, &sfs); err != nil {
		return nil, status.Errorf(codes.Internal, "statfs on %s was failed: %v", volumePath, err)
	}

	var usage []*csi.VolumeUsage
	if sfs.Blocks > 0 {
		usage = append(usage, &csi.VolumeUsage{
			Unit:      csi.VolumeUsage_BYTES,
			Total:     int64(sfs.Blocks) * int64(sfs.Frsize),
			Used:      int64(sfs.Blocks-sfs.Bfree) * int64(sfs.Frsize),
			Available: int64(sfs.Bavail) * int64(sfs.Frsize),
		})
	}
	if sfs.Files > 0 {
		usage = append(usage, &csi.VolumeUsage{
			Unit:      csi.VolumeUsage_INODES,
			Total:     int64(sfs.Files),
			Used:      int64(sfs.Files - sfs.Ffree),
			Available: int64(sfs.Ffree),
		})
	}
	return &csi.NodeGetVolumeStatsResponse{Usage: usage}, nil
}

func (s *nodeServerNoLocked) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	volumeID := req.GetVolumeId()
	volumePath := req.GetVolumePath()

	nodeLogger.Info("NodeExpandVolume is called",
		"volume_id", volumeID,
		"volume_path", volumePath,
		"required", req.GetCapacityRange().GetRequiredBytes(),
		"limit", req.GetCapacityRange().GetLimitBytes(),
	)

	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_id is provided")
	}
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no volume_path is provided")
	}

	// We need to check the capacity range but don't use the converted value
	// because the filesystem can be resized without the requested size.
	_, err := convertRequestCapacity(req.GetCapacityRange().GetRequiredBytes(), req.GetCapacityRange().GetLimitBytes())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// Device type (block or fs, fs type detection) checking will be removed after CSI v1.2.0
	// because `volume_capability` field will be added in csi.NodeExpandVolumeRequest
	info, err := os.Stat(volumePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "volume path is not exist: %s", volumePath)
		}
		return nil, status.Errorf(codes.Internal, "stat failed for %s: %v", volumePath, err)
	}

	isBlock := !info.IsDir()
	if isBlock {
		nodeLogger.Info("NodeExpandVolume(block) is skipped",
			"volume_id", volumeID,
			"target_path", volumePath,
		)
		return &csi.NodeExpandVolumeResponse{}, nil
	}

	device := filepath.Join(DeviceDirectory, volumeID)
	lvr, err := s.k8sLVService.GetVolume(ctx, volumeID)
	deviceClass := topolvm.DefaultDeviceClassName
	if err == nil {
		deviceClass = lvr.Spec.DeviceClass
	} else if err != k8s.ErrVolumeNotFound {
		return nil, err
	}
	lv, err := s.getLvFromContext(ctx, deviceClass, volumeID)
	if err != nil {
		return nil, err
	}
	if lv == nil {
		return nil, status.Errorf(codes.NotFound, "failed to find LV: %s", volumeID)
	}
	err = s.createDeviceIfNeeded(device, lv)
	if err != nil {
		return nil, err
	}

	args := []string{"-o", "source", "--noheadings", "--target", req.GetVolumePath()}
	output, err := s.mounter.Exec.Command(findmntCmd, args...).Output()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "findmnt error occurred: %v", err)
	}

	devicePath := strings.TrimSpace(string(output))
	if len(devicePath) == 0 {
		return nil, status.Errorf(codes.Internal, "filesystem %s is not mounted at %s", volumeID, volumePath)
	}

	r := mountutil.NewResizeFs(s.mounter.Exec)
	if _, err := r.Resize(device, volumePath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to resize filesystem %s (mounted at: %s): %v", volumeID, volumePath, err)
	}

	nodeLogger.Info("NodeExpandVolume(fs) is succeeded",
		"volume_id", volumeID,
		"target_path", volumePath,
	)

	// `capacity_bytes` in NodeExpandVolumeResponse is defined as OPTIONAL.
	// If this field needs to be filled, the value should be equal to `.status.currentSize` of the corresponding
	// `LogicalVolume`, but currently the node plugin does not have an access to the resource.
	// In addition to this, Kubernetes does not care if the field is blank or not, so leave it blank.
	return &csi.NodeExpandVolumeResponse{}, nil
}

func (s *nodeServerNoLocked) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	capabilities := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
		csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
	}

	csiCaps := make([]*csi.NodeServiceCapability, len(capabilities))
	for i, capability := range capabilities {
		csiCaps[i] = &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: capability,
				},
			},
		}
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}, nil
}

func (s *nodeServerNoLocked) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: s.nodeName,
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				topolvm.GetTopologyNodeKey(): s.nodeName,
			},
		},
	}, nil
}
