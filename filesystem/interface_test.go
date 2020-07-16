package filesystem

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func createDevice() (string, error) {
	f, err := ioutil.TempFile("", "test-filesystem-")
	if err != nil {
		return "", err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	if err := f.Truncate(1 << 30); err != nil {
		return "", err
	}

	out, err := exec.Command("losetup", "-f", "--show", f.Name()).Output()
	if err != nil {
		return "", err
	}

	// for resize test
	if err := f.Truncate(2 << 30); err != nil {
		return "", err
	}

	loopDev := strings.TrimSpace(string(out))
	return loopDev, nil
}

func TestInterface(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("run as root")
	}

	for fsType := range fsTypeMap {
		t.Run(fsType, func(t *testing.T) {
			testFilesystem(t, fsType)
		})
	}

	t.Run("unsupported", testUnsupportedFilesystem)
}

func testFilesystem(t *testing.T, fsType string) {
	f, err := ioutil.TempFile("", "test-nonblock")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	if _, err := New(fsType, f.Name()); err != ErrNonBlockDevice {
		t.Error(`err != ErrNonBlockDevice`, err)
	}

	dev, err := createDevice()
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Command("losetup", "-d", dev).Run()

	fs, err := New(fsType, dev)
	if err != nil {
		t.Fatal(err)
	}

	if fs.Exists() {
		t.Error("empty device should not have any filesystem")
	}
	if err := fs.Mkfs(); err != nil {
		t.Fatal(err)
	}
	if !fs.Exists() {
		t.Error("device should have a filesystem")
	}
	if err := fs.Mkfs(); err != ErrFilesystemExists {
		t.Error(`err != ErrFilesystemExists`, err)
	}

	d, err := ioutil.TempDir("", "test-mount")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(d)

	if err := fs.Unmount(d); err != nil {
		t.Error(err)
	}

	if err := fs.Mount(d, false); err != nil {
		t.Fatal(err)
	}

	if err := exec.Command("mountpoint", "-q", d).Run(); err != nil {
		t.Error(err)
	}
	if err := fs.Mount(d, false); err != nil {
		t.Error("mount on the same directory should succeed", err)
	}

	// file write test
	g, err := os.Create(filepath.Join(d, "test"))
	if err != nil {
		t.Error(err)
	}
	g.Close()

	// resize test
	var stfs unix.Statfs_t
	if err := unix.Statfs(d, &stfs); err != nil {
		t.Fatal(err)
	}
	origSize := stfs.Blocks * uint64(stfs.Frsize)
	if err := exec.Command("losetup", "-c", dev).Run(); err != nil {
		t.Fatal(err)
	}
	if err := fs.Resize(d); err != nil {
		t.Fatal(err)
	}
	if err := unix.Statfs(d, &stfs); err != nil {
		t.Fatal(err)
	}
	newSize := stfs.Blocks * uint64(stfs.Frsize)
	if newSize <= origSize {
		t.Error("filesystem has not been resized")
	}
	t.Log("newSize:", newSize)

	if err := fs.Unmount(d); err != nil {
		t.Error(err)
	}

	// read-only mount
	if err := fs.Mount(d, true); err != nil {
		t.Fatal(err)
	}
	_, err = os.Create(filepath.Join(d, "test2"))
	if err == nil {
		t.Error("os.Create should fail")
	}
	if err := fs.Unmount(d); err != nil {
		t.Error(err)
	}
}

func testUnsupportedFilesystem(t *testing.T) {
	dev, err := createDevice()
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Command("losetup", "-d", dev).Run()

	if _, err := New("unsupported", dev); err != ErrUnsupportedFilesystem {
		t.Error(`err != ErrUnsupportedFilesystem`, err)
	}
}
