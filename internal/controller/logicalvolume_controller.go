package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	snapshot_api "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"github.com/topolvm/topolvm"
	topolvmlegacyv1 "github.com/topolvm/topolvm/api/legacy/v1"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/executor"
	"github.com/topolvm/topolvm/internal/mounter"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

// LogicalVolumeReconciler reconciles a LogicalVolume object
type LogicalVolumeReconciler struct {
	client    client.Client
	nodeName  string
	vgService proto.VGServiceClient
	lvService proto.LVServiceClient
	lvMount   *mounter.LVMount
	executor  executor.Executor
}

//+kubebuilder:rbac:groups=topolvm.io,resources=logicalvolumes,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=topolvm.io,resources=logicalvolumes/status,verbs=get;update;patch

func NewLogicalVolumeReconcilerWithServices(client client.Client, nodeName string, vgService proto.VGServiceClient, lvService proto.LVServiceClient) *LogicalVolumeReconciler {
	return &LogicalVolumeReconciler{
		client:    client,
		nodeName:  nodeName,
		vgService: vgService,
		lvService: lvService,
		lvMount:   mounter.NewLVMount(client, vgService, lvService),
	}
}

// Reconcile creates/deletes LVM logical volume for a LogicalVolume.
func (r *LogicalVolumeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := crlog.FromContext(ctx)

	lv := new(topolvmv1.LogicalVolume)
	if err := r.client.Get(ctx, req.NamespacedName, lv); err != nil {
		if !apierrs.IsNotFound(err) {
			log.Error(err, "unable to fetch LogicalVolume")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	if lv.Spec.NodeName != r.nodeName {
		log.Info("unfiltered logical value", "nodeName", lv.Spec.NodeName)
		return ctrl.Result{}, nil
	}

	if lv.Annotations != nil {
		_, pendingDeletion := lv.Annotations[topolvm.GetLVPendingDeletionKey()]
		if pendingDeletion {
			if controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
				log.Error(nil, "logical volume was pending deletion but still has finalizer", "name", lv.Name)
			} else {
				log.Info("skipping finalizer for logical volume due to its pending deletion", "name", lv.Name)
			}
			return ctrl.Result{}, nil
		}
	}

	if lv.DeletionTimestamp == nil {
		if !controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
			lv2 := lv.DeepCopy()
			controllerutil.AddFinalizer(lv2, topolvm.GetLogicalVolumeFinalizer())
			patch := client.MergeFrom(lv)
			if err := r.client.Patch(ctx, lv2, patch); err != nil {
				log.Error(err, "failed to add finalizer", "name", lv.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: requeueIntervalForSimpleUpdate}, nil
		}

		if !containsKeyAndValue(lv.Labels, topolvm.CreatedbyLabelKey, topolvm.CreatedbyLabelValue) {
			lv2 := lv.DeepCopy()
			if lv2.Labels == nil {
				lv2.Labels = map[string]string{}
			}
			lv2.Labels[topolvm.CreatedbyLabelKey] = topolvm.CreatedbyLabelValue
			patch := client.MergeFrom(lv)
			if err := r.client.Patch(ctx, lv2, patch); err != nil {
				log.Error(err, "failed to add label", "name", lv.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{RequeueAfter: requeueIntervalForSimpleUpdate}, nil
		}

		sourceLV, err := r.getSourceLogicalVolume(ctx, lv)
		if err != nil {
			log.Error(err, "failed to get source snapshot LV", "name", lv.Name)
			return ctrl.Result{}, err
		}

		var (
			shouldRestore bool
			vsClass       *snapshot_api.VolumeSnapshotClass
			vsContent     *snapshot_api.VolumeSnapshotContent
		)
		// -----------------------------------------------------------------------------
		// 1. SOURCE LV (Snapshot restore input)
		// -----------------------------------------------------------------------------
		if sourceLV != nil {
			log.Info("snapshot LV found", "name", sourceLV.Name)

			vsContent, vsClass, err = r.getVolumeSnapshotResources(ctx, sourceLV)
			if err != nil {
				log.Error(err, "failed to get VolumeSnapshotContent/VolumeSnapshotClass", "name", lv.Name)
				return ctrl.Result{}, err
			}

			shouldRestore, err = r.shouldPerformSnapshotRestore(sourceLV, vsClass)
			if err != nil {
				log.Error(err, "failed to check whether to restore from snapshot", "name", lv.Name)
				return ctrl.Result{}, err
			}
		}

		// -----------------------------------------------------------------------------
		// 2. CREATE or EXPAND LV
		// -----------------------------------------------------------------------------
		if lv.Status.VolumeID == "" {
			if err := r.createLV(ctx, log, lv, shouldRestore); err != nil {
				log.Error(err, "failed to create LV", "name", lv.Name)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		if err := r.expandLV(ctx, log, lv); err != nil {
			log.Error(err, "failed to expand LV", "name", lv.Name)
			return ctrl.Result{}, err
		}

		// -----------------------------------------------------------------------------
		// 3. SNAPSHOT RESTORE LOGIC
		// -----------------------------------------------------------------------------
		if shouldRestore {
			exists, err := r.isPersistentVolumeExist(lv)
			if err != nil {
				log.Error(err, "failed to check PV existence", "name", lv.Name)
				return ctrl.Result{}, err
			}

			if !exists {
				log.Info("PV does not exist yet; waiting", "name", lv.Name)
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}

			if err := r.performSnapshotRestore(ctx, log, lv, sourceLV, vsClass); err != nil {
				return ctrl.Result{}, err
			}

			if r.isSnapshotOperationComplete(lv) {
				log.Info("snapshot restore completed successfully", "name", lv.Name)

				if err := r.lvMount.Unmount(ctx, lv); err != nil {
					log.Error(err, "failed to unmount LV", "name", lv.Name)
					return ctrl.Result{}, err
				}

				log.Info("successfully unmounted LV", "name", lv.Name, "uid", lv.UID)

				if !hasSnapshotRestoreExecutorCleanupCondition(lv) {
					if err := r.executeCleanerOperation(ctx, lv, topolvmv1.OperationRestore, log); err != nil {
						log.Error(err, "failed to execute cleaner operation", "name", lv.Name)
						return ctrl.Result{}, err
					}
				}
			}
		}

		// -----------------------------------------------------------------------------
		// 4. SNAPSHOT BACKUP LOGIC
		// -----------------------------------------------------------------------------
		vsContent, vsClass, err = r.getVolumeSnapshotResources(ctx, lv)
		if err != nil {
			log.Error(err, "failed to get VolumeSnapshotContent/VolumeSnapshotClass", "name", lv.Name)
			return ctrl.Result{}, err
		}

		shouldBackup, err := r.shouldPerformSnapshotBackup(vsClass)
		if err != nil {
			log.Error(err, "failed to check whether to perform snapshot backup", "name", lv.Name)
			return ctrl.Result{}, err
		}

		if shouldBackup {
			if err := r.performSnapshotBackup(ctx, log, lv, vsContent, vsClass); err != nil {
				return ctrl.Result{}, err
			}
			if r.isSnapshotOperationComplete(lv) &&
				!hasSnapshotBackupExecutorCleanupCondition(lv) {
				if err := r.executeCleanerOperation(ctx, lv, topolvmv1.OperationBackup, log); err != nil {
					log.Error(err, "failed to execute cleaner operation", "name", lv.Name)
					return ctrl.Result{}, err
				}
			}
		}
		return ctrl.Result{}, nil
	}

	// finalization
	if !controllerutil.ContainsFinalizer(lv, topolvm.GetLogicalVolumeFinalizer()) {
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}
	log.Info("start finalizing LogicalVolume", "name", lv.Name)
	err := r.removeLVIfExists(ctx, log, lv)
	if err != nil {
		return ctrl.Result{}, err
	}

	if r.isLVHasSnapshot(lv) {
		if !hasSnapshotDeleteExecutorCondition(lv) {
			if err := r.executeSnapshotDeleteOperation(ctx, lv, log); err != nil {
				return ctrl.Result{}, err
			}
		}

		if hasConditionSnapshotDeleteSucceeded(lv) {
			if err := r.removeFinalizerAndDeleteLV(ctx, log, lv); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	if err := r.removeFinalizerAndDeleteLV(ctx, log, lv); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *LogicalVolumeReconciler) removeFinalizerAndDeleteLV(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {

	fmt.Println("######################### Why I am here ############# LogicalVolume")
	lv2 := lv.DeepCopy()
	controllerutil.RemoveFinalizer(lv2, topolvm.GetLogicalVolumeFinalizer())
	patch := client.MergeFrom(lv)
	if err := r.client.Patch(ctx, lv2, patch); err != nil {
		log.Error(err, "failed to remove finalizer", "name", lv.Name)
		return err
	}
	return nil
}

func (r *LogicalVolumeReconciler) isLVHasSnapshot(lv *topolvmv1.LogicalVolume) bool {
	if lv.Status.Snapshot == nil {
		return false
	}
	return lv.Status.Snapshot.Phase == topolvmv1.OperationPhaseSucceeded &&
		lv.Status.Snapshot.SnapshotID != ""

}

func (r *LogicalVolumeReconciler) isPersistentVolumeExist(lv *topolvmv1.LogicalVolume) (bool, error) {
	if lv.Spec.Name == "" {
		return false, nil
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: lv.Spec.Name,
		},
	}
	err := r.client.Get(context.Background(), client.ObjectKeyFromObject(pv), pv)
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil
}

func (r *LogicalVolumeReconciler) getVolumeSnapshotResources(ctx context.Context, lv *topolvmv1.LogicalVolume) (*snapshot_api.VolumeSnapshotContent, *snapshot_api.VolumeSnapshotClass, error) {
	content, err := r.getVolumeSnapshotContentIfExists(ctx, lv)
	if err != nil {
		return nil, nil, client.IgnoreNotFound(err)
	}
	if content == nil {
		return nil, nil, nil
	}
	vsClass, err := r.getVolumeSnapshotClassFromContent(ctx, content)
	return content, vsClass, err
}

func (r *LogicalVolumeReconciler) getVolumeSnapshotClassFromContent(ctx context.Context, content *snapshot_api.VolumeSnapshotContent) (*snapshot_api.VolumeSnapshotClass, error) {
	if content.Spec.VolumeSnapshotClassName == nil {
		return nil, fmt.Errorf("VolumeSnapshotContent %s does not have VolumeSnapshotClassName set", content.Name)
	}

	vsClass := &snapshot_api.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: *content.Spec.VolumeSnapshotClassName,
		},
	}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(vsClass), vsClass); err != nil {
		return nil, fmt.Errorf("unable to fetch VolumeSnapshotClass %s: %w", *content.Spec.VolumeSnapshotClassName, err)
	}
	return vsClass, nil
}

func (r *LogicalVolumeReconciler) getVolumeSnapshotContentIfExists(ctx context.Context, lv *topolvmv1.LogicalVolume) (*snapshot_api.VolumeSnapshotContent, error) {
	var content *snapshot_api.VolumeSnapshotContent
	var err error
	if lv.Spec.Source != "" {
		content, err = r.getVolumeSnapshotContent(ctx, lv)
	}
	return content, err
}

func (r *LogicalVolumeReconciler) shouldPerformSnapshotBackup(vsClass *snapshot_api.VolumeSnapshotClass) (bool, error) {
	return r.isOnlineSnapshotEnabled(vsClass), nil
}

func (r *LogicalVolumeReconciler) getSourceLogicalVolume(ctx context.Context, lv *topolvmv1.LogicalVolume) (*topolvmv1.LogicalVolume, error) {
	if lv.Spec.Source == "" {
		return nil, nil
	}
	sourceLV := &topolvmv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: lv.Spec.Source,
		},
	}
	// Ignore errors if the snapshot LV doesn't exist For VolumeSnapshot LV it doesn't exist. It'll only exist while restoring from any VS.
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(sourceLV), sourceLV); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return sourceLV, nil
}

func (r *LogicalVolumeReconciler) shouldPerformSnapshotRestore(sourceLV *topolvmv1.LogicalVolume, vsClass *snapshot_api.VolumeSnapshotClass) (bool, error) {
	// Check if source snapshot is ready or already completed
	if sourceLV.Status.Snapshot == nil ||
		(sourceLV.Status.Snapshot != nil && sourceLV.Status.Snapshot.Phase == topolvmv1.OperationPhaseSucceeded) {
		return r.isOnlineSnapshotEnabled(vsClass), nil
	}
	return false, nil
}

func (r *LogicalVolumeReconciler) isOnlineSnapshotEnabled(vsClass *snapshot_api.VolumeSnapshotClass) bool {
	if vsClass == nil {
		return false
	}

	snapshotMode, exists := vsClass.Parameters[SnapshotMode]
	return exists && snapshotMode == SnapshotModeOnline
}

func (r *LogicalVolumeReconciler) isSnapshotOperationComplete(lv *topolvmv1.LogicalVolume) bool {
	if lv.Status.Snapshot == nil {
		return false
	}
	phase := lv.Status.Snapshot.Phase
	return phase == topolvmv1.OperationPhaseSucceeded || phase == topolvmv1.OperationPhaseFailed
}

func (r *LogicalVolumeReconciler) isSnapshotOperationRunning(lv *topolvmv1.LogicalVolume) bool {
	return lv.Status.Snapshot != nil && lv.Status.Snapshot.Phase == topolvmv1.OperationPhaseRunning
}

func (r *LogicalVolumeReconciler) initializeSnapshotStatus(ctx context.Context, lv *topolvmv1.LogicalVolume, operation topolvmv1.OperationType, log logr.Logger) error {
	if lv.Status.Snapshot != nil && lv.Status.Snapshot.Phase != "" {
		return nil
	}
	message := fmt.Sprintf("Initializing online %s operation", operation)

	if err := r.updateSnapshotOperationStatus(ctx, lv, operation, topolvmv1.OperationPhasePending, message, nil); err != nil {
		log.Error(err, "failed to initialize snapshot status", "operation", operation, "name", lv.Name)
		return err
	}

	log.Info("initialized snapshot status", "operation", operation, "name", lv.Name, "phase", topolvmv1.OperationPhasePending)
	return nil
}

func (r *LogicalVolumeReconciler) mountLogicalVolume(ctx context.Context, lv *topolvmv1.LogicalVolume,
	mountOptions []string, operation topolvmv1.OperationType, log logr.Logger) (*mounter.MountResponse, error) {
	resp, err := r.lvMount.Mount(ctx, lv, mountOptions)
	if err != nil {
		mountType := "mount"
		errorCode := "VolumeMountFailed"
		if len(mountOptions) == 0 {
			mountType = "bind mount"
			errorCode = "VolumeBindMountFailed"
		}

		log.Error(err, "failed to mount logical volume", "mountType", mountType, "name", lv.Name)

		snapshotErr := &topolvmv1.SnapshotError{
			Code:    errorCode,
			Message: fmt.Sprintf("failed to %s logical volume: %v", mountType, err),
		}
		message := fmt.Sprintf("Failed to %s logical volume: %v", mountType, err)

		if updateErr := r.updateSnapshotOperationStatus(ctx, lv, operation, topolvmv1.OperationPhaseFailed, message, snapshotErr); updateErr != nil {
			log.Error(updateErr, "failed to update snapshot status after mount error", "name", lv.Name)
		}
		return nil, err
	}
	return resp, nil
}

func (r *LogicalVolumeReconciler) executeSnapshotOperation(ctx context.Context, lv *topolvmv1.LogicalVolume,
	exec executor.Executor, operation topolvmv1.OperationType, log logr.Logger) error {
	if err := exec.Execute(); err != nil {
		errorCode := "SnapshotExecutionFailed"
		log.Error(err, "failed to execute operation", "operation", operation, "name", lv.Name)

		snapshotErr := &topolvmv1.SnapshotError{
			Code:    errorCode,
			Message: fmt.Sprintf("failed to execute %s: %v", operation, err),
		}
		message := fmt.Sprintf("Failed to execute %s: %v", operation, err)

		if updateErr := r.updateSnapshotOperationStatus(ctx, lv, operation, topolvmv1.OperationPhaseFailed, message, snapshotErr); updateErr != nil {
			log.Error(updateErr, "failed to update snapshot status after execution error", "name", lv.Name)
		}
		return err
	}
	return nil
}

func (r *LogicalVolumeReconciler) executeSnapshotDeleteOperation(ctx context.Context, lv *topolvmv1.LogicalVolume, log logr.Logger) error {
	_, vsClass, err := r.getVolumeSnapshotResources(ctx, lv)
	if err != nil {
		log.Error(err, "failed to get VolumeSnapshotContent/VolumeSnapshotClass", "name", lv.Name)
		return err
	}
	exec := executor.NewSnapshotDeleteExecutor(r.client, lv, vsClass)
	if err := exec.Execute(); err != nil {
		log.Error(err, "failed to execute snapshot delete executor", "name", lv.Name)
		if updateErr := setSnapshotDeleteExecutorEnsuredToFalse(ctx, r.client, lv, err); updateErr != nil {
			log.Error(updateErr, "failed to set snapshot delete executor ensured to false", "name", lv.Name)
		}
		return err
	}
	if err := setSnapshotDeleteExecutorEnsuredToTrue(ctx, r.client, lv); err != nil {
		log.Error(err, "failed to set snapshot delete executor ensured to true", "name", lv.Name)
		return err
	}

	log.Info("successfully executed snapshot delete operation for LogicalVolume", "name", lv.Name)
	return nil
}

func (r *LogicalVolumeReconciler) executeCleanerOperation(ctx context.Context, lv *topolvmv1.LogicalVolume, operation topolvmv1.OperationType, log logr.Logger) error {
	exec := executor.NewCleanerExecutor(r.client, lv, operation)
	if err := exec.Execute(); err != nil {
		if err := setSnapshotExecutorCleanupToFalse(ctx, r.client, operation, lv, err); err != nil {
			log.Error(err, "failed to set snapshot executor cleanup to false", "name", lv.Name)
			return err
		}
	}
	if err := setSnapshotExecutorCleanupToTrue(ctx, r.client, operation, lv); err != nil {
		log.Error(err, "failed to set snapshot executor cleanup to true", "name", lv.Name)
		return err

	}
	log.Info("successfully executed cleaner operation for LogicalVolume", "name", lv.Name)
	return nil
}

func (r *LogicalVolumeReconciler) performSnapshotBackup(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume,
	vsContent *snapshot_api.VolumeSnapshotContent, vsClass *snapshot_api.VolumeSnapshotClass) error {
	// Initialize status if needed
	if err := r.initializeSnapshotStatus(ctx, lv, topolvmv1.OperationBackup, log); err != nil {
		return err
	}
	// Check if operation is already complete
	if r.isSnapshotOperationComplete(lv) {
		log.Info("snapshot backup already completed", "name", lv.Name, "phase", lv.Status.Snapshot.Phase)
		return nil
	}
	// Check if operation is running
	if r.isSnapshotOperationRunning(lv) {
		log.Info("snapshot backup is currently running", "name", lv.Name)
		return nil
	}
	// Mount the logical volume with read-only and no-recovery options
	mountOptions := []string{"ro", "norecovery"}
	mountResponse, err := r.mountLogicalVolume(ctx, lv, mountOptions, topolvmv1.OperationBackup, log)
	if err != nil {
		return err
	}
	// Execute the snapshot backup
	snapshotExecutor := executor.NewSnapshotBackupExecutor(r.client, lv, mountResponse, vsContent, vsClass)
	if err := r.executeSnapshotOperation(ctx, lv, snapshotExecutor, topolvmv1.OperationBackup, log); err != nil {
		if err := setSnapshotBackupExecutorEnsuredToFalse(ctx, r.client, lv, err); err != nil {
			return fmt.Errorf("failed to set snapshot backup executor ensured to false: %w", err)
		}
		return err
	}
	if err = setSnapshotBackupExecutorEnsuredToTrue(ctx, r.client, lv); err != nil {
		return fmt.Errorf("failed to set snapshot backup executor ensured to true: %w", err)
	}
	return nil

}

func (r *LogicalVolumeReconciler) performSnapshotRestore(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume,
	sourceLV *topolvmv1.LogicalVolume, vsClass *snapshot_api.VolumeSnapshotClass) error {
	// Check if operation is already complete
	if r.isSnapshotOperationComplete(lv) {
		log.Info("snapshot restore already completed", "name", lv.Name, "phase", lv.Status.Snapshot.Phase)
		return nil
	}
	// Initialize status if needed
	if err := r.initializeSnapshotStatus(ctx, lv, topolvmv1.OperationRestore, log); err != nil {
		return err
	}
	// Check if operation is running
	if r.isSnapshotOperationRunning(lv) {
		log.Info("snapshot restore is currently running", "name", lv.Name)
		return nil
	}

	// Use bind mount for restore (the pod already mounts LV)
	mountOptions := []string{}
	mountResponse, err := r.mountLogicalVolume(ctx, lv, mountOptions, topolvmv1.OperationRestore, log)
	if err != nil {
		return err
	}

	// Execute the snapshot restore
	restoreExecutor := executor.NewSnapshotRestoreExecutor(r.client, lv, sourceLV, mountResponse, vsClass)
	err = r.executeSnapshotOperation(ctx, lv, restoreExecutor, topolvmv1.OperationRestore, log)
	if err != nil {
		if err := setSnapshotRestoreExecutorEnsuredToFalse(ctx, r.client, lv, err); err != nil {
			return fmt.Errorf("failed to set snapshot restore executor ensured to false: %w", err)
		}
	}
	if err = setSnapshotRestoreExecutorEnsuredToTrue(ctx, r.client, lv); err != nil {
		return fmt.Errorf("failed to set snapshot restore executor ensured to true: %w", err)
	}
	return nil
}

func (r *LogicalVolumeReconciler) getVolumeSnapshotContent(ctx context.Context, lv *topolvmv1.LogicalVolume) (*snapshot_api.VolumeSnapshotContent, error) {
	content := &snapshot_api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			// https://github.com/kubernetes-csi/external-snapshotter/blob/master/pkg/utils/util.go#L283
			Name: fmt.Sprintf("snapcontent%s", strings.TrimPrefix(lv.Spec.Name, "snapshot")),
		},
	}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(content), content); err != nil {
		return nil, err
	}
	return content, nil
}

