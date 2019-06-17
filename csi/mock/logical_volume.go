package mock

import (
	"context"
	"sync"

	"github.com/cybozu-go/topolvm/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
)

type logicalVolume struct {
	name     string
	size     int64
	node     string
	volumeID string
}

type logicalVolumeService struct {
	mu      sync.Mutex
	volumes map[string]logicalVolume
}

// NewLogicalVolumeService returns LogicalVolumeService.
func NewLogicalVolumeService() (csi.LogicalVolumeService, error) {
	return &logicalVolumeService{
		volumes: make(map[string]logicalVolume),
	}, nil
}

func (s *logicalVolumeService) CreateVolume(ctx context.Context, node string, name string, sizeGb int64, capabilities []*csi.VolumeCapability) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.volumes[name]; ok {
		return "", status.Error(codes.ResourceExhausted, "error")
	}
	s.volumes[name] = logicalVolume{
		name:     name,
		size:     sizeGb << 30,
		node:     node,
		volumeID: name,
	}
	return name, nil
}

func (s *logicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.volumes[volumeID]; !ok {
		return status.Error(codes.NotFound, "error")
	}
	delete(s.volumes, volumeID)
	return nil
}

func (s *logicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.volumes[volumeID]
	if !ok {
		return status.Error(codes.NotFound, "error")
	}
	v.size = sizeGb << 30
	s.volumes[volumeID] = v

	return nil
}

func (s *logicalVolumeService) ListNodes(ctx context.Context) (*corev1.NodeList, error) {
	return new(corev1.NodeList), nil
}

func (s *logicalVolumeService) ValidateVolumeCapabilities(ctx context.Context, name string, capabilities []*csi.VolumeCapability) (bool, error) {
	return true, nil
}
