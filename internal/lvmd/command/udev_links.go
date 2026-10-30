package command

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// devDir and sysDir are where the host's /dev and /sys are seen. Tests replace them.
var (
	devDir = "/dev"
	sysDir = "/sys"
)

// AnnounceToUdev makes the kernel send a "change" event for the volume's
// device, as "udevadm trigger --action=change" does.
//
// When systemd-udevd kills the worker for a device's event, it also forgets
// the device, so it can no longer remove the device's /dev links when the
// volume is removed. The new event lets udev learn the device again. It does
// nothing for an inactive volume, which has no device.
func (l *LogicalVolume) AnnounceToUdev() error {
	if l.devMajor == 0 {
		return nil
	}
	uevent := filepath.Join(sysDir, "dev", "block", fmt.Sprintf("%d:%d", l.devMajor, l.devMinor), "uevent")
	f, err := os.OpenFile(uevent, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.Write([]byte("change"))
	return errors.Join(err, f.Close())
}

// removeStaleLinks deletes the /dev links of a removed volume that udev left
// behind, which happens when udev forgot the device before the volume was
// removed. A link is deleted only if the device it points to is gone or now
// has another name or UUID. It returns the deleted links.
func (vg *VolumeGroup) removeStaleLinks(name string) ([]string, error) {
	dmName := dmEscape(vg.Name()) + "-" + dmEscape(name)
	links := map[string]string{
		filepath.Join(devDir, vg.Name(), name):                    dmName,
		filepath.Join(devDir, "mapper", dmName):                   dmName,
		filepath.Join(devDir, "disk", "by-id", "dm-name-"+dmName): dmName,
	}
	var uuidLinks []string
	if vg.state.uuid != "" {
		// The UUID link of a volume is dm-uuid-LVM-<VG UUID><LV UUID> without
		// dashes. The LV UUID went with the volume, so check the VG's links.
		pattern := filepath.Join(devDir, "disk", "by-id", "dm-uuid-LVM-"+strings.ReplaceAll(vg.state.uuid, "-", "")+"*")
		uuidLinks, _ = filepath.Glob(pattern)
	}

	var removed []string
	var errs []error
	remove := func(link, attr, want string) {
		stale, err := linkIsStale(link, attr, want)
		if err == nil && stale {
			err = os.Remove(link)
			if err == nil {
				removed = append(removed, link)
			}
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	for link, want := range links {
		remove(link, "name", want)
	}
	for _, link := range uuidLinks {
		remove(link, "uuid", strings.TrimPrefix(filepath.Base(link), "dm-uuid-"))
	}
	return removed, errors.Join(errs...)
}

// linkIsStale reports whether link is a symlink to a device whose
// device-mapper attribute, "name" or "uuid", is not want, or to a device that
// is gone.
func linkIsStale(link, attr, want string) (bool, error) {
	fi, err := os.Lstat(link)
	if err != nil || fi.Mode()&fs.ModeSymlink == 0 {
		return false, err
	}
	target, err := os.Readlink(link)
	if err != nil {
		return false, err
	}
	have, err := os.ReadFile(filepath.Join(sysDir, "class", "block", filepath.Base(target), "dm", attr))
	if errors.Is(err, fs.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(have)) != want, nil
}

// dmEscape escapes a VG or LV name the way device-mapper names do.
func dmEscape(s string) string {
	return strings.ReplaceAll(s, "-", "--")
}
