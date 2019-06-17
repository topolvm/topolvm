package csi

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

// LogicalVolumeService abstract the operations of logical volumes
type LogicalVolumeService interface {
	CreateVolume(ctx context.Context, node string, name string, sizeGb int64, capabilities []*VolumeCapability) (string, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error
	ValidateVolumeCapabilities(ctx context.Context, volumeID string, capabilities []*VolumeCapability) (bool, error)

	// TODO: delete this method
	ListNodes(ctx context.Context) (*corev1.NodeList, error)
}
