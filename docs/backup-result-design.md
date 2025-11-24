# Backup Result Design

## Overview

This document describes the design for getting backup output from both Restic and Kopia providers, creating a generic output structure, and updating the LogicalVolume status with backup results.

## Architecture

### 1. Generic BackupResult Structure

The `BackupResult` struct provides a unified interface for backup results from any provider (Restic, Kopia, etc.):

```go
type BackupResult struct {
    // Core identification
    SnapshotID string          // Unique identifier of the snapshot
    Repository string          // Repository URL/path
    BackupTime time.Time       // When the backup was taken
    
    // Statistics
    Size   BackupSizeInfo      // Size-related metrics
    Files  BackupFileInfo      // File-related metrics
    
    // Execution details
    Duration     string         // How long the backup took
    Phase        BackupPhase    // Success or failure
    ErrorMessage string         // Error details if failed
    
    // Metadata
    Hostname string            // Hostname used for backup
    Paths    []string          // Paths that were backed up
    Provider string            // Provider name (restic, kopia)
    Version  string            // Provider version
}
```

### 2. Provider Interface

The `Provider` interface has been updated to return `*BackupResult` instead of just a string:

```go
type Provider interface {
    Backup(ctx context.Context, param BackupParam) (*BackupResult, error)
    // ... other methods
}
```

### 3. Data Flow

```
┌─────────────────┐
│  Backup Command │
│  (backup.go)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Provider     │
│  (Restic/Kopia) │
└────────┬────────┘
         │
         ├── Execute backup
         ├── Parse provider-specific output
         ├── Convert to generic BackupResult
         │
         ▼
┌─────────────────┐
│  BackupResult   │
│   (Generic)     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ LogicalVolume   │
│     Status      │
└─────────────────┘
```

## Implementation Details

### 1. Restic Provider Implementation

The Restic provider (`restic.go`) implements the conversion from Restic-specific output to generic `BackupResult`:

```go
func (r *resticProvider) Backup(ctx context.Context, param BackupParam) (*BackupResult, error) {
    // 1. Initialize repository
    // 2. Run backup
    // 3. Convert restic.BackupOutput to BackupResult
    // 4. Return result
}

func (r *resticProvider) convertResticOutputToBackupResult(output *restic.BackupOutput, param BackupParam) *BackupResult {
    // Maps Restic-specific fields to generic fields:
    // - HostBackupStats → BackupResult
    // - SnapshotStats → BackupResult.Size & BackupResult.Files
    // - FileStats → BackupFileInfo
}
```

**Mapping:**
- `restic.BackupOutput.Stats[0].Phase` → `BackupResult.Phase`
- `restic.SnapshotStats.Name` → `BackupResult.SnapshotID`
- `restic.SnapshotStats.TotalSize` → `BackupResult.Size.TotalFormatted`
- `restic.SnapshotStats.Uploaded` → `BackupResult.Size.UploadedFormatted`
- `restic.FileStats` → `BackupResult.Files`

### 2. Kopia Provider Implementation

The Kopia provider (`kopia.go`) will implement similar conversion logic:

```go
func (k *kopiaProvider) Backup(ctx context.Context, param BackupParam) (*BackupResult, error) {
    // TODO: Implement Kopia-specific backup
    // Will follow similar pattern to Restic:
    // 1. Initialize repository
    // 2. Run backup
    // 3. Convert kopia.BackupOutput to BackupResult
    // 4. Return result
}
```

### 3. Backup Command Integration

The backup command (`backup.go`) consumes the `BackupResult` and updates the LogicalVolume status:

```go
func (opt *options) runBackup() error {
    // 1. Get provider
    provider, err := provider.GetProvider(opt.Client, opt.snapshotStorage)
    
    // 2. Execute backup
    result, err := provider.Backup(opt.ctx, backupParams)
    
    // 3. Update LogicalVolume status based on result
    if result.Phase == provider.BackupPhaseFailed {
        opt.updateBackupStatusFailed(result.ErrorMessage)
    } else {
        opt.updateBackupStatusSuccess(result)
    }
}
```

