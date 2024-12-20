package command

import (
	"context"
	"errors"
	"path"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/internal/lvmd/testutils"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestVG_CreateVolume(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	vg, _ := setupVG(ctx, t)

	t.Run("create volume with multiple of Sector Size is fine", func(t *testing.T) {
		err := vg.CreateVolume(ctx, "test1", uint64(topolvm.MinimumSectorSize), []string{"tag"}, 0, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		vol, err := vg.FindVolume(ctx, "test1")
		if err != nil {
			t.Fatal(err)
		}
		if vol.Size()%uint64(topolvm.MinimumSectorSize) != 0 {
			t.Fatalf("expected size to be multiple of sector size %d, got %d", uint64(topolvm.MinimumSectorSize), vol.Size())
		}
		if err := vg.RemoveVolume(ctx, "test1"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("create volume with size not multiple of sector Size to get rejected", func(t *testing.T) {
		err := vg.CreateVolume(ctx, "test1", uint64(topolvm.MinimumSectorSize)+1, []string{"tag"}, 0, "", nil)
		if !errors.Is(err, ErrNoMultipleOfSectorSize) {
			t.Fatalf("expected error to be %v, got %v", ErrNoMultipleOfSectorSize, err)
		}
	})
}

func TestLogicalVolume_IsThin(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	vg, loop := setupVG(ctx, t)

	t.Run("A cached volume should not classified as thin volume.", func(t *testing.T) {
		// create cached LV
		err := vg.CreateVolume(ctx, "test1", uint64(topolvm.MinimumSectorSize), []string{"tag"}, 0, "", []string{"--type", "writecache", "--cachesize", "10M", "--cachedevice", loop})
		if err != nil {
			t.Fatal(err)
		}
		vol, err := vg.FindVolume(ctx, "test1")
		if err != nil {
			t.Fatal(err)
		}
		if vol.IsThin() {
			t.Fatal("Expected test1 to not be thin (eval lv_attr instead of pool)")
		}
		if vol.attr[0] != byte(VolumeTypeCached) {
			t.Fatal("Created a LV but without writecache?")
		}
	})
}

func setupVG(ctx context.Context, t *testing.T) (*VolumeGroup, string) {
	testutils.RequireRoot(t)

	vgName := t.Name()
	file := path.Join(t.TempDir(), vgName)
	loop, err := testutils.MakeLoopbackDevice(ctx, file)
	if err != nil {
		t.Fatal(err)
	}

	err = testutils.MakeLoopbackVG(ctx, vgName, loop)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = testutils.CleanLoopbackVG(vgName, []string{loop}, []string{file}) })

	vg, err := FindVolumeGroup(ctx, vgName)
	if err != nil {
		t.Fatal(err)
	}

	return vg, loop
}
