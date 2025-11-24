# OnlineSnapshotTarget Controller Implementation

## Overview
This implementation provides a Kubernetes controller for managing `OnlineSnapshotTarget` custom resources that define backend storage destinations for online volume snapshots using Restic.

## Implementation Summary

✅ **Completed Components:**
- OnlineSnapshotTarget CRD with full specification
- Controller reconciliation logic
- Restic engine integration for validation
- Support for S3, GCS, and Azure storage backends
- Status tracking and validation
- RBAC configuration
- Example YAML manifests

🎯 **Current Focus:** Restic driver only (Kopia support reserved for future)

## Components

### 1. API Types (`api/v1/onlinesnapshot_target_types.go`)
- **OnlineSnapshotTarget**: Main CRD defining a backend destination for storing snapshots
- **OnlineSnapshotTargetSpec**: Configuration including:
  - `Engine`: Snapshot engine to use (currently only "restic" supported)
  - `StorageBackend`: Backend storage configuration (S3, GCS, Azure)
  - `GlobalFlags`, `BackupFlags`, `RestoreFlags`: Additional flags for operations
  - `ValidateOnCreate`: Whether to validate backend connection on creation
- **OnlineSnapshotTargetStatus**: Status tracking:
  - `Phase`: Current state (Ready, Pending, Error)
  - `Message`: Human-readable status message
  - `LastChecked`: Timestamp of last validation

### 2. Constants (`api/v1/constants.go`)
- **BackupEngine**: Type defining supported engines
  - `EngineRestic`: "restic"
  - `EngineKopia`: "kopia" (reserved for future use)
- **StorageProvider**: Supported storage backends
  - `ProviderS3`: AWS S3 and S3-compatible storage
  - `ProviderGCS`: Google Cloud Storage
  - `ProviderAzure`: Azure Blob Storage
  - `ProviderLocal`: Local filesystem

### 3. Internal Controller (`internal/controller/onlinesnapshot_target_controller.go`)
- **OnlineSnapshotTargetReconciler**: Main reconciliation logic
  - Validates OnlineSnapshotTarget configurations
  - Updates status based on validation results
  - Optionally validates backend connectivity if `ValidateOnCreate` is true

#### Key Methods:
- `Reconcile()`: Main reconciliation loop
- `validateAndUpdateStatus()`: Validates configuration and optionally tests connection
- `validateConfiguration()`: Validates static configuration (engine, provider, credentials)
- `validateBackendConnection()`: Tests actual connectivity to the backend storage

### 4. Snapshot Engine (`internal/controller/snapshot_engine.go`)
- **SnapshotEngine**: Interface for snapshot backend implementations
- **ResticEngine**: Restic implementation of SnapshotEngine
  - `ValidateConnection()`: Executes `restic snapshots` command to verify connectivity

#### Helper Functions:
- `buildRepositoryURL()`: Constructs repository URL based on provider
  - S3: `s3:endpoint/bucket/prefix`
  - GCS: `gs:bucket/prefix`
  - Azure: `azure:container:/prefix`
- `buildEnvironmentVariables()`: Sets up environment for restic command
  - `RESTIC_REPOSITORY`: Repository URL
  - `RESTIC_PASSWORD`: Password for repository access
  - Provider-specific credentials (AWS_*, GOOGLE_*, AZURE_*)

### 5. Public Controller (`pkg/controller/onlinesnapshot_target_controller.go`)
- **SetupOnlineSnapshotTargetReconciler**: Factory function to create and register the controller with the manager

## Validation Flow

1. **Configuration Validation** (always performed):
   - Check engine is "restic"
   - Verify storage provider is specified
   - Validate provider-specific required fields:
     - S3: endpoint, bucket
     - GCS: bucket
     - Azure: storage account, container

2. **Backend Connection Validation** (optional, if `ValidateOnCreate: true`):
   - Build repository URL from backend configuration
   - Set up environment variables
   - Execute `restic snapshots --json --no-lock` command
   - Parse output and verify connectivity

