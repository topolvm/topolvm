package controllers

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
)

// LogicalVolumeReconciler reconciles a LogicalVolume object
type LogicalVolumeReconciler struct {
	client.Client
	APIReader client.Reader
	NodeName  string
}

// Reconcile finalize Node
func (r *LogicalVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx).WithValues(
		"controller", "LogicalVolume",
		"name", req.NamespacedName.Name,
		"namespace", req.NamespacedName.Namespace)
	log.Info("Start Reconcile")

	// check legacy logicalvolume
	lv := &topolvmlegacyv1.LogicalVolume{}
	err := r.Get(ctx, req.NamespacedName, lv)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if lv.DeletionTimestamp != nil {
		log.Info("DeletionTimestamp is not nil", "logicalvolume", lv)
		return ctrl.Result{}, nil
	}

	if lv.Spec.NodeName != r.NodeName {
		log.Info("unfiltered logical value", "nodeName", lv.Spec.NodeName)
		return ctrl.Result{}, nil
	}

	// check new logicalvolume
	newLV := &topolvmv1.LogicalVolume{}
	err = r.Get(ctx, req.NamespacedName, newLV)
	switch {
	case apierrors.IsNotFound(err):
	case err == nil:
		// return if the new logicalvolume already exists
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	log.Info("Start migration")
	newLV = generateNewLogicalVolume(lv)
	if err := r.Create(ctx, newLV); err != nil {
		log.Error(err, "failed to migrate: create a new logical volume", "name", req.NamespacedName.Name, "namespace", req.NamespacedName.Namespace)
		return ctrl.Result{}, err
	}
	log.Info("the new logicalvolume was created")

	if err := r.Delete(ctx, lv); err != nil {
		log.Error(err, "failed to migrate: delete the legacy logical volume", "name", req.NamespacedName.Name, "namespace", req.NamespacedName.Namespace)
		return ctrl.Result{}, err
	}
	log.Info("the legacy logicalvolume was deleted, migration is complete")
	return ctrl.Result{}, nil
}

func generateNewLogicalVolume(lv *topolvmlegacyv1.LogicalVolume) *topolvmv1.LogicalVolume {
	newLV := &topolvmv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        lv.Name,
			Namespace:   lv.Namespace,
			Labels:      lv.Labels,
			Annotations: lv.Annotations,
		},
		Spec: topolvmv1.LogicalVolumeSpec{
			Name:        lv.Spec.Name,
			NodeName:    lv.Spec.NodeName,
			Size:        lv.Spec.Size,
			DeviceClass: lv.Spec.DeviceClass,
			Source:      lv.Spec.Source,
			AccessType:  lv.Spec.AccessType,
		},
		Status: topolvmv1.LogicalVolumeStatus{
			VolumeID:    lv.Status.VolumeID,
			Code:        lv.Status.Code,
			Message:     lv.Status.Message,
			CurrentSize: lv.Status.CurrentSize,
		},
	}

	if val, ok := newLV.Annotations[topolvm.LegacyResizeRequestedAtKey]; ok {
		newLV.Annotations[topolvm.ResizeRequestedAtKey] = val
		delete(newLV.Annotations, topolvm.LegacyResizeRequestedAtKey)
	}

	finalizers := []string{}
	for _, val := range lv.Finalizers {
		if val != topolvm.LegacyLogicalVolumeFinalizer {
			finalizers = append(finalizers, val)
		}
	}
	finalizers = append(finalizers, topolvm.LogicalVolumeFinalizer)
	newLV.Finalizers = finalizers
	return newLV
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogicalVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return filterLogicalVolume(e.Object.(*topolvmlegacyv1.LogicalVolume), r.NodeName)
		},
		DeleteFunc: func(event.DeleteEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return filterLogicalVolume(e.ObjectNew.(*topolvmlegacyv1.LogicalVolume), r.NodeName)
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&topolvmlegacyv1.LogicalVolume{}).
		Complete(r)
}

func filterLogicalVolume(lv *topolvmlegacyv1.LogicalVolume, nodename string) bool {
	if lv == nil {
		return false
	}
	if lv.Spec.NodeName == nodename {
		return true
	}
	return false
}
