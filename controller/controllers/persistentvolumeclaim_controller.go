package controllers

import (
	"context"
	"time"

	"github.com/cybozu-go/topolvm"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeClaimReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups="",resources=pods;persistentvolumeclaims,verbs=get;list;watch;delete

// Reconcile finalize PVC
func (r *PersistentVolumeClaimReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("persistentvolumeclaim", req.NamespacedName)

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

	if pvc.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	needFinalize := false
	for _, fin := range pvc.Finalizers {
		if fin == topolvm.PVCFinalizer {
			needFinalize = true
			break
		}
	}
	if !needFinalize {
		return ctrl.Result{}, nil
	}

	if len(pvc.Finalizers) != 1 {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	pvc.Finalizers = []string{}
	if err := r.Update(ctx, pvc); err != nil {
		log.Error(err, "failed to remove finalizer", "name", pvc.Name)
		return ctrl.Result{}, err
	}

	pods, err := r.getPodsByPVC(ctx, pvc)
	if err != nil {
		log.Error(err, "unable to fetch PodList for a PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)
		return ctrl.Result{}, err
	}
	for _, pod := range pods {
		err := r.Delete(ctx, &pod, client.GracePeriodSeconds(1))
		if err != nil {
			log.Error(err, "unable to delete Pod", "name", pod.Name, "namespace", pod.Namespace)
			return ctrl.Result{}, err
		}
		log.Info("deleted Pod", "name", pod.Name, "namespace", pod.Namespace)
	}

	return ctrl.Result{}, nil
}

func (r *PersistentVolumeClaimReconciler) getPodsByPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) ([]corev1.Pod, error) {
	var pods corev1.PodList
	err := r.List(ctx, &pods, client.InNamespace(pvc.Namespace))
	if err != nil {
		return nil, err
	}

	var result []corev1.Pod
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim == nil {
				continue
			}
			if volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				result = append(result, pod)
			}
		}
	}

	return result, nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *PersistentVolumeClaimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
