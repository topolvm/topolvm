package driver

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cybozu-go/topolvm/lvmd"
	"github.com/cybozu-go/well"

	"github.com/cybozu-go/topolvm/lvmd/command"
)

func TestNode(t *testing.T) {
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

	vgName := "test_node"
	loop, err := lvmd.MakeLoopbackVG(vgName)
	if err != nil {
		t.Fatal(err)
	}
	defer lvmd.CleanLoopbackVG(loop, vgName)

	_, err = command.FindVolumeGroup(vgName)
	if err != nil {
		t.Fatal(err)
	}

	lvName := "ext4lv"
	devicePath := filepath.Join("/dev/"+vgName, lvName)
	fsType := "ext4"
	err = command.CallLVM("lvcreate", "-n", lvName, "-L", "1G", vgName)
	if err != nil {
		t.Fatalf("lvcreate failed for %s: %v", lvName, err)
	}
	_, err = well.CommandContext(context.Background(), mkfsCmd, "-t", fsType, devicePath).CombinedOutput()
	if err != nil {
		t.Errorf("mkfs failed for %s, %s, %v", devicePath, fsType, err)
	}
	dft, err := detectFsType(context.Background(), devicePath)
	if err != nil {
		t.Errorf("detect file system type failed. err: %v", err)
	}
	if dft != fsType {
		t.Errorf("detect file system type failed. expected: %s, but actual: %s", fsType, dft)
	}

	lvName = "unformattedlv"
	devicePath = filepath.Join("/dev/"+vgName, lvName)
	err = command.CallLVM("lvcreate", "-n", lvName, "-L", "1G", vgName)
	if err != nil {
		t.Fatalf("lvcreate failed for %s: %v", lvName, err)
	}
	dft, err = detectFsType(context.Background(), devicePath)
	if err != nil {
		t.Errorf("detect file system type failed. err: %v", err)
	}
	if dft != "" {
		t.Errorf("detect file system type is not empty: %s", dft)
	}

	dft, err = detectFsType(context.Background(), "/dev/null")
	if err == nil {
		t.Errorf("detect file system type unexpectedly succeeds")
	}
	if dft != "" {
		t.Errorf("detect file system type is not empty: %s", dft)
	}
}
