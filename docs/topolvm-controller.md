topolvm-controller
==================

`topolvm-controller` is a Kubernetes custom controller for cleanup.
It is deployed as a sidecar container in the CSI controller Pod.

It watches `Node` resource deletion, then cleanup `PersistentVolumeClaim` and `Pod`
running on the deleting Nodes.

Details
-------

### Finalize

When a `Node` resource is deleted, `Node`'s finalizer is invoked.
Finalizer deletes `PersistentVolumeClaim` and `Pod` that are running on deleting `Node`.
