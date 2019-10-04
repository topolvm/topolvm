package filesystem

import (
	"fmt"
	"os/exec"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	cmdBtrfs       = "/bin/btrfs"
	cmdMkfsBtrfs   = "/bin/mkfs.btrfs"
	btrfsMountOpts = "ssd"
)

var btrfsLogger = logf.Log.WithName("filesystem").WithName("btrfs")

type btrfs struct {
	device string
}

func init() {
	fsTypeMap["btrfs"] = func(device string) Filesystem {
		return btrfs{device}
	}
}

func (fs btrfs) Exists() bool {
	oldCmd, err := exec.LookPath("btrfs-debug-tree")
	if err == nil {
		return exec.Command(oldCmd, fs.device).Run() == nil
	}
	return exec.Command(cmdBtrfs, "inspect-internal", "dump-tree", fs.device).Run() == nil
}

func (fs btrfs) Mkfs() error {
	fsType, err := DetectFilesystem(fs.device)
	if err != nil {
		return err
	}
	if fsType != "" {
		return ErrFilesystemExists
	}
	if fs.Exists() {
		return ErrFilesystemExists
	}

	out, err := exec.Command(cmdMkfsBtrfs, "-f", "-q", fs.device).CombinedOutput()
	if err != nil {
		btrfsLogger.Error(err, "btrfs: failed to create",
			"device", fs.device,
			"output", string(out))
	}

	btrfsLogger.Info("btrfs: created", "device", fs.device)
	return nil
}

func (fs btrfs) Mount(target string, readonly bool) error {
	return Mount(fs.device, target, "btrfs", btrfsMountOpts, readonly)
}

func (fs btrfs) Unmount(target string) error {
	return Unmount(fs.device, target)
}

func (fs btrfs) Resize(target string) error {
	out, err := exec.Command(cmdBtrfs, "filesystem", "resize", "max", target).CombinedOutput()
	if err != nil {
		out := string(out)
		btrfsLogger.Error(err, "failed to resize btrfs filesystem",
			"device", fs.device,
			"directory", target,
			"output", out)
		return fmt.Errorf("failed to resize btrfs filesystem: device=%s, directory=%s, err=%v, output=%s",
			fs.device, target, err, out)
	}

	return nil
}
