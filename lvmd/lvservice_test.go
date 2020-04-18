package lvmd

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

	vgName := "test_lvservice"
	loop, err := MakeLoopbackVG(vgName)
	if err != nil {
		t.Fatal(err)
	}
	defer CleanLoopbackVG(loop, vgName)

	vg, err := command.FindVolumeGroup(vgName)
	if err != nil {
		t.Fatal(err)
	}

	var count int
	notifier := func() {
		count++
	}
	lvService := NewLVService("", notifier)
	res, err := lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:   "test1",
		VgName: vgName,
		SizeGb: 1,
		Tags:   []string{"testtag1", "testtag2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("is not notified: %d", count)
	}
	if res.GetVolume().GetName() != "test1" {
		t.Errorf(`res.Volume.Name != "test1": %s`, res.GetVolume().GetName())
	}
	if res.GetVolume().GetSizeGb() != 1 {
		t.Errorf(`res.Volume.SizeGb != 1: %d`, res.GetVolume().GetSizeGb())
	}
	err = exec.Command("lvs", vg.Name()+"/test1").Run()
	if err != nil {
		t.Error("failed to create logical volume")
	}
	lv, err := vg.FindVolume("test1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Tags()[0] != "testtag1" {
		t.Errorf(`testtag1 not present on volume`)
	}
	if lv.Tags()[1] != "testtag2" {
		t.Errorf(`testtag1 not present on volume`)
	}

	_, err = lvService.CreateLV(context.Background(), &proto.CreateLVRequest{
		Name:   "test2",
		VgName: vgName,
		SizeGb: 3,
	})
	code := status.Code(err)
	if code != codes.ResourceExhausted {
		t.Errorf(`code is not codes.ResouceExhausted: %s`, code)
	}
	if count != 1 {
		t.Errorf("unexpected count: %d", count)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:   "test1",
		VgName: vgName,
		SizeGb: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("unexpected count: %d", count)
	}
	lv, err = vg.FindVolume("test1")
	if err != nil {
		t.Fatal(err)
	}
	if lv.Size() != (2 << 30) {
		t.Errorf(`does not match size 2: %d`, lv.Size()>>30)
	}

	_, err = lvService.ResizeLV(context.Background(), &proto.ResizeLVRequest{
		Name:   "test1",
		VgName: vgName,
		SizeGb: 5,
	})
	code = status.Code(err)
	if code != codes.ResourceExhausted {
		t.Errorf(`code is not codes.ResouceExhausted: %s`, code)
	}
	if count != 2 {
		t.Errorf("unexpected count: %d", count)
	}

	_, err = lvService.RemoveLV(context.Background(), &proto.RemoveLVRequest{
		Name:   "test1",
		VgName: vgName,
	})
	if err != nil {
		t.Error(err)
	}
	if count != 3 {
		t.Errorf("unexpected count: %d", count)
	}
	_, err = vg.FindVolume("test1")
	if err != command.ErrNotFound {
		t.Error("unexpected error: ", err)
	}

}
