package app

import (
	"context"
	"fmt"

	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func NewRuntimeClient(config *restclient.Config) (client.Client, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(topolvmv1.AddToScheme(scheme))

	hc, err := restclient.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}
	mapper, err := apiutil.NewDynamicRESTMapper(config, hc)
	if err != nil {
		return nil, err
	}

	return client.New(config, client.Options{
		Scheme: scheme,
		Mapper: mapper,
	})
}

func fetchSnapshotStorage(ctx context.Context, rClient client.Client,
	objMeta metav1.ObjectMeta) (*topolvmv1.SnapshotBackupStorage, error) {
	storage := &topolvmv1.SnapshotBackupStorage{
		ObjectMeta: objMeta,
	}
	if err := rClient.Get(ctx, client.ObjectKeyFromObject(storage), storage); err != nil {
		return nil, fmt.Errorf("failed to get SnapshotBackupStorage %s/%s: %w",
			storage.Namespace, storage.Name, err)
	}
	return storage, nil
}

func fetchLogicalVolume(ctx context.Context, rClient client.Client,
	objMeta metav1.ObjectMeta) (*topolvmv1.LogicalVolume, error) {
	lv := &topolvmv1.LogicalVolume{
		ObjectMeta: objMeta,
	}
	if err := rClient.Get(ctx, client.ObjectKeyFromObject(lv), lv); err != nil {
		return nil, fmt.Errorf("failed to get LogicalVolume %s: %w", lv.Name, err)
	}
	return lv, nil
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
	lv.ResourceVersion = freshLV.ResourceVersion
	return nil
}

func setStatusFailed(ctx context.Context, rClient client.Client,
	logicalVol *topolvmv1.LogicalVolume, errorMessage string) error {
	snapshotErr := &topolvmv1.SnapshotError{
		Code:    backupErrorCode,
		Message: errorMessage,
	}

	if logicalVol.Status.Snapshot == nil {
		logicalVol.Status.Snapshot = &topolvmv1.SnapshotStatus{
			StartTime: metav1.Now(),
		}
	}
	now := metav1.Now()
	logicalVol.Status.Snapshot.Error = snapshotErr
	logicalVol.Status.Snapshot.Phase = topolvmv1.OperationPhaseFailed
	logicalVol.Status.Snapshot.CompletionTime = &now
	logicalVol.Status.Snapshot.Message = fmt.Sprintf("Backup failed: %s", errorMessage)

	if err := rClient.Status().Update(ctx, logicalVol); err != nil {
		return fmt.Errorf("failed to update online snapshot status: %w", err)
	}
	return nil
}
