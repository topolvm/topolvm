package command

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// fakeDevSys points devDir and sysDir at empty temporary directories.
func fakeDevSys(t *testing.T) (dev, sys string) {
	t.Helper()
	dev, sys = t.TempDir(), t.TempDir()
	origDev, origSys := devDir, sysDir
	devDir, sysDir = dev, sys
	t.Cleanup(func() { devDir, sysDir = origDev, origSys })
	return dev, sys
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func symlink(t *testing.T, path, target string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRemoveStaleLinks(t *testing.T) {
	dev, sys := fakeDevSys(t)

	// dm-1 now belongs to another volume, good-lv; dm-9 is gone.
	writeFile(t, filepath.Join(sys, "class/block/dm-1/dm/name"), "vg--1-good--lv\n")
	writeFile(t, filepath.Join(sys, "class/block/dm-1/dm/uuid"), "LVM-VGUUID0000GOODLV\n")
	vg := &VolumeGroup{state: vg{name: "vg-1", uuid: "VGUUID-0000"}}

	// Links that udev left behind for the removed volume old-lv.
	stale := []string{
		symlink(t, filepath.Join(dev, "vg-1/old-lv"), "../dm-1"),
		symlink(t, filepath.Join(dev, "mapper/vg--1-old--lv"), "../dm-9"),
		symlink(t, filepath.Join(dev, "disk/by-id/dm-name-vg--1-old--lv"), "../../dm-9"),
		symlink(t, filepath.Join(dev, "disk/by-id/dm-uuid-LVM-VGUUID0000OLDLV"), "../../dm-1"),
	}
	// Links that are still right, or belong to another VG.
	kept := []string{
		symlink(t, filepath.Join(dev, "vg-1/good-lv"), "../dm-1"),
		symlink(t, filepath.Join(dev, "mapper/vg--1-good--lv"), "../dm-1"),
		symlink(t, filepath.Join(dev, "disk/by-id/dm-uuid-LVM-VGUUID0000GOODLV"), "../../dm-1"),
		symlink(t, filepath.Join(dev, "disk/by-id/dm-uuid-LVM-OTHERVG00OLDLV"), "../../dm-9"),
	}

	removed, err := vg.removeStaleLinks("old-lv")
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(removed)
	slices.Sort(stale)
	if !slices.Equal(removed, stale) {
		t.Errorf("removed %q, want %q", removed, stale)
	}
	for _, link := range stale {
		if _, err := os.Lstat(link); !os.IsNotExist(err) {
			t.Errorf("stale link %s still exists", link)
		}
	}
	for _, link := range kept {
		if _, err := os.Lstat(link); err != nil {
			t.Errorf("link %s should have been kept: %v", link, err)
		}
	}

	// Nothing is left to do the second time.
	if removed, err := vg.removeStaleLinks("old-lv"); err != nil || len(removed) != 0 {
		t.Errorf("second call: removed %q, err %v", removed, err)
	}
}

func TestAnnounceToUdev(t *testing.T) {
	_, sys := fakeDevSys(t)
	uevent := filepath.Join(sys, "dev/block/253:4/uevent")
	writeFile(t, uevent, "")

	if err := (&LogicalVolume{devMajor: 253, devMinor: 4}).AnnounceToUdev(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(uevent); string(got) != "change" {
		t.Errorf("uevent got %q, want %q", got, "change")
	}

	// An inactive volume has no device to announce.
	if err := (&LogicalVolume{}).AnnounceToUdev(); err != nil {
		t.Errorf("inactive volume: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sys, "dev/block/0:0")); !os.IsNotExist(err) {
		t.Error("expected nothing to be written for an inactive volume")
	}
}
