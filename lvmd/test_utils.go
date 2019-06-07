package lvmd

import (
	"os"
	"os/exec"
	"strings"
)

// MakeLoopbackVG creates a VG made from loopback device by losetup
func MakeLoopbackVG(name string) (string, error) {
	loop, err := exec.Command("losetup", "-f").Output()
	if err != nil {
		return "", err
	}
	loopDev := strings.TrimRight(string(loop), "\n")
	err = exec.Command("truncate", "--size=3G", name).Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("losetup", loopDev, name).Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("vgcreate", name, loopDev).Run()
	if err != nil {
		return "", err
	}
	return loopDev, nil
}

// CleanLoopbackVG deletes a VG made by MakeLoopbackVG
func CleanLoopbackVG(loop, name string) error {
	err := exec.Command("vgremove", "-f", name).Run()
	if err != nil {
		return err
	}
	err = exec.Command("losetup", "-d", loop).Run()
	if err != nil {
		return err
	}
	return os.Remove(name)
}
