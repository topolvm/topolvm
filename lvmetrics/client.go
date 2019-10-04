package lvmetrics

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc"
)

// Metrics is the struct for prometheus metrics
type Metrics struct {
	AvailableBytes uint64
}

// WatchLVMd receives LVM volume group metrics and updates annotations of Node.
func WatchLVMd(ctx context.Context, conn *grpc.ClientConn, patcher *NodePatcher, metricsData *atomic.Value) error {
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
		metricsData.Store(Metrics{AvailableBytes: met.FreeBytes})
		err = patcher.Patch(met)
		if err != nil {
			return err
		}
	}
}
