package command

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/topolvm/topolvm"
	"github.com/topolvm/topolvm/internal/lvmd/testutils"
	ctrl "sigs.k8s.io/controller-runtime"
)

func TestVG_CreateVolume(t *testing.T) {
	ctx := ctrl.LoggerInto(context.Background(), testr.New(t))
	vgName := "lvm_command_test"
	loop, err := testutils.MakeLoopbackDevice(ctx, vgName)
	if err != nil {
		t.Fatal(err)
	}

	err = testutils.MakeLoopbackVG(ctx, vgName, loop)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = testutils.CleanLoopbackVG(vgName, []string{loop}, []string{vgName}) }()

	vg, err := FindVolumeGroup(ctx, vgName)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("create volume with multiple of Sector Size is fine", func(t *testing.T) {
		err = vg.CreateVolume(ctx, "test1", uint64(topolvm.MinimumSectorSize), []string{"tag"}, 0, "", nil)
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
		err = vg.CreateVolume(ctx, "test1", uint64(topolvm.MinimumSectorSize)+1, []string{"tag"}, 0, "", nil)
		if !errors.Is(err, ErrNoMultipleOfSectorSize) {
			t.Fatalf("expected error to be %v, got %v", ErrNoMultipleOfSectorSize, err)
		}
	})

	t.Run("create volume with stripe is fine", func(t *testing.T) {
		err := vg.CreateVolume(ctx, "test2", 1<<30, nil, 2, "4k", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = vg.FindVolume(ctx, "test2")
		if err != nil {
			t.Fatal(err)
		}

		err = vg.CreateVolume(ctx, "test3", 1<<30, nil, 2, "4M", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = vg.FindVolume(ctx, "test3")
		if err != nil {
			t.Fatal(err)
		}
	})
}
