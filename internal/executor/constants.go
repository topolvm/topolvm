package executor

const (
	// BackupCommandName is the command name for the online snapshot container
	// This is the subcommand of the online-snapshotter binary
	BackupCommandName = "backup"

	// RestoreCommandName is the command name for restoring from an online snapshot
	// This is the subcommand of the online-snapshotter binary
	RestoreCommandName = "restore"

	// DefaultSnapshotImage is the default image used for the snapshot container
	DefaultSnapshotImage = "topolvm/topolvm:latest"

	// SnapshotContainerName is the name of the snapshot container
	SnapshotContainerName = "snapshot-executor"

	// RestoreContainerName is the name of the restore container
	RestoreContainerName = "restore-executor"

	SnapshotData    = "snapshot-data"
	SnapshotDataDir = "snapshot-data-dir"

	// LabelAppKey is the label key for the app
	LabelAppKey = "app"

	// LabelAppValue is the label value for topolvm
	LabelAppValue = "topolvm-snapshot"

	// LabelLogicalVolumeKey is the label key for logical volume name
	LabelLogicalVolumeKey = "topolvm.io/logical-volume"

	// LabelSnapshotPodKey is the label key to identify snapshot pods
	LabelSnapshotPodKey = "topolvm.io/snapshot-pod"

	// EnvHostNamespace is the environment variable key for the host namespace
	EnvHostNamespace = "HOST_NAMESPACE"

	// EnvHostName is the environment variable key for the hostname
	EnvHostName = "HOSTNAME"
)

// Online Snapshot-related constants
const (
	SnapshotStorageNamespace = "topolvm.io/snapshotStorageNamespace"
	SnapshotStorageName      = "topolvm.io/snapshotStorageName"
)
