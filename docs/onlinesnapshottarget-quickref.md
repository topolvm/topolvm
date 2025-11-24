# OnlineSnapshotTarget Quick Reference

## Quick Start

### 1. Apply the CRD
```bash
kubectl apply -f config/crd/bases/topolvm.io_onlinesnapshottargets.yaml
```

### 2. Create an S3 Backend
```bash
cat <<EOF | kubectl apply -f -
apiVersion: topolvm.io/v1
kind: OnlineSnapshotTarget
metadata:
  name: s3-backend
spec:
  engine: restic
  validateOnCreate: true
  storageBackend:
    provider: s3
    s3:
      endpoint: s3.amazonaws.com
      bucket: my-backups
      prefix: /topolvm
      region: us-east-1
      secretName: aws-creds
EOF
```

### 3. Check Status
```bash
# List all targets
kubectl get onlinesnapshottargets

# Get detailed status
kubectl describe onlinesnapshottarget s3-backend

# Get status JSON
kubectl get onlinesnapshottarget s3-backend -o jsonpath='{.status}' | jq
```

## Common Operations

### Create GCS Backend
```yaml
apiVersion: topolvm.io/v1
kind: OnlineSnapshotTarget
metadata:
  name: gcs-backend
spec:
  engine: restic
  storageBackend:
    provider: gcs
    gcs:
      bucket: my-gcs-bucket
      prefix: /snapshots
```

### Create Azure Backend
```yaml
apiVersion: topolvm.io/v1
kind: OnlineSnapshotTarget
metadata:
  name: azure-backend
spec:
  engine: restic
  storageBackend:
    provider: azure
    azure:
      storageAccount: mystorageaccount
      container: backups
      prefix: /topolvm
```

### Disable Validation
```yaml
spec:
  engine: restic
  validateOnCreate: false  # Skip connection validation
  storageBackend:
    # ... backend config
```

### Add Custom Flags
```yaml
spec:
  engine: restic
  globalFlags:
    - "--verbose"
    - "--no-lock"
  backupFlags:
    - "--exclude-caches"
  restoreFlags:
    - "--verify"
  storageBackend:
    # ... backend config
```

## Status Values

| Phase | Meaning |
|-------|---------|
| `Ready` | Backend is configured and validated |
| `Error` | Configuration or connection error |
| `Pending` | Validation in progress |

## Common Issues

### Error: "engine is required"
**Solution**: Set `spec.engine: restic`

### Error: "unsupported engine"
**Solution**: Currently only `restic` is supported

### Error: "S3 endpoint is required"
**Solution**: Set `spec.storageBackend.s3.endpoint`

### Error: "restic validation failed"
**Causes**:
- Restic binary not available
- Invalid credentials
- Network connectivity issues
- Repository doesn't exist

## Provider-Specific Requirements

### S3
- ✅ Required: `endpoint`, `bucket`
- 📦 Optional: `prefix`, `region`, `secretName`, `insecureTLS`

### GCS
- ✅ Required: `bucket`
- 📦 Optional: `prefix`, `secretName`, `maxConnections`

### Azure
- ✅ Required: `storageAccount`, `container`
- 📦 Optional: `prefix`, `secretName`, `maxConnections`

## Integration Example

```go
import (
    topolvmcontroller "github.com/topolvm/topolvm/pkg/controller"
)

// In your controller manager setup
if err := topolvmcontroller.SetupOnlineSnapshotTargetReconciler(mgr, mgr.GetClient()); err != nil {
    log.Error(err, "failed to setup OnlineSnapshotTarget controller")
    os.Exit(1)
}
```

## RBAC

Required permissions for the controller:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: onlinesnapshottarget-controller
rules:
- apiGroups: ["topolvm.io"]
  resources: ["onlinesnapshottargets"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["topolvm.io"]
  resources: ["onlinesnapshottargets/status"]
  verbs: ["get", "update", "patch"]
```

## Environment Variables (for Restic)

The controller sets these when validating:
- `RESTIC_REPOSITORY` - Constructed from backend config
- `RESTIC_PASSWORD` - From secret
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` - For S3
- `GOOGLE_APPLICATION_CREDENTIALS` - For GCS
- `AZURE_ACCOUNT_NAME`, `AZURE_ACCOUNT_KEY` - For Azure

## Troubleshooting

### Check Controller Logs
```bash
kubectl logs -n topolvm-system deployment/topolvm-controller | grep OnlineSnapshotTarget
```

### Verify CRD Installed
```bash
kubectl get crd onlinesnapshottargets.topolvm.io
```

### Test Restic Manually
```bash
export RESTIC_REPOSITORY="s3:s3.amazonaws.com/bucket/prefix"
export RESTIC_PASSWORD="password"
restic snapshots --json --no-lock
```

## More Information

- Full Documentation: `docs/onlinesnapshottarget-controller.md`
- Implementation Details: `ONLINESNAPSHOTTARGET_IMPLEMENTATION.md`
- Examples: `example/onlinesnapshottarget-*.yaml`