### 4. LogicalVolume Status Update

The status update functions map `BackupResult` to `OnlineSnapshotStatus`:

```go
func (opt *options) updateBackupStatusSuccess(result *provider.BackupResult) error {
    lv.Status.OnlineSnapshot = &OnlineSnapshotStatus{
        Phase:          SnapshotCompleted,
        SnapshotID:     result.SnapshotID,
        Message:        "Backup completed successfully",
        Version:        result.Provider,
        URL:            result.Repository,
        CompletionTime: &now,
        Progress: &BackupProgress{
            TotalBytes: result.Size.TotalBytes,
            BytesDone:  result.Size.UploadedBytes,
            Percentage: calculatePercentage(result.Size),
        },
    }
}
```

## Benefits

### 1. Provider Abstraction
- Clean separation between provider-specific implementations and generic interfaces
- Easy to add new providers (e.g., Borg, Duplicati)
- Consistent behavior across all providers

### 2. Rich Status Information
- Detailed backup statistics in LogicalVolume status
- Progress tracking (bytes uploaded, percentage complete)
- File statistics (new, modified, unmodified files)
- Duration and timestamp information

### 3. Error Handling
- Structured error information
- Consistent error reporting across providers
- Detailed failure messages for debugging

### 4. Extensibility
- Easy to add new metrics to `BackupResult`
- Version tracking for provider compatibility
- Support for future features (incremental backups, compression stats, etc.)

## Data Structures

### BackupSizeInfo
```go
type BackupSizeInfo struct {
    TotalBytes        int64  // Total bytes processed
    UploadedBytes     int64  // Bytes uploaded (may be less due to deduplication)
    TotalFormatted    string // Human-readable total (e.g., "1.5 GiB")
    UploadedFormatted string // Human-readable uploaded size
}
```

### BackupFileInfo
```go
type BackupFileInfo struct {
    Total      int64 // Total files processed
    New        int64 // New files
    Modified   int64 // Modified files
    Unmodified int64 // Unchanged files
}
```

### LogicalVolume OnlineSnapshotStatus
```go
type OnlineSnapshotStatus struct {
    Phase          SnapshotPhase         // Current phase
    SnapshotID     string                // Snapshot identifier
    Message        string                // Status message
    Version        string                // Provider used
    URL            string                // Repository URL
    StartTime      *metav1.Time          // When backup started
    CompletionTime *metav1.Time          // When backup completed
    Progress       *BackupProgress       // Progress information
    Error          *OnlineSnapshotError  // Error details if failed
}
```

## Future Enhancements

1. **Real-time Progress Updates**
   - Stream backup progress during execution
   - Update LogicalVolume status incrementally

2. **Restore Operation**
   - Similar `RestoreResult` structure
   - Track restore progress and statistics

3. **Provider-Specific Metrics**
   - Compression ratios
   - Deduplication savings
   - Network transfer speeds

4. **Snapshot Management**
   - List snapshots with detailed metadata
   - Snapshot retention policies
   - Automatic cleanup based on age/count

## Example Usage

```go
// Create provider
provider, err := provider.GetProvider(client, snapshotStorage)

// Execute backup
result, err := provider.Backup(ctx, BackupParam{
    RepoParam: RepoParam{
        Repository: "namespace/pvcname",
        Hostname:   "filesystem",
    },
    BackupPaths: []string{"/mnt/data"},
})

// Check result
if result.Phase == provider.BackupPhaseSucceeded {
    fmt.Printf("Backup successful! Snapshot ID: %s\n", result.SnapshotID)
    fmt.Printf("Uploaded: %s / Total: %s\n", 
        result.Size.UploadedFormatted, 
        result.Size.TotalFormatted)
    fmt.Printf("Files: %d total, %d new, %d modified\n",
        result.Files.Total,
        result.Files.New,
        result.Files.Modified)
} else {
    fmt.Printf("Backup failed: %s\n", result.ErrorMessage)
}
```

