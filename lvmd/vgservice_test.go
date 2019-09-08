package lvmd

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc/metadata"
)

type mockWatchServer struct {
	ch  chan struct{}
	ctx context.Context
}

func (s *mockWatchServer) Send(r *proto.WatchResponse) error {
	s.ch <- struct{}{}
	return nil
}

func (s *mockWatchServer) SetHeader(metadata.MD) error {
	panic("implement me")
}

func (s *mockWatchServer) SendHeader(metadata.MD) error {
	panic("implement me")
}

func (s *mockWatchServer) SetTrailer(metadata.MD) {
	panic("implement me")
}

func (s *mockWatchServer) Context() context.Context {
	return s.ctx
}

func (s *mockWatchServer) SendMsg(m interface{}) error {
	panic("implement me")
}

func (s *mockWatchServer) RecvMsg(m interface{}) error {
	panic("implement me")
}

func testWatch(t *testing.T, vg *command.VolumeGroup) {
	ctx, cancel := context.WithCancel(context.Background())
	vgService, notifier := NewVGService(vg, 1)

	ch1 := make(chan struct{})
	server1 := &mockWatchServer{
		ctx: ctx,
		ch:  ch1,
	}
	done := make(chan struct{})
	go func() {
		vgService.Watch(&proto.Empty{}, server1)
		done <- struct{}{}
	}()

	select {
	case <-ch1:
	case <-time.After(1 * time.Second):
		t.Fatal("not received the first event")
	}

	for {
		if res, _ := vgService.GetWatchersCount(context.Background(), &proto.Empty{}); res.Count == 1 || time.Now().After(time.Now().Add(2*time.Second)) {
			break
		}
	}

	notifier()

	select {
	case <-ch1:
	case <-time.After(1 * time.Second):
		t.Fatal("not received")
	}

	select {
	case <-ch1:
		t.Fatal("unexpected event")
	default:
	}

	ch2 := make(chan struct{})
	server2 := &mockWatchServer{
		ctx: ctx,
		ch:  ch2,
	}
	go func() {
		vgService.Watch(&proto.Empty{}, server2)
	}()

	notifier()

	select {
	case <-ch1:
	case <-time.After(1 * time.Second):
		t.Fatal("not received")
	}
	select {
	case <-ch2:
	case <-time.After(1 * time.Second):
		t.Fatal("not received")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("not done")
	}
}

func testVGService(t *testing.T, vg *command.VolumeGroup) {
	vgService, _ := NewVGService(vg, 1)
	res, err := vgService.GetLVList(context.Background(), &proto.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	numVols1 := len(res.GetVolumes())
	if numVols1 != 0 {
		t.Errorf("numVolumes must be 0: %d", numVols1)
	}

	_, err = vg.CreateVolume("test1", 1<<30)
	if err != nil {
		t.Fatal(err)
	}

	res, err = vgService.GetLVList(context.Background(), &proto.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	numVols2 := len(res.GetVolumes())
	if numVols2 != 1 {
		t.Fatalf("numVolumes must be 1: %d", numVols2)
	}

	vol := res.GetVolumes()[0]
	if vol.GetName() != "test1" {
		t.Errorf(`Volume.Name != "test1": %s`, vol.GetName())
	}
	if vol.GetSizeGb() != 1 {
		t.Errorf(`Volume.SizeGb != 1: %d`, vol.GetSizeGb())
	}

	res2, err := vgService.GetFreeBytes(context.Background(), &proto.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	freeBytes, err := vg.Free()
	if err != nil {
		t.Fatal(err)
	}
	expected := freeBytes - (1 << 30)
	if res2.GetFreeBytes() != expected {
		t.Errorf("Free bytes mismatch: %d", res2.GetFreeBytes())
	}
}

func TestVGService(t *testing.T) {
	uid := os.Getuid()
	if uid != 0 {
		t.Skip("run as root")
	}
	circleci := os.Getenv("CIRCLECI") == "true"
	if circleci {
		executorType := os.Getenv("CIRCLECI_EXECUTOR")
		if executorType != "machine" {
			t.Skip("run on machine executor")
		}
	}

	vgName := "test_vgservice"
	loop, err := MakeLoopbackVG(vgName)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanLoopbackVG(loop, vgName)

	vg, err := command.FindVolumeGroup(vgName)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("VGService", func(t *testing.T) {
		testVGService(t, vg)
	})
	t.Run("Watch", func(t *testing.T) {
		testWatch(t, vg)
	})
}
