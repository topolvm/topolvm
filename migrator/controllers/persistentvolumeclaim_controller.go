package controllers

import (
	"context"
	"reflect"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeClaimReconciler struct {
	client.Client
	APIReader client.Reader
}

func (r *PersistentVolumeClaimReconciler) RunOnce(ctx context.Context) error {
	log := crlog.FromContext(ctx).WithValues("controller", "PersistentVolumeClaim")
	log.Info("Start RunOnce")
	pvcs := &corev1.PersistentVolumeClaimList{}
	err := r.List(ctx, pvcs)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return nil
	default:
		return err
	}

	for _, pvc := range pvcs.Items {
		_, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
			},
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// Reconcile finalize PVC
func (r *PersistentVolumeClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx).WithValues("controller", "PersistentVolumeClaim")
	log.Info("Start Reconcile", "name", req.NamespacedName.Name, "namespace", req.NamespacedName.Namespace)
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
	if reflect.DeepEqual(finalizers, pvc.Finalizers) {
		log.V(6).Info("skipped updating finalizer", "name", pvc.Name)
		return ctrl.Result{}, nil
	}

	pvc2 := pvc.DeepCopy()
	pvc2.Finalizers = finalizers
	if err := r.Update(ctx, pvc2); err != nil {
		log.Error(err, "failed to migrate finalizer", "name", pvc2.Name)
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
