package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	topolvmv1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/backupengine/provider"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	masterURL      string
	kubeconfigPath string
)

type BackupOptions struct {
	log    logr.Logger
	client client.Client

	// Logical volume details
	lvName      string
	nodeName    string
	deviceClass string
	mountPath   string
	logicalVol  *topolvmv1.LogicalVolume

	// References
	timeout            time.Duration
	targetedPVCRef     types.NamespacedName
	snapshotStorageRef types.NamespacedName
	snapshotStorage    *topolvmv1.OnlineSnapshotStorage
}

var bOpt = new(BackupOptions)

func newBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Take an online snapshot of a logical volume",
		Long: `The backup command performs an online snapshot of a logical volume.
		It backs up the mounted filesystem to a remote repository using Restic or Kopia.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			bOpt.log = ctrl.Log.WithName("backup")

			if err := bOpt.initialize(ctx); err != nil {
				bOpt.log.Error(err, "initialization failed")
				return err
			}

			if err := bOpt.execute(ctx); err != nil {
				bOpt.log.Error(err, "backup execution failed")
				return err
			}
			return nil
		},
	}
	parseBackupFlags(cmd)
	return cmd
}

func parseBackupFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&masterURL, "master", masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag)")

	cmd.Flags().StringVar(&bOpt.lvName, "lv-name", "", "Name of the logical volume to backup (required)")
	cmd.Flags().StringVar(&bOpt.nodeName, "node-name", "", "Node name where the logical volume resides (required)")
	cmd.Flags().StringVar(&bOpt.mountPath, "mount-path", "", "Mount path of the logical volume (required)")

	cmd.Flags().StringVar(&bOpt.targetedPVCRef.Namespace, "targeted-pvc-namespace", "", "Namespace of the targeted PVC")
	cmd.Flags().StringVar(&bOpt.targetedPVCRef.Name, "targeted-pvc-name", "", "Name of the targeted PVC")

	cmd.Flags().StringVar(&bOpt.snapshotStorageRef.Namespace, "snapshot-storage-namespace", "", "Namespace of the OnlineSnapshotStorage CR")
	cmd.Flags().StringVar(&bOpt.snapshotStorageRef.Name, "snapshot-storage-name", "", "Name of the OnlineSnapshotStorage CR")
}

func (opt *BackupOptions) initialize(ctx context.Context) error {
	if err := opt.setupKubernetesClient(); err != nil {
		return fmt.Errorf("failed to setup kubernetes client: %w", err)
	}

	if err := opt.loadResources(ctx); err != nil {
		return fmt.Errorf("failed to load resources: %w", err)
	}

	return nil
}

func (opt *BackupOptions) setupKubernetesClient() error {
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

func (opt *BackupOptions) loadResources(ctx context.Context) error {
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

func (opt *BackupOptions) execute(ctx context.Context) error {
	if err := opt.setStatusRunning(ctx); err != nil {
		return fmt.Errorf("failed to set status to running: %w", err)
	}

	if err := opt.performBackup(ctx); err != nil {
		return fmt.Errorf("backup operation failed: %w", err)
	}

	return nil
}

func (opt *BackupOptions) performBackup(ctx context.Context) error {
	opt.log.Info("starting backup operation",
		"lvName", opt.lvName,
		"nodeName", opt.nodeName,
		"mountPath", opt.mountPath)

	pvider, err := opt.getBackupProvider()
	if err != nil {
		opt.handleBackupError(ctx, "failed to initialize backup provider", err)
		return err
	}

	result, err := opt.executeBackup(ctx, pvider)
	if err != nil {
		opt.handleBackupError(ctx, "backup execution failed", err)
		return err
	}

	if err := opt.handleBackupSuccess(ctx, result); err != nil {
		return fmt.Errorf("failed to update success status: %w", err)
	}
	return nil
}

func (opt *BackupOptions) getBackupProvider() (provider.Provider, error) {
	pvider, err := provider.GetProvider(opt.client, opt.snapshotStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot provider: %w", err)
	}
	opt.log.Info("backup provider initialized", "engine", opt.snapshotStorage.Spec.Engine)
	return pvider, nil
}

func (opt *BackupOptions) executeBackup(ctx context.Context, pvider provider.Provider) (*provider.BackupResult, error) {
	params := opt.buildBackupParams()
	opt.log.Info("executing backup with params",
		"repository", params.Suffix,
		"paths", params.BackupPaths,
		"exclude", params.Exclude,
		"args", params.Args,
	)

	result, err := pvider.Backup(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("backup operation failed: %w", err)
	}

	if result != nil && result.Phase == provider.BackupPhaseFailed {
		return nil, fmt.Errorf("backup failed: %s", result.ErrorMessage)
	}

	return result, nil
}

func (opt *BackupOptions) handleBackupError(ctx context.Context, message string, err error) {
	opt.log.Error(err, message, "lvName", opt.lvName)

	errorMsg := err.Error()
	if updateErr := opt.setStatusFailed(ctx, errorMsg); updateErr != nil {
		opt.log.Error(updateErr, "failed to update error status", "originalError", errorMsg)
	}

	opt.log.Info("status updated to failed", "lvName", opt.lvName, "error", errorMsg)
}

func (opt *BackupOptions) handleBackupSuccess(ctx context.Context, result *provider.BackupResult) error {
	if result == nil {
		return fmt.Errorf("backup result is nil")
	}

	if err := opt.setStatusSuccess(ctx, result); err != nil {
		opt.log.Error(err, "failed to update success status")
		return err
	}
	opt.log.Info("backup completed successfully",
		"snapshotID", result.SnapshotID,
		"duration", result.Duration,
		"uploaded", result.Size.UploadedFormatted,
		"files", result.Files.Total)
	return nil
}

func (opt *BackupOptions) buildBackupParams() provider.BackupParam {
	return provider.BackupParam{
		RepoRef: provider.RepoRef{
			Suffix:   filepath.Join(opt.targetedPVCRef.Namespace, opt.targetedPVCRef.Name),
			Hostname: hostname,
		},
		BackupPaths: []string{opt.mountPath},
	}
}

func (opt *BackupOptions) setStatusRunning(ctx context.Context) error {
	if opt.logicalVol.Status.Snapshot == nil {
		opt.logicalVol.Status.Snapshot = &topolvmv1.SnapshotStatus{}
	}

	opt.logicalVol.Status.Snapshot.StartTime = metav1.Now()
	opt.logicalVol.Status.Snapshot.Phase = topolvmv1.OperationPhaseRunning
	opt.logicalVol.Status.Snapshot.Message = fmt.Sprintf("Snapshot execution in progress")
	if err := opt.client.Status().Update(ctx, opt.logicalVol); err != nil {
		return fmt.Errorf("failed to update online snapshot status: %w", err)
	}
	opt.log.Info("status updated to running", "lvName", opt.lvName)
	return nil
}

func (opt *BackupOptions) setStatusSuccess(ctx context.Context, result *provider.BackupResult) error {
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
	lv.Status.Snapshot.SnapshotID = result.SnapshotID
	lv.Status.Snapshot.Message = fmt.Sprintf("Backup completed successfully")
	lv.Status.Snapshot.CompletionTime = &now
	lv.Status.Snapshot.Duration = result.Duration
	lv.Status.Snapshot.Version = result.Provider
	lv.Status.Snapshot.Error = nil
	lv.Status.Snapshot.Repository = result.Repository
	if err := opt.client.Status().Update(ctx, lv); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	opt.log.Info("status updated to completed",
		"lvName", lv.Name,
		"snapshotID", result.SnapshotID,
		"uploaded", result.Size.UploadedFormatted,
		"total", result.Size.TotalFormatted,
		"files", result.Files.Total,
		"duration", result.Duration)

	return nil
}

func (opt *BackupOptions) setStatusFailed(ctx context.Context, errorMessage string) error {
	snapshotErr := &topolvmv1.SnapshotError{
		Code:    backupErrorCode,
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
	opt.logicalVol.Status.Snapshot.Message = fmt.Sprintf("Backup failed: %s", errorMessage)

	if err := opt.client.Status().Update(ctx, opt.logicalVol); err != nil {
		return fmt.Errorf("failed to update online snapshot status: %w", err)
	}
	return nil
}
