package controllers

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
}

func (r *LogicalVolumeReconciler) RunOnce(ctx context.Context) error {
	lvs := &topolvmlegacyv1.LogicalVolumeList{}
	err := r.List(ctx, lvs)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return nil
	default:
		return err
	}

	for _, node := range lvs.Items {
		_, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      node.Name,
			},
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// Reconcile finalize Node
func (r *LogicalVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)
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
		return ctrl.Result{}, nil
	}

	newLV := &topolvmlegacyv1.LogicalVolume{}
	err = r.Get(ctx, req.NamespacedName, newLV)
	switch {
	case apierrors.IsNotFound(err):
	case err == nil:
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	newLV2 := generateNewLogicalVolume(lv)
	if err := r.Create(ctx, newLV2); err != nil {
		log.Error(err, "failed to migrate logical volume", "name", req.NamespacedName.Name, "namespace", req.NamespacedName.Namespace)
		return ctrl.Result{}, err
	}
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
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&topolvmlegacyv1.LogicalVolume{}).
		Complete(r)
}
