topolvm-controller
==================

`topolvm-controller` is a Kubernetes custom controller for cleanup.
It is deployed as a sidecar container in the CSI controller `csi-topolvm` Pod.

It watches `Node` resource deletion, then cleanup `PersistentVolumeClaim` and `Pod`.

Details
-------

### Finalize

When a `Node` resource is deleted, `Node`'s finalizer is invoked.
Finalizer deletes `PersistentVolumeClaim` and `Pod` where are running on deletion `Node`.

Also, `topolvm-controller` checks `topolvm-nodes`'s finalizer for `LogicalVolume` deletion,
If `LogicalVolume`'s finalization is not done until the `--finalizer-timeout`,
then the controller itself deletes `LogicalVolume` finalizer and `LogicalVolmue`.

Command-line flags
------------------

| Name              | Type | Default | Description                               |
| ----------------- | ---- | ------- | ----------------------------------------- |
| finalizer-timeout | int  | `8s`    | The timeout for `LogicalVolume` finalizer |
