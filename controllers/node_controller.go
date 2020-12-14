package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;delete
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

	if node.DeletionTimestamp == nil {
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

	if result, err := r.doFinalize(ctx, log, node); result.Requeue || err != nil {
		return result, err
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

func (r *NodeReconciler) doFinalize(ctx context.Context, log logr.Logger, node *corev1.Node) (ctrl.Result, error) {
	scs, err := r.targetStorageClasses(ctx)
	if err != nil {
		log.Error(err, "unable to fetch StorageClass")
		return ctrl.Result{}, err
	}

	var pvcs corev1.PersistentVolumeClaimList
	err = r.List(ctx, &pvcs, client.MatchingFields{KeySelectedNode: node.Name})
	if err != nil {
		log.Error(err, "unable to fetch PersistentVolumeClaimList")
		return ctrl.Result{}, err
	}

	for _, pvc := range pvcs.Items {
		if pvc.Spec.StorageClassName == nil {
			continue
		}
		if !scs[*pvc.Spec.StorageClassName] {
			continue
		}

		err = r.Delete(ctx, &pvc)
		if err != nil {
			log.Error(err, "unable to delete PVC", "name", pvc.Name, "namespace", pvc.Namespace)
			return ctrl.Result{}, err
		}
		log.Info("deleted PVC", "name", pvc.Name, "namespace", pvc.Namespace)
	}

	lvList := new(topolvmv1.LogicalVolumeList)
	err = r.List(ctx, lvList, client.MatchingFields{KeyLogicalVolumeNode: node.Name})
	if err != nil {
		log.Error(err, "failed to get LogicalVolumes")
		return ctrl.Result{}, err
	}

	for _, lv := range lvList.Items {
		err = r.cleanupLogicalVolume(ctx, log, &lv)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *NodeReconciler) cleanupLogicalVolume(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	finExists := false
	for _, fin := range lv.Finalizers {
		if fin == topolvm.LogicalVolumeFinalizer {
			finExists = true
			break
		}
	}

	if finExists {
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
	}

	err := r.Delete(ctx, lv)
	if err != nil {
		log.Error(err, "failed to delete LogicalVolume", "name", lv.Name)
		return err
	}

	log.Info("deleted LogicalVolume", "name", lv.Name)
	return nil
}

// SetupWithManager sets up Reconciler with Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()
	err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.PersistentVolumeClaim{}, KeySelectedNode, func(o runtime.Object) []string {
		return []string{o.(*corev1.PersistentVolumeClaim).Annotations[AnnSelectedNode]}
	})
	if err != nil {
		return err
	}

	err = mgr.GetFieldIndexer().IndexField(ctx, &topolvmv1.LogicalVolume{}, KeyLogicalVolumeNode, func(o runtime.Object) []string {
		return []string{o.(*topolvmv1.LogicalVolume).Spec.NodeName}
	})
	if err != nil {
		return err
	}

	pred := predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&corev1.Node{}).
		Complete(r)
}
