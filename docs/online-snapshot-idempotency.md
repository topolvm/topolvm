# Online Snapshot Idempotency Fix

## Problem

The previous implementation had an idempotency issue where the controller would overwrite the online snapshot status to `Pending` on every reconciliation loop, even if the snapshot was already in `Running`, `Succeeded`, or `Failed` state.

**Previous Code:**
```go
if onlineSnapshot {
    // Initialize online snapshot status to Pending
    if err := r.updateOnlineSnapshotStatus(ctx, log, lv, topolvmv1.SnapshotPending, "", nil); err != nil {
        log.Error(err, "failed to set online snapshot status to Pending", "name", lv.Name)
        return ctrl.Result{}, err
    }

    // Attempt to mount the logical volume
    resp, err := r.lvMount.Mount(ctx, lv)
    // ... rest of the code
}
```

**Issues:**
1. **Status Overwrites**: Every reconciliation would reset the status to `Pending`, potentially overwriting `Running`, `Succeeded`, or `Failed` states
2. **Duplicate Operations**: The snapshot executor could be triggered multiple times for the same logical volume
3. **Race Conditions**: If the executor was running, the controller could interfere by resetting the status

## Solution

The fix implements proper idempotency checks with state-based logic:

**Fixed Code:**
```go
if onlineSnapshot {
    // Initialize online snapshot status to Pending only if no status exists yet
    if lv.Status.OnlineSnapshot == nil || lv.Status.OnlineSnapshot.Phase == "" {
        if err := r.updateOnlineSnapshotStatus(ctx, log, lv, topolvmv1.SnapshotPending, "Initializing online snapshot", nil); err != nil {
            log.Error(err, "failed to set online snapshot status to Pending", "name", lv.Name)
            return ctrl.Result{}, err
        }
        // Requeue to process the mounted state in the next reconciliation
        return ctrl.Result{Requeue: true}, nil
    }

    // Skip if snapshot is already completed or failed
    if lv.Status.OnlineSnapshot.Phase == topolvmv1.SnapshotSucceeded || 
       lv.Status.OnlineSnapshot.Phase == topolvmv1.SnapshotFailed {
        log.Info("online snapshot already processed", "name", lv.Name, "phase", lv.Status.OnlineSnapshot.Phase)
        return ctrl.Result{}, nil
    }

    // Skip if snapshot is currently running (being processed by executor)
    if lv.Status.OnlineSnapshot.Phase == topolvmv1.SnapshotRunning {
        log.Info("online snapshot is currently running", "name", lv.Name)
        return ctrl.Result{}, nil
    }

    // Attempt to mount the logical volume (only when in Pending state)
    resp, err := r.lvMount.Mount(ctx, lv)
    // ... rest of the code
}
```

## Key Improvements

### 1. **Initialization Guard**
```go
if lv.Status.OnlineSnapshot == nil || lv.Status.OnlineSnapshot.Phase == "" {
    // Only set to Pending if no status exists
    r.updateOnlineSnapshotStatus(ctx, log, lv, topolvmv1.SnapshotPending, "Initializing online snapshot", nil)
    return ctrl.Result{Requeue: true}, nil
}
```
- Sets `Pending` status **only** when no status exists
- Returns early with requeue to allow the status update to persist
- Prevents overwriting existing status

### 2. **Terminal State Check**
```go
if lv.Status.OnlineSnapshot.Phase == topolvmv1.SnapshotSucceeded || 
   lv.Status.OnlineSnapshot.Phase == topolvmv1.SnapshotFailed {
    log.Info("online snapshot already processed", "name", lv.Name, "phase", lv.Status.OnlineSnapshot.Phase)
    return ctrl.Result{}, nil
}
```
- Skips processing if snapshot is in a terminal state (`Succeeded` or `Failed`)
- Prevents duplicate snapshot operations
- Ensures completed snapshots are not reprocessed

### 3. **In-Progress State Check**
```go
if lv.Status.OnlineSnapshot.Phase == topolvmv1.SnapshotRunning {
    log.Info("online snapshot is currently running", "name", lv.Name)
    return ctrl.Result{}, nil
}
```
- Skips processing if snapshot is currently running
- Prevents race conditions with the executor
- Allows the executor to complete without interference

## State Transition Flow

```
┌─────────────┐
│   No Status │
│  (initial)  │
└──────┬──────┘
       │
       │ Controller sets to Pending
       ▼
┌─────────────┐
│   Pending   │
└──────┬──────┘
       │
       │ Controller mounts LV and triggers executor
       ▼
┌─────────────┐
│   Running   │◄─── Executor sets to Running
└──────┬──────┘
       │
       ├──── Success ───► ┌────────────┐
       │                  │  Succeeded │ (terminal)
       │                  └────────────┘
       │
       └──── Failure ───► ┌────────────┐
                          │   Failed   │ (terminal)
                          └────────────┘
```

## Benefits

1. **Idempotency**: Controller can safely reconcile multiple times without side effects
2. **No Status Overwrites**: Existing status (Running, Succeeded, Failed) is preserved
3. **No Duplicate Operations**: Snapshot is taken exactly once per logical volume
4. **Race Condition Prevention**: Running snapshots are not interrupted
5. **Clear State Machine**: Well-defined state transitions and guards

## Testing Scenarios

### Scenario 1: First Time Snapshot
1. LogicalVolume created with online snapshot enabled
2. Controller sets status to `Pending` ✓
3. Controller requeues
4. Next reconciliation mounts LV and triggers executor
5. Executor sets status to `Running`
6. Executor completes and sets status to `Succeeded`

### Scenario 2: Controller Restart During Snapshot
1. Snapshot is `Running` when controller crashes
2. Controller restarts and reconciles
3. Controller sees `Running` status and skips processing ✓
4. Executor continues and completes

### Scenario 3: Multiple Reconciliations
1. Snapshot completed with `Succeeded` status
2. Controller reconciles multiple times
3. Each reconciliation sees `Succeeded` and skips processing ✓
4. No duplicate operations

### Scenario 4: Failed Snapshot Retry
1. Snapshot fails during mount with `Failed` status
2. Controller reconciles
3. Controller sees `Failed` and skips processing ✓
4. User can manually update the LogicalVolume to trigger retry if needed

## Related Files

- `/home/anisur/go/src/github.com/anisurrahman75/topolvm/internal/controller/logicalvolume_controller.go` - Controller implementation
- `/home/anisur/go/src/github.com/anisurrahman75/topolvm/api/v1/constants.go` - Snapshot phase constants
- `/home/anisur/go/src/github.com/anisurrahman75/topolvm/api/v1/logicalvolume_types.go` - LogicalVolume CRD definition

