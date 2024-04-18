/*
 * kernelHasMountinfoBug() and the constants only used in this function are copied
 * from the following code:
 * https://github.com/kubernetes/mount-utils/blob/6f4aae5a6ab58574cac605cdd48bf5c0862c047f/mount_helper_unix.go#L211-L242
 *    LICENSE: http://www.apache.org/licenses/LICENSE-2.0
 *    Copyright The Kubernetes Authors.
 */

package filesystem

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	blkidCmd = "/sbin/blkid"
)

type temporaryer interface {
	Temporary() bool
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