func (r *LogicalVolumeReconciler) updateSnapshotOperationStatus(ctx context.Context, lv *topolvmv1.LogicalVolume,
	operation topolvmv1.OperationType, phase topolvmv1.OperationPhase, message string, snapshotErr *topolvmv1.SnapshotError) error {
	// Refresh the LogicalVolume to get the latest version
	freshLV := &topolvmv1.LogicalVolume{}
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(lv), freshLV); err != nil {
		return fmt.Errorf("failed to get latest LogicalVolume: %w", err)
	}
	// Initialize snapshot status if it doesn't exist
	if freshLV.Status.Snapshot == nil {
		freshLV.Status.Snapshot = &topolvmv1.SnapshotStatus{
			StartTime: metav1.Now(),
		}
	}
	// Update operation details
	freshLV.Status.Snapshot.Operation = operation
	freshLV.Status.Snapshot.Phase = phase
	freshLV.Status.Snapshot.Message = message
	// Set error if provided
	if snapshotErr != nil {
		freshLV.Status.Snapshot.Error = snapshotErr
	}
	// Update the status
	if err := r.client.Status().Update(ctx, freshLV); err != nil {
		return fmt.Errorf("failed to update snapshot status: %w", err)
	}
	// Sync the original object with the latest status
	lv.Status = freshLV.Status
	lv.ObjectMeta.ResourceVersion = freshLV.ObjectMeta.ResourceVersion
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LogicalVolumeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	builder := ctrl.NewControllerManagedBy(mgr)
	if topolvm.UseLegacy() {
		builder = builder.For(&topolvmlegacyv1.LogicalVolume{})
	} else {
		builder = builder.For(&topolvmv1.LogicalVolume{})
	}
	return builder.WithEventFilter(&logicalVolumeFilter{r.nodeName}).Complete(r)
}

