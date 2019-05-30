package csi

import (
	"context"
)

type LogicalVolumeService interface {
	CreateVolume(ctx context.Context, node string, name string, sizeGb int64) (string, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error
}
