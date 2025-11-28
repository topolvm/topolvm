package controller

import (
	"context"
	"fmt"

	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func setSnapshotBackupExecutorEnsuredToTrue(ctx context.Context, client client.Client, lv *topolvmv1.LogicalVolume) error {
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotBackupExecutorEnsured,
		Status:  metav1.ConditionTrue,
		Reason:  topolvmv1.ReasonSuccessfullyEnsuredSnapshotBackupExecutor,
		Message: "Snapshot Backup Executor has been ensured successfully.",
	}
	meta.SetStatusCondition(&lv.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, client, lv)
}

func setSnapshotBackupExecutorEnsuredToFalse(ctx context.Context, client client.Client, lv *topolvmv1.LogicalVolume, err error) error {
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotBackupExecutorEnsured,
		Status:  metav1.ConditionFalse,
		Reason:  topolvmv1.ReasonFailedToEnsureSnapshotBackupExecutor,
		Message: fmt.Sprintf("Failed to ensure Snapshot Backup Executor: %q", err.Error()),
	}
	meta.SetStatusCondition(&lv.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, client, lv)
}

func setSnapshotRestoreExecutorEnsuredToTrue(ctx context.Context, client client.Client, lv *topolvmv1.LogicalVolume) error {
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotRestoreExecutorEnsured,
		Status:  metav1.ConditionTrue,
		Reason:  topolvmv1.ReasonSuccessfullyEnsuredSnapshotRestoreExecutor,
		Message: "Snapshot Restore Executor has been ensured successfully.",
	}
	meta.SetStatusCondition(&lv.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, client, lv)
}

func setSnapshotRestoreExecutorEnsuredToFalse(ctx context.Context, client client.Client, lv *topolvmv1.LogicalVolume, err error) error {
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotRestoreExecutorEnsured,
		Status:  metav1.ConditionFalse,
		Reason:  topolvmv1.ReasonFailedToEnsureSnapshotRestoreExecutor,
		Message: fmt.Sprintf("Failed to ensure Snapshot Restore Executor: %q", err.Error()),
	}
	meta.SetStatusCondition(&lv.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, client, lv)
}

func setSnapshotExecutorCleanupToTrue(ctx context.Context, client client.Client, operation topolvmv1.OperationType, lv *topolvmv1.LogicalVolume) error {
	var newCond metav1.Condition
	if operation == topolvmv1.OperationBackup {
		newCond = metav1.Condition{
			Type:    topolvmv1.TypeSnapshotBackupExecutorCleaned,
			Status:  metav1.ConditionTrue,
			Reason:  topolvmv1.ReasonSuccessfullyCleanedSnapshotBackupExecutor,
			Message: "Snapshot Backup Executor has been cleaned successfully.",
		}
	} else if operation == topolvmv1.OperationRestore {
		newCond = metav1.Condition{
			Type:    topolvmv1.TypeSnapshotRestoreExecutorCleaned,
			Status:  metav1.ConditionTrue,
			Reason:  topolvmv1.ReasonSuccessfullyCleanedSnapshotRestoreExecutor,
			Message: "Snapshot Restore Executor has been cleaned successfully.",
		}
	}
	meta.SetStatusCondition(&lv.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, client, lv)
}

func setSnapshotExecutorCleanupToFalse(ctx context.Context, client client.Client, operation topolvmv1.OperationType, lv *topolvmv1.LogicalVolume, err error) error {
	var newCond metav1.Condition
	if operation == topolvmv1.OperationBackup {
		newCond = metav1.Condition{
			Type:    topolvmv1.TypeSnapshotBackupExecutorCleaned,
			Status:  metav1.ConditionFalse,
			Reason:  topolvmv1.ReasonFailedToCleanedSnapshotBackupExecutor,
			Message: fmt.Sprintf("Failed to clean Snapshot Backup Executor: %q", err.Error()),
		}
	} else if operation == topolvmv1.OperationRestore {
		newCond = metav1.Condition{
			Type:    topolvmv1.TypeSnapshotRestoreExecutorCleaned,
			Status:  metav1.ConditionFalse,
			Reason:  topolvmv1.ReasonFailedToCleanedSnapshotRestoreExecutor,
			Message: fmt.Sprintf("Failed to clean Snapshot Restore Executor: %q", err.Error()),
		}
	}
	meta.SetStatusCondition(&lv.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, client, lv)
}

