package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/topolvm/topolvm/internal/backupengine/config"
)

// NewKopiaProvider creates a new Kopia repository provider
func NewKopiaProvider(_ *config.SetupOptions) (Provider, error) {
	return &kopiaProvider{}, nil
}

type kopiaProvider struct {
}

func (k *kopiaProvider) Delete(ctx context.Context, param DeleteParam) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (k *kopiaProvider) ValidateConnection(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

// InitRepo initializes a new repository
func (k *kopiaProvider) InitRepo(_ context.Context, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// ConnectToRepo establishes connection to an existing repository
func (k *kopiaProvider) ConnectToRepo(_ context.Context, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// PrepareRepo combines InitRepo and ConnectToRepo
func (k *kopiaProvider) PrepareRepo(_ context.Context, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// BoostRepoConnect re-ensures local connection to the repo
func (k *kopiaProvider) BoostRepoConnect(_ context.Context, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// EnsureUnlockRepo removes any stale file locks
func (k *kopiaProvider) EnsureUnlockRepo(_ context.Context, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// PruneRepo performs full maintenance/pruning
func (k *kopiaProvider) PruneRepo(_ context.Context, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// Backup creates a new snapshot
func (k *kopiaProvider) Backup(_ context.Context, param BackupParam) (*BackupResult, error) {
	// TODO: Implement Kopia backup
	return &BackupResult{
		Phase:        BackupPhaseFailed,
		ErrorMessage: "kopia provider not implemented yet",
		Provider:     "kopia",
		Hostname:     param.Hostname,
		Paths:        param.BackupPaths,
		Repository:   param.RepoRef.FullPath,
	}, fmt.Errorf("kopia provider not implemented yet")
}

// Restore restores files from a snapshot
func (k *kopiaProvider) Restore(_ context.Context, _ RestoreParam) (*RestoreResult, error) {
	return nil, fmt.Errorf("kopia provider not implemented yet")
}

// ListSnapshots lists all snapshots in the repository
func (k *kopiaProvider) ListSnapshots(_ context.Context, _ RepoRef) ([]SnapshotInfo, error) {
	return nil, fmt.Errorf("kopia provider not implemented yet")
}

// DeleteSnapshot deletes a specific snapshot by ID
func (k *kopiaProvider) DeleteSnapshot(_ context.Context, _ string, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// Forget removes a snapshot from the repository
func (k *kopiaProvider) Forget(_ context.Context, _ string, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// BatchForget removes multiple snapshots
func (k *kopiaProvider) BatchForget(_ context.Context, _ []string, _ RepoRef) []error {
	return []error{fmt.Errorf("kopia provider not implemented yet")}
}

// CheckRepository verifies the repository integrity
func (k *kopiaProvider) CheckRepository(_ context.Context, _ RepoRef) error {
	return fmt.Errorf("kopia provider not implemented yet")
}

// DefaultMaintenanceFrequency returns the default frequency to run maintenance
func (k *kopiaProvider) DefaultMaintenanceFrequency(_ context.Context, _ RepoRef) time.Duration {
	// Default maintenance frequency for kopia is 7 days
	return 7 * 24 * time.Hour
}

// GetRepositoryStats returns statistics about the repository
func (k *kopiaProvider) GetRepositoryStats(_ context.Context, _ RepoRef) (*RepositoryStats, error) {
	return nil, fmt.Errorf("kopia provider not implemented yet")
}
