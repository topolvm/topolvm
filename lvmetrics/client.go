package lvmetrics

import (
	"context"
	"io"

	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc"
)

// WatchLVMd receives LVM volume group metrics and updates annotations of Node.
func WatchLVMd(ctx context.Context, conn *grpc.ClientConn, patcher *NodePatcher) error {
	client := proto.NewVGServiceClient(conn)
	wClient, err := client.Watch(ctx, &proto.Empty{})
	if err != nil {
		return err
	}

	for {
		res, err := wClient.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		met := &NodeMetrics{
			FreeBytes: res.GetFreeBytes(),
		}
		err = patcher.Patch(met)
		if err != nil {
			return err
		}
	}
}
