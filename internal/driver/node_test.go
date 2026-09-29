package driver

import (
	"context"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
)

func TestMakeMountOptions(t *testing.T) {
	_, err := makeMountOptions(true, &csi.VolumeCapability_MountVolume{
		MountFlags: []string{"rw"},
	})
	if err == nil {
		t.Fatalf("err should happen")
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
