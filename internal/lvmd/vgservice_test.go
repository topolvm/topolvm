package lvmd

import (
	"context"
	"math"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/topolvm/topolvm/internal/lvmd/command"
	"github.com/topolvm/topolvm/internal/lvmd/testutils"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
	"google.golang.org/grpc/metadata"
	ctrl "sigs.k8s.io/controller-runtime"
)

type mockWatchServer struct {
	ch  chan struct{}
	ctx context.Context
}

var waitDuration time.Duration = 2

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
	overprovisionRatio := float64(2.0)
	tests := []struct {
		name          string
		deviceClasses []*lvmdTypes.DeviceClass
	}{

		{"volumegroup", []*lvmdTypes.DeviceClass{
			{
				Name:        "dc",
				VolumeGroup: "test_vgservice",
			}},
		},
		{"thinpool", []*lvmdTypes.DeviceClass{
			{
				Name:        "dc",
				VolumeGroup: "test_vgservice",
				Type:        lvmdTypes.TypeThin,
				ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
					Name:               "test_pool",
					OverprovisionRatio: overprovisionRatio,
				},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			vgService, notifier := NewVGService(NewDeviceClassManager(tt.deviceClasses))

			ch1 := make(chan struct{})
			server1 := &mockWatchServer{
				ctx: ctx,
				ch:  ch1,
			}
			done := make(chan struct{})
			go func() {
				_ = vgService.Watch(&proto.Empty{}, server1)
				done <- struct{}{}
			}()

			select {
			case <-ch1:
			case <-time.After(waitDuration * time.Second):
				t.Fatal("not received the first event")
			}

			notifier()

			select {
			case <-ch1:
			case <-time.After(waitDuration * time.Second):
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
				_ = vgService.Watch(&proto.Empty{}, server2)
			}()

			notifier()

			select {
			case <-ch1:
			case <-time.After(waitDuration * time.Second):
				t.Fatal("not received")
			}
			select {
			case <-ch2:
			case <-time.After(waitDuration * time.Second):
				t.Fatal("not received")
			}

			cancel()

			select {
			case <-done:
			case <-time.After(waitDuration * time.Second):
				t.Fatal("not done")
			}

		})
	}
}

