package v1

type StorageProvider string

const (
	ProviderLocal StorageProvider = "local"
	ProviderS3    StorageProvider = "s3"
	ProviderGCS   StorageProvider = "gcs"
	ProviderAzure StorageProvider = "azure"
	ProviderB2    StorageProvider = "b2"
	ProviderSwift StorageProvider = "swift"
	ProviderRest  StorageProvider = "rest"
)

type BackupEngine string

const (
	EngineRestic BackupEngine = "restic"
	EngineKopia  BackupEngine = "kopia"
)

const (
	// PhaseReady Phase constants for OnlineSnapshotStorage
	PhaseReady = "Ready"
	PhaseError = "Error"
)

// OperationType distinguishes backup vs restore.
type OperationType string

const (
	OperationBackup  OperationType = "Backup"
	OperationRestore OperationType = "Restore"
)

// OperationPhase represents the current phase of a backup or restore operation.
type OperationPhase string

const (
	// OperationPhasePending indicates that the operation has been accepted by the system, but has not yet begun.
	OperationPhasePending OperationPhase = "Pending"
	// OperationPhaseRunning indicates that the operation is currently running.
	OperationPhaseRunning OperationPhase = "Running"
	// OperationPhaseBackingUp indicates that a backup operation is in progress.
	OperationPhaseBackingUp OperationPhase = "BackingUp"
	// OperationPhaseRestoring indicates that a restore operation is in progress.
	OperationPhaseRestoring OperationPhase = "Restoring"
	// OperationPhaseSucceeded indicates that the operation completed successfully.
	OperationPhaseSucceeded OperationPhase = "Succeeded"
	// OperationPhaseCompleted indicates that the operation has completed successfully.
	OperationPhaseCompleted OperationPhase = "Completed"
	// OperationPhaseFailed indicates that the operation has failed.
	OperationPhaseFailed OperationPhase = "Failed"
)
