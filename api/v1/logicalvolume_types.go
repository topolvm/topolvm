package v1

import (
	"google.golang.org/grpc/codes"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// LogicalVolumeSpec defines the desired state of LogicalVolume
type LogicalVolumeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Name                string            `json:"name"`
	NodeName            string            `json:"nodeName"`
	Size                resource.Quantity `json:"size"`
	DeviceClass         string            `json:"deviceClass,omitempty"`
	LvcreateOptionClass string            `json:"lvcreateOptionClass,omitempty"`

	// 'source' specifies the logicalvolume name of the source; if present.
	// This field is populated only when LogicalVolume has a source.
	//+kubebuilder:validation:Optional
	Source string `json:"source,omitempty"`

	//'accessType' specifies how the user intends to consume the snapshot logical volume.
	// Set to "ro" when creating a snapshot and to "rw" when restoring a snapshot or creating a clone.
	// This field is populated only when LogicalVolume has a source.
	//+kubebuilder:validation:Optional
	AccessType string `json:"accessType,omitempty"`
}

// LogicalVolumeStatus defines the observed state of LogicalVolume
type LogicalVolumeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	VolumeID    string             `json:"volumeID,omitempty"`
	Code        codes.Code         `json:"code,omitempty"`
	Message     string             `json:"message,omitempty"`
	CurrentSize *resource.Quantity `json:"currentSize,omitempty"`

	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Snapshot *SnapshotStatus `json:"snapshot,omitempty"`
}

// SnapshotStatus defines the observed state of a backup or restore operation.
type SnapshotStatus struct {
	// Operation indicates whether this status is for a backup or a restore.
	// +optional
	Operation OperationType `json:"operation,omitempty"`
	// Phase represents the current phase of the backup or restore operation.
	Phase OperationPhase `json:"phase"`
	// StartTime is the time at which the operation was started.
	StartTime metav1.Time `json:"startTime"`
	// CompletionTime is the time at which the operation completed (success or failure).
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// Duration is how long operation took to complete.
	// +optional
	Duration string `json:"duration,omitempty"`
	// Progress contains information about the progress of the operation.
	// +optional
	Progress *OperationProgress `json:"progress,omitempty"`
	// Message provides a short description of the snapshot’s state
	// +optional
	Message string `json:"message,omitempty"`
	// Error contains details if the operation encountered an error.
	// +optional
	Error *SnapshotError `json:"error,omitempty"`
	// Paths are the paths that were backed up or restored
	// +optional
	Paths []string `json:"paths,omitempty"`
	// Repository is the Restic repository path where the snapshot is stored
	// +optional
	Repository string `json:"repository,omitempty"`
	// SnapshotID is the identifier of the Restic snapshot involved in the operation.
	// +optional
	SnapshotID string `json:"snapshotID,omitempty"`
	// Version keeps track of restic binary or backup engine version used
	// +optional
	Version string `json:"version,omitempty"`
}

type OperationProgress struct {
	// +optional
	TotalBytes int64 `json:"totalBytes,omitempty"`

	// +optional
	BytesDone int64 `json:"bytesDone,omitempty"`

	// Percentage can be calculated by controller or client (UploadedBytes / TotalBytes * 100)
	// +optional
	Percentage string `json:"percentage,omitempty"`
}

type SnapshotError struct {
	Code    string `json:"code,omitempty"`    // e.g., "RepositoryNotReachable", "VolumeMountFailed"
	Message string `json:"message,omitempty"` // human-readable error
}

type BackupProgress struct {
	// +optional
	TotalBytes int64 `json:"totalBytes,omitempty"`

	// +optional
	BytesDone int64 `json:"bytesDone,omitempty"`

	// Percentage can be calculated by controller or client (UploadedBytes / TotalBytes * 100)
	// +optional
	Percentage string `json:"percentage,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// LogicalVolume is the Schema for the logicalvolumes API
type LogicalVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LogicalVolumeSpec   `json:"spec,omitempty"`
	Status LogicalVolumeStatus `json:"status,omitempty"`
}

// IsCompatibleWith returns true if the LogicalVolume is compatible.
func (lv *LogicalVolume) IsCompatibleWith(lv2 *LogicalVolume) bool {
	if lv.Spec.Name != lv2.Spec.Name {
		return false
	}
	if lv.Spec.Source != lv2.Spec.Source {
		return false
	}
	if lv.Spec.Size.Cmp(lv2.Spec.Size) != 0 {
		return false
	}
	return true
}

//+kubebuilder:object:root=true

// LogicalVolumeList contains a list of LogicalVolume
type LogicalVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LogicalVolume `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LogicalVolume{}, &LogicalVolumeList{})
}
