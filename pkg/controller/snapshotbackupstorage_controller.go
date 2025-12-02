package controller

import (
	internalController "github.com/topolvm/topolvm/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupSnapshotBackupStorageReconciler creates SnapshotBackupStorageReconciler and sets up with manager.
func SetupSnapshotBackupStorageReconciler(mgr ctrl.Manager, client client.Client) error {
	reconciler := internalController.NewSnapshotBackupStorageReconciler(client)
	return reconciler.SetupWithManager(mgr)
}
