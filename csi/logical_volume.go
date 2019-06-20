package csi

import (
	"context"
)

// LogicalVolumeService abstract the operations of logical volumes
type LogicalVolumeService interface {
	CreateVolume(ctx context.Context, node string, name string, sizeGb int64, capabilities []*VolumeCapability) (string, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error
	ValidateVolumeCapabilities(ctx context.Context, volumeID string, capabilities []*VolumeCapability) (bool, error)
	GetCapacity(ctx context.Context, requestNodeNumber string) (int64, error)
	GetMaxCapacity(ctx context.Context) (string, int64, error)
}
