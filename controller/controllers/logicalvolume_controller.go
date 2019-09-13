package controllers

import (
	"context"

	logicalvolumev1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LogicalVolumeReconciler reconciles a LogicalVolume object
type LogicalVolumeReconciler struct {
	client.Client
	Log      logr.Logger
	NodeName string
}

// +kubebuilder:rbac:groups=topolvm.cybozu.com,resources=logicalvolumes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;delete

func (r *LogicalVolumeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("logicalvolume", req.NamespacedName)

	// your logic here

	return ctrl.Result{}, nil
}

func (r *LogicalVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&logicalvolumev1.LogicalVolume{}).
		Complete(r)
}
