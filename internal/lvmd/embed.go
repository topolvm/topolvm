package lvmd

import (
	"context"
	"time"

	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	gproto "google.golang.org/protobuf/proto"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NewEmbeddedServiceClients creates clients locally calling instead of using gRPC.
func NewEmbeddedServiceClients(ctx context.Context, dcmapper *DeviceClassManager, ocmapper *LvcreateOptionClassManager) (
	proto.LVServiceClient,
	proto.VGServiceClient,
) {
	vgServiceServerInstance, notifier := NewVGService(dcmapper)
	lvServiceServerInstance := NewLVService(dcmapper, ocmapper, notifier)

	caller := &embeddedServiceClients{
		lvServiceServer: lvServiceServerInstance,
		vgServiceServer: vgServiceServerInstance,
	}
	caller.vgWatch = &embeddedChannelWatch{ctx: ctx, watch: make(chan any)}
	go func() {
		if err := caller.vgServiceServer.Watch(&proto.Empty{}, caller.vgWatch); err != nil {
			log.FromContext(ctx).Error(err, "embedded channel watch error")
		}
	}()

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				notifier()
			}
		}
	}()

	return caller, caller
}

// embeddedServiceClients is a struct holding indirections to the local lvmd server.
// It implements both the LVServiceClient and VGServiceClient interfaces.
// It also includes a watch for redirecting the vg watch to the local caller.
type embeddedServiceClients struct {
	lvServiceServer proto.LVServiceServer
	vgServiceServer proto.VGServiceServer
	vgWatch         *embeddedChannelWatch
}

// embeddedChannelWatch is a local implementation of the VGService_WatchClient and VGService_WatchServer that is used by vgService.
// it uses an unbound blocking channel to send and receive messages.
type embeddedChannelWatch struct {
	watch chan any
	ctx   context.Context
}

// SetHeader is stubbed out to satisfy the grpc.ServerStream interface.
func (l *embeddedChannelWatch) SetHeader(md metadata.MD) error { return nil }

// SendHeader is stubbed out to satisfy the grpc.ServerStream interface.
func (l *embeddedChannelWatch) SendHeader(md metadata.MD) error { return nil }

// SetTrailer is stubbed out to satisfy the grpc.ServerStream interface.
func (l *embeddedChannelWatch) SetTrailer(md metadata.MD) {}

// Header is stubbed out to satisfy the grpc.ClientStream interface.
func (l *embeddedChannelWatch) Header() (metadata.MD, error) { return nil, nil }

// Trailer is stubbed out to satisfy the grpc.ClientStream interface.
func (l *embeddedChannelWatch) Trailer() metadata.MD { return nil }

// CloseSend is stubbed out to satisfy the grpc.ClientStream interface.
func (l *embeddedChannelWatch) CloseSend() error { return nil }

// Context is relaying the server context to satisfy the grpc.ClientStream and grpc.ServerStream interface.
// Client Context is ignored as the server context is always living longer.
func (l *embeddedChannelWatch) Context() context.Context { return l.ctx }

// Recv is used to receive a WatchResponse as a VGService_WatchClient.
func (l *embeddedChannelWatch) Recv() (*proto.WatchResponse, error) {
	m := new(proto.WatchResponse)
	if err := l.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Send is used to send a WatchResponse as a VGService_WatchServer.
func (l *embeddedChannelWatch) Send(m *proto.WatchResponse) error {
	return l.SendMsg(m)
}

// SendMsg is used to send messages to the channel both as a grpc.ClientStream and as a grpc.ServerStream.
func (l *embeddedChannelWatch) SendMsg(m interface{}) error {
	l.watch <- m
	return nil
}

// RecvMsg is used to receive messages from the channel both as a grpc.ClientStream and as a grpc.ServerStream.
func (l *embeddedChannelWatch) RecvMsg(m interface{}) error {
	select {
	case received, ok := <-l.watch:
		if !ok {
			return status.Error(codes.Aborted, "watch closed")
		}
		receivedMsg, ok := received.(gproto.Message)
		if !ok {
			return status.Error(codes.Internal, "did not receive Message")
		}
		mMsg, ok := m.(gproto.Message)
		if !ok {
			return status.Error(codes.Internal, "pointer passed is not a Message")
		}
		gproto.Merge(mMsg, receivedMsg)
	case <-l.ctx.Done():
		return l.ctx.Err()
	}
	return nil
}

// Watch returns a local implementation of the VGService_WatchClient interface that is also implementing the VGService_WatchServer interface.
// This is used to redirect the watch to the local caller via channel.
// Any call other than send and receive will be ignored as they are not necessary.
func (l *embeddedServiceClients) Watch(_ context.Context, _ *proto.Empty, _ ...grpc.CallOption) (proto.VGService_WatchClient, error) {
	return l.vgWatch, nil
}

func (l *embeddedServiceClients) CreateLV(ctx context.Context, in *proto.CreateLVRequest, _ ...grpc.CallOption) (*proto.CreateLVResponse, error) {
	return l.lvServiceServer.CreateLV(ctx, in)
}

func (l *embeddedServiceClients) RemoveLV(ctx context.Context, in *proto.RemoveLVRequest, _ ...grpc.CallOption) (*proto.Empty, error) {
	return l.lvServiceServer.RemoveLV(ctx, in)
}

func (l *embeddedServiceClients) ResizeLV(ctx context.Context, in *proto.ResizeLVRequest, _ ...grpc.CallOption) (*proto.Empty, error) {
	return l.lvServiceServer.ResizeLV(ctx, in)
}

func (l *embeddedServiceClients) CreateLVSnapshot(ctx context.Context, in *proto.CreateLVSnapshotRequest, _ ...grpc.CallOption) (*proto.CreateLVSnapshotResponse, error) {
	return l.lvServiceServer.CreateLVSnapshot(ctx, in)
}

func (l *embeddedServiceClients) GetLVList(ctx context.Context, in *proto.GetLVListRequest, _ ...grpc.CallOption) (*proto.GetLVListResponse, error) {
	return l.vgServiceServer.GetLVList(ctx, in)
}

func (l *embeddedServiceClients) GetFreeBytes(ctx context.Context, in *proto.GetFreeBytesRequest, _ ...grpc.CallOption) (*proto.GetFreeBytesResponse, error) {
	return l.vgServiceServer.GetFreeBytes(ctx, in)
}