func (r *LogicalVolumeReconciler) removeLVIfExists(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	// First, unmount the LV if it's mounted (e.g., for online snapshots)
	if err := r.lvMount.Unmount(ctx, lv); err != nil {
		log.Error(err, "failed to unmount LV before removal", "name", lv.Name, "uid", lv.UID)
		// Continue with removal even if unmount fails, as the LV might not be mounted
		// or the mount might have been manually removed
	} else {
		log.Info("successfully unmounted LV", "name", lv.Name, "uid", lv.UID)
	}

	// Finalizer's process ( RemoveLV then removeString ) is not atomic,
	// so checking existence of LV to ensure its idempotence
	_, err := r.lvService.RemoveLV(ctx, &proto.RemoveLVRequest{Name: string(lv.UID), DeviceClass: lv.Spec.DeviceClass})
	if status.Code(err) == codes.NotFound {
		log.Info("LV already removed", "name", lv.Name, "uid", lv.UID)
		return nil
	}
	if err != nil {
		log.Error(err, "failed to remove LV", "name", lv.Name, "uid", lv.UID)
		return err
	}
	log.Info("removed LV", "name", lv.Name, "uid", lv.UID)
	return nil
}

func (r *LogicalVolumeReconciler) volumeExists(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) (bool, error) {
	respList, err := r.vgService.GetLVList(ctx, &proto.GetLVListRequest{DeviceClass: lv.Spec.DeviceClass})
	if err != nil {
		log.Error(err, "failed to get list of LV")
		return false, err
	}

	for _, v := range respList.Volumes {
		if v.Name != string(lv.UID) {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (r *LogicalVolumeReconciler) createLV(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, restoreFrSnapshot bool) error {
	// When lv.Status.Code is not codes.OK (== 0), CreateLV has already failed.
	// LogicalVolume CRD will be deleted soon by the controller.
	if lv.Status.Code != codes.OK {
		return nil
	}

	reqBytes := lv.Spec.Size.Value()

	err := func() error {
		// In case the controller crashed just after LVM LV creation, LV may already exist.
		found, err := r.volumeExists(ctx, log, lv)
		if err != nil {
			lv.Status.Code = codes.Internal
			lv.Status.Message = "failed to check volume existence"
			return err
		}
		if found {
			log.Info("set volumeID to existing LogicalVolume", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID)
			// Don't set CurrentSize here because the Spec.Size field may be updated after the LVM LV is created.
			lv.Status.VolumeID = string(lv.UID)
			lv.Status.Code = codes.OK
			lv.Status.Message = ""
			return nil
		}

		var volume *proto.LogicalVolume

		// Create a snapshot LV
		if lv.Spec.Source != "" && !restoreFrSnapshot {
			// accessType should be either "readonly" or "readwrite".
			if lv.Spec.AccessType != "ro" && lv.Spec.AccessType != "rw" {
				return fmt.Errorf("invalid access type for source volume: %s", lv.Spec.AccessType)
			}
			sourcelv := new(topolvmv1.LogicalVolume)
			if err := r.client.Get(ctx, types.NamespacedName{Namespace: lv.Namespace, Name: lv.Spec.Source}, sourcelv); err != nil {
				log.Error(err, "unable to fetch source LogicalVolume", "name", lv.Name)
				return err
			}
			sourceVolID := sourcelv.Status.VolumeID
			currentSize := sourcelv.Status.CurrentSize.Value()
			if reqBytes < currentSize {
				return fmt.Errorf("cannot create new LV, requested size %d is smaller than source LV size %d", reqBytes, currentSize)
			}

			// Create a snapshot lv
			resp, err := r.lvService.CreateLVSnapshot(ctx, &proto.CreateLVSnapshotRequest{
				Name:         string(lv.UID),
				DeviceClass:  lv.Spec.DeviceClass,
				SourceVolume: sourceVolID,
				SizeBytes:    reqBytes,
				AccessType:   lv.Spec.AccessType,
			})
			if err != nil {
				code, message := extractFromError(err)
				log.Error(err, message)
				lv.Status.Code = code
				lv.Status.Message = message
				return err
			}
			volume = resp.Snapshot
		} else {
			// Create a regular lv
			resp, err := r.lvService.CreateLV(ctx, &proto.CreateLVRequest{
				Name:                string(lv.UID),
				DeviceClass:         lv.Spec.DeviceClass,
				LvcreateOptionClass: lv.Spec.LvcreateOptionClass,
				SizeBytes:           reqBytes,
			})
			if err != nil {
				code, message := extractFromError(err)
				log.Error(err, message)
				lv.Status.Code = code
				lv.Status.Message = message
				return err
			}
			volume = resp.Volume
		}

		lv.Status.VolumeID = volume.Name
		lv.Status.CurrentSize = resource.NewQuantity(volume.SizeBytes, resource.BinarySI)
		lv.Status.Code = codes.OK
		lv.Status.Message = ""
		return nil
	}()

	if err != nil {
		if err2 := r.client.Status().Update(ctx, lv); err2 != nil {
			// err2 is logged but not returned because err is more important
			log.Error(err2, "failed to update status", "name", lv.Name, "uid", lv.UID)
		}
		return err
	}

	if err := r.client.Status().Update(ctx, lv); err != nil {
		log.Error(err, "failed to update status", "name", lv.Name, "uid", lv.UID)
		return err
	}

	log.Info("created new LV", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID)
	return nil
}

func (r *LogicalVolumeReconciler) expandLV(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	// We denote unknown size as -1.
	var origBytes int64 = -1
	switch {
	case lv.Status.CurrentSize == nil:
		// topolvm-node may be crashed before setting Status.CurrentSize.
		// Since the actual volume size is unknown,
		// we need to do resizing to set Status.CurrentSize to the same value as Spec.Size.
	case lv.Spec.Size.Cmp(*lv.Status.CurrentSize) <= 0:
		return nil
	default:
		origBytes = (*lv.Status.CurrentSize).Value()
	}

	reqBytes := lv.Spec.Size.Value()

	err := func() error {
		resp, err := r.lvService.ResizeLV(ctx, &proto.ResizeLVRequest{
			Name:        string(lv.UID),
			SizeBytes:   reqBytes,
			DeviceClass: lv.Spec.DeviceClass,
		})
		if err != nil {
			code, message := extractFromError(err)
			log.Error(err, message)
			lv.Status.Code = code
			lv.Status.Message = message
			return err
		}

		lv.Status.CurrentSize = resource.NewQuantity(resp.SizeBytes, resource.BinarySI)
		lv.Status.Code = codes.OK
		lv.Status.Message = ""
		return nil
	}()

	if err != nil {
		if err2 := r.client.Status().Update(ctx, lv); err2 != nil {
			// err2 is logged but not returned because err is more important
			log.Error(err2, "failed to update status", "name", lv.Name, "uid", lv.UID)
		}
		return err
	}

	if err := r.client.Status().Update(ctx, lv); err != nil {
		log.Error(err, "failed to update status", "name", lv.Name, "uid", lv.UID)
		return err
	}

	log.Info("expanded LV", "name", lv.Name, "uid", lv.UID, "status.volumeID", lv.Status.VolumeID,
		"original status.currentSize", origBytes, "status.currentSize", lv.Status.CurrentSize, "spec.size", reqBytes)
	return nil
}

type logicalVolumeFilter struct {
	nodeName string
}

func (f logicalVolumeFilter) filter(obj client.Object) bool {
	var name string
	if topolvm.UseLegacy() {
		lv, ok := obj.(*topolvmlegacyv1.LogicalVolume)
		if !ok {
			return false
		}
		name = lv.Spec.NodeName
	} else {
		lv, ok := obj.(*topolvmv1.LogicalVolume)
		if !ok {
			return false
		}
		name = lv.Spec.NodeName
	}
	if name == f.nodeName {
		return true
	}
	return false
}

func (f logicalVolumeFilter) Create(e event.CreateEvent) bool {
	return f.filter(e.Object)
}

func (f logicalVolumeFilter) Delete(e event.DeleteEvent) bool {
	return f.filter(e.Object)
}

func (f logicalVolumeFilter) Update(e event.UpdateEvent) bool {
	return f.filter(e.ObjectNew)
}

func (f logicalVolumeFilter) Generic(e event.GenericEvent) bool {
	return f.filter(e.Object)
}

func extractFromError(err error) (codes.Code, string) {
	s, ok := status.FromError(err)
	if !ok {
		return codes.Internal, err.Error()
	}
	return s.Code(), s.Message()
}

func containsKeyAndValue(labels map[string]string, key, value string) bool {
	for k, v := range labels {
		if k == key && v == value {
			return true
		}
	}
	return false
}
