package filesystem

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	blkidCmd = "/sbin/blkid"
)

type temporaryer interface {
	Temporary() bool
}

func isSameDevice(dev1, dev2 string) (bool, error) {
	if dev1 == dev2 {
		return true, nil
	}

	var st1, st2 unix.Stat_t
	if err := Stat(dev1, &st1); err != nil {
		return false, fmt.Errorf("stat failed for %s: %v", dev1, err)
	}
	if err := Stat(dev2, &st2); err != nil {
		return false, fmt.Errorf("stat failed for %s: %v", dev2, err)
	}

	return st1.Rdev == st2.Rdev, nil
}

// isMounted returns true if device is mounted on target.
// The implementation uses /proc/mounts because some filesystem uses a virtual device.
func isMounted(device, target string) (bool, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	target, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return false, err
	}

	data, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return false, fmt.Errorf("could not read /proc/mounts: %v", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		d, err := filepath.EvalSymlinks(fields[1])
		if err != nil {
			return false, err
		}
		if d == target {
			return isSameDevice(device, fields[0])
		}
	}

	return false, nil
}

// Mount mounts a block device onto target with filesystem-specific opts.
// target directory must exist.
func Mount(device, target, fsType, opts string, readonly bool) error {
	switch mounted, err := isMounted(device, target); {
	case err != nil:
		return err
	case mounted:
		return nil
	}

	var flg uintptr = unix.MS_LAZYTIME
	if readonly {
		flg |= unix.MS_RDONLY
	}

	// Handle EINTR signal for Go 1.14+
	for {
		err := unix.Mount(device, target, fsType, flg, opts)
		if err == nil {
			return nil
		}
		if e, ok := err.(temporaryer); ok && e.Temporary() {
			continue
		}
		return err
	}
}

// Unmount unmounts the device if it is mounted.
func Unmount(device, target string) error {
	for i := 0; i < 10; i++ {
		switch mounted, err := isMounted(device, target); {
		case err != nil:
			return err
		case !mounted:
			return nil
		}

		// Handle EINTR signal for Go 1.14+
		var err error
		for {
			err = unix.Unmount(target, unix.UMOUNT_NOFOLLOW)
			if err == nil {
				break
			}
			if e, ok := err.(temporaryer); ok && e.Temporary() {
				continue
			}
			break
		}

		switch err {
		case nil, unix.EBUSY:
			// umount(2) can lie that it could have unmounted, so recheck.
			time.Sleep(500 * time.Millisecond)
		default:
			return err
		}
	}
	return unix.EBUSY
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

// Stat wrapped a golang.org/x/sys/unix.Stat function to handle EINTR signal for Go 1.14+
func Stat(path string, stat *unix.Stat_t) error {
	for {
		err := unix.Stat(path, stat)
		if err == nil {
			return nil
		}
		if e, ok := err.(temporaryer); ok && e.Temporary() {
			continue
		}
		return err
	}
}

// Mknod wrapped a golang.org/x/sys/unix.Mknod function to handle EINTR signal for Go 1.14+
func Mknod(path string, mode uint32, dev int) (err error) {
	for {
		err := unix.Mknod(path, mode, dev)
		if err == nil {
			return nil
		}
		if e, ok := err.(temporaryer); ok && e.Temporary() {
			continue
		}
		return err
	}
}

// Statfs wrapped a golang.org/x/sys/unix.Statfs function to handle EINTR signal for Go 1.14+
func Statfs(path string, buf *unix.Statfs_t) (err error) {
	for {
		err := unix.Statfs(path, buf)
		if err == nil {
			return nil
		}
		if e, ok := err.(temporaryer); ok && e.Temporary() {
			continue
		}
		return err
	}
}
