package lvmd

import (
	"context"
	"fmt"
	"math"

	"github.com/cybozu-go/log"
	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	if s.notifyFunc == nil {
		return
	}
	s.notifyFunc()
}

func (s *lvService) CreateLV(_ context.Context, req *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	oc := s.ocmapper.LvcreateOptionClass(req.LvcreateOptionClass)
	requested := req.GetSizeGb() << 30
	free := uint64(0)
	var pool *command.ThinPool
	switch dc.Type {
	case TypeThick:
		free, err = vg.Free()
		if err != nil {
			log.Error("failed to get free bytes", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
	case TypeThin:
		pool, err = vg.FindPool(dc.ThinPoolConfig.Name)
		if err != nil {
			log.Error("failed to get thinpool", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
		tpu, err := pool.Free()
		if err != nil {
			log.Error("failed to get free bytes", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
		free = uint64(math.Floor(dc.ThinPoolConfig.OverprovisionRatio*float64(tpu.SizeBytes))) - tpu.VirtualBytes
	default:
		// technically this block will not be hit however make sure we return error
		// in such cases where deviceclass target is neither thick or thinpool
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
	}

	if free < requested {
		log.Error("no enough space left on VG", map[string]interface{}{
			"free":      free,
			"requested": requested,
		})
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

	var lv *command.LogicalVolume
	switch dc.Type {
	case TypeThick:
		lv, err = vg.CreateVolume(req.GetName(), requested, req.GetTags(), stripe, stripeSize, lvcreateOptions)
	case TypeThin:
		lv, err = pool.CreateVolume(req.GetName(), requested, req.GetTags(), stripe, stripeSize, lvcreateOptions)
	default:
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
	}

	if err != nil {
		log.Error("failed to create volume", map[string]interface{}{
			"name":      req.GetName(),
			"requested": requested,
			"tags":      req.GetTags(),
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.notify()

	log.Info("created a new LV", map[string]interface{}{
		"name": req.GetName(),
		"size": requested,
	})

	return &proto.CreateLVResponse{
		Volume: &proto.LogicalVolume{
			Name:     lv.Name(),
			SizeGb:   lv.Size() >> 30,
			DevMajor: lv.MajorNumber(),
			DevMinor: lv.MinorNumber(),
		},
	}, nil
}

func (s *lvService) RemoveLV(_ context.Context, req *proto.RemoveLVRequest) (*proto.Empty, error) {
	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	// ListVolumes on VolumeGroup or ThinPool returns ThinLogicalVolumes as well
	// and no special handling for removal of LogicalVolume is needed
	for _, lv := range vg.ListVolumes() {
		if lv.Name() != req.GetName() {
			continue
		}

		err = lv.Remove()
		if err != nil {
			log.Error("failed to remove volume", map[string]interface{}{
				log.FnError: err,
				"name":      lv.Name(),
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
		s.notify()

		log.Info("removed a LV", map[string]interface{}{
			"name": req.GetName(),
		})
		break
	}

	return &proto.Empty{}, nil
}

func (s *lvService) CreateLVSnapshot(_ context.Context, req *proto.CreateLVSnapshotRequest) (*proto.CreateLVSnapshotResponse, error) {
	var snapType string
	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}

	switch dc.Type {
	case TypeThin:
		snapType = "thin-snapshot"
	case TypeThick:
		return nil, status.Error(codes.Unimplemented, "device class is not thin. Thick snapshots are not implemented yet")
	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid device class type %v", string(dc.Type))
	}

	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}

	// Fetch the source logical volume
	sourceVolume := req.GetSourceVolume()
	sourceLV, err := vg.FindVolume(sourceVolume)
	if err == command.ErrNotFound {
		log.Error("source logical volume is not found", map[string]interface{}{
			log.FnError: err,
			"name":      sourceVolume,
		})
		return nil, status.Errorf(codes.NotFound, "source logical volume %s is not found", sourceVolume)
	}
	if err != nil {
		log.Error("failed to find source volume", map[string]interface{}{
			log.FnError: err,
			"name":      sourceVolume,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	if !sourceLV.IsThin() {
		return nil, status.Error(codes.Unimplemented, "snapshot can be created for only thin volumes")
	}

	// In case of thin-snapshots, the size is the same as the source volume on snapshot creation, and then
	// gets resized after extension into the correct size
	sizeOnCreation := sourceLV.Size()
	desiredSize := req.GetSizeGb() << 30

	// in case there is no desired size in the request, we can still attempt to create the Snapshot with Source size.
	if desiredSize == 0 {
		desiredSize = sizeOnCreation
	}

	if sizeOnCreation > desiredSize {
		return nil, status.Errorf(codes.OutOfRange, "requested size %v is smaller than source logical volume: %v", desiredSize, sizeOnCreation)
	}

	log.Info("lvservice req", map[string]interface{}{
		"name":           req.Name,
		"sizeOnCreation": sizeOnCreation,
		"desiredSize":    desiredSize,
		"sourceVol":      sourceVolume,
		"snapType":       snapType,
		"accessType":     req.GetAccessType(),
	})
	// Create snapshot lv
	snapLV, err := sourceLV.Snapshot(req.GetName(), sizeOnCreation, req.GetTags(), sourceLV.IsThin())
	if err != nil {
		log.Error("failed to create snapshot volume", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := snapLV.Resize(desiredSize); err != nil {
		log.Error("failed to extend snapshot after creation to desired size", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	// If source volume is thin, activate the thin snapshot lv with accessmode.
	if err := snapLV.Activate(req.AccessType); err != nil {
		log.Error("failed to activate snap volume, deleting snapshot", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
		})
		err := snapLV.Remove()
		if err != nil {
			log.Error("failed to delete snapshot", map[string]interface{}{
				log.FnError: err,
				"name":      snapLV.Name(),
			})
		} else {
			log.Info("deleted a snapshot", map[string]interface{}{
				"name": req.GetName(),
			})
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.notify()

	log.Info("created a new snapshot LV", map[string]interface{}{
		"name":       req.GetName(),
		"size":       desiredSize,
		"accessType": req.AccessType,
		"sourceID":   sourceVolume,
	})

	return &proto.CreateLVSnapshotResponse{
		Snapshot: &proto.LogicalVolume{
			Name:     snapLV.Name(),
			SizeGb:   snapLV.Size() >> 30,
			DevMajor: snapLV.MajorNumber(),
			DevMinor: snapLV.MinorNumber(),
		},
	}, nil
}

func (s *lvService) ResizeLV(_ context.Context, req *proto.ResizeLVRequest) (*proto.Empty, error) {
	dc, err := s.dcmapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	// FindVolume on VolumeGroup or ThinPool returns ThinLogicalVolumes as well
	// and no special handling for resize of LogicalVolume is needed
	lv, err := vg.FindVolume(req.GetName())
	if err == command.ErrNotFound {
		log.Error("logical volume is not found", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
		})
		return nil, status.Errorf(codes.NotFound, "logical volume %s is not found", req.GetName())
	}
	if err != nil {
		log.Error("failed to find volume", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	requested := req.GetSizeGb() << 30
	current := lv.Size()

	if requested < current {
		log.Error("shrinking volume size is not allowed", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
			"requested": requested,
			"current":   current,
		})
		return nil, status.Error(codes.OutOfRange, "shrinking volume size is not allowed")
	}

	free := uint64(0)
	var pool *command.ThinPool
	switch dc.Type {
	case TypeThick:
		free, err = vg.Free()
		if err != nil {
			log.Error("failed to get free bytes", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
	case TypeThin:
		pool, err = vg.FindPool(dc.ThinPoolConfig.Name)
		if err != nil {
			log.Error("failed to get thinpool", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
		tpu, err := pool.Free()
		if err != nil {
			log.Error("failed to get free bytes", map[string]interface{}{
				log.FnError: err,
			})
			return nil, status.Error(codes.Internal, err.Error())
		}
		free = uint64(math.Floor(dc.ThinPoolConfig.OverprovisionRatio*float64(tpu.SizeBytes))) - tpu.VirtualBytes
	default:
		// technically this block will not be hit however make sure we return error
		// in such cases where deviceclass target is neither thick or thinpool
		return nil, status.Error(codes.Internal, fmt.Sprintf("unsupported device class target: %s", dc.Type))
	}

	if free < (requested - current) {
		log.Error("no enough space left on VG", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
			"requested": requested,
			"current":   current,
			"free":      free,
		})
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested-current)
	}

	err = lv.Resize(requested)
	if err != nil {
		log.Error("failed to resize LV", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
			"requested": requested,
			"current":   current,
			"free":      free,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.notify()

	log.Info("resized a LV", map[string]interface{}{
		"name": req.GetName(),
		"size": requested,
	})

	return &proto.Empty{}, nil
}
