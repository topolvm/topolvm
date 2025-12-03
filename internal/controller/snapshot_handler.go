package controller

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	snapshot_api "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"github.com/topolvm/topolvm"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/executor"
	"github.com/topolvm/topolvm/internal/mounter"
	"github.com/topolvm/topolvm/pkg/lvmd/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// snapshotHandler holds the context for snapshot operations.
type snapshotHandler struct {
	*LogicalVolumeReconciler
	// Source information
	sourceLV  *topolvmv1.LogicalVolume
	vsContent *snapshot_api.VolumeSnapshotContent
	vsClass   *snapshot_api.VolumeSnapshotClass

	// Decisions
	shouldBackup  bool
	shouldRestore bool
}

// getSnapshotHandler prepares the snapshot context for restore and backup operations.
func newSnapshotHandler(r *LogicalVolumeReconciler) *snapshotHandler {
	return &snapshotHandler{
		LogicalVolumeReconciler: r,
	}
}

func (h *snapshotHandler) buildSnapshotContext(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	sourceLV, err := h.getSourceLV(ctx, lv)
	if err != nil {
		log.Error(err, "failed to get source snapshot LV", "name", lv.Name)
		return err
	}
	if sourceLV != nil {
		log.Info("snapshot LV found", "name", sourceLV.Name)
		if err := h.setVolumeSnapshotInfo(ctx, log, sourceLV); err != nil {
			return fmt.Errorf("failed to set volume snapshot info: %w", err)
		}
		h.sourceLV = sourceLV
		h.shouldRestore = h.shouldPerformSnapshotRestore(sourceLV)
	}
	return nil
}

func (h *snapshotHandler) buildSnapshotContextFrBackup(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	if err := h.setVolumeSnapshotInfo(ctx, log, lv); err != nil {
		return fmt.Errorf("failed to set volume snapshot info: %w", err)
	}
	h.shouldBackup = h.vsClass != nil && isOnlineSnapshotEnabled(h.vsClass)
	return nil
}

func (h *snapshotHandler) setVolumeSnapshotInfo(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	var err error
	h.vsContent, h.vsClass, err = h.getVolumeSnapshotInfo(ctx, lv)
	if err != nil {
		log.Error(err, "failed to get VolumeSnapshotContent/VolumeSnapshotClass", "name", lv.Name)
		return err
	}
	return nil
}

func (h *snapshotHandler) getSourceLV(ctx context.Context, lv *topolvmv1.LogicalVolume) (*topolvmv1.LogicalVolume, error) {
	if lv.Spec.Source == "" {
		return nil, nil
	}
	sourceLV := &topolvmv1.LogicalVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: lv.Spec.Source,
		},
	}
	// Ignore errors if the snapshot LV doesn't exist For VolumeSnapshot LV it doesn't exist. It'll only exist while restoring from any VS.
	if err := h.client.Get(ctx, client.ObjectKeyFromObject(sourceLV), sourceLV); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return sourceLV, nil
}

func (h *snapshotHandler) getVolumeSnapshotInfo(ctx context.Context, lv *topolvmv1.LogicalVolume) (*snapshot_api.VolumeSnapshotContent, *snapshot_api.VolumeSnapshotClass, error) {
	vsContent, err := h.getVolumeSnapshotContentIfExists(ctx, lv)
	if err != nil {
		return nil, nil, client.IgnoreNotFound(err)
	}
	if vsContent == nil {
		return nil, nil, nil
	}
	vsClass, err := h.getVolumeSnapshotClassFromContent(ctx, vsContent)
	return vsContent, vsClass, err
}

func (h *snapshotHandler) getVolumeSnapshotContentIfExists(ctx context.Context, lv *topolvmv1.LogicalVolume) (*snapshot_api.VolumeSnapshotContent, error) {
	var content *snapshot_api.VolumeSnapshotContent
	var err error
	if lv.Spec.Source != "" {
		content, err = getVolumeSnapshotContent(ctx, h.client, lv)
	}
	return content, err
}

func (h *snapshotHandler) getVolumeSnapshotClassFromContent(ctx context.Context, content *snapshot_api.VolumeSnapshotContent) (*snapshot_api.VolumeSnapshotClass, error) {
	if content.Spec.VolumeSnapshotClassName == nil {
		return nil, fmt.Errorf("VolumeSnapshotContent %s does not have VolumeSnapshotClassName set", content.Name)
	}

	vsClass := &snapshot_api.VolumeSnapshotClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: *content.Spec.VolumeSnapshotClassName,
		},
	}
	if err := h.client.Get(ctx, client.ObjectKeyFromObject(vsClass), vsClass); err != nil {
		return nil, fmt.Errorf("unable to fetch VolumeSnapshotClass %s: %w", *content.Spec.VolumeSnapshotClassName, err)
	}
	return vsClass, nil
}

