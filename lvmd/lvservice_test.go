package lvmd

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/topolvm/topolvm/lvmd/command"
	"github.com/topolvm/topolvm/lvmd/proto"
	"github.com/topolvm/topolvm/lvmd/testutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLVService(t *testing.T) {
	uid := os.Getuid()
	if uid != 0 {
		t.Skip("run as root")
	}

	vgName := "test_lvservice"
	loop, err := testutils.MakeLoopbackDevice(vgName)
	if err != nil {
		t.Fatal(err)
	}

	err = testutils.MakeLoopbackVG(vgName, loop)
	if err != nil {
		t.Fatal(err)
	}
	defer testutils.CleanLoopbackVG(vgName, []string{loop}, []string{vgName})

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
		), NewLvcreateOptionClassManager([]*LvcreateOptionClass{}), notifier)

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

	if err := vg.Update(); err != nil {
		t.Fatal(err)
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

	if err := vg.Update(); err != nil {
		t.Fatal(err)
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

	if err := vg.Update(); err != nil {
		t.Fatal(err)
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

	if err := vg.Update(); err != nil {
		t.Fatal(err)
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

	if err := vg.Update(); err != nil {
		t.Fatal(err)
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

	if err := vg.Update(); err != nil {
		t.Fatal(err)
	}
	_, err = pool.FindVolume("test1")
	if err != command.ErrNotFound {
		t.Error("unexpected error: ", err)
	}

	// thin snapshots validation

	// create sourceVolume
	count = 0
	res, err = lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "sourceVol",
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
	if res.GetVolume().GetName() != "sourceVol" {
		t.Errorf(`res.Volume.Name != "sourceVol": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeGb() != 1 {
		t.Errorf(`res.Volume.SizeGb != 1: %d`, res.GetVolume().GetSizeGb())
	}
	err = exec.Command("lvs", vg.Name()+"/sourceVol").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}

	if err := vg.Update(); err != nil {
		t.Fatal(err)
	}

	lv, err = pool.FindVolume("sourceVol")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Tags()[0] != "testtag1" {
		t.Errorf(`testtag1 not present on volume`)
	}
	if lv.Tags()[1] != "testtag2" {
		t.Errorf(`testtag1 not present on volume`)
	}

	// create snapshot of sourceVol

	var snapRes *proto.CreateLVSnapshotResponse
	snapRes, err = lvService.CreateLVSnapshot(context.Background(), &proto.CreateLVSnapshotRequest{
		Name:         "snap1",
		DeviceClass:  thindev,
		SourceVolume: "sourceVol",
		AccessType:   "ro",
		Tags:         []string{"testsnaptag1", "testsnaptag2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("is not notified: %d", count)
	}
	if snapRes.GetSnapshot().GetName() != "snap1" {
		t.Errorf(`res.Volume.Name != "snap1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeGb() != 1 {
		t.Errorf(`res.Volume.SizeGb != 1: %d`, res.GetVolume().GetSizeGb())
	}
	err = exec.Command("lvs", vg.Name()+"/snap1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}

	if err := vg.Update(); err != nil {
		t.Fatal(err)
	}

	lv, err = pool.FindVolume("snap1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Tags()[0] != "testsnaptag1" {
		t.Errorf(`testsnaptag1 not present on snapshot`)
	}
	if lv.Tags()[1] != "testsnaptag2" {
		t.Errorf(`testsnaptag1 not present on snapshot`)
	}

	// restore the created snapshot to a new logical volume.

	snapRes, err = lvService.CreateLVSnapshot(context.Background(), &proto.CreateLVSnapshotRequest{
		Name:         "restoredsnap1",
		DeviceClass:  thindev,
		SourceVolume: "sourceVol",
		AccessType:   "rw",
		Tags:         []string{"testrestoretag1", "testrestoretag2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("is not notified: %d", count)
	}
	if snapRes.GetSnapshot().GetName() != "restoredsnap1" {
		t.Errorf(`res.Volume.Name != "restoredsnap1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeGb() != 1 {
		t.Errorf(`res.Volume.SizeGb != 1: %d`, res.GetVolume().GetSizeGb())
	}
	err = exec.Command("lvs", vg.Name()+"/restoredsnap1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}

	if err := vg.Update(); err != nil {
		t.Fatal(err)
	}

	lv, err = pool.FindVolume("restoredsnap1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Tags()[0] != "testrestoretag1" {
		t.Errorf(`testsnaptag1 not present on snapshot`)
	}
	if lv.Tags()[1] != "testrestoretag2" {
		t.Errorf(`testsnaptag1 not present on snapshot`)
	}
}
