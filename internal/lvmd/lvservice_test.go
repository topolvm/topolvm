package lvmd

import (
	"context"
	"errors"
	"os/exec"
	"path"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/topolvm/topolvm/internal/lvmd/command"
	"github.com/topolvm/topolvm/internal/lvmd/testutils"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	lvServiceTestVGName   = "test_lvservice"
	lvServiceTestPoolName = "test_lvservice_pool"
	lvServiceTestThickDC  = lvServiceTestVGName
	lvServiceTestThinDC   = lvServiceTestPoolName
	lvServiceTestTag1     = "testtag1"
	lvServiceTestTag2     = "testtag2"
)

func setupLVService(ctx context.Context, t *testing.T) (
	proto.LVServiceServer,
	*int,
	*command.VolumeGroup,
	*command.ThinPool,
) {
	testutils.RequireRoot(t)

	file := path.Join(t.TempDir(), t.Name())

	loop, err := testutils.MakeLoopbackDevice(ctx, file)
	if err != nil {
		t.Fatal(err)
	}

	err = testutils.MakeLoopbackVG(ctx, lvServiceTestVGName, loop)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = testutils.CleanLoopbackVG(lvServiceTestVGName, []string{loop}, []string{file})
	})

	vg, err := command.FindVolumeGroup(ctx, lvServiceTestVGName)
	if err != nil {
		t.Fatal(err)
	}

	count := 0
	notifier := func() {
		count++
	}

	// thinpool details
	overprovisionRatio := float64(10.0)
	poolName := "test_pool"
	poolSize := uint64(1 << 30)
	pool, err := vg.CreatePool(ctx, poolName, poolSize)
	if err != nil {
		t.Fatal(err)
	}

	lvService := NewLVService(
		NewDeviceClassManager(
			[]*lvmdTypes.DeviceClass{
				{
					// volumegroup target
					Name:        lvServiceTestThickDC,
					VolumeGroup: vg.Name(),
				},
				{
					// thinpool target
					Name:        lvServiceTestThinDC,
					VolumeGroup: vg.Name(),
					Type:        lvmdTypes.TypeThin,
					ThinPoolConfig: &lvmdTypes.ThinPoolConfig{
						Name:               poolName,
						OverprovisionRatio: overprovisionRatio,
					},
				},
			},
		),
		NewLvcreateOptionClassManager([]*lvmdTypes.LvcreateOptionClass{}),
		notifier,
	)

	return lvService, &count, vg, pool
}

func TestLVService_ThickLV(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	lvService, count, vg, _ := setupLVService(ctx, t)

	res, err := lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "test1",
		DeviceClass: lvServiceTestThickDC,
		SizeBytes:   1 << 30, // 1 GiB
		Tags:        []string{lvServiceTestTag1, lvServiceTestTag2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 1 {
		t.Errorf("is not notified: %d", count)
	}
	if res.GetVolume().GetName() != "test1" {
		t.Errorf(`res.Volume.Name != "test1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeBytes() != 1<<30 {
		t.Errorf(`res.Volume.SizeBytes != %d: %d`, 1<<30, res.GetVolume().GetSizeBytes())
	}
	err = exec.Command("lvs", vg.Name()+"/test1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}

	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}

	lv, err := vg.FindVolume(ctx, "test1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Tags()[0] != lvServiceTestTag1 {
		t.Errorf(`testtag1 not present on volume`)
	}
	if lv.Tags()[1] != lvServiceTestTag2 {
		t.Errorf(`testtag1 not present on volume`)
	}

	_, err = lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "test2",
		DeviceClass: lvServiceTestThickDC,
		SizeBytes:   3 << 30, // 3 GiB
	})
	code := status.Code(err)
	if code != codes.ResourceExhausted {
		t.Errorf(`code is not codes.ResouceExhausted: %s`, code)
	}
	if *count != 1 {
		t.Errorf("unexpected count: %d", count)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:        "test1",
		DeviceClass: lvServiceTestThickDC,
		SizeBytes:   2 << 30, // 2 GiB
	})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 2 {
		t.Errorf("unexpected count: %d", count)
	}

	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}

	lv, err = vg.FindVolume(ctx, "test1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Size() != (2 << 30) {
		t.Errorf(`does not match size 2: %d`, lv.Size()>>30)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:        "test1",
		DeviceClass: lvServiceTestThickDC,
		SizeBytes:   5 << 30, // 5 GiB
	})
	code = status.Code(err)
	if code != codes.ResourceExhausted {
		t.Errorf(`code is not codes.ResouceExhausted: %s`, code)
	}
	if *count != 2 {
		t.Errorf("unexpected count: %d", count)
	}

	_, err = lvService.RemoveLV(context.Background(), &proto.RemoveLVRequest{
		Name:        "test1",
		DeviceClass: lvServiceTestThickDC,
	})
	if err != nil {
		t.Error(err)
	}
	if *count != 3 {
		t.Errorf("unexpected count: %d", count)
	}

	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = vg.FindVolume(ctx, "test1")
	if !errors.Is(err, command.ErrNotFound) {
		t.Error("unexpected error: ", err)
	}
}

func TestLVService_ThinLV(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	lvService, count, vg, pool := setupLVService(ctx, t)

	res, err := lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "testp1",
		DeviceClass: lvServiceTestThinDC,
		SizeBytes:   1 << 30, // 1 GiB
		Tags:        []string{lvServiceTestTag1, lvServiceTestTag2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 1 {
		t.Errorf("is not notified: %d", count)
	}
	if res.GetVolume().GetName() != "testp1" {
		t.Errorf(`res.Volume.Name != "testp1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeBytes() != 1<<30 {
		t.Errorf(`res.Volume.SizeBytes != %d: %d`, 1<<30, res.GetVolume().GetSizeBytes())
	}
	err = exec.Command("lvs", vg.Name()+"/testp1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}

	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}

	lv, err := pool.FindVolume(ctx, "testp1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Tags()[0] != lvServiceTestTag1 {
		t.Errorf(`testtag1 not present on volume`)
	}
	if lv.Tags()[1] != lvServiceTestTag2 {
		t.Errorf(`testtag1 not present on volume`)
	}

	// overprovision should work
	_, err = lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "testp2",
		DeviceClass: lvServiceTestThinDC,
		SizeBytes:   3 << 30, // 3 GiB
	})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 2 {
		t.Errorf("unexpected count: %d", count)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:        "testp1",
		DeviceClass: lvServiceTestThinDC,
		SizeBytes:   2 << 30, // 2 GiB
	})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 3 {
		t.Errorf("unexpected count: %d", count)
	}

	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}

	lv, err = pool.FindVolume(ctx, "testp1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Size() != (2 << 30) {
		t.Errorf(`does not match size 2: %d`, lv.Size()>>30)
	}

	_, err = lvService.RemoveLV(context.Background(), &proto.RemoveLVRequest{
		Name:        "testp1",
		DeviceClass: lvServiceTestThinDC,
	})
	if err != nil {
		t.Error(err)
	}
	if *count != 4 {
		t.Errorf("unexpected count: %d", count)
	}

	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = pool.FindVolume(ctx, "test1")
	if !errors.Is(err, command.ErrNotFound) {
		t.Error("unexpected error: ", err)
	}

}

