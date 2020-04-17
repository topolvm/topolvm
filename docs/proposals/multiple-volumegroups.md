# Multiple Volume Groups

<!-- toc -->
- [Multiple Volume Groups](#multiple-volume-groups)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals](#non-goals)
  - [Proposal](#proposal)
  - [Design Details](#design-details)
    - [How to expose free storage capacity of nodes](#how-to-expose-free-storage-capacity-of-nodes)
    - [How to annotate resources](#how-to-annotate-resources)
      - [1) insert multiple resources](#1-insert-multiple-resources)
      - [2) insert multiple annotations](#2-insert-multiple-annotations)
    - [How to schedule pods](#how-to-schedule-pods)
    - [Default Volume Group](#default-volume-group)
    - [Ephemeral Volume](#ephemeral-volume)
    - [Test Plan](#test-plan)
    - [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
<!-- /toc -->

## Summary

The Multiple Volume Groups makes it possible to use multiple arbitrary Volume Groups
by specifying the name of Volume Group for in StorageClass.

## Motivation

The current TopoLVM Implementation (v0.4.5) can handle only a single Volume Group.
However, for example, in the case where a node has HDDs and SSDs, we would like to
prepare and use Volume Group for each device. 

### Goals

- Enable to create Logical Volume using Volume Group specified in StorageClass
- Schedule pods according to free storage space for each Volume Group
- Assign Logical Volume for Ephemeral Volume from default Volume Group

### Non-Goals


## Proposal

This proposal make it possible to specify a name of Volume Group
as a parameter of a StorageClass as follows:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-hdd
provisioner: topolvm.cybozu.com
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.cybozu.com/volume-group": "hdd"
volumeBindingMode: WaitForFirstConsumer
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-ssd
provisioner: topolvm.cybozu.com
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.cybozu.com/volume-group": "ssd"
volumeBindingMode: WaitForFirstConsumer
```

The components of TopoLVM use the name of VolumeGroup to create, update and delete
LogicalVolume.

## Design Details

### How to expose free storage capacity of nodes

Currently `topolvm-node` exposes free storage capacity as `topolvm.cybozu.com/capacity` annotation of each Node as follows:

```yaml
kind: Node
metdta:
  name: wroker-1
  annotations:
    topolvm.cybozu.com/capacity: "1073741824"
```

This proposal will change annotation to `topolvm.cybozu.com/capacity-<volume group>` as follows 
to expose the capacity of each node:

```yaml
kind: Node
metdta:
  name: wroker-1
  annotations:
    topolvm.cybozu.com/capacity-vg1: "1073741824"
    topolvm.cybozu.com/capacity-vg2: "1073741824"
    topolvm.cybozu.com/capacity-vg3: "1073741824"
```

### How to annotate resources

Currently, the mutating webhook inserts `topolvm.cybozu.com/capacity` to the first container as follows:

```yaml
spec:
  containers:
  - name: testhttpd
    resources:
      limits:
        topolvm.cybozu.com/capacity: "1073741824"
      requests:
        topolvm.cybozu.com/capacity: "1073741824"
```

Then, `topolvm-scheduler` need to be configured in scheduler policy as follows:

```json
{
    ...
    "extenders": [{
        "urlPrefix": "http://...",
        "filterVerb": "predicate",
        "prioritizeVerb": "prioritize",
        "managedResources":
        [{
          "name": "topolvm.cybozu.com/capacity",
          "ignoredByScheduler": true
        }],
        "nodeCacheCapable": false
    }]
}
```

I have two proposal to manage capacity for multiple Volume Groups:

#### 1) insert multiple resources

This proposal would insert `topolvm.cybozu.com/capacity-<volme gurp>` as follows:

```yaml
spec:
  containers:
  - name: testhttpd
    resources:
      limits:
        topolvm.cybozu.com/capacity-vg1: "1073741824"
        topolvm.cybozu.com/capacity-vg2: "1073741824"
        topolvm.cybozu.com/capacity-vg3: "1073741824"
      requests:
        topolvm.cybozu.com/capacity-vg1: "1073741824"
        topolvm.cybozu.com/capacity-vg2: "1073741824"
        topolvm.cybozu.com/capacity-vg3: "1073741824"
```

Then users should modify the scheduler policy as follows:

```json
{
    ...
    "extenders": [{
        "urlPrefix": "http://...",
        "filterVerb": "predicate",
        "prioritizeVerb": "prioritize",
        "managedResources":
        [{
          "name": "topolvm.cybozu.com/capacity-vg1",
          "ignoredByScheduler": true
        },
        {
          "name": "topolvm.cybozu.com/capacity-vg2",
          "ignoredByScheduler": true
        },
        {
          "name": "topolvm.cybozu.com/capacity-vg3",
          "ignoredByScheduler": true
        }],
        "nodeCacheCapable": false
    }]
}
```

#### 2) insert multiple annotations

This proposal would insert `topolvm.cybozu.com/capacity` to resources and `topolvm.cybozu.com/capacity-<volme gurp>` annotation as follows:

```yaml
metdta:
  annotations:
    topolvm.cybozu.com/capacity-vg1: "1073741824"
    topolvm.cybozu.com/capacity-vg2: "1073741824"
    topolvm.cybozu.com/capacity-vg3: "1073741824"
spec:
  containers:
  - name: testhttpd
    resources:
      limits:
        topolvm.cybozu.com/capacity: "1"
      requests:
        topolvm.cybozu.com/capacity: "1"
```

The vales of `topolvm.cybozu.com/capacity` don't matter.

Users shouldn't modify the scheduler policy.

### How to schedule pods

Currently, topolvm-scheduler calculates the score of a node by this formula:

    min(10, max(0, log2(capacity >> 30 / divisor)))

This proposal would calculate the score of each VG by the above formula, 
and use the average of them as the final score.

### Default Volume Group

The current TopoLVM can handle only a single Volume Group.

When you upgrade TopoLVM, the existing StorageClasses don't contain a VolumeGroup, 
so TopoLVM cannot know the name of VolumeGroup.

This proposal would prepare a ConfigMap resource as follows and share it between components of TopoLVM.

```yaml
apiVersion: v1
kind: ConfigMap
metaadta:
  name: topolvm-config
  namespace: topolvm-system
data:
  default-vg: myvg1
```

This approach will help maintain compatibility.

### Ephemeral Volume

As explained above, the name of Volume Group can be specified in StorageClass.
The name of Volume Group for Ephemeral Volume cannot be specified,
because Ephemeral Volume doesn't have StorageClass.

This proposal would use the default Volume Group to create a Logical Volume for Ephemeral Volume.

### Test Plan

T.B.D.

### Upgrade / Downgrade Strategy

Perform the following steps to upgrade:

1. Add `ConfigMap` resource for default Volume Group.
1. Replace `lvmd` binary and restart `lvmd.service`.
1. Update container images for TopoLVM.
1. Add the name of Volume Group to LogicalVolume resources and StorageClass resources. (optional)

If the name of Volume Group in LogicalVolume resources and StorageClass Resources is empty,
the default name of Volume Group will be used.
