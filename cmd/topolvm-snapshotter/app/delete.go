package app

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/backupengine/provider"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DeleteOptions struct {
	log    logr.Logger
	client client.Client

	// Logical volume details
	lvName     string
	repository string
	logicalVol *topolvmv1.LogicalVolume

	//timeout            time.Duration
	snapshotStorageRef types.NamespacedName
	snapshotStorage    *topolvmv1.SnapshotBackupStorage
}

var dOpt = new(DeleteOptions)

func newDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete snapshot of a logical volume ",
		Long:  `The delete command performs delete opeeration of an online snapshot of a logical volume.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			dOpt.log = ctrl.Log.WithName("delete")

			if err := dOpt.initialize(ctx); err != nil {
				dOpt.log.Error(err, "initialization failed")
				return err
			}

			if err := dOpt.execute(ctx); err != nil {
				dOpt.log.Error(err, "backup execution failed")
				return err
			}
			return nil
		},
	}
	parseDeleteFlags(cmd)
	return cmd
}

func parseDeleteFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&masterURL, "master", masterURL,
		"The address of the Kubernetes API server")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", kubeconfigPath,
		"Path to kubeconfig file with authorization information")

	cmd.Flags().StringVar(&dOpt.repository, "repository",
		"", "Repository URL (required)")
	cmd.Flags().StringVar(&dOpt.lvName, "lv-name",
		"", "Name of the logical volume to backup (required)")
	cmd.Flags().StringVar(&dOpt.snapshotStorageRef.Namespace, "snapshot-storage-namespace",
		"", "Namespace of the SnapshotBackupStorage CR")
	cmd.Flags().StringVar(&dOpt.snapshotStorageRef.Name, "snapshot-storage-name",
		"", "Name of the SnapshotBackupStorage CR")
}

func (opt *DeleteOptions) execute(ctx context.Context) error {
	if err := opt.setDeleteConditionToRunning(ctx); err != nil {
		return fmt.Errorf("failed to set delete condition to running: %w", err)
	}
	if err := opt.performDelete(ctx); err != nil {
		opt.log.Error(err, "delete operation failed")
	}
	return nil
}

func (opt *DeleteOptions) performDelete(ctx context.Context) error {
	opt.log.Info("Starting delete operation", "lvName", opt.lvName)
	pvider, err := opt.getDeleteProvider()
	if err != nil {
		opt.handleDeleteError(ctx, "failed to initialize delete provider", err)
		return err
	}

	if err = opt.executeDelete(ctx, pvider); err != nil {
		opt.handleDeleteError(ctx, "delete execution failed", err)
		return err
	}
	if err = opt.setDeleteConditionToSuccess(ctx); err != nil {
		return fmt.Errorf("failed to set delete condition to success: %w", err)
	}
	return nil
}

func (opt *DeleteOptions) executeDelete(ctx context.Context, pvider provider.Provider) error {
	param := opt.buildDeleteParams()
	_, err := pvider.Delete(ctx, param)
	if err != nil {
		return fmt.Errorf("delete operation failed: %w", err)
	}
	return nil
}

func (opt *DeleteOptions) buildDeleteParams() provider.DeleteParam {
	return provider.DeleteParam{
		RepoRef: provider.RepoRef{
			Repository: &opt.repository,
		},
		SnapshotIDs: []string{opt.logicalVol.Status.Snapshot.SnapshotID},
	}
}

func (opt *DeleteOptions) getDeleteProvider() (provider.Provider, error) {
	pvider, err := provider.GetProvider(opt.client, opt.snapshotStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot provider: %w", err)
	}
	opt.log.Info("delete provider initialized", "engine", opt.snapshotStorage.Spec.Engine)
	return pvider, nil
}

func (opt *DeleteOptions) handleDeleteError(ctx context.Context, message string, err error) {
	opt.log.Error(err, message, "lvName", opt.lvName)
	errorMsg := err.Error()
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotDeleteEnsured,
		Status:  metav1.ConditionTrue,
		Reason:  topolvmv1.ConditionDeleteCleanupFailed,
		Message: errorMsg,
	}
	meta.SetStatusCondition(&opt.logicalVol.Status.Conditions, newCond)
	if updateErr := updateLVStatusCondition(ctx, opt.client, opt.logicalVol); updateErr != nil {
		opt.log.Error(updateErr, "failed to set error status in condition",
			"originalError", errorMsg)
	}
	opt.log.Info("Set status condition", "lvName", opt.lvName, "error", errorMsg)
}

func (opt *DeleteOptions) setDeleteConditionToRunning(ctx context.Context) error {
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotDeleteEnsured,
		Status:  metav1.ConditionTrue,
		Reason:  topolvmv1.ConditionSnapshotDeleteRunning,
		Message: "Snapshot Delete is in progress.",
	}
	meta.SetStatusCondition(&opt.logicalVol.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, opt.client, opt.logicalVol)
}

//func (opt *DeleteOptions) setDeleteConditionToFailed(ctx context.Context, err error) error {
//	errorMsg := err.Error()
//	newCond := metav1.Condition{
//		Type:    topolvmv1.TypeSnapshotDeleteEnsured,
//		Status:  metav1.ConditionTrue,
//		Reason:  topolvmv1.ConditionDeleteCleanupFailed,
//		Message: errorMsg,
//	}
//	meta.SetStatusCondition(&opt.logicalVol.Status.Conditions, newCond)
//	if updateErr := updateLVStatusCondition(ctx, opt.client, opt.logicalVol); updateErr != nil {
//		return fmt.Errorf("failed to update status condition: %w", updateErr)
//	}
//	return nil
//}

func (opt *DeleteOptions) setDeleteConditionToSuccess(ctx context.Context) error {
	newCond := metav1.Condition{
		Type:    topolvmv1.TypeSnapshotDeleteEnsured,
		Status:  metav1.ConditionTrue,
		Reason:  topolvmv1.ConditionSnapshotDeleteSucceeded,
		Message: "Snapshot Delete has been completed successfully.",
	}
	meta.SetStatusCondition(&opt.logicalVol.Status.Conditions, newCond)
	return updateLVStatusCondition(ctx, opt.client, opt.logicalVol)
}

func (opt *DeleteOptions) initialize(ctx context.Context) error {
	if err := opt.setupKubernetesClient(); err != nil {
		return fmt.Errorf("failed to setup kubernetes client: %w", err)
	}

	if err := opt.loadResources(ctx); err != nil {
		return fmt.Errorf("failed to load resources: %w", err)
	}

	return nil
}

func (opt *DeleteOptions) setupKubernetesClient() error {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to build config: %w", err)
	}

	opt.client, err = NewRuntimeClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	opt.log.Info("kubernetes client initialized successfully")
	return nil
}

func (opt *DeleteOptions) loadResources(ctx context.Context) error {
	var err error
	opt.snapshotStorage, err = fetchSnapshotStorage(ctx, opt.client,
		metav1.ObjectMeta{Name: opt.snapshotStorageRef.Name, Namespace: opt.snapshotStorageRef.Namespace})
	if err != nil {
		return fmt.Errorf("failed to fetch SnapshotBackupStorage: %w", err)
	}
	opt.log.Info("loaded snapshot storage", "name", opt.snapshotStorage.Name, "namespace", opt.snapshotStorage.Namespace)

	opt.logicalVol, err = fetchLogicalVolume(ctx, opt.client, metav1.ObjectMeta{Name: opt.lvName})
	if err != nil {
		return fmt.Errorf("failed to fetch LogicalVolume: %w", err)
	}
	opt.log.Info("loaded logical volume", "name", opt.logicalVol.Name)

	return nil
}
