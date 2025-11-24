# LogicalVolume Controller Architecture Diagram
## High-Level Flow Diagram
```
┌─────────────────────────────────────────────────────────────────────┐
│                    Kubernetes API Server                             │
│                  (LogicalVolume CRD Resources)                       │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             │ Watch Events
                             │ (Create/Update/Delete)
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Controller Manager                                │
│              (Filters by nodeName)                                   │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             │ Reconcile Request
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│              LogicalVolumeReconciler.Reconcile()                     │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │ 1. Fetch LogicalVolume CR                                     │  │
│  │ 2. Check if belongs to this node (nodeName filter)            │  │
│  │ 3. Check for pending deletion annotation                      │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                             │                                         │
│                   ┌─────────┴─────────┐                              │
│                   │                   │                              │
│                   ▼                   ▼                              │
│         ┌─────────────────┐  ┌─────────────────┐                   │
│         │  DeletionTime   │  │  DeletionTime   │                   │
│         │  != nil         │  │  == nil         │                   │
│         │  (Finalization) │  │  (Creation/     │                   │
│         │                 │  │   Update)       │                   │
│         └────────┬────────┘  └────────┬────────┘                   │
│                  │                     │                             │
│                  ▼                     ▼                             │
│         [Go to Finalization]    [Go to Creation/Update]             │
└─────────────────────────────────────────────────────────────────────┘
```
## Creation/Update Flow
```
┌─────────────────────────────────────────────────────────────────────┐
│                   Creation/Update Flow                               │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ Add          │    │ Add Label    │    │ Get Source   │
│ Finalizer    │    │ (created-by) │    │ LogicalVolume│
└──────┬───────┘    └──────┬───────┘    └──────┬───────┘
       │                   │                    │
       │                   │                    │
       └───────────────────┴────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │ Source LV exists?      │
              └───────┬────────────────┘
                      │
            ┌─────────┴─────────┐
            │                   │
            ▼                   ▼
      ┌──────────┐        ┌──────────┐
      │   YES    │        │    NO    │
      └────┬─────┘        └────┬─────┘
           │                   │
           ▼                   │
  ┌────────────────────┐      │
  │Get Snapshot        │      │
  │Resources:          │      │
  │• VolumeSnapshot    │      │
  │  Content           │      │
  │• VolumeSnapshot    │      │
  │  Class             │      │
  └────────┬───────────┘      │
           │                   │
           ▼                   │
  ┌────────────────────┐      │
  │Should Perform      │      │
  │Snapshot Restore?   │      │
  └────────┬───────────┘      │
           │                   │
           └───────────────────┘
                      │
                      ▼
         ┌────────────────────────┐
         │ Status.VolumeID == ""? │
         │ (LV not created yet)   │
         └───────┬────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
        ▼                 ▼
   ┌─────────┐      ┌─────────┐
   │   YES   │      │   NO    │
   └────┬────┘      └────┬────┘
        │                │
        ▼                ▼
 [CREATE LV]      [EXPAND LV]
        │                │
        └────────┬───────┘
                 │
                 ▼
    ┌────────────────────────┐
    │ shouldRestoreFromSnap? │
    └───────┬────────────────┘
            │
       ┌────┴────┐
       │         │
       ▼         ▼
   ┌─────┐   ┌─────┐
   │ YES │   │ NO  │
   └──┬──┘   └──┬──┘
      │         │
      ▼         │
 ┌──────────────────┐
 │ Wait for PV      │
 │ to be ready      │
 └────┬─────────────┘
      │
      ▼
 ┌──────────────────┐
 │ Perform Snapshot │
 │ Restore          │
 └────┬─────────────┘
      │
      ▼
 ┌──────────────────┐
 │ Restore Success? │
 └────┬─────────────┘
      │
      ▼
 ┌──────────────────┐
 │ Unmount LV       │
 └────┬─────────────┘
      │
      └──────────────┘
                 │
                 ▼
    ┌────────────────────────┐
    │ Get Snapshot Resources │
    │ for this LV            │
    └───────┬────────────────┘
            │
            ▼
    ┌────────────────────────┐
    │ Should Perform Backup? │
    └───────┬────────────────┘
            │
       ┌────┴────┐
       │         │
       ▼         ▼
   ┌─────┐   ┌─────┐
   │ YES │   │ NO  │
   └──┬──┘   └──┬──┘
      │         │
      ▼         │
 ┌──────────────────┐
 │ Perform Snapshot │
 │ Backup           │
 └────┬─────────────┘
      │
      └─────────────┘
                 │
                 ▼
           ┌─────────┐
           │ SUCCESS │
           └─────────┘
```
## Snapshot Backup Flow (performSnapshotBackup)
```
┌─────────────────────────────────────────────────────────────────────┐
│                    performSnapshotBackup()                           │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
                ┌────────────────────────┐
                │ Initialize Snapshot    │
                │ Status (if needed)     │
                │ → Set to PENDING       │
                └────────┬───────────────┘
                         │
                         ▼
                ┌────────────────────────┐
                │ Is Operation Complete? │
                │ (SUCCESS or FAILED)    │
                └────────┬───────────────┘
                         │
                    ┌────┴────┐
                    │         │
                    ▼         ▼
                ┌──────┐  ┌──────┐
                │ YES  │  │  NO  │
                └───┬──┘  └───┬──┘
                    │         │
                    ▼         ▼
              [RETURN]  ┌────────────────────┐
                        │ Is Operation       │
                        │ Running?           │
                        └────────┬───────────┘
                                 │
                            ┌────┴────┐
                            │         │
                            ▼         ▼
                        ┌──────┐  ┌──────┐
                        │ YES  │  │  NO  │
                        └───┬──┘  └───┬──┘
                            │         │
                            ▼         ▼
                      [RETURN]  ┌────────────────────┐
                                │ Mount LV           │
                                │ Options:           │
                                │ • ro (read-only)   │
                                │ • norecovery       │
                                └────────┬───────────┘
                                         │
                                    ┌────┴────┐
                                    │         │
                                    ▼         ▼
                              ┌─────────┐ ┌─────────┐
                              │ SUCCESS │ │ FAILED  │
                              └────┬────┘ └────┬────┘
                                   │           │
                                   │           ▼
                                   │     ┌──────────────┐
                                   │     │ Update Status│
                                   │     │ → FAILED     │
                                   │     │ Return Error │
                                   │     └──────────────┘
                                   │
                                   ▼
                        ┌───────────────────────┐
                        │ Create Snapshot       │
                        │ Executor              │
                        └────────┬──────────────┘
                                 │
                                 ▼
                        ┌───────────────────────┐
                        │ Execute Snapshot      │
                        │ Operation             │
                        └────────┬──────────────┘
                                 │
                            ┌────┴────┐
                            │         │
                            ▼         ▼
                      ┌─────────┐ ┌─────────┐
                      │ SUCCESS │ │ FAILED  │
                      └────┬────┘ └────┬────┘
                           │           │
                           │           ▼
                           │     ┌──────────────┐
                           │     │ Update Status│
                           │     │ → FAILED     │
                           │     │ Return Error │
                           │     └──────────────┘
                           │
                           ▼
                     ┌──────────┐
                     │ SUCCESS  │
                     └──────────┘
```
## Snapshot Restore Flow (performSnapshotRestore)
```
┌─────────────────────────────────────────────────────────────────────┐
│                   performSnapshotRestore()                           │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
                ┌────────────────────────┐
                │ Is Operation Complete? │
                │ (SUCCESS or FAILED)    │
                └────────┬───────────────┘
                         │
                    ┌────┴────┐
                    │         │
                    ▼         ▼
                ┌──────┐  ┌──────┐
                │ YES  │  │  NO  │
                └───┬──┘  └───┬──┘
                    │         │
                    ▼         ▼
              [RETURN]  ┌────────────────────┐
                        │ Initialize Snapshot│
                        │ Status (if needed) │
                        │ → Set to PENDING   │
                        └────────┬───────────┘
                                 │
                                 ▼
                        ┌────────────────────┐
                        │ Is Operation       │
                        │ Running?           │
                        └────────┬───────────┘
                                 │
                            ┌────┴────┐
                            │         │
                            ▼         ▼
                        ┌──────┐  ┌──────┐
                        │ YES  │  │  NO  │
                        └───┬──┘  └───┬──┘
                            │         │
                            ▼         ▼
                      [RETURN]  ┌────────────────────┐
                                │ Mount LV           │
                                │ (Bind Mount)       │
                                │ Options: []        │
                                └────────┬───────────┘
                                         │
                                    ┌────┴────┐
                                    │         │
                                    ▼         ▼
                              ┌─────────┐ ┌─────────┐
                              │ SUCCESS │ │ FAILED  │
                              └────┬────┘ └────┬────┘
                                   │           │
                                   │           ▼
                                   │     ┌──────────────┐
                                   │     │ Update Status│
                                   │     │ → FAILED     │
                                   │     │ Return Error │
                                   │     └──────────────┘
                                   │
                                   ▼
                        ┌───────────────────────┐
                        │ Create Restore        │
                        │ Executor              │
                        │ (with source LV)      │
                        └────────┬──────────────┘
                                 │
                                 ▼
                        ┌───────────────────────┐
                        │ Execute Restore       │
                        │ Operation             │
                        └────────┬──────────────┘
                                 │
                            ┌────┴────┐
                            │         │
                            ▼         ▼
                      ┌─────────┐ ┌─────────┐
                      │ SUCCESS │ │ FAILED  │
                      └────┬────┘ └────┬────┘
                           │           │
                           │           ▼
                           │     ┌──────────────┐
                           │     │ Update Status│
                           │     │ → FAILED     │
                           │     │ Return Error │
                           │     └──────────────┘
                           │
                           ▼
                     ┌──────────┐
                     │ SUCCESS  │
                     └──────────┘
```
## Finalization Flow
```
┌─────────────────────────────────────────────────────────────────────┐
│                      Finalization Flow                               │
│                (DeletionTimestamp != nil)                            │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
                ┌────────────────────────┐
                │ Has Finalizer?         │
                └────────┬───────────────┘
                         │
                    ┌────┴────┐
                    │         │
                    ▼         ▼
                ┌──────┐  ┌──────┐
                │  NO  │  │ YES  │
                └───┬──┘  └───┬──┘
                    │         │
                    ▼         ▼
              [RETURN]  ┌────────────────────┐
                        │ Unmount LV         │
                        │ (if mounted)       │
                        └────────┬───────────┘
                                 │
                                 ▼
                        ┌────────────────────┐
                        │ Call lvService     │
                        │ .RemoveLV()        │
                        └────────┬───────────┘
                                 │
                            ┌────┴────┐
                            │         │
                            ▼         ▼
                    ┌──────────┐  ┌──────────┐
                    │ SUCCESS  │  │ NOT FOUND│
                    │ or       │  │ (already │
                    │          │  │  removed)│
                    └────┬─────┘  └────┬─────┘
                         │             │
                         └──────┬──────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │ Remove Finalizer│
                       │ from LV CR      │
                       └────────┬────────┘
                                │
                                ▼
                          ┌──────────┐
                          │ SUCCESS  │
                          │ LV will  │
                          │ be deleted│
                          └──────────┘
```
## LV Creation Flow (createLV)
```
┌─────────────────────────────────────────────────────────────────────┐
│                         createLV()                                   │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
                ┌────────────────────────┐
                │ Check if LV already    │
                │ exists (crash recovery)│
                └────────┬───────────────┘
                         │
                    ┌────┴────┐
                    │         │
                    ▼         ▼
                ┌──────┐  ┌──────┐
                │ YES  │  │  NO  │
                └───┬──┘  └───┬──┘
                    │         │
                    ▼         │
           ┌─────────────┐   │
           │ Set Status: │   │
           │ VolumeID    │   │
           │ Code: OK    │   │
           └─────────────┘   │
                             │
                    ┌────────┴────────┐
                    │                 │
                    ▼                 ▼
        ┌─────────────────┐  ┌─────────────────┐
        │ Has Source LV?  │  │ Regular LV      │
        │ (Spec.Source    │  │ Creation        │
        │  != "")         │  │                 │
        └────────┬────────┘  └────────┬────────┘
                 │                    │
            ┌────┴────┐               │
            │         │               │
            ▼         ▼               │
    ┌───────────┐ ┌───────────┐     │
    │ Restore   │ │ Snapshot  │     │
    │ Mode      │ │ LV Mode   │     │
    └─────┬─────┘ └─────┬─────┘     │
          │             │             │
          │             ▼             │
          │    ┌────────────────┐    │
          │    │ Fetch Source LV│    │
          │    │ Get sourceVolID│    │
          │    └────────┬───────┘    │
          │             │             │
          │             ▼             │
          │    ┌────────────────┐    │
          │    │ Call lvService │    │
          │    │.CreateLVSnapshot│   │
          │    └────────┬───────┘    │
          │             │             │
          └─────────────┘             │
                    │                 │
                    │                 ▼
                    │        ┌────────────────┐
                    │        │ Call lvService │
                    │        │ .CreateLV()    │
                    │        └────────┬───────┘
                    │                 │
                    └────────┬────────┘
                             │
                             ▼
                  ┌──────────────────┐
                  │ Update Status:   │
                  │ • VolumeID       │
                  │ • CurrentSize    │
                  │ • Code: OK       │
                  └──────────────────┘
```
## Component Interactions
```
┌──────────────────────────────────────────────────────────────────────┐
│                    Component Architecture                             │
└──────────────────────────────────────────────────────────────────────┘
┌─────────────────────┐
│ LogicalVolume       │
│ Reconciler          │
│                     │
│ ┌─────────────────┐ │
│ │ client          │ │──────────┐
│ │ (K8s API)       │ │          │
│ └─────────────────┘ │          │
│                     │          │
│ ┌─────────────────┐ │          │
│ │ vgService       │ │──┐       │
│ │ (VG operations) │ │  │       │
│ └─────────────────┘ │  │       │
│                     │  │       │
│ ┌─────────────────┐ │  │       │
│ │ lvService       │ │──┤       │
│ │ (LV operations) │ │  │       │
│ └─────────────────┘ │  │       │
│                     │  │       │
│ ┌─────────────────┐ │  │       │
│ │ lvMount         │ │──┤       │
│ │ (Mount ops)     │ │  │       │
│ └─────────────────┘ │  │       │
│                     │  │       │
│ ┌─────────────────┐ │  │       │
│ │ executor        │ │──┤       │
│ │ (Snap/Restore)  │ │  │       │
│ └─────────────────┘ │  │       │
└─────────────────────┘  │       │
                         │       │
                         ▼       ▼
         ┌───────────────────────────────┐
         │       LVMD Service             │
         │  (LVM Daemon - gRPC Server)   │
         │                               │
         │  ┌─────────────────────────┐  │
         │  │ VGService               │  │
         │  │ • GetLVList()           │  │
         │  └─────────────────────────┘  │
         │                               │
         │  ┌─────────────────────────┐  │
         │  │ LVService               │  │
         │  │ • CreateLV()            │  │
         │  │ • CreateLVSnapshot()    │  │
         │  │ • ResizeLV()            │  │
         │  │ • RemoveLV()            │  │
         │  └─────────────────────────┘  │
         └───────────┬───────────────────┘
                     │
                     ▼
         ┌───────────────────────┐
         │   LVM2 System         │
         │   (Logical Volume     │
         │    Management)        │
         └───────────────────────┘
```
## State Machine (Snapshot Operations)
```
┌─────────────────────────────────────────────────────────────────────┐
│           Snapshot Operation State Machine                           │
└─────────────────────────────────────────────────────────────────────┘
                    ┌──────────┐
                    │  EMPTY   │
                    │ (initial)│
                    └────┬─────┘
                         │
                         │ initializeSnapshotStatus()
                         │
                         ▼
                   ┌───────────┐
              ┌───│  PENDING  │
              │   └─────┬─────┘
              │         │
              │         │ Mount + Execute
              │         │
              │         ▼
              │   ┌───────────┐
              ├──│  RUNNING  │──┐
              │   └─────┬─────┘  │
              │         │         │
              │         │         │ Already running
              │         │         │ (requeue)
              │         ▼         │
    Error     │   ┌───────────┐  │
    occurred  │   │ SUCCEEDED │  │
              │   └───────────┘  │
              │         │         │
              │         │         │
              │         │ Final state
              │         │ (no more changes)
              │         │
              │         ▼
              │   ┌──────────┐
              └──│  FAILED  │
                  └──────────┘
                       │
                       │ Final state
                       │ (requires manual intervention)
                       │
                       ▼
                  [TERMINAL]
```
## Decision Tree (shouldPerform methods)
```
┌─────────────────────────────────────────────────────────────────────┐
│                  Decision Making Logic                               │
└─────────────────────────────────────────────────────────────────────┘
shouldPerformSnapshotBackup(vsClass):
    │
    ├─► vsClass == nil? ──YES──► return false
    │
    ├─► vsClass.Parameters[SnapshotMode] exists? ──NO──► return false
    │
    └─► vsClass.Parameters[SnapshotMode] == "online"? 
        │
        ├─► YES ──► return true (Perform backup)
        │
        └─► NO ──► return false
shouldPerformSnapshotRestore(sourceLV, vsClass):
    │
    ├─► sourceLV.Status.Snapshot == nil? ──YES──┐
    │                                            │
    ├─► sourceLV.Status.Snapshot.Phase == SUCCEEDED? ──YES──┐
    │                                                         │
    └─► Other phases? ──► return false                       │
                                                              │
                                        ┌─────────────────────┘
                                        │
                                        ▼
                            isOnlineSnapshotEnabled(vsClass)
                                        │
                                        ├─► YES ──► return true
                                        │
                                        └─► NO ──► return false
isPersistentVolumeReady(lv):
    │
    ├─► lv.Spec.Name == ""? ──YES──► return false
    │
    ├─► Get PV from K8s API
    │
    ├─► PV exists? ──YES──► return true
    │
    └─► PV not found? ──► return false
```
## Error Handling Flow
```
┌─────────────────────────────────────────────────────────────────────┐
│                     Error Handling                                   │
└─────────────────────────────────────────────────────────────────────┘
Operation Error Occurs:
    │
    ├─► Mount Error
    │   └─► mountLogicalVolume()
    │       ├─► Determine error code
    │       │   • VolumeMountFailed
    │       │   • VolumeBindMountFailed
    │       │
    │       ├─► Create SnapshotError object
    │       │
    │       ├─► updateSnapshotOperationStatus()
    │       │   └─► Set Phase = FAILED
    │       │
    │       └─► Log error & return
    │
    ├─► Execution Error
    │   └─► executeSnapshotOperation()
    │       ├─► Determine error code
    │       │   • SnapshotExecutionFailed
    │       │   • RestoreExecutionFailed
    │       │
    │       ├─► Create SnapshotError object
    │       │
    │       ├─► updateSnapshotOperationStatus()
    │       │   └─► Set Phase = FAILED
    │       │
    │       └─► Log error & return
    │
    └─► LVM Error
        └─► createLV() / expandLV() / removeLV()
            ├─► Extract gRPC status code
            │
            ├─► Update LV.Status.Code
            │
            ├─► Update LV.Status.Message
            │
            └─► Return error to reconciler
```
## Key Features Summary
### 1. **Node Filtering**
- Only processes LogicalVolumes assigned to this node
- Uses `nodeName` filter in event handlers
### 2. **Finalizer Management**
- Adds finalizer on creation
- Ensures cleanup before deletion
- Handles crash recovery
### 3. **Snapshot Operations**
- **Backup**: Read-only mount → Execute snapshot
- **Restore**: Bind mount → Execute restore
- Idempotent operations with state tracking
### 4. **LV Lifecycle**
- Create: Regular LV or Snapshot LV
- Expand: Resize existing LV
- Delete: Unmount + Remove LV
### 5. **State Tracking**
- VolumeID, CurrentSize, Code, Message
- Snapshot status with phases
- Error tracking with codes
### 6. **Crash Recovery**
- Checks if LV already exists before creating
- Handles partially completed operations
- Requeue on temporary failures
---
## Sequence Diagrams
### Sequence 1: Create LogicalVolume with Snapshot Restore
```
User/K8s  Controller  LVService  Executor  VGService  PV/PVC
   │          │          │          │         │         │
   │ Create LV│          │          │         │         │
   │ with     │          │          │         │         │
   │ Source   │          │          │         │         │
   ├─────────>│          │          │         │         │
   │          │          │          │         │         │
   │          │ Get      │          │         │         │
   │          │ Source LV│          │         │         │
   │          ├──────────┤          │         │         │
   │          │<─────────┤          │         │         │
   │          │          │          │         │         │
   │          │ Get VS   │          │         │         │
   │          │ Resources│          │         │         │
   │          ├──────────┤          │         │         │
   │          │<─────────┤          │         │         │
   │          │          │          │         │         │
   │          │ Should   │          │         │         │
   │          │ Restore? │          │         │         │
   │          ├──────────┤          │         │         │
   │          │<─YES─────┤          │         │         │
   │          │          │          │         │         │
   │          │ CreateLV │          │         │         │
   │          ├─────────>│          │         │         │
   │          │          │ Create   │         │         │
   │          │          │ in LVM   │         │         │
   │          │          ├──────────┤         │         │
   │          │          │<─────────┤         │         │
   │          │<─────────┤          │         │         │
   │          │ VolumeID │          │         │         │
   │          │          │          │         │         │
   │          │ Wait for │          │         │         │
   │          │ PV Ready │          │         │         │
   │          ├──────────┼──────────┼─────────┼────────>│
   │          │          │          │         │         │
   │          │<─────────┼──────────┼─────────┼─────────┤
   │          │ PV Ready │          │         │         │
   │          │          │          │         │         │
   │          │ Init     │          │         │         │
   │          │ Status   │          │         │         │
   │          │→PENDING  │          │         │         │
   │          │          │          │         │         │
   │          │ Mount LV │          │         │         │
   │          │ (bind)   │          │         │         │
   │          ├──────────┤          │         │         │
   │          │<─────────┤          │         │         │
   │          │ Mount    │          │         │         │
   │          │ Response │          │         │         │
   │          │          │          │         │         │
   │          │ Create   │          │         │         │
   │          │ Restore  │          │         │         │
   │          │ Executor │          │         │         │
   │          ├──────────┼─────────>│         │         │
   │          │          │          │         │         │
   │          │ Execute  │          │         │         │
   │          │ Restore  │          │         │         │
   │          ├──────────┼─────────>│         │         │
   │          │          │          │ Copy    │         │
   │          │          │          │ Data    │         │
   │          │          │          ├─────────┤         │
   │          │          │          │<────────┤         │
   │          │<─────────┼──────────┤         │         │
   │          │ SUCCESS  │          │         │         │
   │          │          │          │         │         │
   │          │ Update   │          │         │         │
   │          │ Status   │          │         │         │
   │          │→SUCCEEDED│          │         │         │
   │          │          │          │         │         │
   │          │ Unmount  │          │         │         │
   │          │ LV       │          │         │         │
   │          ├──────────┤          │         │         │
   │          │<─────────┤          │         │         │
   │          │          │          │         │         │
   │<─────────┤          │          │         │         │
   │ Success  │          │          │         │         │
```
### Sequence 2: Take Snapshot Backup
```
User/K8s  Controller  LVService  Executor  VSContent VSClass
   │          │          │          │         │         │
   │ Create   │          │          │         │         │
   │ VS for LV│          │          │         │         │
   ├─────────>│          │          │         │         │
   │          │          │          │         │         │
   │          │ Get VS   │          │         │         │
   │          │ Content  │          │         │         │
   │          ├──────────┼──────────┼────────>│         │
   │          │<─────────┼──────────┼─────────┤         │
   │          │          │          │         │         │
   │          │ Get VS   │          │         │         │
   │          │ Class    │          │         │         │
   │          ├──────────┼──────────┼─────────┼────────>│
   │          │<─────────┼──────────┼─────────┼─────────┤
   │          │          │          │         │         │
   │          │ Should   │          │         │         │
   │          │ Backup?  │          │         │         │
   │          ├──────────┤          │         │         │
   │          │<─YES─────┤          │         │         │
   │          │ (online  │          │         │         │
   │          │  mode)   │          │         │         │
   │          │          │          │         │         │
   │          │ Init     │          │         │         │
   │          │ Status   │          │         │         │
   │          │→PENDING  │          │         │         │
   │          │          │          │         │         │
   │          │ Is       │          │         │         │
   │          │ Complete?│          │         │         │
   │          ├──NO──────┤          │         │         │
   │          │          │          │         │         │
   │          │ Is       │          │         │         │
   │          │ Running? │          │         │         │
   │          ├──NO──────┤          │         │         │
   │          │          │          │         │         │
   │          │ Mount LV │          │         │         │
   │          │ (ro,     │          │         │         │
   │          │ norecov.)│          │         │         │
   │          ├──────────┤          │         │         │
   │          │<─────────┤          │         │         │
   │          │ Mount    │          │         │         │
   │          │ Response │          │         │         │
   │          │          │          │         │         │
   │          │ Create   │          │         │         │
   │          │ Snapshot │          │         │         │
   │          │ Executor │          │         │         │
   │          ├──────────┼─────────>│         │         │
   │          │          │          │         │         │
   │          │ Execute  │          │         │         │
   │          │ Snapshot │          │         │         │
   │          ├──────────┼─────────>│         │         │
   │          │          │          │ Upload  │         │
   │          │          │          │ to      │         │
   │          │          │          │ Backend │         │
   │          │          │          ├─────────┤         │
   │          │          │          │<────────┤         │
   │          │<─────────┼──────────┤         │         │
   │          │ SUCCESS  │          │         │         │
   │          │          │          │         │         │
   │          │ Update   │          │         │         │
   │          │ Status   │          │         │         │
   │          │→SUCCEEDED│          │         │         │
   │          │          │          │         │         │
   │<─────────┤          │          │         │         │
   │ Success  │          │          │         │         │
```
### Sequence 3: Delete LogicalVolume
```
User/K8s  Controller  LVService  LVMount
   │          │          │          │
   │ Delete LV│          │          │
   ├─────────>│          │          │
   │          │          │          │
   │          │ Check    │          │
   │          │ Finalizer│          │
   │          ├──────────┤          │
   │          │<─EXISTS──┤          │
   │          │          │          │
   │          │ Unmount  │          │
   │          │ LV       │          │
   │          ├──────────┼─────────>│
   │          │          │          │
   │          │          │          │ Check if
   │          │          │          │ mounted
   │          │          │          ├────────┤
   │          │          │          │<───────┤
   │          │          │          │
   │          │          │          │ Unmount
   │          │          │          │ (if mnt)
   │          │          │          ├────────┤
   │          │          │          │<───────┤
   │          │<─────────┼──────────┤
   │          │ Success  │          │
   │          │          │          │
   │          │ RemoveLV │          │
   │          ├─────────>│          │
   │          │          │          │
   │          │          │ Delete   │
   │          │          │ from LVM │
   │          │          ├──────────┤
   │          │          │<─────────┤
   │          │<─────────┤          │
   │          │ Success  │          │
   │          │ or       │          │
   │          │ NotFound │          │
   │          │          │          │
   │          │ Remove   │          │
   │          │ Finalizer│          │
   │          ├──────────┤          │
   │          │<─────────┤          │
   │          │          │          │
   │<─────────┤          │          │
   │ LV       │          │          │
   │ Deleted  │          │          │
```
---
## Method Call Hierarchy
```
Reconcile()
│
├─► Get LogicalVolume CR
│
├─► Check nodeName filter
│
├─► Check pending deletion
│
├─► DeletionTimestamp check
│   │
│   ├─► nil (Creation/Update path)
│   │   │
│   │   ├─► Add Finalizer (if missing)
│   │   │
│   │   ├─► Add Label (if missing)
│   │   │
│   │   ├─► getSourceLogicalVolume()
│   │   │   └─► client.Get()
│   │   │
│   │   ├─► If source exists:
│   │   │   ├─► getVolumeSnapshotResources()
│   │   │   │   ├─► getVolumeSnapshotContentIfExists()
│   │   │   │   │   └─► getVolumeSnapshotContent()
│   │   │   │   │       └─► client.Get()
│   │   │   │   └─► getVolumeSnapshotClassFromContent()
│   │   │   │       └─► client.Get()
│   │   │   │
│   │   │   └─► shouldPerformSnapshotRestore()
│   │   │       └─► isOnlineSnapshotEnabled()
│   │   │
│   │   ├─► If VolumeID == "":
│   │   │   └─► createLV()
│   │   │       ├─► volumeExists()
│   │   │       │   └─► vgService.GetLVList()
│   │   │       │
│   │   │       ├─► If Source + !restore:
│   │   │       │   └─► lvService.CreateLVSnapshot()
│   │   │       │
│   │   │       └─► Else:
│   │   │           └─► lvService.CreateLV()
│   │   │
│   │   ├─► expandLV()
│   │   │   └─► lvService.ResizeLV()
│   │   │
│   │   ├─► If shouldRestoreFromSnapshot:
│   │   │   ├─► isPersistentVolumeReady()
│   │   │   │   └─► client.Get(PV)
│   │   │   │
│   │   │   ├─► performSnapshotRestore()
│   │   │   │   ├─► isSnapshotOperationComplete()
│   │   │   │   ├─► initializeSnapshotStatus()
│   │   │   │   │   └─► updateSnapshotOperationStatus()
│   │   │   │   ├─► isSnapshotOperationRunning()
│   │   │   │   ├─► mountLogicalVolume()
│   │   │   │   │   └─► lvMount.Mount()
│   │   │   │   └─► executeSnapshotOperation()
│   │   │   │       └─► executor.Execute()
│   │   │   │
│   │   │   └─► If restore succeeded:
│   │   │       └─► lvMount.Unmount()
│   │   │
│   │   ├─► getVolumeSnapshotResources() (for current LV)
│   │   │
│   │   ├─► shouldPerformSnapshotBackup()
│   │   │   └─► isOnlineSnapshotEnabled()
│   │   │
│   │   └─► If shouldPerformBackup:
│   │       └─► performSnapshotBackup()
│   │           ├─► initializeSnapshotStatus()
│   │           ├─► isSnapshotOperationComplete()
│   │           ├─► isSnapshotOperationRunning()
│   │           ├─► mountLogicalVolume()
│   │           │   └─► lvMount.Mount()
│   │           └─► executeSnapshotOperation()
│   │               └─► executor.Execute()
│   │
│   └─► !nil (Finalization path)
│       │
│       ├─► Check Finalizer exists
│       │
│       ├─► removeLVIfExists()
│       │   ├─► lvMount.Unmount()
│       │   └─► lvService.RemoveLV()
│       │
│       └─► Remove Finalizer
│
└─► Return Result
```
---
## Data Flow
```
┌─────────────────────────────────────────────────────────────────────┐
│                         Data Flow                                    │
└─────────────────────────────────────────────────────────────────────┘
Input (LogicalVolume CR):
┌──────────────────────────┐
│ Spec:                    │
│ • NodeName               │────► Filter (must match this node)
│ • DeviceClass            │────► Select VG/LVM device class
│ • Size                   │────► LV size (expandable)
│ • Source (optional)      │────► Source LV for snapshot/restore
│ • Name                   │────► PV name (for restore)
│ • AccessType             │────► ro/rw for snapshot LV
│ • LvcreateOptionClass    │────► LVM options
└──────────────────────────┘
           │
           ▼
┌──────────────────────────┐
│ Controller Processing    │
│ • Validate               │
│ • Check source           │
│ • Get VS resources       │
│ • Decide operations      │
└──────────────────────────┘
           │
           ▼
┌──────────────────────────┐
│ LVMD gRPC Calls:         │
│ • GetLVList()            │────► List existing LVs
│ • CreateLV()             │────► Create new LV
│ • CreateLVSnapshot()     │────► Create snapshot LV
│ • ResizeLV()             │────► Expand LV
│ • RemoveLV()             │────► Delete LV
└──────────────────────────┘
           │
           ▼
┌──────────────────────────┐
│ LVM2 System              │
│ • lvcreate               │
│ • lvresize               │
│ • lvremove               │
│ • mount/umount           │
└──────────────────────────┘
           │
           ▼
┌──────────────────────────┐
│ Mount Operations:        │
│ • lvMount.Mount()        │────► Mount for access
│ • lvMount.Unmount()      │────► Unmount after ops
└──────────────────────────┘
           │
           ▼
┌──────────────────────────┐
│ Executor:                │
│ • Snapshot backup        │────► Upload to backend
│ • Snapshot restore       │────► Download & restore
└──────────────────────────┘
           │
           ▼
Output (Status Update):
┌──────────────────────────┐
│ Status:                  │
│ • VolumeID               │────► LVM LV name (UID)
│ • CurrentSize            │────► Actual LV size
│ • Code                   │────► gRPC status code
│ • Message                │────► Status message
│ • Snapshot               │────► Snapshot operation status
│   ├─ Operation           │      (Backup/Restore)
│   ├─ Phase               │      (Pending/Running/Succeeded/Failed)
│   ├─ Message             │      
│   ├─ Error               │      (if failed)
│   └─ StartTime           │
└──────────────────────────┘
```
---
## Summary
This LogicalVolume Controller is a Kubernetes operator that:
1. **Manages LVM Logical Volumes** as Kubernetes Custom Resources
2. **Filters by node** - each node runs its own controller instance
3. **Handles full lifecycle**:
   - Creation (regular or snapshot-based)
   - Expansion (resize)
   - Deletion (cleanup with finalizers)
4. **Supports snapshot operations**:
   - Online backup (mount read-only, snapshot to backend)
   - Online restore (bind mount, restore from backend)
5. **Ensures idempotency** with state tracking and crash recovery
6. **Communicates via gRPC** with LVMD (LVM daemon)
7. **Uses finalizers** for proper cleanup before deletion
The refactored code provides clean separation of concerns with helper methods for:
- Resource retrieval
- Decision making
- Operation execution
- Status management
- Error handling
