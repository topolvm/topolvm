package lvmd

import (
	"context"
	"os"
	"testing"

	"github.com/cybozu-go/topolvm/lvmd/command"
	"github.com/cybozu-go/topolvm/lvmd/proto"
)

func TestVGService(t *testing.T) {
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

	vgName := "test_vgservice"
	loop, err := makeVG(vgName)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanVG(loop, vgName)

	vg, err := command.FindVolumeGroup(vgName)
	if err != nil {
		t.Fatal(err)
	}
	vgService := NewVGService(vg)
	res, err := vgService.GetLVList(context.Background(), &proto.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	numVols1 := len(res.GetVolumes())
	if numVols1 != 0 {
		t.Errorf("numVolumes must be 0: %d", numVols1)
	}

	_, err = vg.CreateVolume("test1", 1<<30)
	if err != nil {
		t.Fatal(err)
	}

	res, err = vgService.GetLVList(context.Background(), &proto.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	numVols2 := len(res.GetVolumes())
	if numVols2 != 1 {
		t.Fatalf("numVolumes must be 1: %d", numVols2)
	}

	vol := res.GetVolumes()[0]
	if vol.GetName() != "test1" {
		t.Errorf(`Volume.Name != "test1": %s`, vol.GetName())
	}
	if vol.GetSizeGb() != 1 {
		t.Errorf(`Volume.SizeGb != 1: %d`, vol.GetSizeGb())
	}

	res2, err := vgService.GetFreeBytes(context.Background(), &proto.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	freeBytes, err := vg.Free()
	if err != nil {
		t.Fatal(err)
	}
	if res2.GetFreeBytes() != freeBytes {
		t.Errorf("Free bytes mismatch: %d", res2.GetFreeBytes())
	}
}
