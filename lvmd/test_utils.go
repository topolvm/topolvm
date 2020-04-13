package lvmd

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"github.com/cybozu-go/log"
)

// MakeLoopbackVG creates a VG made from loopback device by losetup
func MakeLoopbackVG(name string) (string, error) {
	command := exec.Command("losetup", "-f")
	command.Stderr = os.Stderr
	loop := bytes.Buffer{}
	command.Stdout = &loop
	err := command.Run()
	if err != nil {
		return "", err
	}
	loopDev := strings.TrimRight(loop.String(), "\n")
	err = exec.Command("truncate", "--size=4G", name).Run()
	if err != nil {
		log.Error("failed to truncate", nil)
		return "", err
	}
	err = exec.Command("losetup", loopDev, name).Run()
	if err != nil {
		log.Error("failed to losetup", nil)
		return "", err
	}
	err = exec.Command("vgcreate", name, loopDev).Run()
	if err != nil {
		log.Error("failed to vgcreate", nil)
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
