# PVC-PVC Cloning

> Note: Based on thin-provisioning of volumes and snapshots. Please refer to [thin-provisioning proposal doc](thin-volumes.md) and [thin-lv snapshots proposal doc](thin-snapshots-restore.md).

> Warning: Support for creation of PVC-PVC clones is available only for `thinly-provisioned` volumes.

## Goal

Allow users to create PVC-PVC Clones for PVCs backed by thinly-provisioned logical volumes.

These PVC clones are independent of the source volumes, and can be used for performing read-write operations.

## Introduction

### Creating PVC-PVC Clones using thin-snapshots

* By design, a thin-snapshot of thin-volumes behave just as a new independent thinly-provisioned volume which is pre-populated with data from the parent.

* Thus, this thin-snapshot can be then provided to an user for Read-Write operations; on which, writes are performed on a Copy-On-Write basis (fast clones!).

* These thin logical volumes can be activated as `Read-Write` copies of the parent, and provided to kubernetes users as a PersistentVolume object.

* The PVC Clones created using thin-snapshots will also be `copy-on-write`, so increasing the number of clones of the origin should yield no major slowdown.

> Note: Although both kubernetes snapshots and clones are thin-snapshots internally, their access is set by activating them for required permissions. So, a snapshot is just activated for Read-Only mode, on the other hand, clones are activated to allow Read-Write operations by a user.

![](https://i.imgur.com/bdyqraR.png)


## LV Operations

* STEP 1: Snapshot creation from a thinly-provisioned volume
    ```bash
    $ lvcreate -s --name thinsnap VG0/thinvolume
    ```

* STEP 2: Activation of the logical volume

  A thinly-provisioned snapshot will be activated as Read-Write for Clone scenarios; so that it can be provided as a Persistent Volume object to kubernetes.

    For e.g: Activating a logical volume for Read-Write operations:
    ```bash
    $ lvchange -kn -ay VG0/thinsnap
    ```

## Flow of Operations for Cloning

* An user creates a `PersistentVolumeClaim` custom resource with data source as a `PersistentVolumeClaim`
* The `external-provisioner` kubernetes sidecar receives this request, and then sends a `CreateVolume` gRPC call to TopoLVM CSI driver.
* This `CreateVolume` request containing a dataSource is received by the `topolovm-controller`.
* From the `CreateVolume` req, validate if the volume creation request has a dataSource
  * If `req.VolumeContentSource` is nil, proceed as normal volume creation
  * On the other hand, the Volume creation request might have a data source as either a Snapshot or Persistent Volume.

    * i.e., `volumeSource.Type.(type)` can be
      * `VolumeContentSource_Snapshot`: for PVC-Restore from snapshot.
      * `VolumeContentSource_Volume`: in case of PVC-PVC clone request.

* Fetch the Source Volume corresponding to the volumeID of the parent.
* Since we have the source volume now, pass it to the `lvservice.CreateVolume`; so that we can populate the `LogicalVolume` CRD with the same parameters as parent:

  ```yaml
  Spec: topolvmv1.LogicalVolumeSpec{
            Name:        pvcName,
            # The cloned lv must be created on the same node as source.
            NodeName:    sourceVolume.Spec.NodeName,
            # cloned lv created on the same deviceClass as the source.
            DeviceClass: sourceVolume.Spec.DeviceClass,
            # the size of the cloned LogicalVolume must be the same as the source.
            Size:        sourceVolume.Spec.Size,
            # 'source' specifies the LogicalVolume name of the source; if present.
            Source:      <string>
            # Set to "rw" when cloning a volume from a source.
            accessType:  rw
  },
  ```

  This will make sure that the **Cloned volumes are placed on the same node and the same DeviceClass as the parent**.

* `topolvm-node` on the target node finds the earlier created `LogicalVolume` CR.
* `topolvm-node` sends a volume create request to lvmd to create a thin-snapshot from the thin-volume datasource.
* `lvmd` creates an LVM logical volume as requested.
* `lvmd` activates the thin logical volume with `Read-Write` permissions.
* `topolvm-node` updates the status of `LogicalVolume`.
* `topolvm-controller` finds the updated status of `LogicalVolume`.
* `topolvm-controller` sends the success (or failure) to `provisioner` sidecar.

## Proposed LogicalVolume CR for Clone operation

```yaml
apiVersion: topolvm.cybozu.com/v1
kind: LogicalVolume
metadata:
  name: pvc-42041aee-79a2-4184-91aa-5e1df6068b9f # restore pv name
  annotations:
spec:
  deviceClass: <string>
  name: pvc-42041aee-79a2-4184-91aa-5e1df6068b9f
  nodeName: 192.168.26.40
  # 'source' specifies the `LogicalVolume` name of the source; if present.
  # This field is populated only when `LogicalVolume` has a source.
  source: <string>
  # 'accessType' specifies how the user intents to consume the
  # thin-snapshot logical volume.
  # Set to "ro" when creating a snapshot and to "rw" when restoring a snapshot.
  # This field is populated only when `LogicalVolume` has a source.
  accessType: rw
status:
  volumeID: <Store the volume id of the lvm snapshot as same as existing LV.>

```

## Deployment Changes

No deployment changes required for adding support for cloning feature.

## Metrics and Monitoring

Should be already covered during adding support for `thin` logical volumes. [See](thin-volumes.md)

## Additional Notes & Limitations

* The cloned PVCs will also be created using the same storageclass as the parent.

* The PVC Clones are independent of their parent's deletion.

* The PVC clones will be provisioned on the same node and device class as their source.

* To achieve that, a user needs to specify the node for scheduling the cloned application as a node affinity, so that it
co-exists with the source on the same node.
