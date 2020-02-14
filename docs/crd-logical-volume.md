LogicalVolume
=============

`LogicalVolume` is a custom resource definition (CRD) that represents
a TopoLVM volume and helps communication between CSI controller and
node services.

| Field        | Type                | Description                                              |
| ------------ | ------------------- | -------------------------------------------------------- |
| `apiVersion` | string              | APIVersion.                                              |
| `kind`       | string              | Kind.                                                    |
| `metadata`   | [ObjectMeta][]      | Standard object's metadata.                              |
| `spec`       | LogicalVolumeSpec   | Specification of desired behavior of the logical volume. |
| `status`     | LogicalVolumeStatus | Most recently observed status of the logical volume.     |

LogicalVolumeSpec
-----------------

| Field      | Type         | Description                                                  |
| ---------- | ------------ | ------------------------------------------------------------ |
| `name`     | string       | Suggested name of the logical volume.                        |
| `nodeName` | string       | Name of the node where the logical volume should be created. |
| `size`     | [Quantity][] | Amount of local storage required for the logical volume.     |

LogicalVolumeStatus
-------------------

| Field        | Type         | Description                                                                        |
| ------------ | ------------ | ---------------------------------------------------------------------------------- |
| `volumeID`   | string       | Name of the logical volume.  Also used as the unique volume ID in the CSI context. |
| `code`       | uint32       | gRPC error code. If there is no error, `0` is assigned.                            |
| `message`    | string       | Error message.                                                                     |
| `targetSize` | [Quantity][] | Amount of the resized local storage required for the logical volume.               |

Lifecycle
---------

Initially, `status.volumeID` is empty.  It is set by `topolvm-node` on target nodes
after it creates an LVM logical volume.

`status.targetSize` is added before the volume is resized.
The target node watches the `LogicalVolume` resource and resizes the LVM logical
volume when it finds the `status.targetSize` being added.
If the LVM logical volume has successfully extended, `topolvm-controller` updates
`spec.size` with the `status.targetSize` value and removes the `.status.targetSize`.
If fails, `topolvm-controller` updates the `status.code` and `status.message` with
the returned error and removes the `.status.targetSize`.

`LogicalVolume` is created with a [finalizer](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers).
When a `LogicalVolume` is being deleted, `topolvm-node` on the target node deletes
the corresponding LVM logical volume and clears the finalizer.

[ObjectMeta]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#objectmeta-v1-meta
[Quantity]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.14/#quantity-resource-core
