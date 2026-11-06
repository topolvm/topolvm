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

	// DefaultLVMOperationTimeout bounds each call the LogicalVolume reconciler
	// makes to lvmd. An LVM command that waits on a udev cookie which a killed
	// udev worker never completed would otherwise block forever, holding the
	// VG lock and this node's only reconcile worker. It is longer than udevd's
	// default event_timeout (180s), after which the cookie cannot complete.
	DefaultLVMOperationTimeout = 5 * time.Minute
)
