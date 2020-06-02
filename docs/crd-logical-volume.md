LogicalVolume
=============

`LogicalVolume` is a custom resource definition (CRD) that represents
a TopoLVM volume and helps communication between CSI controller and
node services.

| Field        | Type                | Description                                                           |
| ------------ | ------------------- | --------------------------------------------------------------------- |
| `apiVersion` | string              | APIVersion.                                                           |
| `kind`       | string              | Kind.                                                                 |
| `metadata`   | [ObjectMeta][]      | Standard object's metadata with a special annotation described below. |
| `spec`       | LogicalVolumeSpec   | Specification of desired behavior of the logical volume.              |
| `status`     | LogicalVolumeStatus | Most recently observed status of the logical volume.                  |

LogicalVolumeSpec
-----------------

| Field      | Type         | Description                                                    |
| ---------- | ------------ | -------------------------------------------------------------- |
| `name`     | string       | Suggested name of the logical volume.                          |
| `nodeName` | string       | Name of the node where the logical volume should be created.   |
| `size`     | [Quantity][] | Amount of local storage required for the logical volume.       |
| `vgName`   | string       | Name of the volume group that the logical volume belongs with. |

LogicalVolumeStatus
-------------------

| Field         | Type         | Description                                                                        |
| ------------- | ------------ | ---------------------------------------------------------------------------------- |
| `volumeID`    | string       | Name of the logical volume.  Also used as the unique volume ID in the CSI context. |
| `code`        | uint32       | [gRPC error code](https://github.com/grpc/grpc/blob/master/doc/statuscodes.md).    |
| `message`     | string       | Error message.                                                                     |
| `currentSize` | [Quantity][] | Amount of the local storage assigned for the logical volume.                       |

Lifecycle
---------

Initially, `status.volumeID` and `status.currentSize` are empty. They are set by `topolvm-node` on target nodes
after it creates an LVM logical volume.

`spec.size` of `LogicalVolume` is updated by `topolvm-controller`
when the volume size of the corresponding PVC is increased.
`topolvm-node` watches the `LogicalVolume` resource and resizes the LVM logical
volume when it finds the difference between `status.currentSize` and `spec.size`.
In order for `topolvm-node` to retry resizing, `topolvm-controller` updates
`metadata.annotations["topolvm.cybozu.com/resize-requested-at"]` of `LogicalVolume`.

After the LVM logical volume is expanded successfully, `topolvm-node` updates
`status.currentSize` value.
If fails, `topolvm-node` updates the `status.code` and `status.message` with
the returned error.

`LogicalVolume` is created with a [finalizer](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers).
When a `LogicalVolume` is being deleted, `topolvm-node` on the target node deletes
the corresponding LVM logical volume and clears the finalizer.

[ObjectMeta]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#objectmeta-v1-meta
[Quantity]: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#quantity-resource-core
