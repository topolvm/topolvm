package lvmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/topolvm/topolvm/internal/lvmd/command"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NewLVService creates a new LVServiceServer
func NewLVService(dcmapper *DeviceClassManager, ocmapper *LvcreateOptionClassManager, notifyFunc func()) proto.LVServiceServer {
	return &lvService{
		dcmapper:   dcmapper,
		ocmapper:   ocmapper,
		notifyFunc: notifyFunc,
	}
}

type lvService struct {
	proto.UnimplementedLVServiceServer
	dcmapper   *DeviceClassManager
	ocmapper   *LvcreateOptionClassManager
	notifyFunc func()
}

func (s *lvService) notify() {
	if s.notifyFunc != nil {
		s.notifyFunc()
	}
}

func (s *lvService) CreateLV(ctx context.Context, req *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
	logger := log.FromContext(ctx).WithValues("name", req.GetName())

	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	pool, err := storagePoolForDeviceClass(ctx, dc)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get pool from device class: %v", err)
	}
	oc := s.ocmapper.LvcreateOptionClass(req.LvcreateOptionClass)

	requested := uint64(req.GetSizeBytes())
	free, err := pool.Free(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get free bytes: %v", err)
	}
	if free < requested {
		logger.Error(err, "not enough space left on VG", "free", free, "requested", requested)
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested)
	}

	var stripe uint
	var stripeSize string
	var lvcreateOptions []string
	if oc != nil {
		lvcreateOptions = oc.Options
	} else if req.LvcreateOptionClass != "" {
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported lvcreate-option-class target: %s", req.LvcreateOptionClass))
	} else {
		stripeSize = dc.StripeSize
		if dc.Stripe != nil {
			stripe = *dc.Stripe
		}
		if dc.LVCreateOptions != nil {
			lvcreateOptions = dc.LVCreateOptions
		}
	}

	err = pool.CreateVolume(ctx, req.GetName(), requested, req.GetTags(), stripe, stripeSize, lvcreateOptions)
	if err != nil {
		logger.Error(err, "failed to create volume",
			"requested", requested,
			"tags", req.GetTags())
		return nil, status.Error(codes.Internal, err.Error())
	}

	lv, err := pool.FindVolume(ctx, req.GetName())
	if err != nil {
		logger.Error(err, "failed to find volume",
			"requested", requested,
			"tags", req.GetTags())
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.notify()

	logger.Info("created a new LV", "size", requested)

	return &proto.CreateLVResponse{
		Volume: &proto.LogicalVolume{
			Name: lv.Name(),
			// convert to int64 because lvmd internals and lvm use uint64 but CSI uses int64.
			// For most conventional lvm use cases overflow here will never occur (9223372 TB or above cause overflow)
			SizeBytes: int64(lv.Size()),
			DevMajor:  lv.MajorNumber(),
			DevMinor:  lv.MinorNumber(),
		},
	}, nil
}

func (s *lvService) RemoveLV(ctx context.Context, req *proto.RemoveLVRequest) (*proto.Empty, error) {
	logger := log.FromContext(ctx).WithValues("name", req.GetName())

	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}

	vg, err := command.FindVolumeGroup(ctx, dc.VolumeGroup)
	if errors.Is(err, command.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	} else if err != nil {
		logger.Error(err, "failed to get volume group", "name", dc.VolumeGroup)
		return nil, err
	}

	if err := vg.RemoveVolume(ctx, req.GetName()); errors.Is(err, command.ErrNotFound) {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	} else if err != nil {
		logger.Error(err, "failed to remove volume", "name", req.GetName())
		return nil, err
	}

	s.notify()

	logger.Info("removed a LV", "name", req.GetName())

	return &proto.Empty{}, nil
}