func testVGService(t *testing.T, vg *command.VolumeGroup) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))

	// thinpool details
	overprovisionRatio := float64(10.0)
	poolName := "test_pool"
	poolSize := uint64(1 << 30)
	pool, err := vg.CreatePool(ctx, poolName, poolSize)
	if err != nil {
		t.Fatal(err)
	}

	spareGB := uint64(1)
	thickdev := vg.Name()
	thindev := poolName
	vgService, _ := NewVGService(
		NewDeviceClassManager(
			[]*lvmdTypes.DeviceClass{
				{
					// volumegroup target
					Name:        thickdev,
					VolumeGroup: vg.Name(),
					SpareGB:     &spareGB,
				},
				{
					// thinpool target
					Name:        thindev,
					VolumeGroup: vg.Name(),
					SpareGB:     &spareGB,
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               poolName,
						OverprovisionRatio: overprovisionRatio,
					},
				},
			},
		),
	)

	// thick lvs
	res, err := vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: thickdev})
	if err != nil {
		t.Fatal(err)
	}
	numVols1 := len(res.GetVolumes())
	if numVols1 != 0 {
		t.Errorf("numVolumes must be 0: %d", numVols1)
	}

	// thin lvs
	res, err = vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: thindev})
	if err != nil {
		t.Fatal(err)
	}
	numVols1 = len(res.GetVolumes())

	if numVols1 != 0 {
		t.Errorf("numVolumes must be 0: %d in pool %s", numVols1, poolName)
	}

	// create thick volume
	testtag := "testtag"
	if err := vg.CreateVolume(ctx, "test1", 1<<30, []string{testtag}, 0, "", nil); err != nil {
		t.Fatal(err)
	}

	// thick lv validation
	res, err = vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: thickdev})
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
	if vol.GetSizeBytes() != 1<<30 {
		t.Errorf(`Volume.SizeBytes != %d: %d`, 1<<30, vol.GetSizeBytes())
	}
	if len(vol.GetTags()) != 1 {
		t.Fatalf("number of tags must be 1")
	}
	if vol.GetTags()[0] != testtag {
		t.Errorf(`Volume.Tags[0] != %s: %v`, testtag, vol.GetTags())
	}

	// create thin volume
	if err = pool.CreateVolume(ctx, "testp1", 1<<30, []string{testtag}, 0, "", nil); err != nil {
		t.Fatal(err)
	}

	// thin lv validation
	res, err = vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: thindev})
	if err != nil {
		t.Fatal(err)
	}
	numVols2 = len(res.GetVolumes())
	if numVols2 != 1 {
		t.Fatalf("numVolumes must be 1: %d in pool %s", numVols2, poolName)
	}

	vol = res.GetVolumes()[0]
	if vol.GetName() != "testp1" {
		t.Errorf(`Volume.Name != "test1": %s`, vol.GetName())
	}
	if sizeGB := vol.GetSizeGb(); sizeGB != 1 {
		t.Errorf(`Volume.SizeGb != 1: %d`, sizeGB)
	}
	if vol.GetSizeBytes() != 1<<30 {
		t.Errorf(`Volume.SizeBytes != %d: %d`, 1<<30, vol.GetSizeBytes())
	}
	if len(vol.GetTags()) != 1 {
		t.Fatalf("number of tags must be 1")
	}
	if vol.GetTags()[0] != testtag {
		t.Errorf(`Volume.Tags[0] != %s: %v`, testtag, vol.GetTags())
	}

	// thick lv creation

	if err = vg.CreateVolume(ctx, "test2", 1<<30, nil, 0, "", nil); err != nil {
		t.Fatal(err)
	}

	// thin lv creation, within overprovision limit (10G)

	if err := pool.CreateVolume(ctx, "testp2", 1<<30, nil, 0, "", nil); err != nil {
		t.Fatal(err)
	}

	testp2, err := pool.FindVolume(ctx, "testp2")
	if err != nil {
		t.Fatal(err)
	}

	// resize of thick volume
	err = exec.Command("lvresize", "-L", "+12m", vg.Name()+"/test1").Run()
	if err != nil {
		t.Fatal(err)
	}

	// resize of thin volumes
	err = exec.Command("lvresize", "-L", "+12m", vg.Name()+"/testp1").Run()
	if err != nil {
		t.Fatal(err)
	}

	// thick lv validation
	res, err = vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: thickdev})
	if err != nil {
		t.Fatal(err)
	}
	numVols3 := len(res.GetVolumes())
	if numVols3 != 2 {
		t.Fatalf("numVolumes must be 2: %d", numVols3)
	}

	// thick lv validation
	res, err = vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: thickdev})
	if err != nil {
		t.Fatal(err)
	}
	numVols3 = len(res.GetVolumes())
	if numVols3 != 2 {
		t.Fatalf("numVolumes must be 2: %d on thinpool %s", numVols3, poolName)
	}

	// thick lv size validations
	res2, err := vgService.GetFreeBytes(ctx, &proto.GetFreeBytesRequest{DeviceClass: thickdev})
	if err != nil {
		t.Fatal(err)
	}

	if err := vg.Update(ctx); err != nil {
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

	// thin lv size validations
	res2, err = vgService.GetFreeBytes(ctx, &proto.GetFreeBytesRequest{DeviceClass: thindev})
	if err != nil {
		t.Fatal(err)
	}
	tpu, err := pool.Free(ctx)
	if err != nil {
		t.Fatal(err)
	}
	opb := uint64(math.Floor(overprovisionRatio*float64(tpu.SizeBytes))) - tpu.VirtualBytes
	expected = opb - (1 << 30)
	// there'll be round off in free bytes of thin pool
	if res2.GetFreeBytes() > expected {
		t.Errorf("Free bytes mismatch: %d, expected: %d, freeBytes: %d", res2.GetFreeBytes(), expected, opb)
	}

	// Creation of thick volumes

	if err := vg.CreateVolume(ctx, "test3", 1<<30, nil, 2, "4k", nil); err != nil {
		t.Fatal(err)
	}
	test3Vol, err := vg.FindVolume(ctx, "test3")
	if err != nil {
		t.Fatal(err)
	}

	if err := vg.CreateVolume(ctx, "test4", 1<<30, nil, 2, "4M", nil); err != nil {
		t.Fatal(err)
	}
	test4Vol, err := vg.FindVolume(ctx, "test4")
	if err != nil {
		t.Fatal(err)
	}

	// Remove volumes to make room for a raid volume
	_ = vg.RemoveVolume(ctx, test3Vol.Name())
	_ = vg.RemoveVolume(ctx, test4Vol.Name())

	// Remove one of the thin lvs
	_ = vg.RemoveVolume(ctx, testp2.Name())

	t.Run("thinpool-stripe-raid", func(t *testing.T) {
		t.Skip("investigate support of striped and raid for thinlvs")
		// TODO (leelavg):
		// 1. confirm that stripe, stripesize and raid isn't possible on thin lv
		// 2. if above is true, enforce some sensible defaults during validation of deviceclass
		// thick lv with raid
		if err := vg.CreateVolume(ctx, "test5", 1<<30, nil, 0, "", []string{"--type=raid1"}); err != nil {
			t.Fatal(err)
		}

		// thin lv with stripe, stripesize and raid options
		if err := pool.CreateVolume(ctx, "test3", 1<<30, nil, 2, "4k", nil); err != nil {
			t.Fatal(err)
		}
		testp3Vol, err := vg.FindVolume(ctx, "test3")
		if err != nil {
			t.Fatal(err)
		}

		if err := pool.CreateVolume(ctx, "test4", 1<<30, nil, 2, "4M", nil); err != nil {
			t.Fatal(err)
		}
		testp4Vol, err := vg.FindVolume(ctx, "test4")
		if err != nil {
			t.Fatal(err)
		}

		// thin lv with raid
		if err = pool.CreateVolume(ctx, "test5", 1<<30, nil, 0, "", []string{"--type=raid1"}); err != nil {
			t.Fatal(err)
		}

		// Remove thin volumes
		_ = vg.RemoveVolume(ctx, testp3Vol.Name())
		_ = vg.RemoveVolume(ctx, testp4Vol.Name())

	})
}

func TestVGService(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	uid := os.Getuid()
	if uid != 0 {
		t.Skip("run as root")
	}

	vgName := "test_vgservice"
	loop1, err := testutils.MakeLoopbackDevice(ctx, vgName+"1")
	if err != nil {
		t.Fatal(err)
	}
	loop2, err := testutils.MakeLoopbackDevice(ctx, vgName+"2")
	if err != nil {
		t.Fatal(err)
	}
	loop3, err := testutils.MakeLoopbackDevice(ctx, vgName+"3")
	if err != nil {
		t.Fatal(err)
	}

	err = testutils.MakeLoopbackVG(ctx, vgName, loop1, loop2, loop3)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = testutils.CleanLoopbackVG(vgName, []string{loop1, loop2, loop3}, []string{vgName + "1", vgName + "2", vgName + "3"})
	}()

	vg, err := command.FindVolumeGroup(ctx, vgName)
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
