# Snapshot & Restore

> Note: Based on thin-provisioning of volumes. Please refer to [thin-provisioning proposal doc](thin-volumes.md).

> Warning: Support for creation of VolumeSnapshots is available only for `thinly-provisioned` volumes.

## Goals

Allow users to create thin-snapshots from thinly-provisioned logical volumes.
These thin-snapshots, then set the foundation for allowing creation of snapshots of persistent volume claims.

* **Allow Snapshot Creation**: Enable users to take snapshots of PVs that are provisioned by TopoLVM.

* **Restoring Snapshots**: Based on a volumesnapshot, a new PVC can be created that contains the contents of the snapshot.

## Introduction

### Thin-Snapshots

* Snapshots of thin LVs are efficient because the data blocks common to a thin LV and its snapshot are shared.

* As common data blocks are shared, A thin snapshot volume can reduce disk usage when there are multiple snapshots of the same origin volume.

* The thin snapshot volume is independent of source volume deletion; and can be used independently just like a standalone volume.

* These thin logical volumes can be activated as `Read-Only` copies of the source, and provided to kubernetes users as a VolumeSnapshotContent.

### Restoring thin-snapshots

* Thin snapshot volumes can be used as a logical volume origin for another snapshot. This allows for an arbitrary depth of recursive snapshots (snapshots of snapshots of snapshots ... )

* Blocks common to recursive snapshots are also shared in the thin pool. There is no limit to or degradation from sequences of snapshots.

* To provide a restored PVC to the user, we create another thin-provisioned snapshot of the available snapshot; and since lvm snapshots,  by design, are writable, so they can be provided to users to use as a volume for a new PVC.

* This way, we also don't lose the identity of the snapshot after restoring; so we can restore a snapshot multiple times, to create multiple copies.

* These thin snapshot volumes are activated as `Read-Write` copies of the source, and provided to kubernetes users as a PersistentVolume object.

