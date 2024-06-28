package testutils

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

func MakeLoopbackDevice(ctx context.Context, name string) (string, error) {
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
		log.FromContext(ctx).Error(err, "failed truncate", "output", string(out))
		return "", err
	}
	out, err = exec.Command("losetup", loopDev, name).CombinedOutput()
	if err != nil {
		log.FromContext(ctx).Error(err, "failed losetup", "output", string(out))
		return "", err
	}
	return loopDev, nil
}

// MakeLoopbackVG creates a VG made from loopback device by losetup
func MakeLoopbackVG(ctx context.Context, name string, devices ...string) error {
	args := append([]string{name}, devices...)
	out, err := exec.Command("vgcreate", args...).CombinedOutput()
	if err != nil {
		log.FromContext(ctx).Error(err, "failed vgcreate", "output", string(out))
		return err
	}
	return nil
}

func MakeLoopbackLV(ctx context.Context, name string, vg string) error {
	args := []string{"-L1G", "-y", "-n", name, vg}
	out, err := exec.Command("lvcreate", args...).CombinedOutput()
	if err != nil {
		log.FromContext(ctx).Error(err, "failed lvcreate", "output", string(out))
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
