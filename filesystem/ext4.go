package filesystem

import (
	"fmt"
	"os/exec"

	"github.com/cybozu-go/log"
)

const (
	cmdDumpe2fs   = "/sbin/dumpe2fs"
	cmdMkfsExt4   = "/sbin/mkfs.ext4"
	cmdResize2fs  = "/sbin/resize2fs"
	ext4MountOpts = ""
)

type ext4 struct {
	device string
}

func init() {
	fsTypeMap["ext4"] = func(device string) Filesystem {
		return ext4{device}
	}
}

func (fs ext4) Exists() bool {
	return exec.Command(cmdDumpe2fs, "-h", fs.device).Run() == nil
}

func (fs ext4) Mkfs() error {
	fsType, err := DetectFilesystem(fs.device)
	if err != nil {
		return err
	}
	if fsType != "" {
		return ErrFilesystemExists
	}
	if err := exec.Command(cmdDumpe2fs, "-h", fs.device).Run(); err == nil {
		return ErrFilesystemExists
	}

	out, err := exec.Command(cmdMkfsExt4, "-F", "-q", "-m", "0", fs.device).CombinedOutput()
	if err != nil {
		log.Error("ext4: failed to create", map[string]interface{}{
			"device":    fs.device,
			log.FnError: err,
			"output":    string(out),
		})
	}

	log.Info("ext4: created", map[string]interface{}{
		"device": fs.device,
	})
	return nil
}

func (fs ext4) Mount(target string, readonly bool) error {
	return Mount(fs.device, target, "ext4", ext4MountOpts, readonly)
}

func (fs ext4) Unmount(target string) error {
	return Unmount(fs.device, target)
}

func (fs ext4) Resize(_ string) error {
	out, err := exec.Command(cmdResize2fs, fs.device).CombinedOutput()
	if err != nil {
		out := string(out)
		log.Error("failed to resize ext4 filesystem", map[string]interface{}{
			"device":    fs.device,
			log.FnError: err,
			"output":    out,
		})
		return fmt.Errorf("failed to resize ext4 filesystem: device=%s, err=%v, output=%s",
			fs.device, err, out)
	}

	return nil
}
