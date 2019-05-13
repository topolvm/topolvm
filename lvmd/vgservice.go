package lvmd

import (
	"context"

	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewVGService creates a VGServiceServer
func NewVGService(vg *command.VolumeGroup) proto.VGServiceServer {
	return vgService{vg}
}

type vgService struct {
	vg *command.VolumeGroup
}

func (s vgService) GetLVList(context.Context, *proto.Empty) (*proto.GetLVListResponse, error) {
	lvs, err := s.vg.ListVolumes()
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	vols := make([]*proto.LogicalVolume, len(lvs))
	for i, lv := range lvs {

		vols[i] = &proto.LogicalVolume{
			Name:     lv.Name(),
			SizeGb:   (lv.Size() + (1 << 30) - 1) >> 30,
			DevMajor: lv.MajorNumber(),
			DevMinor: lv.MinorNumber(),
		}
	}
	return &proto.GetLVListResponse{Volumes: vols}, nil
}

func (s vgService) GetFreeBytes(context.Context, *proto.Empty) (*proto.GetFreeBytesResponse, error) {
	vgFree, err := s.vg.Free()
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	return &proto.GetFreeBytesResponse{
		FreeBytes: vgFree,
	}, nil
}