3. **Status Update**:
   - Set `Phase` to "Ready" if all validations pass
   - Set `Phase` to "Error" if validation fails
   - Update `Message` with appropriate context
   - Record `LastChecked` timestamp

## Usage Example

```yaml
apiVersion: topolvm.io/v1
kind: OnlineSnapshotTarget
metadata:
  name: my-s3-backend
spec:
  engine: restic
  validateOnCreate: true
  storageBackend:
    provider: s3
    s3:
      endpoint: s3.amazonaws.com
      bucket: my-backup-bucket
      prefix: /topolvm/snapshots
      region: us-east-1
      secretName: aws-credentials
  globalFlags:
    - "--no-lock"
    - "--verbose"
```

## RBAC Permissions

The controller requires the following permissions:
- `onlinesnapshottargets`: get, list, watch, update, patch
- `onlinesnapshottargets/status`: get, update, patch

## Future Enhancements

1. **Kopia Support**: Add implementation for Kopia engine
2. **Secret Integration**: Fetch credentials from Kubernetes Secrets
3. **Local Provider**: Support for local filesystem backends
4. **Periodic Validation**: Background validation of backend connectivity
5. **Metrics**: Export metrics for monitoring
6. **Repository Initialization**: Auto-initialize repositories if they don't exist

## Notes

- Currently only Restic engine is implemented and supported
- Backend connection validation requires restic binary to be available
- Credentials should be provided via Kubernetes Secrets (currently not fully implemented)
- The controller validates configuration but does not manage actual backup/restore operations

## Files Created/Modified

### API Types
- `api/v1/onlinesnapshot_target_types.go` - Main CRD type definitions
- `api/v1/backend_types.go` - Storage backend type definitions
- `api/v1/constants.go` - Engine and provider constants
- `api/v1/zz_generated.deepcopy.go` - Auto-generated DeepCopy methods

### Controllers
- `internal/controller/onlinesnapshot_target_controller.go` - Main reconciler implementation
- `internal/controller/snapshot_engine.go` - Restic engine interface and implementation
- `pkg/controller/onlinesnapshot_target_controller.go` - Public setup function

### Generated Resources
- `config/crd/bases/topolvm.io_onlinesnapshottargets.yaml` - CRD manifest
- `config/crd/bases/topolvm.cybozu.com_onlinesnapshottargets.yaml` - Legacy CRD manifest

### Examples
- `example/onlinesnapshottarget-s3.yaml` - S3 backend example
- `example/onlinesnapshottarget-multi.yaml` - GCS and Azure examples

### Documentation
- `docs/onlinesnapshottarget-controller.md` - This file

## Testing

To test the controller:

1. **Deploy the CRD:**
   ```bash
   kubectl apply -f config/crd/bases/topolvm.io_onlinesnapshottargets.yaml
   ```

2. **Create a test OnlineSnapshotTarget:**
   ```bash
   kubectl apply -f example/onlinesnapshottarget-s3.yaml
   ```

3. **Check the status:**
   ```bash
   kubectl get onlinesnapshottargets
   kubectl describe onlinesnapshottarget s3-backup-target
   ```

4. **Verify validation:**
   ```bash
   kubectl get onlinesnapshottarget s3-backup-target -o jsonpath='{.status}'
   ```

## Integration with TopoLVM

To use this controller with TopoLVM:

1. Register the controller in your manager
2. Ensure restic binary is available in the controller pod
3. Configure storage backend credentials via Secrets
4. Reference the OnlineSnapshotTarget in VolumeSnapshotClass

Example controller registration:
```go
import (
    topolvmcontroller "github.com/topolvm/topolvm/pkg/controller"
)

func main() {
    // ... manager setup ...
    
    if err := topolvmcontroller.SetupOnlineSnapshotTargetReconciler(mgr, mgr.GetClient()); err != nil {
        setupLog.Error(err, "unable to create controller", "controller", "OnlineSnapshotTarget")
        os.Exit(1)
    }
    
    // ... start manager ...
}
```

