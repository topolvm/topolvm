package filesystem

import (
	"fmt"
	"os/exec"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	cmdXFSAdmin  = "/usr/sbin/xfs_admin"
	cmdMkfsXfs   = "/sbin/mkfs.xfs"
	cmdXFSGrowFS = "/usr/sbin/xfs_growfs"
	xfsMountOpts = "wsync"
)

var xfsLogger = ctrl.Log.WithName("filesystem").WithName("xfs")

type xfs struct {
	device string
}

func init() {
	fsTypeMap["xfs"] = func(device string) Filesystem {
		return xfs{device}
	}
}

func (fs xfs) Exists() bool {
	return exec.Command(cmdXFSAdmin, "-l", fs.device).Run() == nil
}

func (fs xfs) Mkfs() error {
	fsType, err := DetectFilesystem(fs.device)
	if err != nil {
		return err
	}
	if fsType != "" {
		return ErrFilesystemExists
	}
	if err := exec.Command(cmdXFSAdmin, "-l", fs.device).Run(); err == nil {
		return ErrFilesystemExists
	}

	out, err := exec.Command(cmdMkfsXfs, "-f", "-q", fs.device).CombinedOutput()
	if err != nil {
		xfsLogger.Error(err, "xfs: failed to create",
			"device", fs.device,
			"output", string(out))
	}

	xfsLogger.Info("xfs: created", "device", fs.device)
	return nil
}

func (fs xfs) Mount(target string, readonly bool) error {
	return Mount(fs.device, target, "xfs", xfsMountOpts, readonly)
}

func (fs xfs) Unmount(target string) error {
	return Unmount(fs.device, target)
}

func (fs xfs) Resize(target string) error {
	out, err := exec.Command(cmdXFSGrowFS, target).CombinedOutput()
	if err != nil {
		out := string(out)
		xfsLogger.Error(err, "failed to resize xfs filesystem",
			"device", fs.device,
			"directory", target,
			"output", out)
		return fmt.Errorf("failed to resize xfs filesystem: device=%s, directory=%s, err=%v, output=%s",
			fs.device, target, err, out)
	}

	return nil
}
