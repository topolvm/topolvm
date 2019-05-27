package mock

import (
	"context"

	"github.com/cybozu-go/topolvm/csi"
)

type LogicalVolumeService struct {
}

func NewLogicalVolumeService() (csi.LogicalVolumeService, error) {
	return &LogicalVolumeService{}, nil
}

func (s *LogicalVolumeService) CreateVolume(ctx context.Context, node string, name string, size int64) (string, error) {
	panic("implement me")
}

func (s *LogicalVolumeService) DeleteVolume(ctx context.Context, volumeID string) error {
	panic("implement me")
}

func (s *LogicalVolumeService) ExpandVolume(ctx context.Context, volumeID string, size int64) error {
	panic("implement me")
}
