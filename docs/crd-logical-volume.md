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
`code`     uint32 | gRPC error code.
`message`  string | Error message.


Phase of Logical Volume Lifecycle
---------------------------------

A `LogicalVolume` resource is in one of the following lifecycle phases.

* `""`:
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

* `TERMINATE_FAILED`
The resource is being deleted but a corresponding logical volume failed to be deleted.

TODO: denote who makes transitions.

![component diagram](http://www.plantuml.com/plantuml/svg/ROxH2e8m58RlznG7hohm1Rm8fJG4yyAacuWOso46KsCx8thxD1KqTdVs_zyv-s9Bt91hDEi7GWWssBpeims0Mr2j8iKrk-tk4ChktORxEOkWGjiv8n2K1ODFPGaDIZRr4FRieKgJEZr6S75284gW3eJ1uP_YwY4VMP8N0vznfV_n5V9RwhN6T7hNQNNEowJEonDRpAjkjXclIzGuNlVp7YVFbaThBXPHZArqZVu2)


[ObjectMeta]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta
[Quantity]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#quantity-resource-core
