package driver

import (
	"context"
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

func TestNodeGetVolumeStatsWithoutVolumeHealth(t *testing.T) {
	// k8sLVService is left nil so that looking up the logical volume would panic.
	s := &nodeServerNoLocked{volumeHealth: false}

	res, err := s.NodeGetVolumeStats(context.Background(), &csi.NodeGetVolumeStatsRequest{
		VolumeId:   "test",
		VolumePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("NodeGetVolumeStats should succeed: %v", err)
	}
	if len(res.GetUsage()) == 0 {
		t.Fatal("usage should be reported")
	}
	if res.GetVolumeCondition() != nil {
		t.Fatalf("volume condition should not be reported: %v", res.GetVolumeCondition())
	}
}

func TestNodeGetCapabilitiesAdvertisesVolumeConditionOnlyWithVolumeHealth(t *testing.T) {
	for _, volumeHealth := range []bool{true, false} {
		s := &nodeServerNoLocked{volumeHealth: volumeHealth}

		res, err := s.NodeGetCapabilities(context.Background(), &csi.NodeGetCapabilitiesRequest{})
		if err != nil {
			t.Fatalf("NodeGetCapabilities should succeed: %v", err)
		}

		advertised := map[csi.NodeServiceCapability_RPC_Type]bool{}
		for _, capability := range res.GetCapabilities() {
			advertised[capability.GetRpc().GetType()] = true
		}

		for _, expected := range []csi.NodeServiceCapability_RPC_Type{
			csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
			csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
		} {
			if !advertised[expected] {
				t.Errorf("%v should be advertised when volumeHealth is %v", expected, volumeHealth)
			}
		}

		if got := advertised[csi.NodeServiceCapability_RPC_VOLUME_CONDITION]; got != volumeHealth {
			t.Errorf("VOLUME_CONDITION should be advertised only when volumeHealth is true, but it was %v when volumeHealth is %v", got, volumeHealth)
		}

	}
}
