package controllers

import (
	"context"
	"math/rand"
	"strconv"
	"time"

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

	ANNPVMigrate = topolvm.PluginName + "pv-migrate"

	MigrateStatusUpdateRetaintPolicy = "retained"
	MigrateStatusDeletePV            = "deleted" // どのように再開するの？
	MigrateStatusComplete            = "complete"
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

	if pv.Spec.CSI.Driver != topolvm.LegacyPluginName {
		return ctrl.Result{}, nil
	}

	if pv.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	return r.reconcile(ctx, log, pv)
}

// TODO 途中でエラーになった際の処理再開をどうするか検討する
func (r *PersistentVolumeReconciler) reconcile(ctx context.Context, log logr.Logger, pv *corev1.PersistentVolume) (ctrl.Result, error) {
	// 1. save original pv copy
	pvcopy := copyPV(pv)

	// 2. change reclaim policy to Retain if needed
	if pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimDelete {
		pv2 := pv.DeepCopy()
		pv2.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		pv2.Annotations[ANNPVMigrate] = MigrateStatusUpdateRetaintPolicy
		if err := r.Update(ctx, pv2); err != nil {
			log.Error(err, "failed to update reclaim policy")
			return ctrl.Result{}, err
		}
		pv = pv2
	}

	// 3. delete pv
	name := types.NamespacedName{Name: pv.Name}
	needDeleteFinalizer := contain(pv.Finalizers, PVProtectionFinalizer)
	if err := r.Delete(ctx, pv); err != nil {
		log.Error(err, "failed to delete pv")
		return ctrl.Result{}, err
	}

	// 4. delete pv finalizer if needed
	if needDeleteFinalizer {
		pv = &corev1.PersistentVolume{}
		err := r.Get(ctx, name, pv)
		switch {
		case err == nil:
		case apierrors.IsNotFound(err):
			// TODO not foundでれば削除されているということなので次に行く
			goto Recreate
		default:
			log.Error(err, "failed to get pv")
			return ctrl.Result{}, err
		}

		var finalizer []string
		for _, v := range pv.Finalizers {
			if v != PVProtectionFinalizer {
				finalizer = append(finalizer, v)
			}
		}
		pv2 := pv.DeepCopy()
		pv2.Finalizers = finalizer
		if err := r.Update(ctx, pv2); err != nil {
			log.Error(err, "failed to update finalizer")
			return ctrl.Result{}, err
		}
	}

Recreate:
	// 5. recreate pv
	if err := r.Create(ctx, pvcopy); err != nil {
		log.Error(err, "failed to recreate pv")
		return ctrl.Result{}, err
	}
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
	return pv.Annotations[ANNPVMigrate] != MigrateStatusComplete
}

func contain(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}
	return false
}

func copyPV(pv *corev1.PersistentVolume) *corev1.PersistentVolume {
	pv2 := pv.DeepCopy()
	pv2.Annotations[AnnDynamicallyProvisioned] = topolvm.PluginName
	pv2.Spec.CSI.Driver = topolvm.PluginName
	pv2.Spec.CSI.Driver = topolvm.PluginName
	pv2.Spec.CSI.VolumeAttributes[provisionerIDKey] = identity(topolvm.PluginName)
	pv2.Annotations[ANNPVMigrate] = MigrateStatusComplete
	return pv2
}

func identity(provisionerName string) string {
	timeStamp := time.Now().UnixNano() / int64(time.Millisecond)
	return strconv.FormatInt(timeStamp, 10) + "-" + strconv.Itoa(rand.Intn(10000)) + "-" + provisionerName
}
