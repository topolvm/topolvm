package lvmd

import (
	"context"

	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewLVService creates a new LVServiceServer
func NewLVService(vg *command.VolumeGroup) proto.LVServiceServer {
	return lvService{vg}
}

type lvService struct {
	vg *command.VolumeGroup
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

	return &proto.CreateLVResponse{
		Volume: &proto.LogicalVolume{
			Name:     lv.Name(),
			SizeGb:   lv.Size() >> 30,
			DevMajor: lv.MajorNumber(),
			DevMinor: lv.MinorNumber(),
		},
	}, nil
}

func (s lvService) RemoveLV(context.Context, *proto.RemoveLVRequest) (*proto.Empty, error) {
	panic("implement me")
}

func (s lvService) ResizeLV(context.Context, *proto.ResizeLVRequest) (*proto.Empty, error) {
	panic("implement me")
}
