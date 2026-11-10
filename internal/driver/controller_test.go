package driver

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/topolvm/topolvm"
	v1 "github.com/topolvm/topolvm/api/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// fakeLogicalVolumeService serves canned LogicalVolumes so the CSI methods can be
// driven without an API server.
type fakeLogicalVolumeService struct {
	sourceVolume    *v1.LogicalVolume
	createdVolume   *v1.LogicalVolume
	createdSnapshot *v1.LogicalVolume

	getVolumeCalls int
}

func (f *fakeLogicalVolumeService) GetVolume(ctx context.Context, volumeID string) (*v1.LogicalVolume, error) {
	f.getVolumeCalls++
	return f.sourceVolume, nil
}

func (f *fakeLogicalVolumeService) CreateVolume(ctx context.Context, node, dc, oc, name, sourceName string, requestBytes int64) (*v1.LogicalVolume, error) {
	return f.createdVolume, nil
}

func (f *fakeLogicalVolumeService) CreateSnapshot(ctx context.Context, node, dc, sourceVol, sname, accessType string, snapSize resource.Quantity) (*v1.LogicalVolume, error) {
	return f.createdSnapshot, nil
}

func (f *fakeLogicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	return nil
}

func (f *fakeLogicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, requestBytes int64) (*v1.LogicalVolume, error) {
	return nil, nil
}

const testSizeBytes = 1 << 30

func newCreateVolumeRequest(source *csi.VolumeContentSource) *csi.CreateVolumeRequest {
	return &csi.CreateVolumeRequest{
		Name:          "volume",
		CapacityRange: &csi.CapacityRange{RequiredBytes: testSizeBytes},
		VolumeCapabilities: []*csi.VolumeCapability{
			{
				AccessType: &csi.VolumeCapability_Mount{Mount: &csi.VolumeCapability_MountVolume{FsType: "ext4"}},
				AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
			},
		},
		AccessibilityRequirements: &csi.TopologyRequirement{
			Requisite: []*csi.Topology{
				{Segments: map[string]string{topolvm.GetTopologyNodeKey(): "node"}},
			},
		},
		VolumeContentSource: source,
	}
}

// provisionedLV is a LogicalVolume the node has created but whose size it has not
// reported yet, which is the state that used to crash the controller.
func provisionedLV(name string) *v1.LogicalVolume {
	return &v1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.LogicalVolumeSpec{
			Name:     name,
			NodeName: "node",
			Size:     *resource.NewQuantity(testSizeBytes, resource.BinarySI),
		},
		Status: v1.LogicalVolumeStatus{VolumeID: name + "-id"},
	}
}

func TestCreateVolumeWithMissingCurrentSize(t *testing.T) {
	t.Run("returns Aborted when the source size is unknown", func(t *testing.T) {
		lvService := &fakeLogicalVolumeService{sourceVolume: provisionedLV("source")}
		s := controllerServerNoLocked{lvService: lvService}

		source := &csi.VolumeContentSource{
			Type: &csi.VolumeContentSource_Volume{
				Volume: &csi.VolumeContentSource_VolumeSource{VolumeId: "source-id"},
			},
		}
		_, err := s.CreateVolume(context.Background(), newCreateVolumeRequest(source))
		if got := status.Code(err); got != codes.Aborted {
			t.Errorf("expected %s, but got %s (err: %v)", codes.Aborted, got, err)
		}
	})

	t.Run("returns Aborted rather than reporting a capacity the node has not confirmed", func(t *testing.T) {
		lvService := &fakeLogicalVolumeService{createdVolume: provisionedLV("volume")}
		s := controllerServerNoLocked{lvService: lvService}

		_, err := s.CreateVolume(context.Background(), newCreateVolumeRequest(nil))
		if got := status.Code(err); got != codes.Aborted {
			t.Errorf("expected %s, but got %s (err: %v)", codes.Aborted, got, err)
		}
	})

	t.Run("does not look up a source volume when none is requested", func(t *testing.T) {
		volume := provisionedLV("volume")
		volume.Status.CurrentSize = resource.NewQuantity(testSizeBytes, resource.BinarySI)
		lvService := &fakeLogicalVolumeService{createdVolume: volume}
		s := controllerServerNoLocked{lvService: lvService}

		if _, err := s.CreateVolume(context.Background(), newCreateVolumeRequest(nil)); err != nil {
			t.Fatalf("expected no error, but got %v", err)
		}
		if lvService.getVolumeCalls != 0 {
			t.Errorf("expected no source volume lookup, but got %d", lvService.getVolumeCalls)
		}
	})
}

func TestCreateSnapshotWithMissingCurrentSize(t *testing.T) {
	req := &csi.CreateSnapshotRequest{Name: "snapshot", SourceVolumeId: "source-id"}

	t.Run("returns Aborted when the source size is unknown", func(t *testing.T) {
		lvService := &fakeLogicalVolumeService{sourceVolume: provisionedLV("source")}
		s := controllerServerNoLocked{lvService: lvService}

		_, err := s.CreateSnapshot(context.Background(), req)
		if got := status.Code(err); got != codes.Aborted {
			t.Errorf("expected %s, but got %s (err: %v)", codes.Aborted, got, err)
		}
	})

	t.Run("returns Aborted rather than reporting a size the node has not confirmed", func(t *testing.T) {
		source := provisionedLV("source")
		source.Status.CurrentSize = resource.NewQuantity(testSizeBytes, resource.BinarySI)
		lvService := &fakeLogicalVolumeService{
			sourceVolume:    source,
			createdSnapshot: provisionedLV("snapshot"),
		}
		s := controllerServerNoLocked{lvService: lvService}

		_, err := s.CreateSnapshot(context.Background(), req)
		if got := status.Code(err); got != codes.Aborted {
			t.Errorf("expected %s, but got %s (err: %v)", codes.Aborted, got, err)
		}
	})
}
