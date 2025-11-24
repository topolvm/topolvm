package controller

import (
	"context"
	"fmt"

	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/snapshotengine/provider"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

// OnlineSnapshotStorageReconciler reconciles an OnlineSnapshotStorage object
type OnlineSnapshotStorageReconciler struct {
	client client.Client
}

// NewOnlineSnapshotStorageReconciler returns OnlineSnapshotStorageReconciler.
func NewOnlineSnapshotStorageReconciler(client client.Client) *OnlineSnapshotStorageReconciler {
	return &OnlineSnapshotStorageReconciler{
		client: client,
	}
}

//+kubebuilder:rbac:groups=topolvm.io,resources=onlinesnapshotstorages,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=topolvm.io,resources=onlinesnapshotstorages/status,verbs=get;update;patch

// Reconcile OnlineSnapshotStorage
func (r *OnlineSnapshotStorageReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	storage := &topolvmv1.OnlineSnapshotStorage{}
	err := r.client.Get(ctx, req.NamespacedName, storage)
	switch {
	case err == nil:
	case apierrors.IsNotFound(err):
		return ctrl.Result{}, nil
	default:
		log.Error(err, "unable to fetch OnlineSnapshotStorage")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if storage.DeletionTimestamp != nil {
		// OnlineSnapshotStorage doesn't need finalization logic for now
		// Resources should be cleaned up by dependent objects
		return ctrl.Result{}, nil
	}

	// Validate and update status
	if err := r.validateAndUpdateStatus(ctx, storage); err != nil {
		log.Error(err, "failed to validate and update status", "name", storage.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// validateAndUpdateStatus validates the OnlineSnapshotStorage configuration and updates its status
func (r *OnlineSnapshotStorageReconciler) validateAndUpdateStatus(ctx context.Context, target *topolvmv1.OnlineSnapshotStorage) error {
	log := crlog.FromContext(ctx)

	// If validation is not required on create and status is already set, skip validation
	if !target.Spec.ValidateOnCreate && target.Status.Phase != "" {
		return nil
	}

	// Create a copy for status update
	targetCopy := target.DeepCopy()
	now := metav1.Now()
	targetCopy.Status.LastChecked = &now

	// If ValidateOnCreate is true, validate backend storage connection
	if target.Spec.ValidateOnCreate {
		if err := r.validateBackendConnection(ctx, targetCopy); err != nil {
			targetCopy.Status.Phase = topolvmv1.PhaseError
			targetCopy.Status.Message = fmt.Sprintf("Storage storage connection validation failed: %v", err)
			log.Info("OnlineSnapshotStorage backend connection validation failed", "name", target.Name, "error", err)

			// Update status and return
			if err := r.client.Status().Update(ctx, targetCopy); err != nil {
				log.Error(err, "failed to update status", "name", target.Name)
				return err
			}
			return nil
		}
		log.Info("OnlineSnapshotStorage backend connection validation passed", "name", target.Name)
	}

	// All validations passed
	targetCopy.Status.Phase = topolvmv1.PhaseReady
	if target.Spec.ValidateOnCreate {
		targetCopy.Status.Message = "Storage configuration is valid and connection established"
	} else {
		targetCopy.Status.Message = "Storage configuration is valid"
	}
	log.Info("OnlineSnapshotStorage is ready", "name", target.Name)

	// Update status if changed
	if targetCopy.Status.Phase != target.Status.Phase ||
		targetCopy.Status.Message != target.Status.Message {
		if err := r.client.Status().Update(ctx, targetCopy); err != nil {
			log.Error(err, "failed to update status", "name", target.Name)
			return err
		}
	}

	return nil
}

// validateBackendConnection validates the actual connection to the backend storage
func (r *OnlineSnapshotStorageReconciler) validateBackendConnection(ctx context.Context, storage *topolvmv1.OnlineSnapshotStorage) error {
	// Get the appropriate snapshot engine
	engine, err := provider.GetProvider(r.client, storage)
	if err != nil {
		return fmt.Errorf("failed to get snapshot engine: %w", err)
	}

	// Validate connection to the backend
	if err := engine.ValidateConnection(ctx); err != nil {
		return fmt.Errorf("backend connection validation failed: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OnlineSnapshotStorageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&topolvmv1.OnlineSnapshotStorage{}).
		Complete(r)
}