func (h *snapshotHandler) shouldPerformSnapshotRestore(sourceLV *topolvmv1.LogicalVolume) bool {
	// Check if source snapshot is ready or already completed
	if sourceLV.Status.Snapshot == nil ||
		(sourceLV.Status.Snapshot != nil && sourceLV.Status.Snapshot.Phase == topolvmv1.OperationPhaseSucceeded) {
		return isOnlineSnapshotEnabled(h.vsClass)
	}
	return false
}

func (h *snapshotHandler) checkPVExists(ctx context.Context, lv *topolvmv1.LogicalVolume) (bool, error) {
	if lv.Spec.Name == "" {
		return false, nil
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: lv.Spec.Name,
		},
	}
	err := h.client.Get(ctx, client.ObjectKeyFromObject(pv), pv)
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return true, nil

}

func (h *snapshotHandler) restoreFromSnapshot(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	// Initialize status if needed
	if err := h.initializeSnapshotStatus(ctx, log, lv, topolvmv1.OperationRestore); err != nil {
		return err
	}

	// Use bind mount for restore (the pod already mounts LV)
	mountOptions := []string{}
	mountResponse, err := h.mountLogicalVolume(ctx, log, lv, mountOptions, topolvmv1.OperationRestore)
	if err == nil {
		restoreExecutor := executor.NewSnapshotRestoreExecutor(h.client, lv, h.sourceLV, mountResponse, h.vsClass)
		err = h.executeSnapshotOperation(ctx, lv, restoreExecutor, topolvmv1.OperationRestore, log)
	}
	if err != nil {
		if newErr := setSnapshotRestoreExecutorEnsuredToFalse(ctx, h.client, lv, err); newErr != nil {
			return fmt.Errorf("failed to set snapshot restore executor ensured to false: originalErr=%w wrapErr=%w", err, newErr)
		}
		return fmt.Errorf("failed to execute snapshot restore operation: %w", err)
	} else {
		if err = setSnapshotRestoreExecutorEnsuredToTrue(ctx, h.client, lv); err != nil {
			return fmt.Errorf("failed to set snapshot restore executor ensured to true: %w", err)
		}
	}
	return nil
}

func (h *snapshotHandler) backupSnapshot(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	// Initialize status if needed
	if err := h.initializeSnapshotStatus(ctx, log, lv, topolvmv1.OperationBackup); err != nil {
		return err
	}

	// Mount the logical volume with read-only and no-recovery options
	mountOptions := []string{"ro", "norecovery"}
	mountResponse, err := h.mountLogicalVolume(ctx, log, lv, mountOptions, topolvmv1.OperationBackup)
	if err == nil {
		snapshotExecutor := executor.NewSnapshotBackupExecutor(h.client, lv, mountResponse, h.vsContent, h.vsClass)
		err = h.executeSnapshotOperation(ctx, lv, snapshotExecutor, topolvmv1.OperationBackup, log)
	}
	if err != nil {
		if newErr := setSnapshotBackupExecutorEnsuredToFalse(ctx, h.client, lv, err); newErr != nil {
			return fmt.Errorf("failed to set snapshot backup executor ensured to false: originalErr=%w wrapErr=%w", err, newErr)
		}
	} else {
		if err = setSnapshotBackupExecutorEnsuredToTrue(ctx, h.client, lv); err != nil {
			return fmt.Errorf("failed to set snapshot backup executor ensured to true: %w", err)
		}
	}
	return nil
}

func (h *snapshotHandler) executeSnapshotOperation(ctx context.Context, lv *topolvmv1.LogicalVolume, exec executor.Executor, operation topolvmv1.OperationType, log logr.Logger) error {
	if err := exec.Execute(); err != nil {
		errorCode := "SnapshotExecutionFailed"
		log.Error(err, "failed to execute operation", "operation", operation, "name", lv.Name)

		snapshotErr := &topolvmv1.SnapshotError{
			Code:    errorCode,
			Message: fmt.Sprintf("failed to execute %s: %v", operation, err),
		}
		message := fmt.Sprintf("Failed to execute %s: %v", operation, err)

		if updateErr := h.updateSnapshotOperationStatus(ctx, lv, operation, topolvmv1.OperationPhaseFailed, message, snapshotErr); updateErr != nil {
			log.Error(updateErr, "failed to update snapshot status after execution error", "name", lv.Name)
		}
		return err
	}
	return nil
}

