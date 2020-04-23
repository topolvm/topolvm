# Multiple Volume Groups

<!-- toc -->
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
  - [The default volume group](#the-default-volume-group)
  - [Ephemeral Volume](#ephemeral-volume)
  - [Test Plan](#test-plan)
  - [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
<!-- /toc -->

## Summary

Multiple Volume Groups adds capability to use multiple arbitrary volume groups to TopoLVM.
The volume group to be used is specified as a parameter in StorageClass.
If no volume group is specified, the default volume group is used.

## Motivation

In cases where a node has different types of storage devices such as HDD and SSD,
users may want to prepare and use volume groups for each storage type.

### Goals

- Create logical volume on the volume group specified in the StorageClass
- Schedule pods respecting the free storage space of the target volume group
- Use the default volume group for ephemeral inline volumes
- Keep backward compatibility through the default volume group

### Non-Goals


## Proposal

This proposal make it possible to specify a name of volume group
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

The components of TopoLVM use the name of volume group to create, update and delete
logical volume.

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

This proposal will change annotation to `capacity.topolvm.cybozu.com/<volume group>` as follows 
to expose the capacity of each node:

```yaml
kind: Node
metdta:
  name: wroker-1
  annotations:
    capacity.topolvm.cybozu.com/vg1: "1073741824"
    capacity.topolvm.cybozu.com/vg2: "1073741824"
    capacity.topolvm.cybozu.com/vg3: "1073741824"
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

There are two possible designs to manage capacities of multiple volume groups as shown below.

#### 1) insert multiple resources

This proposal would insert `capacity.topolvm.cybozu.com/<volme gurp>` as follows:

```yaml
spec:
  containers:
  - name: testhttpd
    resources:
      limits:
        capacity.topolvm.cybozu.com/vg1: "1073741824"
        capacity.topolvm.cybozu.com/vg2: "1073741824"
        capacity.topolvm.cybozu.com/vg3: "1073741824"
      requests:
        capacity.topolvm.cybozu.com/vg1: "1073741824"
        capacity.topolvm.cybozu.com/vg2: "1073741824"
        capacity.topolvm.cybozu.com/vg3: "1073741824"
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
          "name": "capacity.topolvm.cybozu.com/vg1",
          "ignoredByScheduler": true
        },
        {
          "name": "capacity.topolvm.cybozu.com/vg2",
          "ignoredByScheduler": true
        },
        {
          "name": "capacity.topolvm.cybozu.com/vg3",
          "ignoredByScheduler": true
        }],
        "nodeCacheCapable": false
    }]
}
```

Currently, topolvm-scheduler calculates the score of a node by this formula:

    min(10, max(0, log2(capacity >> 30 / divisor)))

This proposal would calculate the score of each volume group by the above formula, 
and users can specify dedicated `divisor` parameter for each volume group.

Pros:
- Can calculate the score correctly by `divisor` parameters for each volume group

Cons:
- Requires to modify the scheduler policy depending on volume groups

#### 2) insert multiple annotations

This proposal would insert `topolvm.cybozu.com/capacity` to resources and `capacity.topolvm.cybozu.com/<volme gurp>` annotation as follows:

```yaml
metdta:
  annotations:
    capacity.topolvm.cybozu.com/vg1: "1073741824"
    capacity.topolvm.cybozu.com/vg2: "1073741824"
    capacity.topolvm.cybozu.com/vg3: "1073741824"
spec:
  containers:
  - name: testhttpd
    resources:
      limits:
        topolvm.cybozu.com/capacity: "1"
      requests:
        topolvm.cybozu.com/capacity: "1"
```

The values of `topolvm.cybozu.com/capacity` don't matter.

Users shouldn't modify the scheduler policy.

Currently, topolvm-scheduler calculates the score of a node by this formula:

    min(10, max(0, log2(capacity >> 30 / divisor)))

This proposal would calculate the score of each volume group by the above formula, 
and use the average of them as the final score.

Pros:
- No need to modify the scheduler policy depending on volume groups

Cons:
- Cannot calculate the score by specifying weight for each volume group

### The default volume group

The current TopoLVM can handle only a single volume group.

When you upgrade TopoLVM, the existing StorageClasses don't contain a volume group, 
so TopoLVM cannot know the name of volume group.

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

Also, If [1) insert multiple resources](#1-insert-multiple-resources) is adopted, 
the ConfigMap can contain `divisor` parameter for each volume group.
  
```yaml
apiVersion: v1
kind: ConfigMap
metaadta:
  name: topolvm-config
  namespace: topolvm-system
data:
  default-vg: myvg1
  divisors:
    myvg1: 1
    myvg2: 10
```

### Ephemeral Volume

As explained above, the name of volume group can be specified in StorageClass.
The name of volume group for Ephemeral Volume cannot be specified,
because Ephemeral Volume doesn't have StorageClass.

This proposal would use the default volume group to create a logical volume for Ephemeral Volume.

### Test Plan

T.B.D.

### Upgrade / Downgrade Strategy

Perform the following steps to upgrade:

1. Add `ConfigMap` resource for default volume group.
1. Replace `lvmd` binary and restart `lvmd.service`.
1. Update container images for TopoLVM.
1. Add the name of volume group to LogicalVolume resources and StorageClass resources. (optional)

If the name of volume group in LogicalVolume resources and StorageClass Resources is empty,
the default name of volume group will be used.
