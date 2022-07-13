package controllers

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/go-logr/logr"
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

const (
	PVProtectionFinalizer     = "kubernetes.io/pv-protection"
	AnnDynamicallyProvisioned = "pv.kubernetes.io/provisioned-by"
	provisionerIDKey          = "storage.kubernetes.io/csiProvisionerIdentity"

	ANNPVOriginalPolicy = topolvm.PluginName + "/pv-migrate-original-policy"
	ANNPVProtection     = topolvm.PluginName + "/pv-migrate-protection"
)

// PersistentVolumeReconciler reconciles a PersistentVolume object
type PersistentVolumeReconciler struct {
	client.Client
	APIReader client.Reader
}

// Reconcile finalize PVC
func (r *PersistentVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx).WithValues(
		"controller", "PersistentVolume",
		"name", req.NamespacedName.Name)
	log.Info("Start Reconcile")
	pv := &corev1.PersistentVolume{}
	err := r.Get(ctx, req.NamespacedName, pv)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, err
	}

	if !filterPV(pv) {
		return ctrl.Result{}, nil
	}

	if pv.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, log, pv)
}

func (r *PersistentVolumeReconciler) reconcile(ctx context.Context, log logr.Logger, pv *corev1.PersistentVolume) (ctrl.Result, error) {
	log.Info("start migration")
	pvcopy := migratePV(pv)

	// step1: update PV
	pv2 := pv.DeepCopy()
	var changed bool
	if pv2.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
		pv2.Annotations[ANNPVOriginalPolicy] = string(pv2.Spec.PersistentVolumeReclaimPolicy)
		pv2.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		changed = true
	}

	if !changed {
		log.Info("skip update pv")
	} else {
		log.Info("update pv")
		if err := r.Update(ctx, pv2); err != nil {
			log.Error(err, "failed to update PV")
			return ctrl.Result{}, err
		}

		pv2 = &corev1.PersistentVolume{}
		err := r.Get(ctx, types.NamespacedName{Name: pv.Name}, pv2)
		switch {
		case err == nil:
		case apierrors.IsNotFound(err):
			return ctrl.Result{}, nil
		default:
			return ctrl.Result{}, err
		}
	}

	// step2: delete PV
	log.Info("delete pv")
	if err := r.Delete(ctx, pv2, client.GracePeriodSeconds(0)); err != nil {
		log.Error(err, "failed to delete PV")
		return ctrl.Result{}, err
	}

	// step3: re-create PV
	log.Info("re-create pv")
	if err := r.Create(ctx, pvcopy); err != nil {
		data, _ := json.Marshal(pvcopy)
		log.Error(err, "failed to re-create PV", "PersistentVolume", data)
		return ctrl.Result{}, err
	}

	log.Info("complete migration")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PersistentVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	pred := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return filterPV(e.Object.(*corev1.PersistentVolume))
		},
		DeleteFunc: func(event.DeleteEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return filterPV(e.ObjectNew.(*corev1.PersistentVolume))
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(pred).
		For(&corev1.PersistentVolume{}).
		Complete(r)
}

func filterPV(pv *corev1.PersistentVolume) bool {
	if pv == nil {
		return false
	}
	if pv.Spec.CSI == nil || pv.Spec.CSI.Driver != topolvm.LegacyPluginName {
		return false
	}
	return true
}

func migratePV(pv *corev1.PersistentVolume) *corev1.PersistentVolume {
	pv2 := pv.DeepCopy()
	pv2.ResourceVersion = ""
	pv2.UID = ""
	pv2.Annotations[AnnDynamicallyProvisioned] = topolvm.PluginName
	pv2.Spec.CSI.Driver = topolvm.PluginName
	pv2.Spec.CSI.VolumeAttributes[provisionerIDKey] = identity(pv2.Spec.CSI.VolumeAttributes[provisionerIDKey])

	if original, ok := pv2.Annotations[ANNPVOriginalPolicy]; ok {
		pv2.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimPolicy(original)
		delete(pv2.Annotations, ANNPVOriginalPolicy)
	}

	var found bool
	for _, f := range pv2.Finalizers {
		if f == PVProtectionFinalizer {
			found = true
		}
	}
	if _, ok := pv2.Annotations[ANNPVProtection]; ok {
		delete(pv2.Annotations, ANNPVProtection)
		if !found {
			pv2.Finalizers = append(pv2.Finalizers, PVProtectionFinalizer)
		}
	}

	if pv2.Spec.NodeAffinity != nil && pv2.Spec.NodeAffinity.Required != nil {
		nodeSelectorTerms := []corev1.NodeSelectorTerm{}
		for _, term := range pv2.Spec.NodeAffinity.Required.NodeSelectorTerms {
			matchExpressions := []corev1.NodeSelectorRequirement{}
			for _, e := range term.MatchExpressions {
				if e.Key == topolvm.LegacyTopologyNodeKey {
					e.Key = topolvm.TopologyNodeKey
				}
				matchExpressions = append(matchExpressions, e)
			}
			term.MatchExpressions = matchExpressions
			nodeSelectorTerms = append(nodeSelectorTerms, term)
		}
		pv2.Spec.NodeAffinity.Required.NodeSelectorTerms = nodeSelectorTerms
	}
	return pv2
}

func identity(original string) string {
	names := strings.Split(original, topolvm.LegacyPluginName)
	if len(names) != 2 {
		return original
	}
	return names[0] + topolvm.PluginName
}
