# LV create options on StorageClass

## Summary

Allow specifying LV create options in StorageClass parameter.

## Motivation

We want to specify different LV create options for a VG, however, it is not
impossible because the options exist for each device-class on lvmd.conf and
each device-class can not share the same VG in the current implementation.

Although one possible solution is relaxing the restriction, it may introduce
difficulties to calculate VG capacity. For example, there are two device-class A and B.
They use the same VG whose capacity is 100 GB. A specifies `spare-gb: 10` and B does `spare-gb: 20`.
If users create a 90GB PV from A, it succeeds, but the remaining capacity of the VG is now 10GB.
It violates the invariant of `spare-gb: 20` of B which requires at least 20GB free space in the VG.

Moreover, theoretically, LV create options are not related to VG or device-class.

The proposed solution is that split the option from device-class and allow us to specify the option on StorageClass.

## Proposal

Introduce a new StorageClass parameter `topolvm.io/lvcreate-option-class` to specify LV create options.
The parameter refers to an option class that specifies lv options on lvmd.conf rather than real options.
This indirection provides flexibility to change options on each node.

We considered [the similar proposal](lvcreate-options.md) before, but it was not chosen because it is harder to be implemented.
Now, we revisit the idea and conclude that it is better to specify lv create options on wider use-case.

## Design Detail

Change lvmd.conf format as bellow:

```yaml
socket-name: /run/topolvm/lvmd.sock
device-classes:
  - name: dc
    volume-group: vg
    spare-gb: 10
lvcreate-option-classes:
  - name: raid1
    options:
      - --type=raid1
  - name: raid10
    options:
      - --type=raid10
```

We can specify the option using the new StorageClass parameter.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: raid1
parameters:
  csi.storage.k8s.io/fstype: ext4
  topolvm.io/device-class: dc
  topolvm.io/lvcreate-option-class: raid1
provisioner: topolvm.io
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```

The parameter value is passed to the topolvm-controller via CreateVolume RPC.
We have to modify LogicalVolume CR to pass the value from the controller to the topolvm-node
and also modify lvmd's CreateLVRequest RPC.

### Migration

The new parameter supersedes `stripe`, `stripe-size`, and `lvcreate-options` on device-class,
we deprecate these parameters and will remove them in the feature.
But this breaks current behavior, we use these parameters if and only if the new parameter is not specified.
