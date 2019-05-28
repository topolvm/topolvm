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
func NewLVService(vg *command.VolumeGroup, notifyFunc func()) proto.LVServiceServer {
	return lvService{vg, notifyFunc}
}

type lvService struct {
	vg         *command.VolumeGroup
	notifyFunc func()
}

func (s lvService) notify() {
	if s.notifyFunc == nil {
		return
	}
	s.notifyFunc()
}

func (s lvService) CreateLV(_ context.Context, req *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
	requested := req.GetSizeGb() << 30
	free, err := s.vg.Free()
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	if free < requested {
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested)
	}

	lv, err := s.vg.CreateVolume(req.GetName(), requested)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	s.notify()

	log.Info("created a new LV", map[string]interface{}{
		"name": req.GetName(),
		"size": requested,
		"uuid": lv.UUID(),
	})

	return &proto.CreateLVResponse{
		Volume: &proto.LogicalVolume{
			Name:   lv.Name(),
			SizeGb: lv.Size() >> 30,
			Uuid:   lv.UUID(),
		},
	}, nil
}

func (s lvService) RemoveLV(_ context.Context, req *proto.RemoveLVRequest) (*proto.Empty, error) {
	lvs, err := s.vg.ListVolumes()
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	for _, lv := range lvs {
		if lv.Name() != req.GetName() {
			continue
		}

		err = lv.Remove()
		if err != nil {
			return nil, status.Errorf(codes.Internal, err.Error())
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
	lv, err := s.vg.FindVolume(req.GetName())
	if err == command.ErrNotFound {
		return nil, status.Errorf(codes.NotFound, "logical volume %s is not found", req.GetName())
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	requested := req.GetSizeGb() << 30
	current := lv.Size()

	if requested < current {
		return nil, status.Errorf(codes.OutOfRange, "shrinking volume size is not allowed")
	}

	free, err := s.vg.Free()
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	if free < (requested - current) {
		return nil, status.Errorf(codes.ResourceExhausted, "no enough space left on VG: free=%d, requested=%d", free, requested-current)
	}

	err = lv.Resize(requested)
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	s.notify()

	log.Info("resized a LV", map[string]interface{}{
		"name": req.GetName(),
		"size": requested,
	})

	return &proto.Empty{}, nil
}