func TestLVService_ThinSnapshots(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	lvService, count, vg, pool := setupLVService(ctx, t)

	// create sourceVolume
	var originalSizeBytes int64 = 1 << 30 // 1 GiB
	res, err := lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:        "sourceVol",
		DeviceClass: lvServiceTestThinDC,
		SizeBytes:   originalSizeBytes,
		Tags:        []string{lvServiceTestTag1, lvServiceTestTag2},
	})
	if err != nil {
		t.Fatal(err)
	}

	// create snapshot of sourceVol
	var snapRes *proto.CreateLVSnapshotResponse
	var snapshotDesiredSizeBytes int64 = 2 << 30 // 2 GiB
	snapRes, err = lvService.CreateLVSnapshot(context.Background(), &proto.CreateLVSnapshotRequest{
		Name:         "snap1",
		DeviceClass:  lvServiceTestThinDC,
		SourceVolume: "sourceVol",
		// use a bigger size here to also simulate resizing to a bigger target than source
		SizeBytes:  snapshotDesiredSizeBytes,
		AccessType: "ro",
		Tags:       []string{"testsnaptag1", "testsnaptag2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 2 {
		t.Errorf("is not notified: %d", count)
	}
	if snapRes.GetSnapshot().GetName() != "snap1" {
		t.Errorf(`res.Volume.Name != "snap1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeBytes() != originalSizeBytes {
		t.Errorf(`res.Volume.SizeBytes != %d: %d`, originalSizeBytes, res.GetVolume().GetSizeBytes())
	}
	if snapRes.GetSnapshot().GetSizeBytes() != snapshotDesiredSizeBytes {
		t.Errorf(`snapRes.ThinSnapshot.SizeBytes != %d: %d`, snapshotDesiredSizeBytes, snapRes.GetSnapshot().GetSizeBytes())
	}
	err = exec.Command("lvs", vg.Name()+"/snap1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}

	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}

	lv, err := pool.FindVolume(ctx, "snap1")
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
		DeviceClass:  lvServiceTestThinDC,
		SourceVolume: snapRes.GetSnapshot().GetName(),
		AccessType:   "rw",
		Tags:         []string{"testrestoretag1", "testrestoretag2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 3 {
		t.Errorf("is not notified: %d", count)
	}
	if snapRes.GetSnapshot().GetName() != "restoredsnap1" {
		t.Errorf(`res.Volume.Name != "restoredsnap1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeBytes() != originalSizeBytes {
		t.Errorf(`res.Volume.SizeBytes != %d: %d`, originalSizeBytes, res.GetVolume().GetSizeBytes())
	}
	if snapRes.GetSnapshot().GetSizeBytes() != snapshotDesiredSizeBytes {
		t.Errorf(`snapRes.ThinSnapshot.SizeBytes != %d: %d`, snapshotDesiredSizeBytes, snapRes.GetSnapshot().GetSizeBytes())
	}
	err = exec.Command("lvs", vg.Name()+"/restoredsnap1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}

	if err := vg.Update(ctx); err != nil {
		t.Fatal(err)
	}

	lv, err = pool.FindVolume(ctx, "restoredsnap1")
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
