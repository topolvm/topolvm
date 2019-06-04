package csi

import (
	"context"
)

// LogicalVolumeService abstract the operations of logical volumes
type LogicalVolumeService interface {
	CreateVolume(ctx context.Context, node string, name string, sizeGb int64) (string, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error
}
