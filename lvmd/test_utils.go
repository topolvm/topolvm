package lvmd

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"github.com/cybozu-go/log"
)

func MakeLoopbackDevice(name string) (string, error) {
	command := exec.Command("losetup", "-f")
	command.Stderr = os.Stderr
	loop := bytes.Buffer{}
	command.Stdout = &loop
	err := command.Run()
	if err != nil {
		return "", err
	}
	loopDev := strings.TrimRight(loop.String(), "\n")
	out, err := exec.Command("truncate", "--size=4G", name).CombinedOutput()
	if err != nil {
		log.Error("failed to truncate", map[string]interface{}{
			"output": string(out),
		})
		return "", err
	}
	out, err = exec.Command("losetup", loopDev, name).CombinedOutput()
	if err != nil {
		log.Error("failed to losetup", map[string]interface{}{
			"output": string(out),
		})
		return "", err
	}
	return loopDev, nil
}

// MakeLoopbackVG creates a VG made from loopback device by losetup
func MakeLoopbackVG(name string, devices ...string) error {
	args := append([]string{name}, devices...)
	out, err := exec.Command("vgcreate", args...).CombinedOutput()
	if err != nil {
		log.Error("failed to vgcreate", map[string]interface{}{
			"output": string(out),
		})
		return err
	}
	return nil
}

// CleanLoopbackVG deletes a VG made by MakeLoopbackVG
func CleanLoopbackVG(name string, loops []string, files []string) error {
	err := exec.Command("vgremove", "-f", name).Run()
	if err != nil {
		return err
	}

	for _, loop := range loops {
		err = exec.Command("losetup", "-d", loop).Run()
		if err != nil {
			return err
		}
	}

	for _, file := range files {
		err = os.Remove(file)
		if err != nil {
			return err
		}
	}

	return nil
}
