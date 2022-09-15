# Allocation strategies

## Summary

Change `topolvm.cybozu.com` used in the CRD and plugin name of TopoLVM to `topolvm.io`.

## Motivation

We are thinking of donating the TopoLVM project to CNCF in the future.
So we would like to remove the domain name "cybozy.com," which contains the specific company's name, from the project.

### Goals

- Change `topolvm.cybozu.com` used in TopoLVM's CRD and plugin name to `topolvm.io`
- Keep TopoLVM available at `topolvm.cybozu.com` for existing users(but does not support concurrent use with topolvm.io partly)

## Proposal

Rename the group name `topolvm.cybozu.com` used below to `topolvm.io`:

- CSI Driver plugin name
- Various annotations and finalizers set by TopoLVM
- Various resource settings including CSIDriver resource
- CRD name

Also, by setting the variable of helm chart, it can be used as it is at `topolvm.cybozu.com` as before.
In addition, we will guide you through the migration procedure for users who wish to migrate their group names.

## Design details

### Changes to topolvm.io

Change the group name `topolvm.cybozu.com` used below to `topolvm.io`.

#### constants.go

constant name | value
--- | ---
CapacityKeyPrefix | capacity.topolvm.cybozu.com/
CapacityResource | topolvm.cybozu.com/capacity
PluginName | topolvm.cybozu.com
TopologyNodeKey | topology.topolvm.cybozu.com/node
DeviceClassKey | topolvm.cybozu.com/device-class
ResizeRequestedAtKey | topolvm.cybozu.com/resize-requested-at
LogicalVolumeFinalizer | topolvm.cybozu.com/logicalvolume
NodeFinalizer | topolvm.cybozu.com/node

#### CRD

Change from `logicalvolumes.topolvm.cybozu.com` to` logicalvolumes.topolvm.io`.

#### Resources

##### CSIDriver

Rename the `topolvm.cybozu.com` CSIDriver resource to `topolvm.io`

##### StorageClass

Modify each StorageClass resource in TopoLVM as follows:

- Change the value of the `provisioner` field from `topolvm.cybozu.com` to `topolvm.io`
- Rename the key name `topolvm.cybozu.com/device-class` in the` parameters` field to `topolvm.io/device-class`

**Since it is not possible to change the StorageClass resource, it is necessary to recreate the resource when migrating the group name.**

##### LogicalVolume

Modify each LogicalVolume resource in TopoLVM as follows:

- Change the CRD to use from `logicalvolumes.topolvm.cybozu.com` to` logicalvolumes.topolvm.io`
- Change the `topolvm.cybozu.com/resize-requested-at` annotation to` topolvm.io/resize-requested-at`
- Change the `topolvm.cybozu.com/logicalvolume` finalizer to` topolvm.io/logicalvolume`

**Since the CRD itself changes for the Logical Volume resource, it is necessary to delete the old CRD resource and create a new CRD resource when migrating the group name. Also, in order to prevent the deletion of the actual LVM volume due to the logical volume migration, it is recommended to migrate with the TopoLVM components stopped.**

##### Node

Make the following changes for each Node resources

- Change the `topolvm.cybozu.com/node` finalizer to` topolvm.io/node`

##### PersistentVolumeClaim

Modify each PersistentVolumeClaim resources in TopoLVM as follows:

- Change the value of the `volume.beta.kubernetes.io/storage-provisioner`[^storage-provisioner] annotation to` topolvm.io`
- Change the value of the `volume.kubernetes.io/storage-provisioner`[^storage-provisioner] annotation to` topolvm.io`

[^storage-provisioner]: https://github.com/kubernetes/kubernetes/blob/v1.24.2/pkg/controller/volume/persistentvolume/pv_controller_base.go#L638-L639

##### PersistentVolume

Modify each PersistentVolume resource in TopoLVM as follows:

- Change the value of the `pv.kubernetes.io/provisioned-by`[^provisioned-by] annotation to` topolvm.io`
- Change the value of the `.spec.csi.driver` field to` topolvm.io`
- Change the string `topolvm.cybozu.com` in the value of the `.spec.csi.volumeAttributes ["storage.kubernetes.io/csiProvisionerIdentity "]` field to` topolvm.io`

[^provisioned-by]: https://github.com/kubernetes/kubernetes/blob/v1.24.2/pkg/controller/volume/persistentvolume/pv_controller.go#L1662

**Since it is not possible to change the above fields of the PersistentVolume resource, it is necessary to recreate the resource when migrating the group name. Also, if `.spec.persistentVolumeReclaimPolicy = Delete` is set, there is a risk of deleting the actual LVM volume, so it is recommended to temporarily change it to `Retain` and then delete it.**

### Enable the use of topolvm.cybozu.com

We will make the following changes to allow users to continue to use `topolvm.cybozu.com`.

#### Addition of group name setting function

- If you set the `USE_LEGACY` environment variable, the group name `topolvm.cybozu.com` will be used
- If the `USE_LEGACY` environment variable is not set, the group name `topolvm.io` will be used

#### Group name setting in helm chart

Add the `.Values.useLegacy` variable to the helm chart and set this variable to `true` to set the installation manifest to use `topolvm.cybozu.com`.
If the `.Values.useLegacy` variable is not set to `true`, the group name will be `topolvm.io`.

If you are going to set the following variables, you may need to set them appropriately to use `topolvm.io` or `topolvm.cybozu.com` in the variables.

- `.Values.storageClasses`
- `.Values.node.volumes`
- `.Values.volumeMounts.topolvmNode`

#### Automatic generation for API and CRD

Automatically generating the API and CRD of `topolvm.cybozu.com` based on the API and CRD of `topolvm.io`.

### Auto group conversion for LogicalVolume

Add a wrap client for controller-runtime client to automatically convert a group of LogicalVolume resources.
Change each controller and other components to use this wrapped client.

### Things to be done by users of topolvm.cybozu.com for upgrading the helm chart

Since this change was released, users of TopoLVM with the group name `topolvm.cybozu.com` will need to take the following actions when upgrading the helm chart.

- Set the `.Values.useLegacy `variable to` true` in `values.yaml` in the helm chart
- If there is a part of the `values.yaml` in the helm chart that uses the string `topolvm.cybozu.com`, change it to `topolvm.io`

If you upgrade the helm chart without taking above actions, it is possible that TopoLVM related resources will be deleted.

### Migrate from topolvm.cybozu.com to topolvm.io

If you are already using TopoLVM with the group name `topolvm.cybozu.com` and want to migrate the group name to `topolvm.io` after the release of this change, you can manually change the data by the following method.

1. Avoid booting pods using TopoLVM volumes during migration.
1. Temporarily stop the following pods:
   - topolvm-controller
   - topolvm-node
   - topolvm-scheduler
1. Remove legacy TopoLVM resources without LogicalVolume CRD
1. Manually install the new TopoLVM without topolvm-node to another namespace
1. Update a scheduler config and restart a kube-scheduler
1. Perform the migration work for each resource as mentioned in the chapter `Changes to topolvm.io`.
   - Please migrate while confirming that the pod using the updated resource (e.g. Persistent Volume) continues to operate without problems.
1. Start topolvm-node after resource migration

## NOTE

### Alternative consideration

#### Automatic migration of group names

We considered a method to automatically perform the migration as described in `Design details` using script or Kubernetes Controller, but resources such as PersistentVolume and LogicalVolume need to be recreated.
There is a risk that the real LVM volume will be deleted due to the re-creation, and it was judged that the risk is high for automatic migration.
