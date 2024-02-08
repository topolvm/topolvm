package driver

import (
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
