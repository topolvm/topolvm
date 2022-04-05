package lvmd

import (
	"context"
	"math"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"google.golang.org/grpc/metadata"
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
		deviceClasses []*DeviceClass
	}{

		{"volumegroup", []*DeviceClass{
			{
				Name:        "ssd",
				VolumeGroup: "test_vgservice",
			}},
		},
		{"thinpool", []*DeviceClass{
			{
				Name:        "ssd",
				VolumeGroup: "test_vgservice",
				Type:        TypeThin,
				ThinPoolConfig: &ThinPoolConfig{
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
				vgService.Watch(&proto.Empty{}, server1)
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
				vgService.Watch(&proto.Empty{}, server2)
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

	// thinpool details
	overprovisionRatio := float64(10.0)
	poolName := "test_pool"
	poolSize := uint64(1 << 30)
	pool, err := vg.CreatePool(poolName, poolSize)
	if err != nil {
		t.Fatal(err)
	}

	spareGB := uint64(1)
	thickdev := vg.Name()
	thindev := poolName
	vgService, _ := NewVGService(
		NewDeviceClassManager(
			[]*DeviceClass{
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
					Type:        TypeThin,
					ThinPoolConfig: &ThinPoolConfig{
						Name:               poolName,
						OverprovisionRatio: overprovisionRatio,
					},
				},
			},
		),
	)

	// thick lvs
	res, err := vgService.GetLVList(context.Background(), &proto.GetLVListRequest{DeviceClass: thickdev})
	if err != nil {
		t.Fatal(err)
	}
	numVols1 := len(res.GetVolumes())
	if numVols1 != 0 {
		t.Errorf("numVolumes must be 0: %d", numVols1)
	}

	// thin lvs
	res, err = vgService.GetLVList(context.Background(), &proto.GetLVListRequest{DeviceClass: thindev})
	if err != nil {
		t.Fatal(err)
	}
	numVols1 = len(res.GetVolumes())

	if numVols1 != 0 {
		t.Errorf("numVolumes must be 0: %d in pool %s", numVols1, poolName)
	}

	// create thick volume
	testtag := "testtag"
	_, err = vg.CreateVolume("test1", 1<<30, []string{testtag}, 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	// thick lv validation
	res, err = vgService.GetLVList(context.Background(), &proto.GetLVListRequest{DeviceClass: thickdev})
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

	// create thin volume
	_, err = pool.CreateVolume("testp1", 1<<30, []string{testtag}, 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	// thin lv validation
	res, err = vgService.GetLVList(context.Background(), &proto.GetLVListRequest{DeviceClass: thindev})
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
	if vol.GetSizeGb() != 1 {
		t.Errorf(`Volume.SizeGb != 1: %d`, vol.GetSizeGb())
	}
	if len(vol.GetTags()) != 1 {
		t.Fatalf("number of tags must be 1")
	}
	if vol.GetTags()[0] != testtag {
		t.Errorf(`Volume.Tags[0] != %s: %v`, testtag, vol.GetTags())
	}

	// thick lv creation
	_, err = vg.CreateVolume("test2", 1<<30, nil, 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	// thin lv creation, within overprovision limit (10G)
	testp2, err := pool.CreateVolume("testp2", 1<<30, nil, 0, "", nil)
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
	res, err = vgService.GetLVList(context.Background(), &proto.GetLVListRequest{DeviceClass: thickdev})
	if err != nil {
		t.Fatal(err)
	}
	numVols3 := len(res.GetVolumes())
	if numVols3 != 2 {
		t.Fatalf("numVolumes must be 2: %d", numVols3)
	}

	// thick lv validation
	res, err = vgService.GetLVList(context.Background(), &proto.GetLVListRequest{DeviceClass: thickdev})
	if err != nil {
		t.Fatal(err)
	}
	numVols3 = len(res.GetVolumes())
	if numVols3 != 2 {
		t.Fatalf("numVolumes must be 2: %d on thinpool %s", numVols3, poolName)
	}

	// thick lv size validations
	res2, err := vgService.GetFreeBytes(context.Background(), &proto.GetFreeBytesRequest{DeviceClass: thickdev})
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

	// thin lv size validations
	res2, err = vgService.GetFreeBytes(context.Background(), &proto.GetFreeBytesRequest{DeviceClass: thindev})
	if err != nil {
		t.Fatal(err)
	}
	tpu, err := pool.Free()
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
	test3Vol, err := vg.CreateVolume("test3", 1<<30, nil, 2, "4k", nil)
	if err != nil {
		t.Fatal(err)
	}

	test4Vol, err := vg.CreateVolume("test4", 1<<30, nil, 2, "4M", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Remove volumes to make room for a raid volume
	test3Vol.Remove()
	test4Vol.Remove()

	// Remove one of the thin lvs
	testp2.Remove()

	t.Run("thinpool-stripe-raid", func(t *testing.T) {
		t.Skip("investigae support of striped and raid for thinlvs")
		// TODO (leelavg):
		// 1. confirm that stripe, stripesize and raid isn't possible on thin lv
		// 2. if above is true, enforce some sensible defaults during validation of deviceclass
		// thick lv with raid
		_, err = vg.CreateVolume("test5", 1<<30, nil, 0, "", []string{"--type=raid1"})
		if err != nil {
			t.Fatal(err)
		}

		// thin lv with stripe, stripesize and raid options
		testp3Vol, err := pool.CreateVolume("test3", 1<<30, nil, 2, "4k", nil)
		if err != nil {
			t.Fatal(err)
		}

		testp4Vol, err := pool.CreateVolume("test4", 1<<30, nil, 2, "4M", nil)
		if err != nil {
			t.Fatal(err)
		}

		// thin lv with raid
		_, err = pool.CreateVolume("test5", 1<<30, nil, 0, "", []string{"--type=raid1"})
		if err != nil {
			t.Fatal(err)
		}

		// Remove thin volumes
		testp3Vol.Remove()
		testp4Vol.Remove()

	})
}

func TestVGService(t *testing.T) {
	uid := os.Getuid()
	if uid != 0 {
		t.Skip("run as root")
	}

	vgName := "test_vgservice"
	loop1, err := MakeLoopbackDevice(vgName + "1")
	if err != nil {
		t.Fatal(err)
	}
	loop2, err := MakeLoopbackDevice(vgName + "2")
	if err != nil {
		t.Fatal(err)
	}
	loop3, err := MakeLoopbackDevice(vgName + "3")
	if err != nil {
		t.Fatal(err)
	}

	err = MakeLoopbackVG(vgName, loop1, loop2, loop3)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanLoopbackVG(vgName, []string{loop1, loop2}, []string{vgName + "1", vgName + "2", vgName + "3"})

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
