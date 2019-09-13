package controllers

import (
	"context"
	"time"

	logicalvolumev1 "github.com/cybozu-go/topolvm/topolvm-node/api/v1"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// LogicalVolumeReconciler reconciles a LogicalVolume object
type LogicalVolumeReconciler struct {
	client.Client
	Log         logr.Logger
	NodeName    string
	Events      <-chan event.GenericEvent
	StalePeriod time.Duration
}

// +kubebuilder:rbac:groups=topolvm.cybozu.com,resources=logicalvolumes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumes,verbs=get;list;watch;delete

// Reconcile deletes staled LogicalVolume and PV
func (r *LogicalVolumeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("logicalvolume", req.NamespacedName)
	log.Info("hoge")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *LogicalVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return true },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&logicalvolumev1.LogicalVolume{}).
		WithEventFilter(pred).
		Watches(
			&source.Channel{
				Source:         r.Events,
				DestBufferSize: 1,
			},
			&handler.EnqueueRequestForObject{},
		).
		Complete(r)
}
