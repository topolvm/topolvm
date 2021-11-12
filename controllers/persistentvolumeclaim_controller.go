package controllers

import (
	"context"
	snapapi "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/topolvm/topolvm"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"time"
)

// PersistentVolumeClaimReconciler reconciles a PersistentVolumeClaim object
type PersistentVolumeClaimReconciler struct {
	client.Client
	APIReader client.Reader
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

	if pvc.DeletionTimestamp == nil {
		if err := r.addNodeSelectorWhenRestore(ctx, pvc); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	needFinalize := false
	onlyTopoLVM := true
	// Due to bug #310, we need to de-dup TopoLVM finalizers.
	for _, f := range pvc.Finalizers {
		if f == topolvm.PVCFinalizer {
			needFinalize = true
			continue
		}
		onlyTopoLVM = false
	}
	if !needFinalize {
		return ctrl.Result{}, nil
	}

	// Requeue until other finalizers complete their jobs.
	if !onlyTopoLVM {
		return ctrl.Result{
			Requeue:      true,
			RequeueAfter: 10 * time.Second,
		}, nil
	}

	pvc.Finalizers = nil
	if err := r.Update(ctx, pvc); err != nil {
		log.Error(err, "failed to remove finalizer", "name", pvc.Name)
		return ctrl.Result{}, err
	}

	// sleep shortly to wait StatefulSet controller notices PVC deletion
	time.Sleep(100 * time.Millisecond)

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
	// query directly to API server to avoid latency for cache updates
	err := r.APIReader.List(ctx, &pods, client.InNamespace(pvc.Namespace))
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

func (r *PersistentVolumeClaimReconciler) addNodeSelectorWhenRestore(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	if pvc.Annotations != nil && metav1.HasAnnotation(pvc.ObjectMeta, AnnSelectedNode) {
		return nil
	}

	if pvc.Spec.StorageClassName == nil {
		return nil
	}

	var sc storagev1.StorageClass
	err := r.Get(ctx, types.NamespacedName{Name: *pvc.Spec.StorageClassName}, &sc)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return nil
	default:
		return err
	}

	if sc.Provisioner != topolvm.PluginName {
		return nil
	}

	if pvc.Spec.DataSource != nil &&
		*pvc.Spec.DataSource.APIGroup == snapapi.SchemeGroupVersion.Group &&
		pvc.Spec.DataSource.Kind == "VolumeSnapshot" {

		var snap = &snapapi.VolumeSnapshot{}
		err = r.Get(ctx, types.NamespacedName{
			Name:      pvc.Spec.DataSource.Name,
			Namespace: pvc.Namespace,
		}, snap)
		if err != nil {
			return err
		}

		source := *snap.Spec.Source.PersistentVolumeClaimName
		sourcePVC := &corev1.PersistentVolumeClaim{}
		err = r.Get(ctx, types.NamespacedName{
			Name:      source,
			Namespace: pvc.Namespace,
		}, sourcePVC)
		if err != nil {
			return err
		}

		if sourcePVC.Annotations == nil || !metav1.HasAnnotation(sourcePVC.ObjectMeta, AnnSelectedNode) {
			return errors.Errorf("source pvc %s not have annotation %s", sourcePVC.Name, AnnSelectedNode)
		}

		metav1.SetMetaDataAnnotation(&pvc.ObjectMeta, AnnSelectedNode, sourcePVC.Annotations[AnnSelectedNode])
		if err := r.Update(ctx, pvc); err != nil {
			crlog.FromContext(ctx).Error(err, "failed to add annotation",
				"pvc", pvc.Name, "annotation", AnnSelectedNode)
			return err
		}
	}

	return nil
}

// SetupWithManager sets up Reconciler with Manager.
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
