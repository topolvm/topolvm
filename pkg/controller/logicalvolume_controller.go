package controller

import (
	internalController "github.com/topolvm/topolvm/internal/controller"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetupLogicalVolumeReconcilerWithServices creates LogicalVolumeReconciler and sets up with manager.
func SetupLogicalVolumeReconcilerWithServices(mgr ctrl.Manager, client client.Client, nodeName string, vgService proto.VGServiceClient, lvService proto.LVServiceClient) error {
	reconciler := internalController.NewLogicalVolumeReconcilerWithServices(client, nodeName, vgService, lvService)
	return reconciler.SetupWithManager(mgr)
}
