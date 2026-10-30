package driver

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	mountutil "k8s.io/mount-utils"
)

func TestMakeMountOptions(t *testing.T) {
	_, err := makeMountOptions(true, &csi.VolumeCapability_MountVolume{
		MountFlags: []string{"rw"},
	})
	if err == nil {
		t.Fatalf("err should happen")
	}
}

func TestNodePublishBlockVolumeIdempotent(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		name := "read-write"
		if readOnly {
			name = "read-only"
		}
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "block")
			mounter := mountutil.NewFakeMounter(nil)
			s := &nodeServerNoLocked{mounter: mountutil.SafeFormatAndMount{Interface: mounter}}
			req := &csi.NodePublishVolumeRequest{VolumeId: "volume", TargetPath: target, Readonly: readOnly}
			lv := &proto.LogicalVolume{Path: "/dev/mapper/volume"}
			if err := s.nodePublishBlockVolume(req, lv); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				if err := s.nodePublishBlockVolume(req, lv); err != nil {
					t.Fatal(err)
				}
			}
			if len(mounter.GetLog()) != 1 || len(mounter.MountPoints) != 1 {
				t.Fatalf("repeated publish stacked bind mounts: log=%v, mounts=%v", mounter.GetLog(), mounter.MountPoints)
			}
			mount := mounter.MountPoints[0]
			if mount.Device != lv.GetPath() || !slices.Contains(mount.Opts, "bind") || slices.Contains(mount.Opts, "ro") != readOnly {
				t.Fatalf("unexpected block mount: %+v", mount)
			}
		})
	}
}

func TestNodePublishBlockVolumeMountCheckFailure(t *testing.T) {
	target := filepath.Join(t.TempDir(), "block")
	mounter := mountutil.NewFakeMounter(nil)
	mounter.MountCheckErrors = map[string]error{target: errors.New("mount check failed")}
	s := &nodeServerNoLocked{mounter: mountutil.SafeFormatAndMount{Interface: mounter}}
	err := s.nodePublishBlockVolume(&csi.NodePublishVolumeRequest{VolumeId: "volume", TargetPath: target},
		&proto.LogicalVolume{Path: "/dev/mapper/volume"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("expected Internal for mount check failure, got %v", err)
	}
	if len(mounter.GetLog()) != 0 {
		t.Fatalf("mount attempted after failed check: %v", mounter.GetLog())
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("mount check failure touched target: %v", err)
	}
}
