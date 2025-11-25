package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "github.com/topolvm/topolvm/api/v1"
	"github.com/topolvm/topolvm/internal/backupengine/config"
	"github.com/topolvm/topolvm/internal/backupengine/restic"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewResticProvider creates a new Restic repository provider
func NewResticProvider(client client.Client, snapStorage *v1.OnlineSnapshotStorage) (Provider, error) {
	setupOptions, err := config.NewSetupOptionsForStorage(client, snapStorage)
	if err != nil {
		return nil, fmt.Errorf("failed to create setup options: %w", err)
	}
	wrapper, err := restic.NewResticWrapper(*setupOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create restic wrapper: %w", err)
	}

	return &resticProvider{
		wrapper: wrapper,
	}, nil
}

type resticProvider struct {
	wrapper *restic.ResticWrapper
}

func (r *resticProvider) Delete(ctx context.Context, param DeleteParam) ([]byte, error) {
	r.wrapper.SetShowCMD(true)
	r.wrapper.SetRepository(*param.RepoRef.Repository)
	fmt.Println("############## REPOSITORY: ", *param.RepoRef.Repository)
	fmt.Println("############## REPO: ", r.wrapper.GetEnv(restic.RESTIC_REPOSITORY))
	return r.wrapper.DeleteSnapshots(param.SnapshotIDs)
}

func (r *resticProvider) ValidateConnection(ctx context.Context) error {
	type resticErrorJSON struct {
		MessageType string `json:"message_type"`
		Code        int    `json:"code"`
		Message     string `json:"message"`
	}

	r.wrapper.SetCombineOutput(true)
	r.wrapper.SetShowCMD(true)
	out, err := r.wrapper.ValidateConnection()
	var resticErr resticErrorJSON
	jsonErr := json.Unmarshal(out, &resticErr)

	// It approach will work only once this PR gets merged: https://github.com/restic/restic/pull/5570
	if jsonErr == nil && resticErr.Message != "" {
		msg := resticErr.Message
		switch {
		case strings.Contains(msg, "repository does not exist"),
			strings.Contains(msg, "The specified key does not exist"):
			return nil

		case strings.Contains(msg, "Access Denied"):
			return fmt.Errorf("invalid credentials: %s", msg)

		case strings.Contains(msg, "no such host"):
			return fmt.Errorf("invalid endpoint or DNS resolution failed: %s", msg)

		case strings.Contains(msg, "The specified bucket does not exist"):
			return fmt.Errorf("bucket not found: %s", msg)

		default:
			return fmt.Errorf("backend verification failed: %s", msg)
		}
	}
	if err != nil || len(out) == 0 || jsonErr != nil {
		return fmt.Errorf("restic command failed: %v\nOutput: %s\nJson extract failed:%v", err, string(out), jsonErr)
	}

	return nil
}

// InitRepo initializes a new repository
func (r *resticProvider) InitRepo(_ context.Context, param RepoRef) error {

	return nil
}

// ConnectToRepo establishes connection to an existing repository
func (r *resticProvider) ConnectToRepo(_ context.Context, param RepoRef) error {

	return nil
}

// updateBackendFromTarget updates the backend configuration with OnlineSnapshotTarget information
func (r *resticProvider) updateBackendFromTarget(param RepoRef) error {
	return nil
}

// PrepareRepo combines InitRepo and ConnectToRepo
func (r *resticProvider) PrepareRepo(_ context.Context, param RepoRef) error {
	return nil
}

// BoostRepoConnect re-ensures local connection to the repo
func (r *resticProvider) BoostRepoConnect(_ context.Context, _ RepoRef) error {
	// Restic doesn't require explicit re-connection
	return nil
}

// EnsureUnlockRepo removes any stale file locks
func (r *resticProvider) EnsureUnlockRepo(_ context.Context, param RepoRef) error {
	return nil
}

// PruneRepo performs full maintenance/pruning
func (r *resticProvider) PruneRepo(_ context.Context, param RepoRef) error {
	return nil
}

// Backup creates a new snapshot
func (r *resticProvider) Backup(ctx context.Context, param BackupParam) (*BackupResult, error) {
	r.wrapper.SetShowCMD(true)
	r.wrapper.AddSuffixToRepository(param.RepoRef.Suffix)
	param.RepoRef.FullPath = r.wrapper.GetEnv(restic.RESTIC_REPOSITORY)
	if exist := r.wrapper.RepositoryAlreadyExist(); !exist {
		err := r.wrapper.InitializeRepository()
		if err != nil {
			return &BackupResult{
				Phase:        BackupPhaseFailed,
				ErrorMessage: fmt.Sprintf("failed to initialize repository: %v", err),
				Provider:     string(v1.EngineRestic),
				Hostname:     param.Hostname,
				Paths:        param.BackupPaths,
				Repository:   param.RepoRef.FullPath,
			}, err
		}
	}

	backupOptions := restic.BackupOptions{
		Host:        param.Hostname,
		BackupPaths: param.BackupPaths,
		Exclude:     param.Exclude,
		Args:        param.Args,
	}

	out, err := r.wrapper.RunBackup(backupOptions)
	if err != nil {
		return &BackupResult{
			Phase:        BackupPhaseFailed,
			ErrorMessage: fmt.Sprintf("backup failed: %v", err),
			Provider:     string(v1.EngineRestic),
			Hostname:     param.Hostname,
			Paths:        param.BackupPaths,
			Repository:   param.RepoRef.FullPath,
		}, err
	}
	result := r.convertOutputToBackupResult(out, param)
	return result, nil
}