func (h *snapshotHandler) executeCleanerOperation(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, operation topolvmv1.OperationType) error {
	err := h.lvMount.Unmount(ctx, lv)
	if err == nil {
		log.Info("successfully unmounted LV", "name", lv.Name, "uid", lv.UID)
		exec := executor.NewCleanerExecutor(h.client, lv, operation)
		err = exec.Execute()
	}
	if err != nil {
		if newErr := setSnapshotExecutorCleanupToFalse(ctx, h.client, operation, lv, err); newErr != nil {
			return fmt.Errorf("failed to set snapshot executor cleanup to false: originalErr=%w wrapErr=%w", err, newErr)
		}
		return err
	}

	if err = setSnapshotExecutorCleanupToTrue(ctx, h.client, operation, lv); err != nil {
		return fmt.Errorf("failed to set snapshot executor cleanup to true: %w", err)
	}
	log.Info("successfully executed cleaner operation for LogicalVolume", "name", lv.Name)

	return nil
}

func (h *snapshotHandler) cleanupLVMSnapshotAfterBackup(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	log.Info("cleaning up LVM snapshot after backup completion", "name", lv.Name, "volumeID", lv.Status.VolumeID)
	if err := h.removeLVMSnapshot(ctx, log, lv); err != nil {
		log.Error(err, "failed to remove LVM snapshot", "name", lv.Name)
		if setErr := setLVMSnapshotCleanedToFalse(ctx, h.client, lv, err); setErr != nil {
			return fmt.Errorf("failed to set LVM snapshot cleanup to false: originalErr=%w wrapErr=%w", err, setErr)
		}
		return err
	}

	if err := setLVMSnapshotCleanedToTrue(ctx, h.client, lv); err != nil {
		return fmt.Errorf("failed to set LVM snapshot cleanup to true: %w", err)
	}

	log.Info("successfully removed LVM snapshot volume", "name", lv.Name, "volumeID", lv.Status.VolumeID)
	return nil
}

func (h *snapshotHandler) removeLVMSnapshot(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	if lv.Status.VolumeID == "" {
		log.Info("no VolumeID set, skipping LVM snapshot removal", "name", lv.Name)
		return nil
	}

	volumeID := lv.Status.VolumeID

	// Remove the LVM snapshot volume
	_, err := h.lvService.RemoveLV(ctx, &proto.RemoveLVRequest{
		Name:        string(lv.UID),
		DeviceClass: lv.Spec.DeviceClass,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			log.Info("LVM snapshot already removed", "name", lv.Name, "uid", lv.UID)
			// Even if not found, we should clear the status
		} else {
			return fmt.Errorf("failed to remove LVM snapshot: %w", err)
		}
	}

	log.Info("removed LVM snapshot", "name", lv.Name, "uid", lv.UID, "volumeID", volumeID)
	return nil
}

func (h *snapshotHandler) updateStatusMessageAfterSnapshotRemoval(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, msg string) error {
	// Refresh the LogicalVolume to get the latest version
	freshLV := &topolvmv1.LogicalVolume{}
	if err := h.client.Get(ctx, client.ObjectKeyFromObject(lv), freshLV); err != nil {
		return fmt.Errorf("failed to get latest LogicalVolume: %w", err)
	}
	freshLV.Status.Message = msg
	if err := h.client.Status().Update(ctx, freshLV); err != nil {
		return fmt.Errorf("failed to update LogicalVolume status: %w", err)
	}
	// Sync the original object with the latest status
	lv.Status = freshLV.Status
	lv.ResourceVersion = freshLV.ResourceVersion

	log.Info("updated LogicalVolume status after LVM snapshot removal", "name", lv.Name, "volumeID", "", "currentSize", "nil")
	return nil
}

func (h *snapshotHandler) executeSnapshotDeleteOperation(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume) error {
	if err := h.setVolumeSnapshotInfo(ctx, log, lv); err != nil {
		return fmt.Errorf("failed to set volume snapshot info: %w", err)
	}
	err := h.lvMount.Unmount(ctx, lv)
	if err == nil {
		exec := executor.NewSnapshotDeleteExecutor(h.client, lv, h.vsClass)
		err = exec.Execute()
	}
	if err != nil {
		if newErr := setSnapshotDeleteExecutorEnsuredToFalse(ctx, h.client, lv, err); newErr != nil {
			return fmt.Errorf("failed to set snapshot delete executor ensured to false: originalErr=%w wrapErr=%w", err, newErr)
		}
		return fmt.Errorf("failed to execute snapshot delete operation: %w", err)
	} else {
		if err = setSnapshotDeleteExecutorEnsuredToTrue(ctx, h.client, lv); err != nil {
			return fmt.Errorf("failed to set snapshot delete executor ensured to true: %w", err)
		}
	}

	log.Info("successfully executed snapshot delete operation for LogicalVolume", "name", lv.Name)
	return nil
}

