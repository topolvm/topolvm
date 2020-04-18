package lvmd

import (
	"context"
	"os"
	"os/exec"
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

func testWatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	vgService, notifier := NewVGService(1, "")

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
	vgService, _ := NewVGService(1, "")
	res, err := vgService.GetLVList(context.Background(), &proto.GetLVListRequest{VgName: vg.Name()})
	if err != nil {
		t.Fatal(err)
	}
	numVols1 := len(res.GetVolumes())
	if numVols1 != 0 {
		t.Errorf("numVolumes must be 0: %d", numVols1)
	}
	testtag := "testtag"
	_, err = vg.CreateVolume("test1", 1<<30, []string{testtag})
	if err != nil {
		t.Fatal(err)
	}

	res, err = vgService.GetLVList(context.Background(), &proto.GetLVListRequest{VgName: vg.Name()})
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
	if len(vol.GetTags()) != 1 {
		t.Fatalf("number of tags must be 1")
	}
	if vol.GetTags()[0] != testtag {
		t.Errorf(`Volume.Tags[0] != %s: %v`, testtag, vol.GetTags())
	}

	_, err = vg.CreateVolume("test2", 1<<30, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = exec.Command("lvresize", "-L", "+12m", vg.Name()+"/test1").Run()
	if err != nil {
		t.Fatal(err)
	}

	res, err = vgService.GetLVList(context.Background(), &proto.GetLVListRequest{VgName: vg.Name()})
	if err != nil {
		t.Fatal(err)
	}
	numVols3 := len(res.GetVolumes())
	if numVols3 != 2 {
		t.Fatalf("numVolumes must be 2: %d", numVols3)
	}

	res2, err := vgService.GetFreeBytes(context.Background(), &proto.GetFreeBytesRequest{VgName: vg.Name()})
	if err != nil {
		t.Fatal(err)
	}
	freeBytes, err := vg.Free()
	if err != nil {
		t.Fatal(err)
	}
	expected := freeBytes - (1 << 30)
	if res2.GetFreeBytes() != expected {
		t.Errorf("Free bytes mismatch: %d, expected: %d, freeBytes: %d", res2.GetFreeBytes(), expected, freeBytes)
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
		testWatch(t)
	})
}
