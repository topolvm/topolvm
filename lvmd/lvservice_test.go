package lvmd

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLVService(t *testing.T) {
	uid := os.Getuid()
	if uid != 0 {
		t.Skip("run as root")
	}

	vgName := "test_lvservice"
	loop, err := MakeLoopbackDevice(vgName)
	if err != nil {
		t.Fatal(err)
	}

	err = MakeLoopbackVG(vgName, loop)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanLoopbackVG(vgName, []string{loop}, []string{vgName})

	vg, err := command.FindVolumeGroup(vgName)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	notifier := func() {
		count++
	}

	// thinpool details
	overprovisionRatio := float64(10.0)
	poolName := "test_pool"
	poolSize := uint64(1 << 30)
	pool, err := vg.CreatePool(poolName, poolSize)
	if err != nil {
		t.Fatal(err)
	}

	thickdev := vgName
	thindev := poolName
	lvService := NewLVService(
		NewDeviceClassManager(
			[]*DeviceClass{
				{
					// volumegroup target
					Name:        thickdev,
					VolumeGroup: vg.Name(),
				},
				{
					// thinpool target
					Name:        thindev,
					VolumeGroup: vg.Name(),
					Type:        TypeThin,
					ThinPoolConfig: &ThinPoolConfig{
						Name:               poolName,
						OverprovisionRatio: overprovisionRatio,
					},
				},
			},
		), notifier)

	// thick logical volume validations
	res, err := lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "test1",
		DeviceClass: thickdev,
		SizeGb:      1,
		Tags:        []string{"testtag1", "testtag2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("is not notified: %d", count)
	}
	if res.GetVolume().GetName() != "test1" {
		t.Errorf(`res.Volume.Name != "test1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeGb() != 1 {
		t.Errorf(`res.Volume.SizeGb != 1: %d`, res.GetVolume().GetSizeGb())
	}
	err = exec.Command("lvs", vg.Name()+"/test1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}
	lv, err := vg.FindVolume("test1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Tags()[0] != "testtag1" {
		t.Errorf(`testtag1 not present on volume`)
	}
	if lv.Tags()[1] != "testtag2" {
		t.Errorf(`testtag1 not present on volume`)
	}

	_, err = lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "test2",
		DeviceClass: thickdev,
		SizeGb:      3,
	})
	code := status.Code(err)
	if code != codes.ResourceExhausted {
		t.Errorf(`code is not codes.ResouceExhausted: %s`, code)
	}
	if count != 1 {
		t.Errorf("unexpected count: %d", count)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:        "test1",
		DeviceClass: thickdev,
		SizeGb:      2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("unexpected count: %d", count)
	}
	lv, err = vg.FindVolume("test1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Size() != (2 << 30) {
		t.Errorf(`does not match size 2: %d`, lv.Size()>>30)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:        "test1",
		DeviceClass: thickdev,
		SizeGb:      5,
	})
	code = status.Code(err)
	if code != codes.ResourceExhausted {
		t.Errorf(`code is not codes.ResouceExhausted: %s`, code)
	}
	if count != 2 {
		t.Errorf("unexpected count: %d", count)
	}

	_, err = lvService.RemoveLV(context.Background(), &proto.RemoveLVRequest{
		Name:        "test1",
		DeviceClass: thickdev,
	})
	if err != nil {
		t.Error(err)
	}
	if count != 3 {
		t.Errorf("unexpected count: %d", count)
	}
	_, err = vg.FindVolume("test1")
	if err != command.ErrNotFound {
		t.Error("unexpected error: ", err)
	}

	// thin logical volume validations
	count = 0
	res, err = lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "testp1",
		DeviceClass: thindev,
		SizeGb:      1,
		Tags:        []string{"testtag1", "testtag2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("is not notified: %d", count)
	}
	if res.GetVolume().GetName() != "testp1" {
		t.Errorf(`res.Volume.Name != "testp1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeGb() != 1 {
		t.Errorf(`res.Volume.SizeGb != 1: %d`, res.GetVolume().GetSizeGb())
	}
	err = exec.Command("lvs", vg.Name()+"/testp1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}
	lv, err = pool.FindVolume("testp1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Tags()[0] != "testtag1" {
		t.Errorf(`testtag1 not present on volume`)
	}
	if lv.Tags()[1] != "testtag2" {
		t.Errorf(`testtag1 not present on volume`)
	}

	// overprovision should work
	_, err = lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "testp2",
		DeviceClass: thindev,
		SizeGb:      3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("unexpected count: %d", count)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:        "testp1",
		DeviceClass: thindev,
		SizeGb:      2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("unexpected count: %d", count)
	}
	lv, err = pool.FindVolume("testp1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Size() != (2 << 30) {
		t.Errorf(`does not match size 2: %d`, lv.Size()>>30)
	}

	_, err = lvService.RemoveLV(context.Background(), &proto.RemoveLVRequest{
		Name:        "testp1",
		DeviceClass: thindev,
	})
	if err != nil {
		t.Error(err)
	}
	if count != 4 {
		t.Errorf("unexpected count: %d", count)
	}
	_, err = pool.FindVolume("test1")
	if err != command.ErrNotFound {
		t.Error("unexpected error: ", err)
	}
}
