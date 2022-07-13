package controllers

import (
	"context"
	"encoding/json"

	"github.com/go-logr/logr"
	"github.com/topolvm/topolvm"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// StorageClassReconciler reconciles a StorageClass object
type StorageClassReconciler struct {
	client.Client
	APIReader client.Reader
}

// Reconcile finalize StorageClass
func (r *StorageClassReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx).WithValues(
		"controller", "StorageClass",
		"name", req.NamespacedName.Name)
	log.Info("Start Reconcile")
	sc := &storagev1.StorageClass{}
	err := r.Get(ctx, req.NamespacedName, sc)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if !filterSC(sc) {
		return ctrl.Result{}, nil
	}

	if sc.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, log, sc)
}

func (r *StorageClassReconciler) reconcile(ctx context.Context, log logr.Logger, sc *storagev1.StorageClass) (ctrl.Result, error) {
	log.Info("start migration")
	sccopy := migrateSC(sc)

	// step2: delete SC
	log.Info("delete sc")
	if err := r.Delete(ctx, sc); err != nil {
		log.Error(err, "failed to delete SC")
		return ctrl.Result{}, err
	}

	// step3: re-create SC
	log.Info("re-create SC")
	if err := r.Create(ctx, sccopy); err != nil {
		data, _ := json.Marshal(sccopy)
		log.Error(err, "failed to re-create SC", "StorageClass", data)
		return ctrl.Result{}, err
	}

	log.Info("complete migration")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return filterSC(e.Object.(*storagev1.StorageClass))
		},
		DeleteFunc: func(event.DeleteEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return filterSC(e.ObjectNew.(*storagev1.StorageClass))
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&storagev1.StorageClass{}).
		Complete(r)
}

func filterSC(sc *storagev1.StorageClass) bool {
	if sc == nil || sc.Provisioner != topolvm.LegacyPluginName {
		return false
	}
	return true
}

func migrateSC(sc *storagev1.StorageClass) *storagev1.StorageClass {
	sc2 := sc.DeepCopy()
	sc2.ResourceVersion = ""
	sc2.UID = ""
	sc2.Provisioner = topolvm.PluginName
	if v, ok := sc2.Parameters[topolvm.LegacyDeviceClassKey]; ok {
		sc2.Parameters[topolvm.DeviceClassKey] = v
		delete(sc2.Parameters, topolvm.LegacyDeviceClassKey)
	}
	return sc2
}
