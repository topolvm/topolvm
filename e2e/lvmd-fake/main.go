package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/cybozu-go/topolvm/lvmd/proto"
	"github.com/cybozu-go/well"
	"google.golang.org/grpc"
)

func main() {
	err := subMain()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func subMain() error {
	// UNIX domain socket file should be removed before listening.
	socketName := "/tmp/lvmd.sock"
	os.Remove(socketName)

	lis, err := net.Listen("unix", socketName)
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	proto.RegisterVGServiceServer(grpcServer, vgService{})
	proto.RegisterLVServiceServer(grpcServer, lvService{})
	well.Go(func(ctx context.Context) error {
		return grpcServer.Serve(lis)
	})
	well.Go(func(ctx context.Context) error {
		<-ctx.Done()
		grpcServer.GracefulStop()
		return nil
	})

	err = well.Wait()
	if err != nil && !well.IsSignaled(err) {
		return err
	}
	return nil
}

type vgService struct {
}

func (s vgService) GetLVList(context.Context, *proto.Empty) (*proto.GetLVListResponse, error) {
	panic("not used")
}

func (s vgService) GetFreeBytes(context.Context, *proto.Empty) (*proto.GetFreeBytesResponse, error) {
	panic("not used")
}

func (s vgService) Watch(_ *proto.Empty, server proto.VGService_WatchServer) error {
	err := server.Send(&proto.WatchResponse{
		FreeBytes: 5 << 30,
	})
	if err != nil {
		return err
	}

	var ch chan struct{}
	<-ch

	return nil
}

type lvService struct {
}

func (g lvService) CreateLV(ctx context.Context, req *proto.CreateLVRequest) (*proto.CreateLVResponse, error) {
	return &proto.CreateLVResponse{
		Volume: &proto.LogicalVolume{
			Name:     req.Name,
			SizeGb:   req.SizeGb,
			DevMajor: 0,
			DevMinor: 0,
		},
	}, nil
}

func (g lvService) RemoveLV(context.Context, *proto.RemoveLVRequest) (*proto.Empty, error) {
	panic("implement me")
}

func (g lvService) ResizeLV(context.Context, *proto.ResizeLVRequest) (*proto.Empty, error) {
	panic("implement me")
}