// Restore restores files from a snapshot
func (r *resticProvider) Restore(ctx context.Context, param RestoreParam) (*RestoreResult, error) {
	r.wrapper.SetShowCMD(true)
	r.wrapper.SetRepository(*param.RepoRef.Repository)
	param.RepoRef.FullPath = r.wrapper.GetEnv(restic.RESTIC_REPOSITORY)
	restoreOpt := restic.RestoreOptions{
		Host:         param.Hostname,
		Destination:  param.Destination,
		RestorePaths: param.RestorePaths,
		Snapshots: []string{
			param.SnapshotID,
		},
		Exclude: param.Exclude,
		Include: param.Include,
		Args:    param.Args,
	}

	out, err := r.wrapper.RunRestore(restoreOpt)
	if err != nil {
		return &RestoreResult{
			Phase:        RestoreFailed,
			ErrorMessage: fmt.Sprintf("restore failed: %v", err),
			Provider:     string(v1.EngineRestic),
			Hostname:     param.Hostname,
			Repository:   param.RepoRef.FullPath,
		}, err
	}
	result := r.convertOutputToRestoreResult(out, param)
	return result, nil
}

func (r *resticProvider) convertOutputToRestoreResult(output *restic.RestoreOutput, param RestoreParam) *RestoreResult {
	result := &RestoreResult{
		Provider:    string(v1.EngineRestic),
		Hostname:    param.Hostname,
		Repository:  param.RepoRef.FullPath,
		RestoreTime: time.Now(),
	}

	if output == nil || len(output.Stats) == 0 {
		result.Phase = RestoreFailed
		result.ErrorMessage = "no restore statistics available"
		return result
	}

	hostStats := output.Stats[0]
	if hostStats.Phase == restic.HostRestoreSucceeded {
		result.Phase = RestoreSucceeded
	} else {
		result.Phase = RestoreFailed
		result.ErrorMessage = hostStats.Error
	}
	result.Duration = hostStats.Duration
	return result
}

func (r *resticProvider) convertOutputToBackupResult(output *restic.BackupOutput, param BackupParam) *BackupResult {
	result := &BackupResult{
		Provider:   string(v1.EngineRestic),
		Hostname:   param.Hostname,
		Paths:      param.BackupPaths,
		Repository: param.RepoRef.FullPath,
		BackupTime: time.Now(),
	}

	if output == nil || len(output.Stats) == 0 {
		result.Phase = BackupPhaseFailed
		result.ErrorMessage = "no backup statistics available"
		return result
	}

	hostStats := output.Stats[0]
	if hostStats.Phase == restic.HostBackupSucceeded {
		result.Phase = BackupPhaseSucceeded
	} else {
		result.Phase = BackupPhaseFailed
		result.ErrorMessage = hostStats.Error
	}
	result.Duration = hostStats.Duration

	// Extract snapshot information (use first snapshot if multiple)
	if len(hostStats.Snapshots) > 0 {
		snapshot := hostStats.Snapshots[0]
		result.SnapshotID = snapshot.Name
		// Parse size information
		result.Size = BackupSizeInfo{
			TotalFormatted:    snapshot.TotalSize,
			UploadedFormatted: snapshot.Uploaded,
		}

		// Parse file statistics
		result.Files = BackupFileInfo{
			Total:      safeInt64(snapshot.FileStats.TotalFiles),
			New:        safeInt64(snapshot.FileStats.NewFiles),
			Modified:   safeInt64(snapshot.FileStats.ModifiedFiles),
			Unmodified: safeInt64(snapshot.FileStats.UnmodifiedFiles),
		}
	}

	return result
}

// safeInt64 safely converts *int64 to int64
func safeInt64(val *int64) int64 {
	if val == nil {
		return 0
	}
	return *val
}

// ListSnapshots lists all snapshots in the repository
func (r *resticProvider) ListSnapshots(_ context.Context, param RepoRef) ([]SnapshotInfo, error) {
	return nil, nil
}

// DeleteSnapshot deletes a specific snapshot by ID
func (r *resticProvider) DeleteSnapshot(_ context.Context, snapshotID string, param RepoRef) error {
	return nil
}

// Forget removes a snapshot from the repository
func (r *resticProvider) Forget(ctx context.Context, snapshotID string, param RepoRef) error {
	return nil
}

// BatchForget removes multiple snapshots
func (r *resticProvider) BatchForget(_ context.Context, snapshotIDs []string, param RepoRef) []error {
	return []error{}
}

// CheckRepository verifies the repository integrity
func (r *resticProvider) CheckRepository(_ context.Context, param RepoRef) error {
	return nil
}

// DefaultMaintenanceFrequency returns the default frequency to run maintenance
func (r *resticProvider) DefaultMaintenanceFrequency(_ context.Context, _ RepoRef) time.Duration {
	// Default maintenance frequency for restic is 7 days
	return 7 * 24 * time.Hour
}

// GetRepositoryStats returns statistics about the repository
func (r *resticProvider) GetRepositoryStats(_ context.Context, param RepoRef) (*RepositoryStats, error) {
	return nil, nil
}
