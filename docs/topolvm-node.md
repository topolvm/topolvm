# topolvm-node

`topolvm-node` provides a CSI node service.  It also works as a custom
Kubernetes controller to implement dynamic volume provisioning.

## CSI Node Features

`topolvm-node` implements following optional features:

- [`GET_VOLUME_STATS`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#nodegetvolumestats)
- [`EXPAND_VOLUME`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#nodeexpandvolume)


## Dynamic Volume Provisioning

`topolvm-node` watches [`LogicalVolume`](./crd-logical-volume.md) and creates
or deletes LVM logical volumes by sending requests to [`LVMd`](./lvmd.md).

### Create a Logical Volume

If `logicalvolume.status.volumeID` is empty,
it means that the logical volume correspond to `LogicalVolume` is not provisioned with `LVMd`.
So in that case, `topolvm-node` sends `CreateLV` request to `LVMd`.
If its response is succeeded, `topolvm-node` set `logicalvolume.status.volumeID`.

Each request to `LVMd` must finish within `lvm-operation-timeout`.
An LVM command can otherwise hang forever, for example when `lvcreate` waits for
a udev event whose udev worker was killed, and stop provisioning on the node.
When the timeout expires, the LVM command is killed and `logicalvolume.status.code`
is set to `DeadlineExceeded`. `topolvm-controller` then deletes the `LogicalVolume`,
the partially created logical volume is removed, and the provisioner retries.

The timeout must be at least 5 minutes, or 0 to disable it, and `topolvm-node`
refuses to start otherwise. A shorter timeout could stop LVM commands that are
still waiting for udev: systemd-udevd gives each event 180 seconds by default
(`event_timeout` in `udev.conf`). If you raise `event_timeout`, keep this timeout
above it.

### Finalize a Logical Volume

When a `LogicalVolume` resource is being deleted, `topolvm-node` sends
a `RemoveLV` request to `LVMd`.

## Prometheus Metrics

### `topolvm_volumegroup_available_bytes`

`topolvm_volumegroup_available_bytes` is a Gauge that indicates the available
free space in the LVM volume group in bytes.

| Label          | Description            |
| -------------- | ---------------------- |
| `node`         | The node resource name |
| `device_class` | The device class name. |

### `topolvm_volumegroup_size_bytes`

`topolvm_volumegroup_size_bytes` is a Gauge that indicates the size of the LVM volume group in bytes.

| Label          | Description            |
| -------------- | ---------------------- |
| `node`         | The node resource name |
| `device_class` | The device class name. |


### `topolvm_thinpool_data_percent`

`topolvm_thinpool_data_percent` is a Gauge that indicates the data space percentage used in the LVM thinpool.

| Label          | Description            |
| -------------- | ---------------------- |
| `node`         | The node resource name |
| `device_class` | The device class name. |


### `topolvm_thinpool_metadata_percent`

`topolvm_thinpool_metadata_percent` is a Gauge that indicates the metadata space percentage used of the LVM thinpool.

| Label          | Description            |
| -------------- | ---------------------- |
| `node`         | The node resource name |
| `device_class` | The device class name. |


### `topolvm_thinpool_size_bytes`

`topolvm_thinpool_size_bytes` is a Gauge that indicates the size of the LVM thin pool in bytes.

| Label          | Description            |
| -------------- | ---------------------- |
| `node`         | The node resource name |
| `device_class` | The device class name. |

### `topolvm_thinpool_overprovisioned_available`

`topolvm_thinpool_overprovisioned_available` is a Gauge that indicates the available overprovisioned size of the LVM thin pool in bytes.

| Label          | Description            |
| -------------- | ---------------------- |
| `node`         | The node resource name |
| `device_class` | The device class name. |

## Operations to Node Resources

`topolvm-node` adds `capacity.topolvm.io/<device-class>` annotations
for each device-class and `capacity.topolvm.io/00default` annotation 
for the default device-class to the corresponding `Node` resource of the running node.
The value is the free storage capacity reported by `LVMd` in bytes.

It also adds `topolvm.io/node` finalizer to the `Node`.
The finalizer will be processed by [`topolvm-controller`](./topolvm-controller.md)
to clean up PVCs and associated Pods bound to the node.

## Command-line Flags

| Name                   | Type   | Default                         | Description                            |
| ---------------------- | ------ | ------------------------------- | -------------------------------------- |
| `csi-socket`           | string | `/run/topolvm/csi-topolvm.sock` | UNIX domain socket of `topolvm-node`.  |
| `lvmd-socket`          | string | `/run/topolvm/lvmd.sock`        | UNIX domain socket of `LVMd` service.  |
| `metrics-bind-address` | string | `:8080`                         | Bind address for the metrics endpoint. |
| `secure-metrics-server`| bool   | `false`                         | Secures the metrics server.            |
| `nodename`             | string |                                 | `Node` resource name.                  |
| `lvm-operation-timeout`| duration | `5m`                          | Maximum time an LVM operation may run before it is stopped. At least `5m`; `0` disables the timeout. |

## Environment Variables

- `NODE_NAME`: `Node` resource name.

If both `NODE_NAME` and `nodename` flag are given, `nodename` flag is preceded.
