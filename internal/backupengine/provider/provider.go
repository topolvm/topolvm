package provider

import (
	"context"
	"time"
)

// RepoRef includes the parameters to manipulate a backup repository
type RepoRef struct {
	Repository *string // Only use when redefining the repository

	FullPath string // It only uses to set the backup/restore result
	Suffix   string // It'll add as a suffix e.g, bucket/prefix/repositorySuffix
	Hostname string
}

// BackupParam includes parameters for backup operations
type BackupParam struct {
	RepoRef
	BackupPaths []string
	Exclude     []string
	Args        []string
}

// RestoreParam includes parameters for restore operations
type RestoreParam struct {
	RepoRef
	SnapshotID   string
	RestorePaths []string
	Destination  string
	Exclude      []string
	Include      []string
	Args         []string
}

// DeleteParam includes parameters for delete operations
type DeleteParam struct {
	RepoRef
	SnapshotIDs []string
}

// SnapshotInfo contains information about a snapshot
type SnapshotInfo struct {
	ID       string
	Time     time.Time
	Hostname string
	Paths    []string
	Tags     []string
}

// RepositoryStats contains statistics about the repository
type RepositoryStats struct {
	Integrity                     *bool
	Size                          string
	SnapshotCount                 int64
	SnapshotsRemovedOnLastCleanup int64
}

// BackupResult contains the result of a backup operation
// This is a generic structure that works for both Restic and Kopia
type BackupResult struct {
	// SnapshotID is the unique identifier of the created snapshot
	SnapshotID string `json:"snapshotID"`

	// Repository is the backup repository URL/path
	Repository string `json:"repository,omitempty"`

	// BackupTime is when the backup was taken
	BackupTime time.Time `json:"backupTime,omitempty"`

	// Size contains size information about the backup
	Size BackupSizeInfo `json:"size,omitempty"`

	// Files contains file statistics
	Files BackupFileInfo `json:"files,omitempty"`

	// Duration is how long the backup took
	Duration string `json:"duration,omitempty"`

	// Phase indicates success or failure
	Phase BackupPhase `json:"phase"`

	// ErrorMessage contains error details if backup failed
	ErrorMessage string `json:"errorMessage,omitempty"`

	// Hostname is the hostname used for the backup
	Hostname string `json:"hostname,omitempty"`

	// Paths are the paths that were backed up
	Paths []string `json:"paths,omitempty"`

	// Provider identifies the backup engine used (restic, kopia)
	Provider string `json:"provider,omitempty"`

	// Version is the version of the backup tool used
	Version string `json:"version,omitempty"`
}

// RestoreResult contains the result of a restore operation
// This is a generic structure that works for both Restic and Kopia
type RestoreResult struct {
	// Hostname indicate name of the host that has been restored
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Phase indicates restore phase of this host
	// +optional
	Phase RestorePhase `json:"phase,omitempty"`

	// Repository is the backup repository URL/path
	Repository string `json:"repository,omitempty"`

	// RestoreTime is when the restore was started
	RestoreTime time.Time `json:"backupTime,omitempty"`

	// Duration is how long the restore took
	Duration string `json:"duration,omitempty"`

	// ErrorMessage contains error details if restore failed
	ErrorMessage string `json:"errorMessage,omitempty"`

	// Provider identifies the backup engine used (restic, kopia)
	Provider string `json:"provider,omitempty"`
}

// BackupPhase represents the phase of backup operation
type BackupPhase string

const (
	BackupPhaseSucceeded BackupPhase = "Succeeded"
	BackupPhaseFailed    BackupPhase = "Failed"
)

type RestorePhase string

const (
	RestoreSucceeded RestorePhase = "Succeeded"
	RestoreFailed    RestorePhase = "Failed"
)

// BackupSizeInfo contains size-related statistics
type BackupSizeInfo struct {
	// TotalBytes is the total size of data processed
	TotalBytes int64 `json:"totalBytes,omitempty"`

	// UploadedBytes is the amount of data actually uploaded (may be less due to deduplication)
	UploadedBytes int64 `json:"uploadedBytes,omitempty"`

	// TotalFormatted is a human-readable total size (e.g., "1.5 GiB")
	TotalFormatted string `json:"totalFormatted,omitempty"`

	// UploadedFormatted is a human-readable uploaded size
	UploadedFormatted string `json:"uploadedFormatted,omitempty"`
}

// BackupFileInfo contains file-related statistics
type BackupFileInfo struct {
	// Total is the total number of files processed
	Total int64 `json:"total,omitempty"`

	// New is the number of new files
	New int64 `json:"new,omitempty"`

	// Modified is the number of modified files
	Modified int64 `json:"modified,omitempty"`

	// Unmodified is the number of unchanged files
	Unmodified int64 `json:"unmodified,omitempty"`
}

// Provider defines the common interface for snapshot engines (restic, kopia, etc.)
type Provider interface {
	// ValidateConnection checks if the connection to the repository is valid
	ValidateConnection(ctx context.Context) error

	// Backup creates a new snapshot and returns detailed backup result
	Backup(ctx context.Context, param BackupParam) (*BackupResult, error)

	// Restore restores files from a snapshot
	Restore(ctx context.Context, param RestoreParam) (*RestoreResult, error)

	Delete(ctx context.Context, param DeleteParam) ([]byte, error)
}