func (s *lvService) CreateLVSnapshot(ctx context.Context, req *proto.CreateLVSnapshotRequest) (*proto.CreateLVSnapshotResponse, error) {
	logger := log.FromContext(ctx).WithValues("name", req.GetName())
	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	var snapType string
	switch dc.Type {
	case lvmdTypes.TypeThin:
		snapType = "thin-snapshot"
	case lvmdTypes.TypeThick:
		return nil, status.Error(codes.Unimplemented, "device class is not thin. Thick snapshots are not implemented yet")
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid device class type %v", string(dc.Type))
	}

	vg, err := command.FindVolumeGroup(ctx, dc.VolumeGroup)
	if err != nil {
		return nil, err
	}

	// Fetch the source logical volume
	sourceVolume := req.GetSourceVolume()
	sourceLV, err := vg.FindVolume(ctx, sourceVolume)
	if errors.Is(err, command.ErrNotFound) {
		logger.Error(err, "source logical volume is not found", "sourceVolume", sourceVolume)
		return nil, status.Errorf(codes.NotFound, "source logical volume %s is not found", sourceVolume)
	}
	if err != nil {
		logger.Error(err, "failed to find source volume", "sourceVolume", sourceVolume)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if !sourceLV.IsThin() {
		return nil, status.Error(codes.Unimplemented, "snapshot can be created for only thin volumes")
	}

	// In case of thin-snapshots, the size is the same as the source volume on snapshot creation, and then
	// gets resized after extension into the correct size
	sizeOnCreation := sourceLV.Size()

	desiredSize := uint64(req.GetSizeBytes())

	// in case there is no desired size in the request, we can still attempt to create the Snapshot with Source size.
	if desiredSize == 0 {
		desiredSize = sizeOnCreation
	}

	if sizeOnCreation > desiredSize {
		return nil, status.Errorf(codes.OutOfRange, "requested size %v is smaller than source logical volume: %v", desiredSize, sizeOnCreation)
	}

	pool, err := vg.FindPool(ctx, dc.ThinPoolConfig.Name)
	if err != nil {
		logger.Error(err, "failed to get thinpool")
		return nil, status.Error(codes.Internal, err.Error())
	}
	poolUsage, err := pool.Usage(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get pool usage: %v", err)
	}
	free, err := poolUsage.FreeBytes(dc.ThinPoolConfig.OverprovisionRatio)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get free bytes: %v", err)
	}
	if free < desiredSize {
		logger.Error(err, "not enough space left on VG", "free", free, "desiredSize", desiredSize)
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, desiredSize=%d", free, desiredSize)
	}

	logger.Info(
		"lvservice req",
		"sizeOnCreation", sizeOnCreation,
		"desiredSize", desiredSize,
		"sourceVol", sourceVolume,
		"snapType", snapType,
		"accessType", req.AccessType,
	)
	// Create snapshot lv

	if err := sourceLV.ThinSnapshot(ctx, req.GetName(), req.GetTags()); err != nil {
		logger.Error(err, "failed to create snapshot volume")
		return nil, status.Error(codes.Internal, err.Error())
	}

	snapLV, err := vg.FindVolume(ctx, req.GetName())
	if err != nil {
		logger.Error(err, "failed to get snapshot after creation")
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := snapLV.Resize(ctx, desiredSize); err != nil {
		logger.Error(err, "failed to resize snapshot volume")
		return nil, status.Error(codes.Internal, err.Error())
	}

	// If source volume is thin, activate the thin snapshot lv with accessmode.
	if err := snapLV.Activate(ctx, req.AccessType); err != nil {
		logger.Error(err, "failed to activate snapshot volume")
		if err := vg.RemoveVolume(ctx, req.GetName()); err != nil {
			logger.Error(err, "failed to delete snapshot after activation failed")
		} else {
			logger.Info("deleted a snapshot")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.notify()

	logger.Info(
		"created a new snapshot LV",
		"size", desiredSize,
		"accessType", req.AccessType,
		"sourceID", sourceVolume,
	)

	return &proto.CreateLVSnapshotResponse{
		Snapshot: &proto.LogicalVolume{
			Name: snapLV.Name(),
			// convert to int64 because lvmd internals and lvm use uint64 but CSI uses int64.
			// For most conventional lvm use cases overflow here will never occur (9223372 TB or above cause overflow)
			SizeBytes: int64(snapLV.Size()),
			DevMajor:  snapLV.MajorNumber(),
			DevMinor:  snapLV.MinorNumber(),
		},
	}, nil
}

func (s *lvService) ResizeLV(ctx context.Context, req *proto.ResizeLVRequest) (*proto.ResizeLVResponse, error) {
	logger := log.FromContext(ctx).WithValues("name", req.GetName())

	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	pool, err := storagePoolForDeviceClass(ctx, dc)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get pool from device class: %v", err)
	}
	// FindVolume on VolumeGroup or ThinPool returns ThinLogicalVolumes as well
	// and no special handling for resize of LogicalVolume is needed
	lv, err := pool.FindVolume(ctx, req.GetName())
	if errors.Is(err, command.ErrNotFound) {
		logger.Error(err, "logical volume is not found")
		return nil, status.Errorf(codes.NotFound, "logical volume %s is not found", req.GetName())
	}
	if err != nil {
		logger.Error(err, "failed to find volume")
		return nil, status.Error(codes.Internal, err.Error())
	}

	requested := uint64(req.GetSizeBytes())
	current := lv.Size()
	if requested <= current {
		logger.Info("skipping resize: requested size is smaller than current size", "requested", requested, "current", current)
		return &proto.ResizeLVResponse{SizeBytes: int64(current)}, nil
	}

	free, err := pool.Free(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get free bytes: %v", err)
	}

	logger.Info(
		"lvservice request - ResizeLV",
		"requested", requested,
		"current", current,
		"free", free,
	)

	if free < (requested - current) {
		logger.Error(err, "no enough space left on VG",
			"requested", requested,
			"current", current,
			"free", free,
		)
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested-current)
	}

	err = lv.Resize(ctx, requested)
	if err != nil {
		logger.Error(err, "failed to resize LV",
			"requested", requested,
			"current", current,
			"free", free,
		)
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.notify()

	logger.Info("resized a LV", "requested", requested, "size", lv.Size())

	return &proto.ResizeLVResponse{SizeBytes: int64(lv.Size())}, nil
}

//func (s *lvService) MountLV(ctx context.Context, req *proto.MountLVRequest) (*proto.MountLVResponse, error) {
//	logger := log.FromContext(ctx).WithValues("name", req.GetName())
//
//	// Get device class
//	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
//	if err != nil {
//		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
//	}
//
//	// Find the volume group
//	vg, err := command.FindVolumeGroup(ctx, dc.VolumeGroup)
//	if errors.Is(err, command.ErrNotFound) {
//		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
//	} else if err != nil {
//		logger.Error(err, "failed to get volume group", "name", dc.VolumeGroup)
//		return nil, status.Error(codes.Internal, err.Error())
//	}
//
//	// Find the logical volume
//	lv, err := vg.FindVolume(ctx, req.GetName())
//	if errors.Is(err, command.ErrNotFound) {
//		logger.Error(err, "logical volume is not found")
//		return nil, status.Errorf(codes.NotFound, "logical volume %s is not found", req.GetName())
//	}
//	if err != nil {
//		logger.Error(err, "failed to find volume")
//		return nil, status.Error(codes.Internal, err.Error())
//	}
//
//	// Get device path
//	devicePath := lv.Path()
//
//	// Set default filesystem type if not specified
//	fsType := req.GetFsType()
//	if fsType == "" {
//		fsType = "ext4"
//	}
//
//	// Validate target path
//	targetPath := req.GetTargetPath()
//	if targetPath == "" {
//		return nil, status.Error(codes.InvalidArgument, "target_path is required")
//	}
//
//	// Create target directory if it doesn't exist
//	if err := os.MkdirAll(targetPath, 0755); err != nil {
//		logger.Error(err, "failed to create target directory", "target", targetPath)
//		return nil, status.Errorf(codes.Internal, "failed to create target directory: %v", err)
//	}
//
//	//Detect existing filesystem
//	existingFS, err := filesystem.DetectFilesystem(devicePath)
//	if err != nil {
//		logger.Error(err, "failed to detect filesystem", "device", devicePath)
//		return nil, status.Errorf(codes.Internal, "failed to detect filesystem: %v", err)
//	}
//
//	// If filesystem exists and differs from requested, return error
//	if existingFS != "" && existingFS != fsType {
//		return nil, status.Errorf(codes.FailedPrecondition,
//			"device already formatted with different filesystem: existing=%s, requested=%s",
//			existingFS, fsType)
//	}
//
//	fsType, err := filesystem.DetectFilesystem(lv.GetPath())
//	if err != nil {
//		return status.Errorf(codes.Internal, "filesystem check failed: volume=%s, error=%v", req.GetVolumeId(), err)
//	}
//
//	if fsType != "" && fsType != mountOption.FsType {
//		return status.Errorf(codes.Internal, "target device is already formatted with different filesystem: volume=%s, current=%s, new:%s", req.GetVolumeId(), fsType, mountOption.FsType)
//	}
//
//	fmt.Println("############## FS type: ", fsType)         // Debug print
//	fmt.Println("############## Existing FS: ", existingFS) // Debug print
//
//	//// Format the device if no filesystem exists
//	//if existingFS == "" {
//	//	logger.Info("formatting device", "device", devicePath, "fsType", fsType)
//	//	var mkfsCmd *exec.Cmd
//	//	switch fsType {
//	//	case "ext4":
//	//		mkfsCmd = exec.CommandContext(ctx, "mkfs.ext4", "-F", devicePath)
//	//	case "ext3":
//	//		mkfsCmd = exec.CommandContext(ctx, "mkfs.ext3", "-F", devicePath)
//	//	case "xfs":
//	//		mkfsCmd = exec.CommandContext(ctx, "mkfs.xfs", "-f", devicePath)
//	//	default:
//	//		return nil, status.Errorf(codes.InvalidArgument, "unsupported filesystem type: %s", fsType)
//	//	}
//	//	fmt.Println("############## Mounting device", "device", devicePath, "fsType", fsType)
//	//
//	//	fmt.Println("############## Mkfs Command: ", mkfsCmd.String()) // Debug print
//	//
//	//	if output, err := mkfsCmd.CombinedOutput(); err != nil {
//	//		logger.Error(err, "failed to format device", "device", devicePath, "output", string(output))
//	//		return nil, status.Errorf(codes.Internal, "failed to format device: %v, output: %s", err, string(output))
//	//	}
//	//	logger.Info("device formatted successfully", "device", devicePath, "fsType", fsType)
//	//}
//
//	// Check if already mounted
//	mountedCheck := exec.CommandContext(ctx, "mountpoint", "-q", targetPath)
//	if err := mountedCheck.Run(); err == nil {
//		logger.Info("target path is already a mount point", "target", targetPath)
//		return &proto.MountLVResponse{
//			DevicePath: devicePath,
//		}, nil
//	}
//
//	// Prepare mount options
//	mountOptions := req.GetMountOptions()
//	if mountOptions == nil {
//		mountOptions = []string{}
//	}
//
//	// Ensure read-write mode unless read-only is explicitly specified
//	hasRW := false
//	hasRO := false
//	for _, opt := range mountOptions {
//		if opt == "rw" {
//			hasRW = true
//		}
//		if opt == "ro" {
//			hasRO = true
//		}
//	}
//	// Add "ro" by default if neither "rw" nor "ro" is specified
//	if !hasRW && !hasRO {
//		mountOptions = append(mountOptions, "ro")
//	}
//
//	//Add nouuid option for xfs
//	if fsType == "xfs" {
//		hasNouuid := false
//		for _, opt := range mountOptions {
//			if opt == "nouuid" {
//				hasNouuid = true
//				break
//			}
//		}
//		if !hasNouuid {
//			mountOptions = append(mountOptions, "nouuid")
//		}
//	}
//
//	// Build mount command
//	var mountArgs []string
//	if len(mountOptions) > 0 {
//		mountArgs = append(mountArgs, "-o", fmt.Sprintf("%s", joinMountOptions(mountOptions)))
//	}
//	mountArgs = append(mountArgs, "-t", fsType, devicePath, targetPath)
//
//	// Execute mount command
//	logger.Info("executing mount command",
//		"device", devicePath,
//		"target", targetPath,
//		"fsType", fsType,
//		"options", mountOptions)
//
//	mountCmd := exec.CommandContext(ctx, "mount", mountArgs...)
//
//	fmt.Println("############## Mount Command: ", mountCmd.String()) // Debug print
//
//	time.Sleep(20 * time.Second)
//
//	//if output, err := mountCmd.CombinedOutput(); err != nil {
//	//	fmt.Println(err)
//	//	logger.Error(err, "failed to mount device",
//	//		"device", devicePath,
//	//		"target", targetPath,
//	//		"output", string(output))
//	//	return nil, status.Errorf(codes.Internal, "failed to mount device: %v, output: %s", err, string(output))
//	//}
//	//
//	//// Set permissions on mounted directory
//	//if err := os.Chmod(targetPath, 0777|os.ModeSetgid); err != nil {
//	//	logger.Error(err, "failed to set permissions on mounted directory", "target", targetPath)
//	//	// Don't fail the operation, just log the warning
//	//	logger.Info("warning: failed to chmod 2777 on target", "target", targetPath, "error", err)
//	//}
//	//
//	logger.Info("successfully mounted logical volume",
//		"device", devicePath,
//		"target", targetPath,
//		"fsType", fsType)
//
//	return &proto.MountLVResponse{
//		DevicePath: devicePath,
//	}, nil
//}
//
//// joinMountOptions joins mount options into a comma-separated string
//func joinMountOptions(options []string) string {
//	if len(options) == 0 {
//		return ""
//	}
//	result := options[0]
//	for i := 1; i < len(options); i++ {
//		result += "," + options[i]
//	}
//	return result
//}
