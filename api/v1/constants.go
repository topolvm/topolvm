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
	// PhaseReady Phase constants for SnapshotBackupStorage
	PhaseReady = "Ready"
	PhaseError = "Error"
)

const (
	TopoLVMSnapshotter = "topolvm-snapshotter"
)

// OperationType distinguishes backup vs restore.
type OperationType string

const (
	OperationBackup  OperationType = "Backup"
	OperationRestore OperationType = "Restore"
	OperationDelete  OperationType = "Delete"
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

// TypeSnapshotBackupExecutorEnsured indicates whether the Snapshot Backup Executor is ensured or not.
const (
	TypeSnapshotBackupExecutorEnsured               = "SnapshotBackupExecutorEnsured"
	ReasonSuccessfullyEnsuredSnapshotBackupExecutor = "SuccessfullyEnsuredSnapshotBackupExecutor"
	ReasonFailedToEnsureSnapshotBackupExecutor      = "FailedToEnsureSnapshotBackupExecutor"
)

// TypeSnapshotRestoreExecutorEnsured indicates whether the Snapshot Restore Executor is ensured or not.
const (
	TypeSnapshotRestoreExecutorEnsured               = "SnapshotRestoreExecutorEnsured"
	ReasonSuccessfullyEnsuredSnapshotRestoreExecutor = "SuccessfullyEnsuredSnapshotRestoreExecutor"
	ReasonFailedToEnsureSnapshotRestoreExecutor      = "FailedToEnsureSnapshotRestoreExecutor"
)

// TypeSnapshotBackupExecutorCleanup indicates whether the Snapshot Backup Executor is cleaned or not.
const (
	TypeSnapshotBackupExecutorCleaned               = "SnapshotBackupExecutorCleaned"
	ReasonSuccessfullyCleanedSnapshotBackupExecutor = "SuccessfullyCleanedSnapshotBackupExecutor"
	ReasonFailedToCleanedSnapshotBackupExecutor     = "FailedToCleanedSnapshotBackupExecutor"
)

// TypeSnapshotRestoreExecutorCleanup indicates whether the Snapshot Restore Executor is cleaned or not.
const (
	TypeSnapshotRestoreExecutorCleaned               = "SnapshotRestoreExecutorCleaned"
	ReasonSuccessfullyCleanedSnapshotRestoreExecutor = "SuccessfullyCleanedSnapshotRestoreExecutor"
	ReasonFailedToCleanedSnapshotRestoreExecutor     = "FailedToCleanedSnapshotRestoreExecutor"
)

// TypeSnapshotDeleteExecutorEnsured indicates whether the Snapshot Delete Executor is ensured or not.
const (
	TypeSnapshotDeleteExecutorEnsured               = "SnapshotDeleteExecutorEnsured"
	ReasonSuccessfullyEnsuredSnapshotDeleteExecutor = "SuccessfullyEnsuredSnapshotDeleteExecutor"
	ReasonFailedToEnsureSnapshotDeleteExecutor      = "FailedToEnsureSnapshotDeleteExecutor"
)

// Condition types for cleaning a LogicalVolume status
const (
	TypeSnapshotDeleteEnsured        = "SnapshotDeleteEnsured"
	ConditionSnapshotDeleteRunning   = "SnapshotDeleteRunning"
	ConditionSnapshotDeleteSucceeded = "SnapshotDeleteSucceeded"
	ConditionDeleteCleanupFailed     = "SnapshotDeleteFailed"
)

// Condition types for LogicalVolume status
const (
	// ConditionTypeBackupReady indicates that the backup operation is ready or completed successfully
	ConditionTypeBackupReady string = "BackupReady"
	// ConditionTypeRestoreReady indicates that the restore operation is ready or completed successfully
	ConditionTypeRestoreReady string = "RestoreReady"
)

// Condition reasons for LogicalVolume operations
const (
	// ReasonBackupInitialized indicates that backup has been initialized
	ReasonBackupInitialized string = "BackupInitialized"
	// ReasonBackupInProgress indicates that backup is currently in progress
	ReasonBackupInProgress string = "BackupInProgress"
	// ReasonBackupSucceeded indicates that backup completed successfully
	ReasonBackupSucceeded string = "BackupSucceeded"
	// ReasonBackupFailed indicates that backup failed
	ReasonBackupFailed string = "BackupFailed"

	// ReasonRestoreInitialized indicates that restore has been initialized
	ReasonRestoreInitialized string = "RestoreInitialized"
	// ReasonRestoreInProgress indicates that restore is currently in progress
	ReasonRestoreInProgress string = "RestoreInProgress"
	// ReasonRestoreSucceeded indicates that restore completed successfully
	ReasonRestoreSucceeded string = "RestoreSucceeded"
	// ReasonRestoreFailed indicates that restore failed
	ReasonRestoreFailed string = "RestoreFailed"
)
