package controller

import (
	"context"
	"time"

	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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

// NewPersistentVolumeClaimReconciler returns PersistentVolumeClaimReconciler.
func NewPersistentVolumeClaimReconciler(client client.Client, apiReader client.Reader) *PersistentVolumeClaimReconciler {
	return &PersistentVolumeClaimReconciler{
		client:    client,
		apiReader: apiReader,
	}
}

//+kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch

// Reconcile PVC
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
			RequeueAfter: requeueIntervalForSimpleUpdate,
		}, nil
	}

	if pvc.DeletionTimestamp == nil {
		err := r.notifyKubeletToResizeFS(ctx, pvc)
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(pvc, topolvm.PVCFinalizer)

	if err := r.client.Update(ctx, pvc); err != nil {
		log.Error(err, "failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PersistentVolumeClaimReconciler) notifyKubeletToResizeFS(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	// kubelet periodically checks pods to resize fs but it may take a long time.
	// To speed up the process, mutate pods by adding an annotation.
	// This process is only done when the PVC has `FileSystemResizePending` condition
	// and pods are not mutated yet.
	log := crlog.FromContext(ctx)
	var condTransitionAt time.Time
	for _, cond := range pvc.Status.Conditions {
		if cond.Type == corev1.PersistentVolumeClaimFileSystemResizePending && cond.Status == corev1.ConditionTrue {
			isRelated, err := r.isPVCRelated(ctx, pvc)
			if err != nil {
				return err
			}
			if isRelated {
				condTransitionAt = cond.LastTransitionTime.Time
				goto needResizing
			}
		}
	}
	return nil

needResizing:
	pods, err := r.getPodsByPVC(ctx, pvc)
	if err != nil {
		return err
	}
	for _, pod := range pods {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}

		// If the parsing fails, someone or a bug may set a wrong value. In this case, we set the annotation to fix the value.
		lastResizeFSRequestedAt, err := time.Parse(time.RFC3339Nano, pod.Annotations[topolvm.LastResizeFSRequestedAtKey])
		if err == nil && lastResizeFSRequestedAt.After(condTransitionAt) {
			continue
		}

		pod.Annotations[topolvm.LastResizeFSRequestedAtKey] = time.Now().Format(time.RFC3339Nano)
		err = r.client.Update(ctx, &pod)
		if err != nil {
			log.Error(err, "unable to annotate Pod", "name", pod.Name, "namespace", pod.Namespace)
			return err
		}
	}
	return nil
}

func (r *PersistentVolumeClaimReconciler) isPVCRelated(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (bool, error) {
	log := crlog.FromContext(ctx)

	if pvc.Spec.StorageClassName == nil {
		return false, nil
	}

	var scl storagev1.StorageClassList
	err := r.client.List(ctx, &scl)
	if err != nil {
		log.Error(err, "unable to fetch StorageClassList")
		return false, err
	}
	for _, sc := range scl.Items {
		if *pvc.Spec.StorageClassName == sc.Name &&
			sc.Provisioner == topolvm.GetPluginName() {
			return true, nil
		}
	}
	return false, nil
}

func (r *PersistentVolumeClaimReconciler) getPodsByPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) ([]corev1.Pod, error) {
	log := crlog.FromContext(ctx)
	var pods corev1.PodList
	// query directly to API server to avoid latency for cache updates
	err := r.apiReader.List(ctx, &pods, client.InNamespace(pvc.Namespace))
	if err != nil {
		log.Error(err, "unable to fetch PodList for a PVC")
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
