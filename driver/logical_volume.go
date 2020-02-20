package driver

import (
	"context"

	"github.com/cybozu-go/topolvm/csi"
)

// LogicalVolumeService abstract the operations of logical volumes
type LogicalVolumeService interface {
	CreateVolume(ctx context.Context, node string, name string, sizeGb int64, capabilities []*csi.VolumeCapability) (string, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error
	VolumeExists(ctx context.Context, volumeID string) error
	GetCapacity(ctx context.Context, requestNodeNumber string) (int64, error)
	GetMaxCapacity(ctx context.Context) (string, int64, error)
	GetCurrentSize(ctx context.Context, volumeID string) (int64, error)
}
