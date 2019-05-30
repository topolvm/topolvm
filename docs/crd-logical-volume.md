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
`volumeID` string | Name of the logical volume.  Also used as the volume ID in the CSI context, so this should be unique in the cluster.
`phase`    string | Phase of the logical volume lifecycle.  See below.
`message`  string | Error message.


Phase of Logical Volume Lifecycle
---------------------------------

A `LogicalVolume` resource is in one of the following lifecycle phases.

* `INITIAL`:
The resource has been registered, and a corresponding logical volume in LVM should be created.
Transit to `CREATED` if the logical volume is created successfully.
Transit to `CREATE_FAILED` if an error occurred.

* `CREATED`
The logical volume in LVM has been created and is now available.
Transit to `TERMINATING` if `LogicalVolume` is being deleted.

* `CREATE_FAILED`
The resource was registered but a corresponding logical volume failed to be created.

* `TERMINATING`
The resource is being deleted, and the corresponding logical volume in LVM should be deleted.
Transit to `TERMINATED` if the logical volume is deleted successfully.

* `TERMINATED`
The corresponding logical volume has been deleted and is now unavailable.

TODO: denote who makes transitions.

![component diagram](http://www.plantuml.com/plantuml/svg/ROzD2i8m44RtESMiXLwW2scmgGp4B69m8o8X6IHG4yWFNjygNIYw6tZl3Nn3gJRNTf_PUNE1pgT7xBQ02WrosOEcabfs1A50fbiebJ9vjdBe5dUd1JTYxE7Od2FoK1EuJBQ6546U_hZNYQDy5PCDys-mFdm7HkW3AcvGxTd7_SN4o0QAVjdm1000)


[ObjectMeta]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta
[Quantity]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#quantity-resource-core
