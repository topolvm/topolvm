CRD LogicalVolume
=================

We use a CRD named `LogicalVolume` to represent a logical volume in LVM on a node.
This document shows the definition of `LogicalVolume`.

LogicalVolume
-------------

Field                                                    | Description
-------------------------------------------------------- | -----------
`apiVersion` string                                      | APIVersion.
`kind`       string                                      | Kind.
`metadata`   [ObjectMeta][]                              | Standard object's metadata.
`spec`       [LogicalVolumeSpec](#logicalvolumespec)     | Specification of desired behavior of the logical volume.
`status`     [LogicalVolumeStatus](#logicalvolumestatus) | Most recently observed status of the logical volume.


LogicalVolumeSpec
-----------------

Field                    | Description
------------------------ | -----------
`name`     string        | Suggested name of the logical volume.
`nodeName` string        | Name of the node where the logical volume should be created.
`size`     [Quantity][]  | Amount of local storage required for the logical volume.


LogicalVolumeStatus
-------------------

Field             | Description
----------------- | -----------
`volumeID` string | Name of the logical volume.  Also used as the volume ID in the CSI context, so this should be unique in the cluster. If this field is not empty, a corresponding logical volume is created.
`code`     uint32 | gRPC error code. If there is no error, `0` is assigned.
`message`  string | Error message.


Logical Volume Lifecycle
------------------------

When `LogicalVolume` resource has been registered, `status.volumeID` is empty.
Reconciler attempts to create logical volume in LVM.

If Reconciler created logical volume, it set `status.volumeID`.

When `LogicalVolume` is deleted, Finalizer is invoked.
Finalizer deletes a corresponding logical volume in LVM.

[ObjectMeta]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta
[Quantity]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#quantity-resource-core
