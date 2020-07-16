package filesystem

import (
	"os"
	"os/exec"
	"testing"
)

func TestDetectFilesystem(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("run as root")
	}

	dev, err := createDevice()
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Command("losetup", "-d", dev).Run()

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
