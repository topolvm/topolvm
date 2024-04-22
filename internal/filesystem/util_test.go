package filesystem

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func createDevice() (string, error) {
	f, err := os.CreateTemp("", "test-filesystem-")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
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

func TestDetectFilesystem(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("run as root")
	}

	dev, err := createDevice()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = exec.Command("losetup", "-d", dev).Run() }()

	fs, err := DetectFilesystem(dev)
	if err != nil {
		t.Error(err)
	}
	if fs != "" {
		t.Error("fs is not empty", fs)
	}

	err = exec.Command("mkfs.ext4", "-q", dev).Run()
	if err != nil {
		t.Fatal(err)
	}

	fs, err = DetectFilesystem(dev)
	if err != nil {
		t.Error(err)
	}
	if fs != "ext4" {
		t.Error("fs is not ext4", fs)
	}
}