![](https://i.imgur.com/bwsK1Dl.png)

> Note: The Snapshots, Restored volumes and Clones are all independent of their source deletion.

## Flow of Operations for Snapshot Creation

* A user creates a `VolumeSnapshot` custom resource; with the source set to the source PVC name.
* The `snapshot-controller` which was watching the `VolumeSnapshot` object, sends a `Create VolumesnapshotContent` req to the api-server.
* The `external-snapshotter` kubernetes sidecar receives this request, and then sends a `CreateSnapshot` gRPC call to TopoLVM CSI driver.
* This `CreateSnapshot` request is received by the `topolvm-controller` which then creates a `LogicalVolume` CR with the source set to the name of Logical Volume of the source PVC.
* `topolvm-node` on the target node reconciles the `LogicalVolume` CR created for the snapshot request.
* `topolvm-node` sends a volume create request to lvmd to create a thin-snapshot from the thin-volume datasource.
* `lvmd` creates an LVM snapshot logical volume as requested.
* `topolvm-node` updates the status of `LogicalVolume`.
* `topolvm-controller` finds the updated status of `LogicalVolume`.
* `topolvm-controller` sends the success (or failure) to `external-snapshotter` sidecar.

## LogicalVolume CR for Snapshots

```yaml
apiVersion: topolvm.cybozu.com/v1
kind: LogicalVolume
metadata:
  name: snapcontent-b083470e-8293-47cc-810d-9561bd1754e6 # restore pv name
  annotations:
spec:
  deviceClass: <string>
  name: snapcontent-b083470e-8293-47cc-810d-9561bd1754e6
  nodeName: 192.168.26.40
  # 'source' specifies the `LogicalVolume` name of the source; if present.
  # This field is populated only when `LogicalVolume` has a source.
  source: <string>
  # 'accessType' specifies how the user intents to consume the
  # thin-snapshot logical volume.
  # Set to "ro" when creating a snapshot and to "rw" when restoring a snapshot.
  # This field is populated only when `LogicalVolume` has a source.
  accessType: ro
status:
  volumeID: <The volume id of the lvm snapshot>
```

## Flow of Operations for Restore

* An user creates a `PersistentVolumeClaim` custom resource with data source as a `VolumeSnapshot`
* The `external-provisioner` kubernetes sidecar receives this request, and then sends a `CreateVolume` gRPC call to TopoLVM CSI driver.
* This `CreateVolume` request containing a dataSource is received by the `topolvm-controller`.
* From the `CreateVolume` req, validate if the volume creation request has a dataSource
  * If `req.VolumeContentSource` is nil, proceed with normal volume creation
  * Or, the Volume creation request might have a data source which is of type Snapshot or Persistent Volume.

    * i.e., `volumeSource.Type.(type)` can be
      * `VolumeContentSource_Snapshot`: for PVC-Restore from snapshot.
      * `VolumeContentSource_Volume`: in case of PVC-PVC clone request.

* Fetch the Logical Volume corresponding to the source.
* Since we have the source volume now, pass it to the `lvservice.CreateVolume`; so that we can populate the `LogicalVolume` CRD with the same parameters as source:
   <!-- snapshot add -->

  ```yaml
  Spec: topolvmv1.LogicalVolumeSpec{
            Name:        pvcName,
            # The restored lv must be created on the same node as source.
            NodeName:    sourceVolume.Spec.NodeName,
            # restored lv created on the same deviceClass as the source.
            DeviceClass: sourceVolume.Spec.DeviceClass,
            # the size of the restored logical volume must be the same as the source.
            Size:        sourceVolume.Spec.Size,
            # 'source' specifies the logicalvolume name of the source; if present.
            Source:      <string>
            # Set to "ro" when creating a snapshot and to "rw" when restoring a snapshot.
            accessType:  rw
        },
  ```
  This will make sure that the **Restored volumes are placed on the same node and the same DeviceClass as the source**.

* `topolvm-node` on the target node reconciles the `LogicalVolume` CR.
* `topolvm-node` sends a volume create request to lvmd to create a thin-snapshot from the thin-volume datasource.
* `lvmd` creates an LVM logical volume as requested.
* `lvmd` activates the thin logical volume with `Read-Write` permissions.
* `topolvm-node` updates the status of `LogicalVolume`.
* `topolvm-controller` finds the updated status of `LogicalVolume`.
* `topolvm-controller` sends the success (or failure) to `provisioner` sidecar.


## LogicalVolume CR for Restore operation

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

## LV Operations

* Snapshot creation from a thinly-provisioned volume
    ```console
    $ lvcreate -s --name thinsnap VG0/thinvolume
    ```

* Activation of a logical volume

  A thinly-provisioned snapshot will be activated as Read-Only for snapshots and Read-Write for Restore scenarios.

  For e.g: Activating a logical volume for Read-Write operations:

  ```console
  $ lvchange -ay -K VG0/thinsnap
  ```

* Restoring a Snapshot


  ```console
  $ lvcreate -s --name thin-restored VG0/thinsnap
  ```

## VolumeSnapshotClass and VolumeSnapshot

### VolumeSnapshotClass

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: topolvm-snapclass
driver: topolvm.cybozu.com
deletionPolicy: Delete
```

### VolumeSnapshot

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: new-snapshot
spec:
  volumeSnapshotClassName: topolvm-snapclass
  source:
    persistentVolumeClaimName: snapshot-pvc
```

## Restoring the Snapshot to a new PVC

When creating a snapshot or restoring it to a new volume; both the snapshot and the restored volume need to be provisioned
on the same node as the source volume.

To make sure the restored volume is provisioned on the same node as that of the source, we use node selectors to select the same
node for provisioning.

To schedule the restored pod on the same node, the restored pod has the node affinity set for the node on which source PVC is provisioned.

`pod-restore.yaml`

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-restore
spec:
# The Restored PVC needs to be provisioned on the same node as the source volume.
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: topology.topolvm.cybozu.com/node
            operator: In
            values: topolvm-example-worker
 ...
 ...
 ...
```
## Deployment Changes

* Add `external-snapshotter` sidecar.

* Perform `snapshot-controller` deployment.

## Metrics and Monitoring

Should be already covered during adding support for `thin` logical volumes. See [thin-provisioning proposal doc](thin-volumes.md).

## Additional Notes & Limitations

* When creating a thin snapshot volume in lvm, you do not specify the size of the volume.

* The PVCs restored from snapshots will also be created using the same storageclass as the source.

* The thin-snapshots will also be provisioned on the same node as their thinly-provisioned source.

* Currently, in case of restoring the snapshots, the user needs to specify the node for scheduling the restored application/pod. See [restoring a
snapshot](#restoring-the-snapshot-to-a-new-pvc) for more details.
