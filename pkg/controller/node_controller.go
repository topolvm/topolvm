package controller

import (
	internalController "github.com/topolvm/topolvm/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupNodeReconciler creates NodeReconciler and sets up with manager.
func SetupNodeReconciler(mgr ctrl.Manager, client client.Client, skipNodeFinalize bool) error {
	reconciler := internalController.NewNodeReconciler(client, skipNodeFinalize)
	return reconciler.SetupWithManager(mgr)
}
