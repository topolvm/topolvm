package controller

import (
	"context"
	"time"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeClaimReconciler struct {
	client    client.Client
	apiReader client.Reader
}

// NewPersistentVolumeClaimReconciler returns NodeReconciler.
func NewPersistentVolumeClaimReconciler(client client.Client, apiReader client.Reader) *PersistentVolumeClaimReconciler {
	return &PersistentVolumeClaimReconciler{
		client:    client,
		apiReader: apiReader,
	}
}

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;delete

// Reconcile finalize PVC
//
// This was originally created as a workaround for the following issue.
// https://github.com/kubernetes/kubernetes/pull/93457
//
// Because the issue was fixed, the PVC reconciler was once removed by PR #536.
// However, it turned out that the PVC finalizer was also useful to
// resolve the issue that a pod created from StatefulSet persists in the PENDING state
// when PVC is deleted for some reasons like node deletion.
// Thus, the reconciler was revived by PR #620.
func (r *PersistentVolumeClaimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
	// your logic here
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(ctx, req.NamespacedName, pvc)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	// Remove deprecated finalizer and requeue.
	removed, err := r.removeDeprecatedFinalizer(ctx, pvc)
	if err != nil {
		log.Error(err, "failed to remove deprecated finalizer", "name", pvc.Name)
		return ctrl.Result{}, err
	} else if removed {
		return ctrl.Result{
			Requeue: true,
		}, nil
	}

	// Skip if the PVC is not deleted or PVC does not have TopoLVM's finalizer.
	if pvc.DeletionTimestamp == nil {
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(pvc, topolvm.PVCFinalizer) {
		return ctrl.Result{}, nil
	}

	// Delete the pods that are using the PV/PVC.
	pods, err := r.getPodsByPVC(ctx, pvc)
	if err != nil {
		log.Error(err, "unable to fetch PodList for a PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)
		return ctrl.Result{}, err
	}
	for _, pod := range pods {
		err := r.client.Delete(ctx, &pod, client.GracePeriodSeconds(1))
		if err != nil {
			log.Error(err, "unable to delete Pod", "name", pod.Name, "namespace", pod.Namespace)
			return ctrl.Result{}, err
		}
		log.Info("deleted Pod", "name", pod.Name, "namespace", pod.Namespace)
	}

	// Requeue until other finalizers complete their jobs.
	if len(pvc.Finalizers) != 1 {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	controllerutil.RemoveFinalizer(pvc, topolvm.PVCFinalizer)

	if err := r.client.Update(ctx, pvc); err != nil {
		log.Error(err, "failed to remove finalizer", "name", pvc.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PersistentVolumeClaimReconciler) getPodsByPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) ([]corev1.Pod, error) {
	var pods corev1.PodList
	// query directly to API server to avoid latency for cache updates
	err := r.apiReader.List(ctx, &pods, client.InNamespace(pvc.Namespace))
	if err != nil {
		return nil, err
	}

	var result []corev1.Pod
OUTER:
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim == nil {
				continue
			}
			if volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				result = append(result, pod)
				continue OUTER
			}
		}
	}

	return result, nil
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

func (r *PersistentVolumeClaimReconciler) removeDeprecatedFinalizer(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	// Due to the bug #310, multiple TopoLVM finalizers can exist in `pvc.Finalizers`.
	// So we need to delete all of them.
	removed := controllerutil.RemoveFinalizer(pvc, topolvm.LegacyPVCFinalizer)
	if removed {
		if err := r.client.Update(ctx, pvc); err != nil {
			return false, err
		}
	}
	return removed, nil
}
