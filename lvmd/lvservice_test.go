package lvmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
)

func makeVG(name string) (string, error) {
	loop, err := exec.Command("losetup", "-f").Output()
	if err != nil {
		return "", err
	}
	loopDev := strings.TrimRight(string(loop), "\n")
	fmt.Println("loop: " + loopDev)
	err = exec.Command("truncate", "--size=3G", name).Run()
	if err != nil {
		return "", err
	}
	err = exec.Command("losetup", loopDev, name).Run()
	if err != nil {
		fmt.Println("failed to losetup: " + loopDev + "," + name)
		return "", err
	}
	err = exec.Command("vgcreate", name, loopDev).Run()
	if err != nil {
		fmt.Print("failed to vgcreate")
		return "", err
	}
	return loopDev, nil
}

func cleanVG(loop, name string) error {
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

func testCreateLV(t *testing.T, vg *command.VolumeGroup) {
	lvService := NewLVService(vg)
	res, err := lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:   "test1",
		SizeGb: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Volume.Name != "test1" {
		t.Errorf(`res.Volume.Name != "test1": %s`, res.Volume.Name)
	}
	if res.Volume.SizeGb != 1 {
		t.Errorf(`res.Volume.SizeGb != 1: %d`, res.Volume.SizeGb)
	}
}

func TestLVService(t *testing.T) {
	uid := os.Getuid()
	if uid != 0 {
		t.Skip("run as root")
	}
	circleci := os.Getenv("CIRCLECI") == "true"
	if circleci {
		executorType := os.Getenv("CIRCLECI_EXECUTOR")
		if executorType != "machine" {
			t.Skip("run on machine executor")
		}
	}

	vgName := "test_lvservice_vg"
	loop, err := makeVG(vgName)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanVG(loop, vgName)

	vg, err := command.FindVolumeGroup(vgName)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("CreateLV", func(t *testing.T) {
		testCreateLV(t, vg)
	})
}
