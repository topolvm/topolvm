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
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
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

func fetchSnapshotStorage(ctx context.Context, client client.Client, objMeta metav1.ObjectMeta) (*topolvmv1.OnlineSnapshotStorage, error) {
	storage := &topolvmv1.OnlineSnapshotStorage{
		ObjectMeta: objMeta,
	}
	if err := client.Get(ctx, runtimeclient.ObjectKeyFromObject(storage), storage); err != nil {
		return nil, fmt.Errorf("failed to get OnlineSnapshotStorage %s/%s: %w",
			storage.Namespace, storage.Name, err)
	}
	return storage, nil
}

func fetchLogicalVolume(ctx context.Context, client client.Client, objMeta metav1.ObjectMeta) (*topolvmv1.LogicalVolume, error) {
	lv := &topolvmv1.LogicalVolume{
		ObjectMeta: objMeta,
	}
	if err := client.Get(ctx, runtimeclient.ObjectKeyFromObject(lv), lv); err != nil {
		return nil, fmt.Errorf("failed to get LogicalVolume %s: %w", lv.Name, err)
	}
	return lv, nil
}

func updateStatus(ctx context.Context, client client.Client, lv *topolvmv1.LogicalVolume,
	phase topolvmv1.OperationPhase, message string, snapshotErr *topolvmv1.SnapshotError) error {

	if lv.Status.Snapshot == nil {
		startTime := metav1.Now()
		lv.Status.Snapshot = &topolvmv1.SnapshotStatus{
			StartTime: startTime,
		}
	}

	lv.Status.Snapshot.Phase = phase
	lv.Status.Snapshot.Message = message

	if snapshotErr != nil {
		lv.Status.Snapshot.Error = snapshotErr
	}

	if err := client.Status().Update(ctx, lv); err != nil {
		return fmt.Errorf("failed to update online snapshot status: %w", err)
	}

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
