package controller

import (
	internalController "github.com/topolvm/topolvm/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupPersistentVolumeClaimReconciler creates PersistentVolumeClaimReconciler and sets up with manager.
func SetupPersistentVolumeClaimReconciler(mgr ctrl.Manager, client client.Client, apiReader client.Reader) error {
	reconciler := internalController.NewPersistentVolumeClaimReconciler(client, apiReader)
	return reconciler.SetupWithManager(mgr)
}
