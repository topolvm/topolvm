package controller

import "time"

const (
	// keySelectedNode is a PVC resource indexing key for the controller
	keySelectedNode = "metadata.annotations.selected-node"

	// keyLogicalVolumeNode is a Logical Volume resource indexing key for the controller
	keyLogicalVolumeNode = "spec.nodeName"

	// AnnSelectedNode annotation is added to a PVC that has been triggered by scheduler to
	// be dynamically provisioned. Its value is the name of the selected node.
	// https://github.com/kubernetes/kubernetes/blob/9bae1bc56804db4905abebcd408e0f02e199ab93/pkg/controller/volume/persistentvolume/util/util.go#L53
	AnnSelectedNode = "volume.kubernetes.io/selected-node"

	// requeueIntervalForSimpleUpdate is the requeue interval when updating the manifest during reconciliation and re-execute loop
	requeueIntervalForSimpleUpdate = 1 * time.Second
)

// Online Snapshot-related constants
const (
	SnapshotModeOnline       = "online"
	SnapshotMode             = "topolvm.io/snapshotMode"
	SnapshotStorageNamespace = "topolvm.io/snapshotStorageNamespace"
	SnapshotStorageName      = "topolvm.io/snapshotStorageName"
)
