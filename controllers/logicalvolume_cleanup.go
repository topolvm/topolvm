package controllers

import (
	"context"
	"time"

	"github.com/cybozu-go/topolvm"
	topolvmv1 "github.com/cybozu-go/topolvm/api/v1"
	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// LogicalVolumeCleanupReconciler reconciles a LogicalVolume object
type LogicalVolumeCleanupReconciler struct {
	client.Client
	Log         logr.Logger
	Events      <-chan event.GenericEvent
	StalePeriod time.Duration
}

// +kubebuilder:rbac:groups=topolvm.io,resources=logicalvolumes,verbs=get;list;watch;update;patch

// Reconcile deletes stale LogicalVolume(s)
func (r *LogicalVolumeCleanupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("logicalvolume", req.NamespacedName)

	var lvl topolvmv1.LogicalVolumeList
	err := r.List(ctx, &lvl)
	if err != nil {
		return ctrl.Result{}, err
	}

	now := time.Now()
	for _, lv := range lvl.Items {
		if err := r.cleanup(ctx, log, &lv, now); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *LogicalVolumeCleanupReconciler) cleanup(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, now time.Time) error {
	if lv.DeletionTimestamp == nil {
		return nil
	}
	if lv.DeletionTimestamp.Add(r.StalePeriod).After(now) {
		return nil
	}
	finExists := false
	for _, fin := range lv.Finalizers {
		if fin == topolvm.LogicalVolumeFinalizer {
			finExists = true
			break
		}
	}
	if !finExists {
		return nil
	}

	log.Info("deleting stale LogicalVolume",
		"name", lv.Name,
		"timestamp", lv.DeletionTimestamp.String())

	lv2 := lv.DeepCopy()
	var finalizers []string
	for _, fin := range lv2.Finalizers {
		if fin == topolvm.LogicalVolumeFinalizer {
			continue
		}
		finalizers = append(finalizers, fin)
	}
	lv2.Finalizers = finalizers

	patch := client.MergeFrom(lv)
	if err := r.Patch(ctx, lv2, patch); err != nil {
		log.Error(err, "failed to patch LogicalVolume", "name", lv.Name)
		return err
	}

	return nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *LogicalVolumeCleanupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return true },
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&topolvmv1.LogicalVolume{}).
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
