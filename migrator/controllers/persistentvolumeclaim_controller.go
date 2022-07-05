package controllers

import (
	"context"
	"reflect"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	annBetaStorageProvisioner = "volume.beta.kubernetes.io/storage-provisioner"
	annStorageProvisioner     = "volume.kubernetes.io/storage-provisioner"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeClaimReconciler struct {
	client.Client
	APIReader client.Reader
}

// Reconcile finalize PVC
func (r *PersistentVolumeClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx).WithValues(
		"controller", "PersistentVolumeClaim",
		"name", req.NamespacedName.Name,
		"namespace", req.NamespacedName.Namespace)
	log.Info("Start Reconcile")
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, req.NamespacedName, pvc)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if pvc.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	var changed bool
	finalizers := []string{}
	var found bool
	for _, f := range pvc.Finalizers {
		if f != topolvm.LegacyPVCFinalizer {
			finalizers = append(finalizers, f)
		}

		if f == topolvm.PVCFinalizer {
			found = true
		}
	}

	if !found {
		finalizers = append(finalizers, topolvm.PVCFinalizer)
	}
	if !reflect.DeepEqual(finalizers, pvc.Finalizers) {
		changed = true
	}

	if pvc.Annotations[annStorageProvisioner] == topolvm.LegacyPluginName {
		pvc.Annotations[annStorageProvisioner] = topolvm.PluginName
		changed = true
	}
	if pvc.Annotations[annBetaStorageProvisioner] == topolvm.LegacyPluginName {
		pvc.Annotations[annBetaStorageProvisioner] = topolvm.PluginName
		changed = true
	}

	if !changed {
		log.V(6).Info("skipped updating")
		return ctrl.Result{}, err
	}

	pvc2 := pvc.DeepCopy()
	pvc2.Finalizers = finalizers
	if err := r.Update(ctx, pvc2); err != nil {
		log.Error(err, "failed to migrate finalizer")
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
