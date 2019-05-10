package lvmd

import (
	"context"

	"github.com/cybozu-go/topolvm/lvmd/proto"
)

// NewLVService creates a new LVServiceServer
func NewLVService(vg string) proto.LVServiceServer {
	return lvService{vg}
}

type lvService struct {
	vg string
}

func (s lvService) CreateLV(context.Context, *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
	panic("implement me")
}

func (s lvService) RemoveLV(context.Context, *proto.RemoveLVRequest) (*proto.RemoveLVResponse, error) {
	panic("implement me")
}

func (s lvService) ResizeLV(context.Context, *proto.ResizeLVRequest) (*proto.ResizeLVResponse, error) {
	panic("implement me")
}
