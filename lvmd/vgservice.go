package lvmd

import (
	"context"

	"github.com/cybozu-go/topolvm/lvmd/proto"
)

// NewVGService creates a VGServiceServer
func NewVGService(vg string) proto.VGServiceServer {
	return vgService{vg}
}

type vgService struct {
	vg string
}

func (s vgService) GetLVList(context.Context, *proto.Empty) (*proto.GetLVListResponse, error) {
	panic("implement me")
}

func (s vgService) GetFreeBytes(context.Context, *proto.Empty) (*proto.GetFreeBytesResponse, error) {
	panic("implement me")
}
