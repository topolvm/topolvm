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
	vgService.GetLVList(context.Background(), &proto.Empty{})

	// _, err := vg.CreateVolume("test1", 1<<30)

}
