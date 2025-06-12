package lvmd

import (
	"context"
	"math"
	"path"
	"strconv"
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

const (
	vgServiceTestOverprovisionRatio = float64(10.0)
	vgServiceTestVGName             = "test_vgservice"
	vgServiceTestPoolName           = "test_vgservice_pool"
	vgServiceTestThickDC            = vgServiceTestVGName
	vgServiceTestThinDC             = vgServiceTestPoolName
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

func TestVGService_Watch(t *testing.T) {
	ctx, cancel := context.WithCancel(ctrl.LoggerInto(context.Background(), testr.New(t)))
	vgService, notifier, _, _ := setupVGService(ctx, t)

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
}

func TestVGService_GetLVList(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	vgService, _, vg, pool := setupVGService(ctx, t)

	testtag := "testtag"
	testCases := []struct {
		name         string
		deviceClass  string
		volumePrefix string
		createVolume func(string) error
	}{
		{
			name:         "thick lv",
			deviceClass:  vgServiceTestThickDC,
			volumePrefix: "thick",
			createVolume: func(volumeName string) error {
				return vg.CreateVolume(ctx, volumeName, 1<<30, []string{testtag}, 0, "", nil)
			},
		},
		{
			name:         "thin lv",
			deviceClass:  vgServiceTestThinDC,
			volumePrefix: "thin",
			createVolume: func(volumeName string) error {
				return pool.CreateVolume(ctx, volumeName, 1<<30, []string{testtag}, 0, "", nil)
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			res, err := vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: tt.deviceClass})
			if err != nil {
				t.Fatal(err)
			}
			numVols1 := len(res.GetVolumes())
			if numVols1 != 0 {
				t.Errorf("numVolumes must be 0: %d", numVols1)
			}

			// create 1st volume
			volumeName := tt.volumePrefix + "1"
			if err := tt.createVolume(volumeName); err != nil {
				t.Fatal(err)
			}

			// validation
			res, err = vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: tt.deviceClass})
			if err != nil {
				t.Fatal(err)
			}
			numVols2 := len(res.GetVolumes())
			if numVols2 != 1 {
				t.Fatalf("numVolumes must be 1: %d", numVols2)
			}

			vol := res.GetVolumes()[0]
			if vol.GetName() != volumeName {
				t.Errorf(`Volume.Name != "%s": %s`, volumeName, vol.GetName())
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

			// create 2nd volume
			volumeName = tt.volumePrefix + "2"
			if err = tt.createVolume(volumeName); err != nil {
				t.Fatal(err)
			}

			// validation
			res, err = vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: tt.deviceClass})
			if err != nil {
				t.Fatal(err)
			}
			numVols3 := len(res.GetVolumes())
			if numVols3 != 2 {
				t.Fatalf("numVolumes must be 2: %d", numVols3)
			}
		})
	}
}

func TestVGService_GetFreeBytes(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	vgService, _, vg, pool := setupVGService(ctx, t)

	// update internal state to get accurate free bytes
	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}

	t.Run("thick lv", func(t *testing.T) {
		res, err := vgService.GetFreeBytes(ctx, &proto.GetFreeBytesRequest{DeviceClass: vgServiceTestThickDC})
		if err != nil {
			t.Fatal(err)
		}
		freeBytes, err := vg.Free()
		if err != nil {
			t.Fatal(err)
		}
		expected := freeBytes - (1 << 30)
		if res.GetFreeBytes() != expected {
			t.Errorf("Free bytes mismatch: %d, expected: %d, freeBytes: %d", res.GetFreeBytes(), expected, freeBytes)
		}
	})

	t.Run("thin lv", func(t *testing.T) {
		res, err := vgService.GetFreeBytes(ctx, &proto.GetFreeBytesRequest{DeviceClass: vgServiceTestThinDC})
		if err != nil {
			t.Fatal(err)
		}
		tpu, err := pool.Usage(ctx)
		if err != nil {
			t.Fatal(err)
		}
		opb := uint64(math.Floor(vgServiceTestOverprovisionRatio*float64(tpu.SizeBytes))) - tpu.VirtualBytes
		expected := opb - (1 << 30)
		// there'll be round off in free bytes of thin pool
		if res.GetFreeBytes() > expected {
			t.Errorf("Free bytes mismatch: %d, expected: %d, freeBytes: %d", res.GetFreeBytes(), expected, opb)
		}
	})
}

func setupVGService(ctx context.Context, t *testing.T) (
	proto.VGServiceServer,
	func(),
	*command.VolumeGroup,
	*command.ThinPool,
) {
	testutils.RequireRoot(t)

	var loops []string
	var files []string
	for i := 0; i < 3; i++ {
		file := path.Join(t.TempDir(), t.Name()+strconv.Itoa(i))
		loop, err := testutils.MakeLoopbackDevice(ctx, file)
		if err != nil {
			t.Fatal(err)
		}
		loops = append(loops, loop)
		files = append(files, file)
	}

	err := testutils.MakeLoopbackVG(ctx, vgServiceTestVGName, loops...)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = testutils.CleanLoopbackVG(vgServiceTestVGName, loops, files)
	})

	vg, err := command.FindVolumeGroup(ctx, vgServiceTestVGName)
	if err != nil {
		t.Fatal(err)
	}

	// thinpool details
	poolSize := uint64(1 << 30)
	pool, err := vg.CreatePool(ctx, vgServiceTestPoolName, poolSize)
	if err != nil {
		t.Fatal(err)
	}

	spareGB := uint64(1)
	vgService, notifier := NewVGService(
		NewDeviceClassManager(
			[]*lvmdTypes.DeviceClass{
				{
					// volumegroup target
					Name:        vgServiceTestThickDC,
					VolumeGroup: vg.Name(),
					SpareGB:     &spareGB,
				},
				{
					// thinpool target
					Name:        vgServiceTestThinDC,
					VolumeGroup: vg.Name(),
					SpareGB:     &spareGB,
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               vgServiceTestPoolName,
						OverprovisionRatio: vgServiceTestOverprovisionRatio,
					},
				},
			},
		),
	)

	return vgService, notifier, vg, pool
}
