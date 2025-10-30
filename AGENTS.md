# TopoLVM Project Context

## Project Overview

TopoLVM is a CSI (Container Storage Interface) plugin for Kubernetes that uses LVM (Logical Volume Manager) for dynamic storage provisioning. It provides local persistent volumes with advanced features like topology-aware scheduling, volume expansion, and snapshots.

- **Repository**: github.com/topolvm/topolvm
- **Language**: Go 1.24.0
- **Supported Kubernetes versions**: 1.33, 1.32, 1.31
- **License**: Apache License 2.0

## Technology Stack

- **Primary Language**: Go 1.24.0
- **Framework**: Kubernetes controller-runtime
- **Build System**: Make
- **Container Runtime**: Docker with buildx for multi-platform images
- **Protocol**: gRPC for lvmd communication
- **Testing**: Ginkgo/Gomega for unit and e2e tests
- **Linting**: golangci-lint
- **Code Generation**: controller-gen, protoc

## Architecture

TopoLVM follows the standard [CSI (Container Storage Interface)](https://github.com/container-storage-interface/spec/) architecture for Kubernetes storage plugins, with additional components for topology-aware scheduling and capacity tracking.

### High-Level Design

TopoLVM addresses the need to run stateful applications (like MySQL, Elasticsearch) using local fast storage while maintaining data replication between servers. The key design goals are:

- **Flexible capacity management** using LVM for dynamic volume sizing
- **Intelligent scheduling** to prefer nodes with larger storage capacity
- **Dynamic provisioning** from PersistentVolumeClaims (PVCs)
- **Volume expansion** support for growing storage needs

### Core Components

#### 1. topolvm-controller

**Role**: CSI controller service and Kubernetes controller

**Responsibilities**:
- Implements CSI controller services (`CreateVolume`, `DeleteVolume`, `ControllerExpandVolume`)
- Acts as a [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) for new Pods
- Adds capacity annotations to Pods: `capacity.topolvm.io/<device-class>`
- Adds resource requests to first container: `topolvm.io/capacity`
- Creates and manages `LogicalVolume` custom resources
- Handles dynamic volume provisioning via `LogicalVolume` CRDs
- Manages resource cleanup when volumes are deleted

**Communication**:
- Works with CSI sidecars (external-provisioner, external-attacher, external-resizer)
- Communicates with Kubernetes API server
- Monitors and creates `LogicalVolume` resources

#### 2. topolvm-node

**Role**: CSI node service and node-level controller

**Responsibilities**:
- Implements CSI node services (`NodeStageVolume`, `NodePublishVolume`, `NodeExpandVolume`)
- Communicates with `LVMd` to manage actual LVM volumes
- Watches volume group free capacity and exports it as Node annotations
- Implements dynamic volume provisioning at the node level
- Watches `LogicalVolume` resources assigned to its node
- Triggers LVM volume creation/deletion/resizing via `LVMd`
- Updates `LogicalVolume` status with operation results
- Adds finalizers to Nodes for PVC cleanup
- Handles filesystem resizing (online or offline)

**Communication**:
- gRPC to `LVMd` via Unix domain socket
- Updates Node annotations with capacity information
- Updates `LogicalVolume` resource status

#### 3. lvmd (LVM Daemon)

**Role**: gRPC service for LVM volume management

**Responsibilities**:
- Provides gRPC API for LVM operations:
  - `CreateLV`: Create logical volumes
  - `RemoveLV`: Delete logical volumes
  - `ResizeLV`: Expand logical volumes
  - `CreateLVSnapshot`: Create snapshots (thin provisioning)
- Watches volume group status and reports free capacity
- Executes actual LVM commands (`lvcreate`, `lvremove`, `lvextend`, etc.)
- Supports multiple device classes (volume groups)

**Deployment**:
- Can run as a standalone process or embedded in `topolvm-node`
- Communicates via Unix domain socket

**Protocol**: See `pkg/lvmd/proto/lvmd.proto` for gRPC service definition

#### 4. topolvm-scheduler

**Role**: [Kubernetes scheduler extender](https://github.com/kubernetes/design-proposals-archive/blob/main/scheduling/scheduler_extender.md)

**Responsibilities**:
- Filters nodes with insufficient storage capacity
- Scores nodes based on available storage (higher score = more free space)
- Works with Pods that have `topolvm.io/capacity` resource requests
- Ensures Pods are scheduled to nodes with adequate storage

**Alternative**: Storage Capacity Tracking mode can be used instead of the scheduler extender

**Communication**:
- HTTP API called by kube-scheduler
- Reads Node capacity annotations

#### 5. CSI Sidecars (Standard Kubernetes Components)

- **external-provisioner**: Watches for new PVCs and triggers volume creation
- **external-attacher**: Manages volume attachment/detachment
- **external-resizer**: Handles PVC resize requests
- **external-snapshotter**: Manages volume snapshots (thin provisioning only)
- **node-driver-registrar**: Registers CSI driver with kubelet

#### 6. hypertopolvm (Unified Binary)

**hypertopolvm** is the main binary that can operate in different modes:
- CSI controller mode
- CSI node mode
- Scheduler extender mode

This simplifies deployment by consolidating multiple functionalities into a single binary.

### Component Communication

```
┌─────────────────┐         ┌──────────────────┐
│  kube-scheduler │◄────────┤ topolvm-scheduler│
│                 │  HTTP   │                  │
└─────────────────┘         └──────────────────┘

┌─────────────────┐         ┌──────────────────┐
│  CSI Sidecars   │◄────────┤ topolvm-         │
│  (provisioner,  │  Unix   │   controller     │
│   resizer, etc) │  Socket │                  │
└─────────────────┘         └──────────────────┘
                                    │
                                    │ K8s API (LogicalVolume CRD)
                                    │
┌─────────────────┐         ┌──────▼───────────┐         ┌──────────────┐
│     kubelet     │◄────────┤  topolvm-node    │◄────────┤    LVMd      │
│                 │  Unix   │                  │  gRPC   │              │
│                 │  Socket │                  │  Unix   │              │
└─────────────────┘         └──────────────────┘  Socket └──────────────┘
```

### Custom Resource Definitions (CRDs)

#### LogicalVolume CRD

The `LogicalVolume` is the primary CRD used for communication between `topolvm-controller` and `topolvm-node`. It represents a logical volume to be created on a specific node.

**API Group**: `topolvm.io/v1` (current), `topolvm.cybozu.com/v1` (legacy)
**Scope**: Cluster-wide
**Source**: `api/v1/logicalvolume_types.go`

##### Spec Fields

```go
type LogicalVolumeSpec struct {
    // Name of the logical volume (LVM LV name)
    Name string `json:"name"`

    // NodeName specifies the target node where the volume should be created
    NodeName string `json:"nodeName"`

    // Size is the requested volume capacity
    Size resource.Quantity `json:"size"`

    // DeviceClass specifies the volume group / device class to use
    // Empty string means default device class
    DeviceClass string `json:"deviceClass,omitempty"`

    // LvcreateOptionClass specifies custom options for lvcreate command
    // References a LvcreateOptionClass resource
    LvcreateOptionClass string `json:"lvcreateOptionClass,omitempty"`

    // Source specifies the source logical volume for snapshots/clones
    // Only populated when creating snapshots or clones
    Source string `json:"source,omitempty"`

    // AccessType specifies the access mode for snapshots/clones
    // "ro" - read-only snapshot
    // "rw" - read-write clone/restore
    AccessType string `json:"accessType,omitempty"`
}
```

##### Status Fields

```go
type LogicalVolumeStatus struct {
    // VolumeID is the unique identifier for the volume
    // Format: <node-name>/<device-class>/<lv-name>
    VolumeID string `json:"volumeID,omitempty"`

    // Code is the gRPC status code of the operation
    // codes.OK (0) means success
    Code codes.Code `json:"code,omitempty"`

    // Message contains error or status message
    Message string `json:"message,omitempty"`

    // CurrentSize is the actual current size of the volume
    // Used to track expansion progress
    CurrentSize *resource.Quantity `json:"currentSize,omitempty"`
}
```

##### Lifecycle States

1. **Created**: Controller creates LogicalVolume with desired spec
2. **Provisioning**: Node sees LogicalVolume, calls LVMd to create LV
3. **Available**: LV created successfully, status updated with VolumeID and CurrentSize
4. **Expanding**: Spec.Size > Status.CurrentSize triggers resize
5. **Deleting**: LogicalVolume deleted, triggers LV removal via LVMd

### Key Concepts

#### Device Classes

Device classes represent different volume groups or storage pools:
- Each device class maps to an LVM volume group
- Allows multiple storage tiers (e.g., SSD, HDD)
- Default device class uses the primary volume group
- Referenced in StorageClass parameters: `topolvm.io/device-class`

#### Topology Awareness

TopoLVM uses CSI topology features to ensure Pods run on nodes where their volumes exist:
- Volumes are local to specific nodes (LVM is node-local)
- CSI topology key: `topology.topolvm.io/node` (value = node name)
- Scheduler ensures Pods with TopoLVM volumes land on correct nodes

#### Capacity Tracking and Scheduling

**Node Annotations**:
- `topolvm-node` annotates each Node with free capacity
- Annotation format: `capacity.topolvm.io/<device-class>: "<bytes>"`
- Updated periodically as volumes are created/deleted/resized

**Pod Annotations and Resources** (added by mutating webhook):
- `capacity.topolvm.io/<device-class>: "<bytes>"` - total capacity needed
- `topolvm.io/capacity: "<bytes>"` - resource request on first container

**Scheduler Extension**:
- Filters out nodes with insufficient capacity
- Scores nodes: higher free capacity = higher score
- Helps balance storage usage across nodes

**Why Not Extended Resources?**

TopoLVM doesn't use Kubernetes Extended Resources because:
- Pod resource requests cannot be changed after creation
- PVC resize would change usage, but Pod requests can't be updated
- TopoLVM tracks free capacity (not usage) to avoid this limitation

#### Storage Capacity Tracking (Alternative to Scheduler)

Instead of `topolvm-scheduler`, you can use Kubernetes [Storage Capacity Tracking](https://kubernetes.io/docs/concepts/storage/storage-capacity/):
- Native Kubernetes feature (stable in 1.24+)
- CSI driver reports capacity via `CSIStorageCapacity` objects
- kube-scheduler uses this for scheduling decisions
- Recommended over scheduler extender for modern clusters

### Dynamic Provisioning Workflow

1. User creates a PVC with TopoLVM StorageClass
2. `external-provisioner` detects unbound PVC
3. `external-provisioner` calls `topolvm-controller.CreateVolume()`
4. `topolvm-controller` creates a `LogicalVolume` resource with:
   - Target node (from topology)
   - Requested size
   - Device class
5. `topolvm-node` on target node watches for `LogicalVolume` resources
6. `topolvm-node` finds the new `LogicalVolume` assigned to its node
7. `topolvm-node` calls `LVMd.CreateLV()` via gRPC
8. `LVMd` executes `lvcreate` to create the actual LVM logical volume
9. `LVMd` returns success/failure
10. `topolvm-node` updates `LogicalVolume.Status`:
    - Sets `VolumeID`
    - Sets `CurrentSize`
    - Sets `Code` to `codes.OK` or error code
11. `topolvm-controller` sees status update
12. `topolvm-controller` returns success to `external-provisioner`
13. `external-provisioner` creates PersistentVolume and binds to PVC

### Volume Expansion Workflow

1. User edits PVC to increase requested size
2. `external-resizer` detects size change
3. `external-resizer` calls `topolvm-controller.ControllerExpandVolume()`
4. `topolvm-controller` updates `LogicalVolume.Spec.Size`
5. `topolvm-node` detects `Spec.Size != Status.CurrentSize`
6. `topolvm-node` calls `LVMd.ResizeLV()`
7. `LVMd` executes `lvextend` to expand the logical volume
8. `topolvm-node` updates `LogicalVolume.Status.CurrentSize`
9. For filesystem volumes (not raw block):
   - If online resize supported: `NodeExpandVolume` resizes filesystem
   - If offline resize needed: resized in `NodePublishVolume` after unmount
   - Current supported filesystems (ext4, xfs, btrfs) support online resize
10. `external-resizer` sees successful expansion and updates PVC status

### Snapshot and Clone Support

**Snapshots** (read-only):
- Requires thin provisioning in LVM
- `Source` field set to source LV name
- `AccessType` set to `"ro"`
- Uses `lvcreate -s` (snapshot)

**Clones** (read-write):
- Requires thin provisioning in LVM
- `Source` field set to source LV name
- `AccessType` set to `"rw"`
- Creates writable copy

### Limitations

- **Node-local storage**: Volumes are tied to specific nodes (cannot migrate)
- **Kubernetes-specific**: Deeply integrated with Kubernetes, not portable to other orchestrators
- **LVM dependency**: Requires LVM2 version 2.02.163+ (for JSON output support)
- **Thin provisioning for snapshots**: Snapshots only work with thin-provisioned volumes
- **Volume leak possibility**: In some edge cases, a logical volume may not be deleted even after removing its PVC (see project limitations documentation)

## Development Setup

**Prerequisites**: Linux with LVM2, Go 1.24.0+, Docker with buildx

**Setup**: Run `make setup` to install all dependencies and development tools

## Common Commands

### Development

```bash
# Run unit tests and linting
make test

# Run only linting
make lint

# Fix linting issues automatically
make lint-fix

# Generate code (CRDs, deepcopy, protobuf, helm docs)
make generate

# Check for uncommitted generated files
make check-uncommitted
```

### Building

```bash
# Build all binaries (hypertopolvm, lvmd, and CSI sidecars)
make build

# Build only TopoLVM binaries
make build-topolvm

# Build Docker images
make image

# Build multi-platform images (amd64, arm64, ppc64le)
make multi-platform-images
```

### Testing

```bash
make test                      # Unit tests with linting
make groupname-test           # Legacy API compatibility tests
cd test/e2e && make start-lvmd && make test  # E2E tests
make container-structure-test # Container structure tests
```

## Code Style and Conventions

### Go Code Style

- **Formatting**: Use `gofmt -s` for formatting
- **Linting**: All code must pass `golangci-lint` and `go vet`
- **Imports**: Follow standard Go import grouping
- **Error Handling**: Use explicit error checking; avoid panic in production code

### Commit Message Guidelines

All commits must be signed with DCO using `git commit -s`

### Code Generation

The project uses code generation for several components:
- **CRDs**: Generated from API types using controller-gen
- **DeepCopy methods**: Generated using controller-gen
- **Protobuf**: gRPC service definitions in `pkg/lvmd/proto/`
- **Helm docs**: Generated from chart values

Always run `make generate` after modifying:
- API types in `api/`
- Protobuf definitions in `pkg/lvmd/proto/lvmd.proto`
- Helm chart values or templates

## Project Structure

```
topolvm/
├── api/                    # Kubernetes API types and CRDs
├── cmd/                    # Main binaries (hypertopolvm, lvmd)
├── internal/               # Internal packages
├── pkg/                    # Public packages
│   └── lvmd/
│       └── proto/          # gRPC protocol definitions
├── charts/                 # Helm charts
├── config/                 # Kubernetes manifests and configurations
├── docs/                   # Documentation
├── example/                # Example configurations for kind
├── test/                   # Test suites
│   └── e2e/               # End-to-end tests
├── Makefile               # Build and development tasks
├── versions.mk            # Tool and dependency versions
└── go.mod                 # Go module definition
```

## Testing Strategy

- **Unit tests**: Use Ginkgo/Gomega, located in `*_test.go` files alongside source
- **E2E tests**: Located in `test/e2e/`, require LVM setup
- **Requirements**: Add tests for new features, ensure `-race` flag passes, update tests for breaking changes

## PR Checklist

Before submitting changes:
1. Run `make test` and relevant e2e tests
2. Run `make generate` if modifying API types or protobuf
3. Run `make check-uncommitted` to verify generated files are committed
4. Sign all commits with `git commit -s` (DCO compliance required)

## Important Files to Never Modify Directly

These files are auto-generated and should not be edited manually:
- `api/v1/zz_generated.deepcopy.go`
- `api/legacy/v1/*` (generated from api/v1)
- `pkg/lvmd/proto/lvmd.pb.go`
- `pkg/lvmd/proto/lvmd_grpc.pb.go`
- `docs/lvmd-protocol.md`
- CRD manifests in `config/crd/bases/`
- Helm CRD templates in `charts/topolvm/templates/crds/`

## Common Issues and Solutions

### LVM Not Available

E2E tests require LVM. On macOS or systems without LVM:
- Use the `kind` example setup in `example/`
- Tests must run in a Linux environment with LVM2

### Protobuf Compilation Errors

If protobuf generation fails:
```bash
make distclean
make tools
make generate
```

### Generated Files Out of Sync

If CI fails on `check-uncommitted`:
```bash
make generate
git add .
git commit -s -m "Update generated files"
```