func setSnapshotDeleteExecutorEnsuredToTrue(ctx context.Context, client client.Client, lv *topolvmv1.LogicalVolume) error {
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotDeleteExecutorEnsured,
		Status:  metav1.ConditionTrue,
		Reason:  topolvmv1.ReasonSuccessfullyEnsuredSnapshotDeleteExecutor,
		Message: "Snapshot Delete Executor has been ensured successfully.",
	}
	meta.SetStatusCondition(&lv.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, client, lv)
}

func setSnapshotDeleteExecutorEnsuredToFalse(ctx context.Context, client client.Client, lv *topolvmv1.LogicalVolume, err error) error {
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotDeleteExecutorEnsured,
		Status:  metav1.ConditionFalse,
		Reason:  topolvmv1.ReasonFailedToEnsureSnapshotDeleteExecutor,
		Message: fmt.Sprintf("Failed to ensure Snapshot Delete Executor: %q", err.Error()),
	}
	meta.SetStatusCondition(&lv.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, client, lv)
}

func hasSnapshotDeleteExecutorCondition(lv *topolvmv1.LogicalVolume) bool {
	return meta.IsStatusConditionFalse(lv.Status.Conditions, topolvmv1.TypeSnapshotDeleteExecutorEnsured) ||
		meta.IsStatusConditionTrue(lv.Status.Conditions, topolvmv1.TypeSnapshotDeleteExecutorEnsured)
}

func hasConditionSnapshotDeleteSucceeded(lv *topolvmv1.LogicalVolume) bool {
	for _, condition := range lv.Status.Conditions {
		if condition.Type == topolvmv1.TypeSnapshotDeleteEnsured &&
			condition.Status == metav1.ConditionTrue &&
			condition.Reason == topolvmv1.ConditionSnapshotDeleteSucceeded {
			return true
		}
	}
	return false
}

func hasSnapshotBackupExecutorCleanupCondition(lv *topolvmv1.LogicalVolume) bool {
	return meta.IsStatusConditionFalse(lv.Status.Conditions, topolvmv1.TypeSnapshotBackupExecutorCleaned) ||
		meta.IsStatusConditionTrue(lv.Status.Conditions, topolvmv1.TypeSnapshotBackupExecutorCleaned)
}

func hasSnapshotRestoreExecutorCleanupCondition(lv *topolvmv1.LogicalVolume) bool {
	return meta.IsStatusConditionFalse(lv.Status.Conditions, topolvmv1.TypeSnapshotRestoreExecutorCleaned) ||
		meta.IsStatusConditionTrue(lv.Status.Conditions, topolvmv1.TypeSnapshotRestoreExecutorCleaned)
}

func updateLVStatus(ctx context.Context, kClient client.Client, lv *topolvmv1.LogicalVolume) error {
	// Refresh the LogicalVolume to get the latest version
	freshLV := &topolvmv1.LogicalVolume{}
	if err := kClient.Get(ctx, client.ObjectKeyFromObject(lv), freshLV); err != nil {
		return fmt.Errorf("failed to get latest LogicalVolume: %w", err)
	}
	freshLV.Status = lv.Status
	if err := kClient.Status().Update(ctx, freshLV); err != nil {
		return fmt.Errorf("failed to update snapshot status: %w", err)
	}
	lv.Status = freshLV.Status
	lv.ObjectMeta.ResourceVersion = freshLV.ObjectMeta.ResourceVersion
	return nil
}

func updateLVStatusCondition(ctx context.Context, kClient client.Client, lv *topolvmv1.LogicalVolume) error {
	// Refresh the LogicalVolume to get the latest version
	freshLV := &topolvmv1.LogicalVolume{}
	if err := kClient.Get(ctx, client.ObjectKeyFromObject(lv), freshLV); err != nil {
		return fmt.Errorf("failed to get latest LogicalVolume: %w", err)
	}
	if freshLV.Status.Snapshot == nil {
		freshLV.Status.Snapshot = &topolvmv1.SnapshotStatus{
			StartTime: metav1.Now(),
		}
	}
	freshLV.Status.Conditions = lv.Status.Conditions
	if err := kClient.Status().Update(ctx, freshLV); err != nil {
		return fmt.Errorf("failed to update snapshot status: %w", err)
	}
	lv.Status = freshLV.Status
	lv.ObjectMeta.ResourceVersion = freshLV.ObjectMeta.ResourceVersion
	return nil
}
