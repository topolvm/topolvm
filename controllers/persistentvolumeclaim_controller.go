package controllers

import (
	"context"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeClaimReconciler struct {
	client.Client
}

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete

// Reconcile finalize PVC
func (r *PersistentVolumeClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	// your logic here
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, req.NamespacedName, pvc)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	err = r.removeTopoLVMFinalizer(ctx, pvc)
	if err != nil {
		log.Error(err, "failed to remove finalizer", "name", pvc.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentVolumeClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

func (r *PersistentVolumeClaimReconciler) removeTopoLVMFinalizer(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	removed := false
	// Due to the bug #310, multiple TopoLVM finalizers can exist in `pvc.Finalizers`.
	// So we need to delete all of them.
	for i := 0; i < len(pvc.Finalizers); {
		if pvc.Finalizers[i] == topolvm.PVCFinalizer {
			pvc.Finalizers = append(pvc.Finalizers[:i], pvc.Finalizers[i+1:]...)
			removed = true
			continue
		}
		i++
	}
	if removed {
		if err := r.Update(ctx, pvc); err != nil {
			return err
		}
	}
	return nil
}
