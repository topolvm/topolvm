package lvmd

import (
	"context"

	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
)

// NewLVService creates a new LVServiceServer
func NewLVService(vg *command.VolumeGroup) proto.LVServiceServer {
	return lvService{vg}
}

type lvService struct {
	vg *command.VolumeGroup
}

func (s lvService) CreateLV(context.Context, *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
	return &proto.CreateLVResponse{}, nil
}

func (s lvService) RemoveLV(context.Context, *proto.RemoveLVRequest) (*proto.RemoveLVResponse, error) {
	panic("implement me")
}

func (s lvService) ResizeLV(context.Context, *proto.ResizeLVRequest) (*proto.ResizeLVResponse, error) {
	panic("implement me")
}
