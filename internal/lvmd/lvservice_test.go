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

//func TestLVService_MountLV(t *testing.T) {
//	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
//	lvService, _, _, _ := setupLVService(ctx, t)
//
//	// Create a test logical volume
//	//res, err := lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
//	//	Name:        "test_mount",
//	//	DeviceClass: lvServiceTestThickDC,
//	//	SizeBytes:   100 << 20, // 100 MiB
//	//})
//	//if err != nil {
//	//	t.Fatal(err)
//	//}
//	//if res.GetVolume().GetName() != "test_mount" {
//	//	t.Errorf(`res.Volume.Name != "test_mount": %s`, res.GetVolume().GetName())
//	//}
//
//	// Create a temporary mount point
//	mountPoint := path.Join(t.TempDir(), "test_mount_point")
//
//	//// Test mounting with default ext4 filesystem
//	//t.Run("MountWithDefaultFS", func(t *testing.T) {
//	//	mountRes, err := lvService.MountLV(context.Background(), &proto.MountLVRequest{
//	//		Name:        "test_mount",
//	//		DeviceClass: lvServiceTestThickDC,
//	//		TargetPath:  mountPoint,
//	//	})
//	//	if err != nil {
//	//		t.Fatalf("failed to mount LV: %v", err)
//	//	}
//	//	if mountRes.GetDevicePath() == "" {
//	//		t.Error("device path is empty")
//	//	}
//	//
//	//	// Verify mount point exists
//	//	cmd := exec.Command("mountpoint", "-q", mountPoint)
//	//	if err := cmd.Run(); err != nil {
//	//		t.Error("mount point verification failed")
//	//	}
//	//
//	//	// Verify filesystem type
//	//	cmd = exec.Command("findmnt", "-n", "-o", "FSTYPE", mountPoint)
//	//	output, err := cmd.Output()
//	//	if err != nil {
//	//		t.Fatalf("failed to check filesystem: %v", err)
//	//	}
//	//	fsType := string(output[:len(output)-1]) // Remove trailing newline
//	//	if fsType != "ext4" {
//	//		t.Errorf("expected ext4, got %s", fsType)
//	//	}
//	//
//	//	// Verify mount is read-write
//	//	cmd = exec.Command("findmnt", "-n", "-o", "OPTIONS", mountPoint)
//	//	output, err = cmd.Output()
//	//	if err != nil {
//	//		t.Fatalf("failed to check mount options: %v", err)
//	//	}
//	//	options := string(output)
//	//	if !contains(options, "rw") {
//	//		t.Errorf("mount should be read-write, got options: %s", options)
//	//	}
//	//
//	//	// Test idempotency - mount again should succeed
//	//	_, err = lvService.MountLV(context.Background(), &proto.MountLVRequest{
//	//		Name:        "test_mount",
//	//		DeviceClass: lvServiceTestThickDC,
//	//		TargetPath:  mountPoint,
//	//	})
//	//	if err != nil {
//	//		t.Errorf("remounting should be idempotent: %v", err)
//	//	}
//	//
//	//	// Cleanup: unmount
//	//	exec.Command("umount", mountPoint).Run()
//	//})
//
//	// Test mounting with XFS filesystem
//	t.Run("MountWithXFS", func(t *testing.T) {
//		// Create another LV for XFS test
//		_, err := lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
//			Name:        "test_mount_xfs",
//			DeviceClass: lvServiceTestThickDC,
//			SizeBytes:   1 << 30, // 1 GiB
//		})
//		if err != nil {
//			t.Fatal(err)
//		}
//
//		xfsMountPoint := path.Join(t.TempDir(), "test_xfs_mount")
//
//		mountRes, err := lvService.MountLV(context.Background(), &proto.MountLVRequest{
//			Name:        "test_mount_xfs",
//			DeviceClass: lvServiceTestThickDC,
//			TargetPath:  xfsMountPoint,
//			FsType:      "xfs",
//		})
//		if err != nil {
//			t.Fatalf("failed to mount XFS LV: %v", err)
//		}
//		if mountRes.GetDevicePath() == "" {
//			t.Error("device path is empty")
//		}
//
//		// Verify XFS filesystem
//		cmd := exec.Command("findmnt", "-n", "-o", "FSTYPE", xfsMountPoint)
//		output, err := cmd.Output()
//		if err != nil {
//			t.Fatalf("failed to check filesystem: %v", err)
//		}
//		fsType := string(output[:len(output)-1])
//		if fsType != "xfs" {
//			t.Errorf("expected xfs, got %s", fsType)
//		}
//
//		// Verify nouuid option is present for XFS
//		cmd = exec.Command("findmnt", "-n", "-o", "OPTIONS", xfsMountPoint)
//		output, err = cmd.Output()
//		if err != nil {
//			t.Fatalf("failed to check mount options: %v", err)
//		}
//		options := string(output)
//		if !contains(options, "nouuid") {
//			t.Errorf("XFS should have nouuid option, got: %s", options)
//		}
//
//		// Cleanup
//		exec.Command("umount", xfsMountPoint).Run()
//	})
//
//	//// Test mounting with custom mount options
//	//t.Run("MountWithCustomOptions", func(t *testing.T) {
//	//	// Unmount if still mounted
//	//	exec.Command("umount", mountPoint).Run()
//	//
//	//	mountRes, err := lvService.MountLV(context.Background(), &proto.MountLVRequest{
//	//		Name:         "test_mount",
//	//		DeviceClass:  lvServiceTestThickDC,
//	//		TargetPath:   mountPoint,
//	//		FsType:       "ext4",
//	//		MountOptions: []string{"noatime", "nodiratime"},
//	//	})
//	//	if err != nil {
//	//		t.Fatalf("failed to mount with custom options: %v", err)
//	//	}
//	//	if mountRes.GetDevicePath() == "" {
//	//		t.Error("device path is empty")
//	//	}
//	//
//	//	// Verify custom options
//	//	cmd := exec.Command("findmnt", "-n", "-o", "OPTIONS", mountPoint)
//	//	output, err := cmd.Output()
//	//	if err != nil {
//	//		t.Fatalf("failed to check mount options: %v", err)
//	//	}
//	//	options := string(output)
//	//	if !contains(options, "noatime") {
//	//		t.Errorf("mount should have noatime option, got: %s", options)
//	//	}
//	//
//	//	// Cleanup
//	//	exec.Command("umount", mountPoint).Run()
//	//})
//	//
//	//// Test mounting in read-only mode
//	//t.Run("MountReadOnly", func(t *testing.T) {
//	//	roMountPoint := path.Join(t.TempDir(), "test_ro_mount")
//	//
//	//	mountRes, err := lvService.MountLV(context.Background(), &proto.MountLVRequest{
//	//		Name:         "test_mount",
//	//		DeviceClass:  lvServiceTestThickDC,
//	//		TargetPath:   roMountPoint,
//	//		FsType:       "ext4",
//	//		MountOptions: []string{"ro"},
//	//	})
//	//	if err != nil {
//	//		t.Fatalf("failed to mount in read-only mode: %v", err)
//	//	}
//	//	if mountRes.GetDevicePath() == "" {
//	//		t.Error("device path is empty")
//	//	}
//	//
//	//	// Verify mount is read-only
//	//	cmd := exec.Command("findmnt", "-n", "-o", "OPTIONS", roMountPoint)
//	//	output, err := cmd.Output()
//	//	if err != nil {
//	//		t.Fatalf("failed to check mount options: %v", err)
//	//	}
//	//	options := string(output)
//	//	if !contains(options, "ro") {
//	//		t.Errorf("mount should be read-only, got options: %s", options)
//	//	}
//	//
//	//	// Cleanup
//	//	exec.Command("umount", roMountPoint).Run()
//	//})
//	//
//	//// Test error: non-existent LV
//	//t.Run("MountNonExistentLV", func(t *testing.T) {
//	//	_, err := lvService.MountLV(context.Background(), &proto.MountLVRequest{
//	//		Name:        "nonexistent",
//	//		DeviceClass: lvServiceTestThickDC,
//	//		TargetPath:  "/tmp/nonexistent",
//	//	})
//	//	if err == nil {
//	//		t.Error("expected error for non-existent LV")
//	//	}
//	//	code := status.Code(err)
//	//	if code != codes.NotFound {
//	//		t.Errorf("expected NotFound error, got: %s", code)
//	//	}
//	//})
//	//
//	//// Test error: empty target path
//	//t.Run("MountEmptyTargetPath", func(t *testing.T) {
//	//	_, err := lvService.MountLV(context.Background(), &proto.MountLVRequest{
//	//		Name:        "test_mount",
//	//		DeviceClass: lvServiceTestThickDC,
//	//		TargetPath:  "",
//	//	})
//	//	if err == nil {
//	//		t.Error("expected error for empty target path")
//	//	}
//	//	code := status.Code(err)
//	//	if code != codes.InvalidArgument {
//	//		t.Errorf("expected InvalidArgument error, got: %s", code)
//	//	}
//	//})
//
//	// Cleanup: remove LVs
//	t.Cleanup(func() {
//		exec.Command("umount", mountPoint).Run()
//		_, _ = lvService.RemoveLV(context.Background(), &proto.RemoveLVRequest{
//			Name:        "test_mount",
//			DeviceClass: lvServiceTestThickDC,
//		})
//		_, _ = lvService.RemoveLV(context.Background(), &proto.RemoveLVRequest{
//			Name:        "test_mount_xfs",
//			DeviceClass: lvServiceTestThickDC,
//		})
//	})
//}
//
//func contains(s, substr string) bool {
//	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
//		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
//			len(s) > len(substr)+1 && s[1:len(substr)+1] == substr))
//}
