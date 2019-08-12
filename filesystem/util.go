package filesystem

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cybozu-go/log"
	"golang.org/x/sys/unix"
)

const (
	blkidCmd = "/sbin/blkid"
)

// MountedDir returns directory where device is mounted.
// It returns ErrNotMounted if device is not mounted.
func MountedDir(device string) (string, error) {
	p, err := filepath.EvalSymlinks(device)
	if err != nil {
		return "", err
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return "", err
	}

	data, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return "", fmt.Errorf("reading /proc/mounts failed: %v", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[0] != p {
			continue
		}
		return fields[1], nil
	}

	return "", ErrNotMounted
}

// Mount mounts a block device onto target with filesystem-specific opts.
func Mount(device, target, fsType, opts string, readonly bool) error {
	switch d, err := MountedDir(device); err {
	case nil:
		if d == target {
			return nil
		}
		log.Error("device is mounted on another directory", map[string]interface{}{
			"device":  device,
			"target":  target,
			"mounted": d,
		})
		return errors.New("device is mounted on another directory")
	case ErrNotMounted:
	default:
		return err
	}

	var flg uintptr = unix.MS_LAZYTIME
	if readonly {
		flg |= unix.MS_RDONLY
	}
	if err := unix.Mount(device, target, fsType, flg, opts); err != nil {
		return err
	}

	// sometimes mount(2) seems asynchronous
	for i := 0; i < 10; i++ {
		if _, err := MountedDir(device); err == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	data, _ := ioutil.ReadFile("/proc/mounts")
	return fmt.Errorf("mounted device does not appear in /proc/mounts: data=%s, device=%s, target=%s",
		string(data), device, target)
}

// Unmount unmounts the device if it is mounted.
func Unmount(device string) error {
	d, err := MountedDir(device)
	switch err {
	case ErrNotMounted:
		return nil
	case nil:
	default:
		return err
	}

	if err := unix.Unmount(d, unix.UMOUNT_NOFOLLOW); err != nil {
		return err
	}

	// sometimes umount(2) seems asynchronous
	for i := 0; i < 10; i++ {
		switch _, err := MountedDir(device); err {
		case nil:
		case ErrNotMounted:
			return nil
		default:
			log.Warn("unexpected error during Unmount", map[string]interface{}{
				log.FnError: err,
			})
		}
		time.Sleep(1 * time.Second)
	}
	return errors.New("unmounted device does not disappear from /proc/mounts")
}

// DetectFilesystem returns filesystem type if device has a filesystem.
// This returns an empty string if no filesystem exists.
func DetectFilesystem(device string) (string, error) {
	f, err := os.Open(device)
	if err != nil {
		return "", err
	}
	// synchronizes dirty data
	f.Sync()
	f.Close()

	out, err := exec.Command(blkidCmd, "-c", "/dev/null", "-o", "export", device).CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// blkid exists with status 2 when anything can be found
			if exitErr.ExitCode() == 2 {
				return "", nil
			}
		}
		return "", fmt.Errorf("blkid failed: output=%s, device=%s, error=%v", string(out), device, err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "TYPE=") {
			return line[5:], nil
		}
	}

	return "", nil
}
