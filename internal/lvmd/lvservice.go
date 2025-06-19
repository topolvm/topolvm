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
	vg, err := command.FindVolumeGroup(ctx, dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	oc := s.ocmapper.LvcreateOptionClass(req.LvcreateOptionClass)

	var requested uint64
	if req.SizeBytes > 0 {
		// convert to uint64 because CSI uses int64 but lvmd internals and lvm use uint64
		requested = uint64(req.GetSizeBytes())
	} else {
		// legacy conversion from SizeGb to SizeBytes
		requested = req.GetSizeGb() << 30
	}

	free := uint64(0)
	var pool *command.ThinPool
	switch dc.Type {
	case lvmdTypes.TypeThick:
		free, err = vg.Free()
		if err != nil {
			logger.Error(err, "failed to get free bytes")
			return nil, status.Error(codes.Internal, err.Error())
		}
	case lvmdTypes.TypeThin:
		pool, err = vg.FindPool(ctx, dc.ThinPoolConfig.Name)
		if err != nil {
			logger.Error(err, "failed to get thinpool")
			return nil, status.Error(codes.Internal, err.Error())
		}
		tpu, err := pool.Free(ctx)
		if err != nil {
			logger.Error(err, "failed to get free bytes")
			return nil, status.Error(codes.Internal, err.Error())
		}
		free = calcThinPoolFreeBytes(
			dc.ThinPoolConfig.OverprovisionRatio, tpu.SizeBytes, tpu.VirtualBytes)
	default:
		// technically this block will not be hit however make sure we return error
		// in such cases where deviceclass target is neither thick or thinpool
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
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

	switch dc.Type {
	case lvmdTypes.TypeThick:
		err = vg.CreateVolume(ctx, req.GetName(), requested, req.GetTags(), stripe, stripeSize, lvcreateOptions)
	case lvmdTypes.TypeThin:
		err = pool.CreateVolume(ctx, req.GetName(), requested, req.GetTags(), stripe, stripeSize, lvcreateOptions)
	default:
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
	}
	if err != nil {
		logger.Error(err, "failed to create volume",
			"requested", requested,
			"tags", req.GetTags())
		return nil, status.Error(codes.Internal, err.Error())
	}

	lv, err := vg.FindVolume(ctx, req.GetName())

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
			// still set sizeGB for legacy purposes, can (but not has to) be removed in next minor release.
			SizeGb: lv.Size() >> 30,
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

	var snapType string
	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}

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

	var desiredSize uint64
	if req.SizeBytes > 0 {
		// convert to uint64 because CSI uses int64 but lvmd internals and lvm use uint64
		desiredSize = uint64(req.GetSizeBytes())
	} else {
		// legacy conversion from SizeGb to SizeBytes
		desiredSize = req.GetSizeGb() << 30
	}

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
	tpu, err := pool.Free(ctx)
	free := calcThinPoolFreeBytes(dc.ThinPoolConfig.OverprovisionRatio, tpu.SizeBytes, tpu.VirtualBytes)
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
			// still set sizeGB for legacy purposes, can (but not has to) be removed in next minor release.
			SizeGb: snapLV.Size() >> 30,
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
	vg, err := command.FindVolumeGroup(ctx, dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	// FindVolume on VolumeGroup or ThinPool returns ThinLogicalVolumes as well
	// and no special handling for resize of LogicalVolume is needed
	lv, err := vg.FindVolume(ctx, req.GetName())
	if errors.Is(err, command.ErrNotFound) {
		logger.Error(err, "logical volume is not found")
		return nil, status.Errorf(codes.NotFound, "logical volume %s is not found", req.GetName())
	}
	if err != nil {
		logger.Error(err, "failed to find volume")
		return nil, status.Error(codes.Internal, err.Error())
	}

	var requested uint64
	if req.SizeBytes > 0 {
		// convert to uint64 because CSI uses int64 but lvmd internals and lvm use uint64
		requested = uint64(req.GetSizeBytes())
	} else {
		// legacy conversion from SizeGb to SizeBytes
		requested = req.GetSizeGb() << 30
	}
	current := lv.Size()
	if requested <= current {
		logger.Info("skipping resize: requested size is smaller than current size", "requested", requested, "current", current)
		return &proto.ResizeLVResponse{SizeBytes: int64(current)}, nil
	}

	free := uint64(0)
	var pool *command.ThinPool
	switch dc.Type {
	case lvmdTypes.TypeThick:
		free, err = vg.Free()
		if err != nil {
			logger.Error(err, "failed to get free bytes")
			return nil, status.Error(codes.Internal, err.Error())
		}
	case lvmdTypes.TypeThin:
		pool, err = vg.FindPool(ctx, dc.ThinPoolConfig.Name)
		if err != nil {
			logger.Error(err, "failed to get thinpool")
			return nil, status.Error(codes.Internal, err.Error())
		}
		tpu, err := pool.Free(ctx)
		if err != nil {
			logger.Error(err, "failed to get free bytes")
			return nil, status.Error(codes.Internal, err.Error())
		}
		free = calcThinPoolFreeBytes(
			dc.ThinPoolConfig.OverprovisionRatio, tpu.SizeBytes, tpu.VirtualBytes)
	default:
		// technically this block will not be hit however make sure we return error
		// in such cases where deviceclass target is neither thick or thinpool
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
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
