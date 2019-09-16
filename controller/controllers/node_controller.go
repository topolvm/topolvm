package controllers

import (
	"context"

	"github.com/cybozu-go/topolvm"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods;persistentvolumeclaims,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="storage.k8s.io",resources=storageclasses,verbs=get;list;watch

// Reconcile finalize Node
func (r *NodeReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("node", req.NamespacedName)

	// your logic here
	node := &corev1.Node{}
	err := r.Get(ctx, req.NamespacedName, node)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if node.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	needFinalize := false
	for _, fin := range node.Finalizers {
		if fin == topolvm.NodeFinalizer {
			needFinalize = true
			break
		}
	}
	if !needFinalize {
		return ctrl.Result{}, nil
	}

	if err := r.doFinalize(ctx, log, node); err != nil {
		return ctrl.Result{}, err
	}

	node2 := node.DeepCopy()
	finalizers := node2.Finalizers[:0]
	for _, fin := range node.Finalizers {
		if fin == topolvm.NodeFinalizer {
			continue
		}
		finalizers = append(finalizers, fin)
	}
	node2.Finalizers = finalizers

	patch := client.MergeFrom(node)
	if err := r.Patch(ctx, node2, patch); err != nil {
		log.Error(err, "failed to remove finalizer", "name", node.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *NodeReconciler) targetStorageClasses(ctx context.Context) (map[string]bool, error) {
	var scl storagev1.StorageClassList
	if err := r.List(ctx, &scl); err != nil {
		return nil, err
	}

	targets := make(map[string]bool)
	for _, sc := range scl.Items {
		if sc.Provisioner != topolvm.PluginName {
			continue
		}
		targets[sc.Name] = true
	}
	return targets, nil
}

func (r *NodeReconciler) getPodsByPVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) ([]*corev1.Pod, error) {
	var pods corev1.PodList
	err := r.List(ctx, &pods, client.InNamespace(pvc.Namespace))
	if err != nil {
		return nil, err
	}

	var result []*corev1.Pod
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				result = append(result, &pod)
			}
		}
	}

	return result, nil
}

func (r *NodeReconciler) doFinalize(ctx context.Context, log logr.Logger, node *corev1.Node) error {
	scs, err := r.targetStorageClasses(ctx)
	if err != nil {
		log.Error(err, "unable to fetch StorageClass")
		return err
	}

	var pvcs corev1.PersistentVolumeClaimList
	err = r.List(ctx, &pvcs, client.MatchingField(KeySelectedNode, node.Name))
	if err != nil {
		log.Error(err, "unable to fetch PersistentVolumeClaimList")
		return err
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.StorageClassName == nil {
			continue
		}
		if !scs[*pvc.Spec.StorageClassName] {
			continue
		}

		pods, err := r.getPodsByPVC(ctx, &pvc)
		if err != nil {
			log.Error(err, "unable to fetch PodList for a PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)
			return err
		}

		for _, pod := range pods {
			err := r.Delete(ctx, pod, client.GracePeriodSeconds(1))
			if err != nil {
				log.Error(err, "unable to delete Pod", "name", pod.Name, "namespace", pod.Namespace)
				return err
			}
			log.Info("deleted Pod", "name", pod.Name, "namespace", pod.Namespace)
		}

		err = r.Delete(ctx, &pvc)
		if err != nil {
			log.Error(err, "unable to delete PVC", "name", pvc.Name, "namespace", pvc.Namespace)
			return err
		}
		log.Info("deleted PVC", "name", pvc.Name, "namespace", pvc.Namespace)
	}

	return nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Complete(r)
}
