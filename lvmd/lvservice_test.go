package lvmd

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func makeVG(name string) (string, error) {
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
	if res.GetVolume().GetName() != "test1" {
		t.Errorf(`res.Volume.Name != "test1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeGb() != 1 {
		t.Errorf(`res.Volume.SizeGb != 1: %d`, res.GetVolume().GetSizeGb())
	}
	err = exec.Command("lvs", vg.Name() + "/test1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}

	_, err = lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:	"test2",
		SizeGb:	3,
	})
	code := status.Code(err)
	if code != codes.ResourceExhausted {
		t.Errorf(`code is not codes.ResouceExhausted: %s`, code)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:	"test1",
		SizeGb:	2,
	})
	if  err != nil {
		t.Fatal(err)
	}
	lv, err := vg.FindVolume("test1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Size() != (2 << 30) {
		t.Errorf(`does not match size 2: %d`, lv.Size() >> 30)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:	"test1",
		SizeGb:	5,
	})
	code = status.Code(err)
	if code != codes.ResourceExhausted {
		t.Errorf(`code is not codes.ResouceExhausted: %s`, code)
	}

	_, err = lvService.RemoveLV(context.Background(), &proto.RemoveLVRequest{
		Name:	"test1",
	})
	if err != nil {
		t.Error(err)
	}
	_, err = vg.FindVolume("test1")
	if err != command.ErrNotFound {
		t.Error("unexpected error: ", err)
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
