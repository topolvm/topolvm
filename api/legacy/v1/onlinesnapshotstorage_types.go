package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Provider",type="string",JSONPath=".spec.storage.provider"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OnlineSnapshotStorage defines a storage destination where online
// VolumeSnapshots are stored and managed (e.g., via Restic or similar tools).
type OnlineSnapshotStorage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OnlineSnapshotStorageSpec   `json:"spec,omitempty"`
	Status OnlineSnapshotStorageStatus `json:"status,omitempty"`
}

// OnlineSnapshotStorageSpec defines configuration for the backend target.\
type OnlineSnapshotStorageSpec struct {
	// Engine defines which snapshot engine to use (e.g., restic, kopia).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=restic;kopia
	// +kubebuilder:default:=restic
	Engine BackupEngine `json:"engine"`

	// Storage specifies the backend storage configuration.
	// +kubebuilder:validation:Required
	Storage *Storage `json:"storage,omitempty"`

	// GlobalFlags defines flags applied to all restic operations.
	// Example: ["--no-lock", "--limit-upload=4", "--verbose"]
	// +optional
	GlobalFlags []string `json:"globalFlags,omitempty"`

	// BackupFlags defines additional flags for backup commands.
	// +optional
	BackupFlags []string `json:"backupFlags,omitempty"`

	// RestoreFlags defines additional flags for restore commands.
	// +optional
	RestoreFlags []string `json:"restoreFlags,omitempty"`

	// ValidateOnCreate controls whether the controller should validate the backend
	// (by attempting to connect/ping the backend storage) immediately after creation.
	// +optional
	// +kubebuilder:default:=true
	ValidateOnCreate bool `json:"validateOnCreate,omitempty"`
}

// OnlineSnapshotStorageStatus defines observed backend health.
type OnlineSnapshotStorageStatus struct {
	// Phase indicates current validation or operational state (e.g., Ready, Error).
	// +kubebuilder:validation:Enum=Ready;Pending;Error
	Phase string `json:"phase,omitempty"`

	// Message provides human-readable context or error info.
	// +optional
	Message string `json:"message,omitempty"`

	// LastChecked records the timestamp of the most recent backend validation.
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
}

type Storage struct {
	// Provider specifies the provider of the storage
	// +kubebuilder:validation:Enum=s3;gcs;azure;local
	Provider StorageProvider `json:"provider,omitempty"`

	// S3 specifies the storage information for AWS S3 and S3 compatible storage.
	// +optional
	S3 *S3Spec `json:"s3,omitempty"`

	// GCS specifies the storage information for GCS bucket
	// +optional
	GCS *GCSSpec `json:"gcs,omitempty"`

	// Azure specifies the storage information for Azure Blob container
	// +optional
	Azure *AzureSpec `json:"azure,omitempty"`
}

//+kubebuilder:object:root=true

// OnlineSnapshotStorageList contains a list of OnlineSnapshotStorage
type OnlineSnapshotStorageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OnlineSnapshotStorage `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OnlineSnapshotStorage{}, &OnlineSnapshotStorageList{})
}
