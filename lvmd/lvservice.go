package lvmd

import (
	"context"

	"github.com/cybozu-go/log"
	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewLVService creates a new LVServiceServer
func NewLVService(mapper *DeviceClassManager, notifyFunc func()) proto.LVServiceServer {
	return &lvService{
		mapper:     mapper,
		notifyFunc: notifyFunc,
	}
}

type lvService struct {
	proto.UnimplementedLVServiceServer
	mapper     *DeviceClassManager
	notifyFunc func()
}

func (s *lvService) notify() {
	if s.notifyFunc == nil {
		return
	}
	s.notifyFunc()
}

func (s *lvService) CreateLV(_ context.Context, req *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
	dc, err := s.mapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}

	lv, err := createLV(dc, vg, req.GetName(), req.GetTags(), req.GetSizeGb())
	if err != nil {
		return nil, err
	}
	s.notify()

	log.Info("created a new LV", map[string]interface{}{
		"name": req.GetName(),
		"size": req.GetSizeGb() << 30,
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
	dc, err := s.mapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	lvs, err := vg.ListVolumes()
	if err != nil {
		log.Error("failed to list volumes", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	for _, lv := range lvs {
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

func (s *lvService) ResizeLV(_ context.Context, req *proto.ResizeLVRequest) (*proto.Empty, error) {
	dc, err := s.mapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
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

	free, err := vg.Free()
	if err != nil {
		log.Error("failed to free VG", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
		})
		return nil, status.Error(codes.Internal, err.Error())
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

func (s *lvService) CreateSnap(_ context.Context, req *proto.CreateSnapRequest) (*proto.CreateSnapResponse, error) {
	dc, err := s.mapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}
	sourceLV, err := vg.FindVolume(req.GetSource())
	if err == command.ErrNotFound {
		log.Error("source logical volume is not found", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetSource(),
		})
		return nil, status.Errorf(codes.NotFound, "source logical volume %s is not found", req.GetSource())
	}
	if err != nil {
		log.Error("failed to find source volume", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetSource(),
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	requested := req.GetSizeGb() << 30
	free, err := vg.Free()
	if err != nil {
		log.Error("failed to get free space of VG", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	if free < requested {
		log.Error("no enough space left on VG", map[string]interface{}{
			"free":      free,
			"requested": requested,
		})
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested)
	}

	// Create snapshot lv
	snapLV, err := sourceLV.Snapshot(req.GetName(), requested)
	if err != nil {
		log.Error("failed to create snap volume", map[string]interface{}{
			log.FnError: err,
			"name":      req.GetName(),
		})
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.notify()

	log.Info("created a new snapshot LV", map[string]interface{}{
		"name": req.GetName(),
		"size": requested,
	})

	return &proto.CreateSnapResponse{
		Snap: &proto.LogicalVolume{
			Name:     snapLV.Name(),
			SizeGb:   snapLV.Size() >> 30,
			DevMajor: snapLV.MajorNumber(),
			DevMinor: snapLV.MinorNumber(),
		},
	}, nil
}

func (s *lvService) RestoreLV(_ context.Context, req *proto.RestoreLVRequest) (*proto.RestoreLVResponse, error) {
	dc, err := s.mapper.DeviceClass(req.DeviceClass)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%s: %s", err.Error(), req.DeviceClass)
	}
	vg, err := command.FindVolumeGroup(dc.VolumeGroup)
	if err != nil {
		return nil, err
	}

	// Create target lv
	targetLV, err := createLV(dc, vg, req.GetName(), req.GetTags(), req.GetSizeGb())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	defer func() {
		if err != nil {
			log.Error("failed to restore data", map[string]interface{}{
				log.FnError: err,
			})
			_ = targetLV.Remove()
		}
	}()

	// Get data source lv
	sourceLv, err := getLv(vg, req.GetSnapshot())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Copy data form parent lv to target lv
	err = targetLV.Copy(sourceLv, req.VolumeMode, req.FsType)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	s.notify()
	return &proto.RestoreLVResponse{
		Volume: &proto.LogicalVolume{
			Name:     targetLV.Name(),
			SizeGb:   targetLV.Size() >> 30,
			DevMajor: targetLV.MajorNumber(),
			DevMinor: targetLV.MinorNumber(),
		},
	}, nil
}

func createLV(dc *DeviceClass, vg *command.VolumeGroup, lvName string, lvTags []string, size uint64) (*command.LogicalVolume, error) {
	free, err := vg.Free()
	if err != nil {
		log.Error("failed to free VG", map[string]interface{}{
			log.FnError: err,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	requested := size << 30
	if free < requested {
		log.Error("no enough space left on VG", map[string]interface{}{
			"free":      free,
			"requested": requested,
		})
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested)
	}

	var stripe uint
	if dc.Stripe != nil {
		stripe = *dc.Stripe
	}

	lv, err := vg.CreateVolume(lvName, requested, lvTags, stripe, dc.StripeSize)
	if err != nil {
		log.Error("failed to create volume", map[string]interface{}{
			"name":      lvName,
			"requested": requested,
			"tags":      lvTags,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	return lv, nil
}

func getLv(vg *command.VolumeGroup, name string) (*command.LogicalVolume, error) {
	parentLV, err := vg.FindVolume(name)
	if err == command.ErrNotFound {
		log.Error("snapshot logical volume is not found", map[string]interface{}{
			log.FnError: err,
			"name":      name,
		})
		return nil, status.Errorf(codes.NotFound, "snapshot logical volume %s is not found", name)
	}
	if err != nil {
		log.Error("failed to find snapshot logical volume", map[string]interface{}{
			log.FnError: err,
			"name":      name,
		})
		return nil, status.Error(codes.Internal, err.Error())
	}

	return parentLV, nil
}
