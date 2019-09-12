topolvm-node
============

`topolvm-node` is a Kubernetes custom controller for `LogicalVolume`.

It watches `LogicalVolume` resource, then manage LVM logical volume with `lvmd`.

Details
-------

`topolvm-node` watches `LogicalVolume` then sends requests to `lvmd` if required.

### Create logical volume

If `logicalvolume.status.volumeID` is empty,
it means that the logical volume correspond to `LogicalVolume` is not provisioned with `lvmd`.
So in that case, `topolvm-node` sends `CreateLV` request to `lvmd`.
If its response is succeeded, `topolvm-node` set `logicalvolume.status.volumeID`.

### Finalize

When a `LogicalVolume` resource is deleted, `topolvm-node`'s finalizer is invoked.
Finalizer sends `RemoveLV` request to `lvmd`.

Command-line flags
------------------

| Name         | Type   |                Default | Description                                         |
| ------------ | ------ | ---------------------: | --------------------------------------------------- |
| metrics-addr | string |                 :28080 | Bind address for the metrics endpoint               |
| lvmd-socket  | string | /run/topolvm/lvmd.sock | UNIX domain socket of `lvmd` service                |
| node-name    | string |                        | The name of the node hosting `topolvm-node` service |
