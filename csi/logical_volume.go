package csi

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
)

// LogicalVolumeService abstract the operations of logical volumes
type LogicalVolumeService interface {
	CreateVolume(ctx context.Context, node string, name string, sizeGb int64) (string, error)
	DeleteVolume(ctx context.Context, volumeID string) error
	ExpandVolume(ctx context.Context, volumeID string, sizeGb int64) error
	GetPVByVolumeID(ctx context.Context, volumeID string) (*corev1.PersistentVolume, error)
	GetStorageClass(ctx context.Context, storageClassName string) (*storagev1.StorageClass, error)
}
