package driver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/topolvm/topolvm"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_convertRequestCapacityBytes(t *testing.T) {
	testCases := []struct {
		requestBytes int64
		limitBytes   int64
		expected     int64
		err          error
	}{
		{
			requestBytes: -1,
			limitBytes:   10,
			err:          ErrNoNegativeRequestBytes,
		},
		{
			requestBytes: 10,
			limitBytes:   -1,
			err:          ErrNoNegativeLimitBytes,
		},
		{
			requestBytes: 20,
			limitBytes:   10,
			err:          ErrRequestedExceedsLimit,
		},
		{
			requestBytes: 1<<30 + 1,
			limitBytes:   1<<30 + 1,
			err:          ErrRequestedExceedsLimit,
		},
		{
			requestBytes: 0,
			limitBytes:   topolvm.MinimumSectorSize - 1,
			err:          ErrResultingRequestIsZero,
		},
		{
			requestBytes: 0,
			limitBytes:   topolvm.MinimumSectorSize + 1,
			expected:     topolvm.MinimumSectorSize,
		},
		{
			requestBytes: 1,
			limitBytes:   topolvm.MinimumSectorSize * 2,
			expected:     topolvm.MinimumSectorSize,
		},
		{
			requestBytes: 0,
			limitBytes:   2 << 30,
			expected:     1 << 30,
		},
		{
			requestBytes: 1,
			limitBytes:   0,
			expected:     topolvm.MinimumSectorSize,
		},
		{
			requestBytes: 1 << 30,
			limitBytes:   1 << 30,
			expected:     1 << 30,
		},
		{
			requestBytes: 0,
			limitBytes:   0,
			expected:     1 << 30,
		},
	}

	for _, tc := range testCases {
		tcName := fmt.Sprintf("request:%d limit:%d", tc.requestBytes, tc.limitBytes)
		if tc.err != nil {
			tcName += fmt.Sprintf(" = %s", tc.err)
		} else {
			tcName += fmt.Sprintf(" = %v", tc.expected)
		}

		t.Run(tcName, func(t *testing.T) {
			v, err := convertRequestCapacityBytes(tc.requestBytes, tc.limitBytes)
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v, but got %v", tc.err, err)
			}
			if v != tc.expected {
				t.Errorf("expected %d, but got %d", tc.expected, v)
			}
		})
	}
}

func Test_roundUp(t *testing.T) {
	testCases := []struct {
		size     int64
		multiple int64
		expected int64
	}{
		{12, 4, 12},
		{11, 4, 12},
		{13, 4, 16},
		{0, 4, 0},
	}

	for _, tc := range testCases {
		name := fmt.Sprintf("nearest rounded up multiple of %d from %d should be %d", tc.multiple, tc.size, tc.expected)
		t.Run(name, func(t *testing.T) {
			rounded := roundUp(tc.size, tc.multiple)
			if rounded != tc.expected {
				t.Errorf("%s, but was %d", name, rounded)
			}
		})
	}
}

func Test_getMinimumAllocatedSize(t *testing.T) {
	mockBlock := &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Block{
		Block: &csi.VolumeCapability_BlockVolume{},
	}}
	mockMount := &csi.VolumeCapability{AccessType: &csi.VolumeCapability_Mount{
		Mount: &csi.VolumeCapability_MountVolume{},
	}}

	testCases := []struct {
		parameters   map[string]string
		capabilities []*csi.VolumeCapability
		expected     resource.Quantity
		err          error
	}{
		{
			parameters:   map[string]string{},
			capabilities: nil,
			expected:     resource.MustParse("0"),
			err:          nil,
		},
		{
			parameters:   map[string]string{},
			capabilities: []*csi.VolumeCapability{mockBlock},
			expected:     resource.MustParse("0"),
			err:          nil,
		},
		{
			parameters: map[string]string{
				topolvm.GetMinimumAllocatedSizeKeyFilesystem(): "5Mi",
				topolvm.GetMinimumAllocatedSizeKeyBlock():      "3Mi",
			},
			capabilities: []*csi.VolumeCapability{mockBlock},
			expected:     resource.MustParse("3Mi"),
			err:          nil,
		},
		{
			parameters: map[string]string{
				topolvm.GetMinimumAllocatedSizeKeyFilesystem(): "5Mi",
				topolvm.GetMinimumAllocatedSizeKeyBlock():      "3Mi",
			},
			capabilities: []*csi.VolumeCapability{mockMount},
			expected:     resource.MustParse("5Mi"),
			err:          nil,
		},
		{
			parameters: map[string]string{
				topolvm.GetMinimumAllocatedSizeKeyBlock(): "-1",
			},
			capabilities: []*csi.VolumeCapability{mockBlock},
			expected:     resource.MustParse("0"),
			err:          nil,
		},
		{
			parameters: map[string]string{
				topolvm.GetMinimumAllocatedSizeKeyBlock(): "abc",
			},
			capabilities: []*csi.VolumeCapability{mockBlock},
			expected:     resource.MustParse("0"),
			err:          resource.ErrFormatWrong,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("parameters: %v, capabilities: %v", tc.parameters, tc.capabilities), func(t *testing.T) {
			v, err := getMinimumAllocatedSize(tc.parameters, tc.capabilities)
			if !errors.Is(err, tc.err) {
				t.Errorf("expected error %v, but got %v", tc.err, err)
			}
			if v.Cmp(tc.expected) != 0 {
				t.Errorf("expected %s, but got %s", tc.expected.String(), v.String())
			}
		})
	}
}