func (h *snapshotHandler) mountLogicalVolume(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, mountOptions []string, operation topolvmv1.OperationType) (*mounter.MountResponse, error) {
	resp, err := h.lvMount.Mount(ctx, lv, mountOptions)
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

		if updateErr := h.updateSnapshotOperationStatus(ctx, lv, operation, topolvmv1.OperationPhaseFailed, message, snapshotErr); updateErr != nil {
			log.Error(updateErr, "failed to update snapshot status after mount error", "name", lv.Name)
		}
		return nil, err
	}
	return resp, nil
}

func (h *snapshotHandler) initializeSnapshotStatus(ctx context.Context, log logr.Logger, lv *topolvmv1.LogicalVolume, operation topolvmv1.OperationType) error {
	if lv.Status.Snapshot != nil && lv.Status.Snapshot.Phase != "" {
		return nil
	}
	message := fmt.Sprintf("Initializing snapshot %s operation", operation)

	if err := h.updateSnapshotOperationStatus(ctx, lv, operation, topolvmv1.OperationPhasePending, message, nil); err != nil {
		log.Error(err, "failed to initialize snapshot status", "operation", operation, "name", lv.Name)
		return err
	}

	log.Info("initialized snapshot status", "operation", operation, "name", lv.Name, "phase", topolvmv1.OperationPhasePending)
	return nil
}

func (h *snapshotHandler) updateSnapshotOperationStatus(ctx context.Context, lv *topolvmv1.LogicalVolume, operation topolvmv1.OperationType, phase topolvmv1.OperationPhase, message string, snapshotErr *topolvmv1.SnapshotError) error {
	// Refresh the LogicalVolume to get the latest version
	freshLV := &topolvmv1.LogicalVolume{}
	if err := h.client.Get(ctx, client.ObjectKeyFromObject(lv), freshLV); err != nil {
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
	if err := h.client.Status().Update(ctx, freshLV); err != nil {
		return fmt.Errorf("failed to update snapshot status: %w", err)
	}

	// Sync the original object with the latest status
	lv.Status = freshLV.Status
	lv.ResourceVersion = freshLV.ResourceVersion
	return nil
}

func isOnlineSnapshotEnabled(vsClass *snapshot_api.VolumeSnapshotClass) bool {
	if vsClass == nil {
		return false
	}

	snapshotMode, exists := vsClass.Parameters[SnapshotMode]
	return exists && snapshotMode == SnapshotModeOnline
}

func getVolumeSnapshotContent(ctx context.Context, rClient client.Client, lv *topolvmv1.LogicalVolume) (*snapshot_api.VolumeSnapshotContent, error) {
	content := &snapshot_api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			// https://github.com/kubernetes-csi/external-snapshotter/blob/master/pkg/utils/util.go#L283
			Name: fmt.Sprintf("snapcontent%s", strings.TrimPrefix(lv.Spec.Name, "snapshot")),
		},
	}
	if err := rClient.Get(ctx, client.ObjectKeyFromObject(content), content); err != nil {
		return nil, err
	}
	return content, nil
}

func isLVHasSnapshot(lv *topolvmv1.LogicalVolume) bool {
	if lv.Status.Snapshot == nil {
		return false
	}
	return lv.Status.Snapshot.Phase == topolvmv1.OperationPhaseSucceeded &&
		lv.Status.Snapshot.SnapshotID != ""
}

func isPendingDeletion(lv *topolvmv1.LogicalVolume) bool {
	if lv.Annotations == nil {
		return false
	}
	_, pendingDeletion := lv.Annotations[topolvm.GetLVPendingDeletionKey()]
	return pendingDeletion
}

func isSnapshotOperationComplete(lv *topolvmv1.LogicalVolume) bool {
	if lv.Status.Snapshot == nil {
		return false
	}
	phase := lv.Status.Snapshot.Phase
	return phase == topolvmv1.OperationPhaseSucceeded || phase == topolvmv1.OperationPhaseFailed
}
