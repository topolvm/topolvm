package mock

import (
	"context"
	"errors"
	"sync"

	"github.com/cybozu-go/topolvm/csi"
)

var (
	ErrResourceExhausted = errors.New("resource exhausted")
	ErrNotFound          = errors.New("not found")
)

type logicalVolume struct {
	name     string
	size     int64
	node     string
	volumeID string
}

type LogicalVolumeService struct {
	mu      sync.Mutex
	volumes map[string]logicalVolume
}

func NewLogicalVolumeService() (csi.LogicalVolumeService, error) {
	return &LogicalVolumeService{
		volumes: make(map[string]logicalVolume),
	}, nil
}

func (s *LogicalVolumeService) CreateVolume(ctx context.Context, node string, name string, size int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.volumes[name]; ok {
		return "", ErrResourceExhausted
	}
	s.volumes[name] = logicalVolume{
		name:     name,
		size:     size,
		node:     node,
		volumeID: name,
	}
	return name, nil
}

func (s *LogicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.volumes[volumeID]; !ok {
		return ErrNotFound
	}
	delete(s.volumes, volumeID)
	return nil
}

func (s *LogicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, size int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.volumes[volumeID]
	if !ok {
		return ErrNotFound
	}
	v.size = size
	s.volumes[volumeID] = v

	return nil
}
