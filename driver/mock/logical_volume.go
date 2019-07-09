package mock

import (
	"context"
	"sync"

	"github.com/cybozu-go/topolvm/csi"
	"github.com/cybozu-go/topolvm/driver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
func NewLogicalVolumeService() (driver.LogicalVolumeService, error) {
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

func (s *logicalVolumeService) VolumeExists(ctx context.Context, name string) error {
	return nil
}

func (s *logicalVolumeService) GetCapacity(ctx context.Context, requestNodeNumber string) (int64, error) {
	return 0, nil
}

func (s *logicalVolumeService) GetMaxCapacity(ctx context.Context) (string, int64, error) {
	return "", 0, nil
}
