package app

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/snapshotengine/provider"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RestoreOptions struct {
	log    logr.Logger
	client client.Client

	// Logical volume details
	lvName      string
	nodeName    string
	deviceClass string
	mountPath   string
	logicalVol  *topolvmv1.LogicalVolume

	// References
	repository         string
	snapshotID         string
	timeout            time.Duration
	snapshotStorageRef types.NamespacedName
	snapshotStorage    *topolvmv1.OnlineSnapshotStorage
}

var rOpt = new(RestoreOptions)

func newRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore a logical volume from an online snapshot",
		Long: `The restore command restores data from a previously created online snapshot.
				It retrieves the data from the remote repository using Restic or Kopia and writes it to the mounted filesystem.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			rOpt.log = ctrl.Log.WithName("restore")

			if err := rOpt.initialize(ctx); err != nil {
				rOpt.log.Error(err, "initialization failed")
				return err
			}
			if err := rOpt.execute(ctx); err != nil {
				rOpt.log.Error(err, "backup execution failed")
				return err
			}
			return nil
		},
	}
	parseRestoreFlags(cmd)
	return cmd
}

func parseRestoreFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&masterURL, "master", masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag)")

	cmd.Flags().StringVar(&rOpt.lvName, "lv-name", "", "Name of the logical volume to backup (required)")
	cmd.Flags().StringVar(&rOpt.nodeName, "node-name", "", "Node name where the logical volume resides (required)")
	cmd.Flags().StringVar(&rOpt.mountPath, "mount-path", "", "Mount path of the logical volume (required)")
	cmd.Flags().StringVar(&rOpt.repository, "repository", "", "Repository URL (required)")
	cmd.Flags().StringVar(&rOpt.snapshotID, "snapshot-id", "", "Snapshot ID (required)")
	cmd.Flags().StringVar(&rOpt.snapshotStorageRef.Namespace, "snapshot-storage-namespace", "", "Namespace of the OnlineSnapshotStorage CR")
	cmd.Flags().StringVar(&rOpt.snapshotStorageRef.Name, "snapshot-storage-name", "", "Name of the OnlineSnapshotStorage CR")
}

func (opt *RestoreOptions) execute(ctx context.Context) error {
	if err := opt.setStatusRunning(ctx); err != nil {
		return fmt.Errorf("failed to set status to running: %w", err)
	}

	if err := opt.performRestore(ctx); err != nil {
		return fmt.Errorf("restore operation failed: %w", err)
	}

	return nil
}

func (opt *RestoreOptions) setStatusRunning(ctx context.Context) error {
	if opt.logicalVol.Status.Snapshot == nil {
		opt.logicalVol.Status.Snapshot = &topolvmv1.SnapshotStatus{}
	}

	opt.logicalVol.Status.Snapshot.StartTime = metav1.Now()
	opt.logicalVol.Status.Snapshot.Phase = topolvmv1.OperationPhaseRunning
	opt.logicalVol.Status.Snapshot.Message = fmt.Sprintf("Restore execution in progress")
	if err := opt.client.Status().Update(ctx, opt.logicalVol); err != nil {
		return fmt.Errorf("failed to update online snapshot status: %w", err)
	}
	opt.log.Info("status updated to running", "lvName", opt.lvName)
	return nil
}

func (opt *RestoreOptions) performRestore(ctx context.Context) error {
	opt.log.Info("starting restore operation",
		"lvName", opt.lvName,
		"nodeName", opt.nodeName,
		"mountPath", opt.mountPath)

	pvider, err := opt.getRestoreProvider()
	if err != nil {
		opt.handleRestoreError(ctx, "failed to initialize restore provider", err)
		return err
	}

	result, err := opt.executeRestore(ctx, pvider)
	if err != nil {
		opt.handleRestoreError(ctx, "restore execution failed", err)
		return err
	}

	if err := opt.handleRestoreSuccess(ctx, result); err != nil {
		return fmt.Errorf("failed to update success status: %w", err)
	}
	return nil
}

func (opt *RestoreOptions) handleRestoreSuccess(ctx context.Context, result *provider.RestoreResult) error {
	if result == nil {
		return fmt.Errorf("restore result is nil")
	}

	if err := opt.setStatusSuccess(ctx, result); err != nil {
		opt.log.Error(err, "failed to update success status")
		return err
	}
	return nil
}

func (opt *RestoreOptions) setStatusSuccess(ctx context.Context, result *provider.RestoreResult) error {
	// Refresh the LogicalVolume to get latest version
	lv, err := fetchLogicalVolume(ctx, opt.client, metav1.ObjectMeta{Name: opt.lvName})
	if err != nil {
		return fmt.Errorf("failed to refresh logical volume: %w", err)
	}

	// Initialize OnlineSnapshot status if needed
	if lv.Status.Snapshot == nil {
		lv.Status.Snapshot = &topolvmv1.SnapshotStatus{}
	}

	now := metav1.Now()
	lv.Status.Snapshot.Phase = topolvmv1.OperationPhaseSucceeded
	lv.Status.Snapshot.Message = fmt.Sprintf("Restore completed successfully")
	lv.Status.Snapshot.CompletionTime = &now
	lv.Status.Snapshot.Duration = result.Duration
	lv.Status.Snapshot.Version = result.Provider
	lv.Status.Snapshot.Error = nil
	lv.Status.Snapshot.Repository = result.Repository
	if err := opt.client.Status().Update(ctx, lv); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	opt.log.Info("status updated to completed", "lvName", lv.Name, "duration", result.Duration)
	return nil
}

func (opt *RestoreOptions) getRestoreProvider() (provider.Provider, error) {
	pvider, err := provider.GetProvider(opt.client, opt.snapshotStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot provider: %w", err)
	}
	opt.log.Info("backup provider initialized", "engine", opt.snapshotStorage.Spec.Engine)
	return pvider, nil
}

func (opt *RestoreOptions) executeRestore(ctx context.Context, pvider provider.Provider) (*provider.RestoreResult, error) {
	params := opt.buildRestoreParams()
	opt.log.Info("executing restore with params",
		"repository", params.Repository,
		"paths", params.RestorePaths,
		"exclude", params.Exclude,
		"args", params.Args)

	result, err := pvider.Restore(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("restore operation failed: %w", err)
	}

	if result != nil && result.Phase == provider.RestoreFailed {
		return nil, fmt.Errorf("result failed: %s", result.ErrorMessage)
	}

	return result, nil
}

func (opt *RestoreOptions) buildRestoreParams() provider.RestoreParam {
	return provider.RestoreParam{
		RepoRef: provider.RepoRef{
			Repository: &opt.repository,
			Hostname:   hostname,
		},
		SnapshotID: opt.snapshotID,
		//Destination: opt.mountPath,
	}
}

func (opt *RestoreOptions) handleRestoreError(ctx context.Context, message string, err error) {
	opt.log.Error(err, message, "lvName", opt.lvName)

	errorMsg := err.Error()
	if updateErr := opt.setStatusFailed(ctx, errorMsg); updateErr != nil {
		opt.log.Error(updateErr, "failed to update error status", "originalError", errorMsg)
	}

	opt.log.Info("status updated to failed", "lvName", opt.lvName, "error", errorMsg)
}

func (opt *RestoreOptions) setStatusFailed(ctx context.Context, errorMessage string) error {
	snapshotErr := &topolvmv1.SnapshotError{
		Code:    restoreErrorCode,
		Message: errorMessage,
	}

	if opt.logicalVol.Status.Snapshot == nil {
		opt.logicalVol.Status.Snapshot = &topolvmv1.SnapshotStatus{
			StartTime: metav1.Now(),
		}
	}
	now := metav1.Now()
	opt.logicalVol.Status.Snapshot.Error = snapshotErr
	opt.logicalVol.Status.Snapshot.Phase = topolvmv1.OperationPhaseFailed
	opt.logicalVol.Status.Snapshot.CompletionTime = &now
	opt.logicalVol.Status.Snapshot.Message = fmt.Sprintf("Restore failed: %s", errorMessage)

	if err := opt.client.Status().Update(ctx, opt.logicalVol); err != nil {
		return fmt.Errorf("failed to update snapshot status: %w", err)
	}
	return nil
}

func (opt *RestoreOptions) initialize(ctx context.Context) error {
	if err := opt.setupKubernetesClient(); err != nil {
		return fmt.Errorf("failed to setup kubernetes client: %w", err)
	}

	if err := opt.loadResources(ctx); err != nil {
		return fmt.Errorf("failed to load resources: %w", err)
	}

	return nil
}

func (opt *RestoreOptions) setupKubernetesClient() error {
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

func (opt *RestoreOptions) loadResources(ctx context.Context) error {
	var err error
	opt.snapshotStorage, err = fetchSnapshotStorage(ctx, opt.client, metav1.ObjectMeta{Name: opt.snapshotStorageRef.Name, Namespace: opt.snapshotStorageRef.Namespace})
	if err != nil {
		return fmt.Errorf("failed to fetch OnlineSnapshotStorage: %w", err)
	}
	opt.log.Info("loaded snapshot storage", "name", opt.snapshotStorage.Name, "namespace", opt.snapshotStorage.Namespace)

	opt.logicalVol, err = fetchLogicalVolume(ctx, opt.client, metav1.ObjectMeta{Name: opt.lvName})
	if err != nil {
		return fmt.Errorf("failed to fetch LogicalVolume: %w", err)
	}
	opt.log.Info("loaded logical volume", "name", opt.logicalVol.Name)

	return nil
}
