package driver

import (
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_MinimumAllocationSettings(t *testing.T) {
	mockBlock := &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Block{
		Block: &csi.VolumeCapability_BlockVolume{},
	}}
	mockMount := &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{
		Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"},
	}}

	type testBytes struct {
		required, limit int64
	}
	testCases := []struct {
		name string

		settings MinimumAllocationSettings

		capabilities []*csi.VolumeCapability

		input    testBytes
		expected testBytes
	}{
		{
			name:     "no settings result in pass through",
			settings: MinimumAllocationSettings{},
			input: testBytes{
				required: 1 << 30,
				limit:    2 << 30,
			},
			expected: testBytes{
				required: 1 << 30,
				limit:    2 << 30,
			},
		},
		{
			name: "minimum should be applied for block storage",
			settings: MinimumAllocationSettings{
				Block: Quantity(resource.MustParse("1Gi")),
			},
			capabilities: []*csi.VolumeCapability{mockBlock},
			input: testBytes{
				required: 0,
				limit:    2 << 30,
			},
			expected: testBytes{
				required: 1 << 30,
				limit:    2 << 30,
			},
		},
		{
			name: "negative minimum should result in no minimum",
			settings: MinimumAllocationSettings{
				Block: Quantity(resource.MustParse("-1")),
			},
			capabilities: []*csi.VolumeCapability{mockBlock},
			input: testBytes{
				required: 0,
				limit:    2 << 30,
			},
			expected: testBytes{
				required: 0,
				limit:    2 << 30,
			},
		},
		{
			name: "minimum should be ignored if request is bigger",
			settings: MinimumAllocationSettings{
				Block: Quantity(resource.MustParse("1Gi")),
			},
			capabilities: []*csi.VolumeCapability{mockBlock},
			input: testBytes{
				required: 2 << 30,
				limit:    2 << 30,
			},
			expected: testBytes{
				required: 2 << 30,
				limit:    2 << 30,
			},
		},
		{
			name: "minimum should be applied for filesystem storage (if fs matches)",
			settings: MinimumAllocationSettings{
				Filesystem: map[string]Quantity{
					"ext4": Quantity(resource.MustParse("1Gi")),
				},
			},
			capabilities: []*csi.VolumeCapability{mockMount},
			input: testBytes{
				required: 0,
				limit:    2 << 30,
			},
			expected: testBytes{
				required: 1 << 30,
				limit:    2 << 30,
			},
		},
		{
			name: "unknown filesystem minimum should be ignored",
			settings: MinimumAllocationSettings{
				Filesystem: map[string]Quantity{
					"foo": Quantity(resource.MustParse("1Gi")),
				},
			},
			capabilities: []*csi.VolumeCapability{mockMount},
			input: testBytes{
				required: 0,
				limit:    2 << 30,
			},
			expected: testBytes{
				required: 0,
				limit:    2 << 30,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			required, limit := tc.settings.MinMaxAllocationsFromSettings(
				tc.input.required, tc.input.limit, tc.capabilities)

			if required != tc.expected.required {
				t.Errorf("expected minimum/required bytes to be %d, but got %d", tc.expected.required, required)
			}
			if limit != tc.expected.limit {
				t.Errorf("expected minimum/required bytes to be %d, but got %d", tc.expected.limit, limit)
			}
		})
	}
}

func TestQuantity_UnmarshalText(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected int64
		err      bool
	}{
		{
			name:     "valid Quantity",
			input:    "1Gi",
			expected: 1 << 30,
		},
		{
			name:     "negative Quantity",
			input:    "-1",
			expected: -1,
		},
		{
			name:     "invalid Quantity",
			input:    "blub",
			expected: 0,
			err:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var q Quantity
			err := q.Set(tc.input)
			if tc.err && err == nil {
				t.Errorf("expected error, but got none")
			}
			if !tc.err && err != nil {
				t.Errorf("expected no error, but got %v", err)
			}
			resourceQuantity := resource.Quantity(q)
			if resourceQuantity.CmpInt64(tc.expected) != 0 {
				t.Errorf("expected %d, but got %s", tc.expected, resourceQuantity.String())
			}
		})
	}
}
