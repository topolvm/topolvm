topolvm-node
============

`topolvm-node` provides a CSI node service.  It also works as a custom
Kubernetes controller to implement dynamic volume provisioning.

CSI node features
-----------------

`topolvm-node` implements following optional features:

- [`GET_VOLUME_STATS`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#nodegetvolumestats)
- [`EXPAND_VOLUME`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#nodeexpandvolume)


Dynamic volume provisioning
---------------------------

`topolvm-node` watches [`LogicalVolume`](./crd-logical-volume.md) and creates
or deletes LVM logical volumes by sending requests to [`lvmd`](./lvmd.md).

### Create a logical volume

If `logicalvolume.status.volumeID` is empty,
it means that the logical volume correspond to `LogicalVolume` is not provisioned with `lvmd`.
So in that case, `topolvm-node` sends `CreateLV` request to `lvmd`.
If its response is succeeded, `topolvm-node` set `logicalvolume.status.volumeID`.

### Finalize LogicalVolume

When a `LogicalVolume` resource is being deleted, `topolvm-node` sends
a `RemoveLV` request to `lvmd`.

Inline ephemeral volume provisioning
------------------------------------

`topolvm-node` manages [inline ephemeral volumes](https://kubernetes-csi.github.io/docs/ephemeral-local-volumes.html) through special handling in the
processing of `NodePublishVolumeRequest` and `NodeUnpublishVolumeRequest`. For volumes
backed by PVCs, `topolvm-node` manages creation and deletion of LVMs via
watches on `LogicalVolume` as explained above. However, for inline ephmeral
volumes, the only CSI calls which will occur for the volume are
`NodePublishVolume` and `NodeUnPublishVolume`. `topolvm-node` must
create and delete the volumes in response to these calls.

### Create an inline ephemeral volume

Since `NodePublishVolumeRequest` and `NodeUnPublishVolumeRequest` are made for
all volumes, `topolvm-node` must determine if the calls are for ephemeral
inline volumes. It does so when processing a `NodePublishVolumeRequest`
by examining the `VolumeContext` map which is passed by the CSI driver to
`NodePublishVolume`.

If the `VolumeContext` has `csi.storage.k8s.io/ephemeral`
set to `true`, then `topolvm-node` recognizes this as a request to publish
an inline ephmeral volume. Since there is no PVC in this case, `topolvm-node`
must create the LVM while processing this request. It does so by sending the
`CreateLV` request to `lvmd`. To facilitate identifying ephemeral volumes when
processing `NodeUnpublishVolume`, the `Tags` field set to `[ephemeral]` in
the `CreateLV` request.

### Delete an inline ephemeral volume

When the csi driver calls `NodeUnpublishVolume`, `topolvm-node` determines
if the request is for an ephemeral volume by checking for the presence of
the tag `ephemeral` on the volume. If and only if the tag is present,
`topolvm-node` sends a `RemoveLV` request to `lvmd`. Otherwise, it will
rely on the Finalizer logic to handle deletion of the LVM.

Prometheus metrics
------------------

### `topolvm_volumegroup_available_bytes`

`topolvm_volumegroup_available_bytes` is a Gauge that indicates the available
free space in the LVM volume group in bytes.

| Label  | Description            |
| ------ | ---------------------- |
| `node` | The node resource name |

Node resource
-------------

`topolvm-node` adds `capacity.topolvm.io/<volume group name>` annotation to the
corresponding `Node` resource of the running node.  The value is the
free storage capacity reported by `lvmd` in bytes.

It also adds `topolvm.cybozu.com/node` finalizer to the `Node`.
The finalizer will be processed by [`topolvm-controller`](./topolvm-controller.md)
to clean up PVCs and associated Pods bound to the node.

Command-line flags
------------------

| Name           | Type   | Default                         | Description                            |
| -------------- | ------ | ------------------------------- | -------------------------------------- |
| `csi-socket`   | string | `/run/topolvm/csi-topolvm.sock` | UNIX domain socket of `topolvm-node`.  |
| `lvmd-socket`  | string | `/run/topolvm/lvmd.sock`        | UNIX domain socket of `lvmd` service.  |
| `metrics-addr` | string | `:8080`                         | Bind address for the metrics endpoint. |
| `nodename`     | string |                                 | `Node` resource name.                  |

Environment variables
---------------------

- `NODE_NAME`: `Node` resource name.

If both `NODE_NAME` and `nodename` flag are given, `nodename` flag is preceded.
