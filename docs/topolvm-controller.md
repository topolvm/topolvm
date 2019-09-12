topolvm-controller
==================

`topolvm-controller` is a Kubernetes custom controller for cleanup.
It is deployed as a sidecar container in the CSI controller Pod.

It watches `Node` resource deletion, then cleanup `PersistentVolumeClaim` and `Pod`
running on the deleting Nodes.

Details
-------

### Node finalization

When a `Node` resource is deleted, `Node`'s finalizer is invoked.
Finalizer deletes `PersistentVolumeClaim` and `Pod` that are running on deleting `Node`.

### Delete stale LogicalVolumes and its associated PersistentVolumes

Sometime LogicalVolumes may be left without completing finalization when the node dies.
To delete such LogicalVolumes and its associated PersistentVolumes, `topolvm-controller` deletes them periodically. By default, it deletes LogicalVolumes whose DeletionTimestamp is behind one day from the current time.

Command-line flags
------------------

| Name           | Type     | Default | Description                                                                         |
| -------------- | -------- | ------- | ----------------------------------------------------------------------------------- |
| `stale-period` | Duration | `24h`   | LogicalVolumes is considered as stale if its DeletionTimestamp is behind this value |
