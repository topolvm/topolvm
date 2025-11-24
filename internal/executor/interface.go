package executor

// Executor defines the interface for executing snapshot operations.
// Implementations of this interface handle the creation and management
// of snapshot pods that perform online snapshots of logical volumes.
type Executor interface {
	// Execute performs the snapshot operation by creating the necessary
	// Kubernetes resources (e.g., pods) to carry out the snapshot.
	// Returns an error if the operation fails.
	Execute() error
}
