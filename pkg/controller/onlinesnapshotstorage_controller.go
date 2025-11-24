package controller

import (
	"fmt"

	internalController "github.com/topolvm/topolvm/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupOnlineSnapshotStorageReconciler creates OnlineSnapshotStorageReconciler and sets up with manager.
func SetupOnlineSnapshotStorageReconciler(mgr ctrl.Manager, client client.Client) error {
	fmt.Println("############## SetupOnlineSnapshotStorageReconciler ###############")
	reconciler := internalController.NewOnlineSnapshotStorageReconciler(client)
	return reconciler.SetupWithManager(mgr)
}
