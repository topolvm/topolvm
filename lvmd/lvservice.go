package lvmd

import (
	"context"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewLVService creates a new LVServiceServer
func NewLVService(vgPrefix string, notifyFunc func()) proto.LVServiceServer {
	return lvService{
		vgPrefix:   vgPrefix,
		notifyFunc: notifyFunc,
	}
}

type lvService struct {
	vgPrefix   string
	notifyFunc func()
}

func (s lvService) notify() {
	if s.notifyFunc == nil {
		return
	}
	s.notifyFunc()
}

func (s lvService) CreateLV(_ context.Context, req *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
	vgName := s.vgPrefix + req.VgName
	vg, err := command.FindVolumeGroup(vgName)
	if err != nil {
		return nil, err
	}
	requested := req.GetSizeGb() << 30
	free, err := vg.Free()
	if err != nil {
		log.Error("failed to free VG", map[string]interface{}{
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

	lv, err := vg.CreateVolume(req.GetName(), requested, req.GetTags())
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

func (s lvService) RemoveLV(_ context.Context, req *proto.RemoveLVRequest) (*proto.Empty, error) {
	vgName := s.vgPrefix + req.VgName
	vg, err := command.FindVolumeGroup(vgName)
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

func (s lvService) ResizeLV(_ context.Context, req *proto.ResizeLVRequest) (*proto.Empty, error) {
	vgName := s.vgPrefix + req.VgName
	vg, err := command.FindVolumeGroup(vgName)
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
