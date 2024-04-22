/*
 * kernelHasMountinfoBug() and the constants only used in this function are copied
 * from the following code:
 * https://github.com/kubernetes/mount-utils/blob/6f4aae5a6ab58574cac605cdd48bf5c0862c047f/mount_helper_unix.go#L211-L242
 *    LICENSE: http://www.apache.org/licenses/LICENSE-2.0
 *    Copyright The Kubernetes Authors.
 */

package filesystem

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/sys/unix"
	"k8s.io/utils/io"
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
		// Some filesystems like tmpfs and nfs aren't backed by block device files.
		// In such case, given device path does not exist,
		// we regard it is not an error but is false.
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat failed for %s: %v", dev1, err)
	}
	if err := Stat(dev2, &st2); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat failed for %s: %v", dev2, err)
	}

	return st1.Rdev == st2.Rdev, nil
}

// IsMounted returns true if device is mounted on target.
// The implementation uses /proc/mounts because some filesystem uses a virtual device.
func IsMounted(device, target string) (bool, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return false, err
	}
	target, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return false, err
	}

	data, err := readMountInfo("/proc/mounts")
	if err != nil {
		return false, fmt.Errorf("could not read /proc/mounts: %v", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// If the filesystem is nfs and its connection is broken, EvalSymlinks will be stuck.
		// So it should be in before calling EvalSymlinks.
		ok, err := isSameDevice(device, fields[0])
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		d, err := filepath.EvalSymlinks(fields[1])
		if err != nil {
			return false, err
		}
		if d == target {
			return true, nil
		}
	}

	return false, nil
}

// DetectFilesystem returns filesystem type if device has a filesystem.
// This returns an empty string if no filesystem exists.
func DetectFilesystem(device string) (string, error) {
	f, err := os.Open(device)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	// synchronizes dirty data
	err = f.Sync()
	if err != nil {
		return "", err
	}

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

// These variables are used solely by kernelHasMountinfoBug.
var (
	hasMountinfoBug        bool
	checkMountinfoBugOnce  sync.Once
	maxConsistentReadTimes = 3
)

// kernelHasMountinfoBug checks if the kernel bug that can lead to incomplete
// mountinfo being read is fixed. It does so by checking the kernel version.
//
// The bug was fixed by the kernel commit 9f6c61f96f2d97 (since Linux 5.8).
// Alas, there is no better way to check if the bug is fixed other than to
// rely on the kernel version returned by uname.
//
// Copied from
// https://github.com/kubernetes/mount-utils/blob/6f4aae5a6ab58574cac605cdd48bf5c0862c047f/mount_helper_unix.go#L204C1-L250
// *    LICENSE: http://www.apache.org/licenses/LICENSE-2.0
// *    Copyright The Kubernetes Authors.
func kernelHasMountinfoBug() bool {
	checkMountinfoBugOnce.Do(func() {
		// Assume old kernel.
		hasMountinfoBug = true

		uname := unix.Utsname{}
		err := unix.Uname(&uname)
		if err != nil {
			return
		}

		end := bytes.IndexByte(uname.Release[:], 0)
		v := bytes.SplitN(uname.Release[:end], []byte{'.'}, 3)
		if len(v) != 3 {
			return
		}
		major, _ := strconv.Atoi(string(v[0]))
		minor, _ := strconv.Atoi(string(v[1]))

		if major > 5 || (major == 5 && minor >= 8) {
			hasMountinfoBug = false
		}
	})

	return hasMountinfoBug
}

func readMountInfo(path string) ([]byte, error) {
	if kernelHasMountinfoBug() {
		return io.ConsistentRead(path, maxConsistentReadTimes)
	}

	return os.ReadFile(path)
}
