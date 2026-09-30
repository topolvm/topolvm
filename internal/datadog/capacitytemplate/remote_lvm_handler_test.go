package capacitytemplate

import (
	"context"
	"math"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func nodeGroupWithVolumes(volumes ...interface{}) *unstructured.Unstructured {
	ng := newNodeGroup()
	ng.Object["spec"] = map[string]interface{}{
		"storage": map[string]interface{}{"remoteVolumes": volumes},
	}
	return ng
}

func remoteVolume(name, mode, size string) map[string]interface{} {
	return map[string]interface{}{"name": name, "provisioningMode": mode, "size": size}
}

func remoteVolumeWithLayout(name, mode, size string, diskCount int64, raidLevel string) map[string]interface{} {
	volume := remoteVolume(name, mode, size)
	volume["diskCount"] = diskCount
	volume["raidLevel"] = raidLevel
	return volume
}

// TestRemoteLVMHandlerCalculateCapacityNGCGoldenCases mirrors the
// GetRemoteLVMCapacity matrix from the k8s-nodegroups remote TopoLVM capacity
// implementation.
func TestRemoteLVMHandlerCalculateCapacityNGCGoldenCases(t *testing.T) {
	tests := []struct {
		name      string
		ng        *unstructured.Unstructured
		spareGB   int64
		want      int64
		wantNil   bool
		wantError string
	}{
		{name: "absent storage", ng: newNodeGroup(), wantNil: true},
		{name: "empty remote volumes", ng: nodeGroupWithVolumes(), wantNil: true},
		{
			name: "static and zfs ignored",
			ng: nodeGroupWithVolumes(
				remoteVolume("static", "static", "10Gi"),
				remoteVolume("zfs", "zfs", "20Gi"),
			),
			wantNil: true,
		},
		{
			name: "single LVM disk",
			ng:   nodeGroupWithVolumes(remoteVolume("data", "lvm", "100Gi")),
			want: 100*(1<<30) - 8*(1<<20),
		},
		{
			name: "multiple LVM disks deduct overhead per entry",
			ng: nodeGroupWithVolumes(
				remoteVolume("one", "lvm", "100Gi"),
				remoteVolume("two", "lvm", "50Gi"),
				remoteVolume("ignored", "zfs", "1Ti"),
			),
			want: 150*(1<<30) - 2*8*(1<<20),
		},
		{
			name: "RAID0 members aggregate into one PV",
			ng: nodeGroupWithVolumes(
				remoteVolumeWithLayout("raid", "lvm", "100Gi", 2, "raid0"),
			),
			want: 200*(1<<30) - 8*(1<<20),
		},
		{
			name: "non-RAID members each produce a PV",
			ng: nodeGroupWithVolumes(
				remoteVolumeWithLayout("linear", "lvm", "50Gi", 3, "none"),
			),
			want: 150*(1<<30) - 3*8*(1<<20),
		},
		{
			name: "mixed RAID layouts share the remote VG",
			ng: nodeGroupWithVolumes(
				remoteVolumeWithLayout("raid", "lvm", "100Gi", 2, "raid0"),
				remoteVolumeWithLayout("linear", "lvm", "50Gi", 3, "none"),
				remoteVolumeWithLayout("ignored", "zfs", "1Ti", 4, "raid0"),
			),
			want: 350*(1<<30) - 4*8*(1<<20),
		},
		{
			name: "cloud size rounds up to GiB for every member",
			ng: nodeGroupWithVolumes(
				remoteVolumeWithLayout("data", "lvm", "1537Mi", 2, "raid0"),
			),
			want: 4*(1<<30) - 8*(1<<20),
		},
		{
			name: "cloud size rounds up to GiB",
			ng:   nodeGroupWithVolumes(remoteVolume("data", "lvm", "1537Mi")),
			want: 2*(1<<30) - 8*(1<<20),
		},
		{
			name: "reserve deducted once",
			ng: nodeGroupWithVolumes(
				remoteVolume("one", "lvm", "10Gi"),
				remoteVolume("two", "lvm", "10Gi"),
			),
			spareGB: 3,
			want:    17*(1<<30) - 2*8*(1<<20),
		},
		{name: "malformed quantity", ng: nodeGroupWithVolumes(remoteVolume("data", "lvm", "nope")), wantError: "invalid"},
		{name: "zero quantity", ng: nodeGroupWithVolumes(remoteVolume("data", "lvm", "0")), wantError: "positive"},
		{name: "negative quantity", ng: nodeGroupWithVolumes(remoteVolume("data", "lvm", "-1Gi")), wantError: "positive"},
		{name: "quantity overflow", ng: nodeGroupWithVolumes(remoteVolume("data", "lvm", "9223372036854775808")), wantError: "supported range"},
		{name: "rounding overflow", ng: nodeGroupWithVolumes(remoteVolume("data", "lvm", "9223372036854775807")), wantError: "rounded capacity"},
		{name: "reserve makes result non-positive", ng: nodeGroupWithVolumes(remoteVolume("data", "lvm", "1Gi")), spareGB: 1, wantError: "not positive"},
		{name: "negative reserve", ng: nodeGroupWithVolumes(remoteVolume("data", "lvm", "1Gi")), spareGB: -1, wantError: "spare-gb"},
		{name: "reserve overflow", ng: nodeGroupWithVolumes(remoteVolume("data", "lvm", "1Gi")), spareGB: math.MaxInt64, wantError: "spare-gb"},
		{name: "invalid mode", ng: nodeGroupWithVolumes(remoteVolume("data", "other", "1Gi")), wantError: "invalid"},
		{name: "negative disk count", ng: nodeGroupWithVolumes(remoteVolumeWithLayout("data", "lvm", "1Gi", -1, "raid0")), wantError: "must not be negative"},
		{name: "invalid RAID level", ng: nodeGroupWithVolumes(remoteVolumeWithLayout("data", "lvm", "1Gi", 1, "raid1")), wantError: "raidLevel is invalid"},
		{
			name: "zero disk count normalizes to one",
			ng:   nodeGroupWithVolumes(remoteVolumeWithLayout("data", "lvm", "1Gi", 0, "raid0")),
			want: 1*(1<<30) - 8*(1<<20),
		},
		{name: "malformed LVM name", ng: nodeGroupWithVolumes(remoteVolume("", "lvm", "1Gi")), wantError: "name"},
		{
			name:      "duplicate LVM names",
			ng:        nodeGroupWithVolumes(remoteVolume("data", "lvm", "1Gi"), remoteVolume("data", "lvm", "1Gi")),
			wantError: "duplicate",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RemoteLVMHandler{}.CalculateCapacity(context.Background(), tc.ng, tc.spareGB)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("RemoteLVMHandler.CalculateCapacity() error = %v, want substring %q", err, tc.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("RemoteLVMHandler.CalculateCapacity() error = %v", err)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("RemoteLVMHandler.CalculateCapacity() = %v, want nil", got)
				}
				return
			}
			if got == nil || got.Value() != tc.want {
				t.Fatalf("RemoteLVMHandler.CalculateCapacity() = %v, want %d bytes", got, tc.want)
			}
			if got.Value()%512 != 0 {
				t.Fatalf("capacity %d is not a 512-byte multiple", got.Value())
			}
		})
	}
}
