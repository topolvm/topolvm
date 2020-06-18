# Multiple Volume Groups

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
- [Proposal](#proposal)
  - [Option A) device-class](#option-a-device-class)
  - [Option B) multiple provisioner](#option-b-multiple-provisioner)
  - [Decision Outcome](#decision-outcome)
- [Design Details](#design-details)
  - [How to expose free storage capacity of nodes](#how-to-expose-free-storage-capacity-of-nodes)
  - [How to annotate resources](#how-to-annotate-resources)
  - [Setting of divisors](#setting-of-divisors)
  - [Ephemeral Inline Volume](#ephemeral-inline-volume)
  - [Device-class setting](#device-class-setting)
  - [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
<!-- /toc -->

## Summary

Multiple Volume Groups is a feature to enable TopoLVM to use multiple arbitrary volume groups.

## Motivation

In cases where a node has different types of storage devices such as HDD and SSD,
users may want to prepare and use volume groups for each storage type.

### Goals

- Introduce a new concept called device-classes to indicate a target volume group.
- Allow users to specify a device-class in StorageClass.
- Create logical volumes on the target volume groups.
- Schedule pods respecting the free storage space of the target volume group.
- Ephemeral inline volumes are created on the default volume group.
- Keep backward compatibility.

## Proposal

### Option A) device-class

This proposal make it possible to specify a name of device-class
as a parameter of a StorageClass as follows:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-hdd
provisioner: topolvm.cybozu.com
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.cybozu.com/device-class": "hdd"
volumeBindingMode: WaitForFirstConsumer
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-ssd
provisioner: topolvm.cybozu.com
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.cybozu.com/device-class": "ssd"
volumeBindingMode: WaitForFirstConsumer
```

The device-class name is then passed to `lvmd`.
`lvmd` has a mapping between device-classes and LVM volume groups.
It can, therefore, create a logical volume in multiple volume groups.

If no device-class is given, `lvmd` will use the default volume group.
Therefore, it is possible to keep compatibility without changing existing storageClasses when upgrading.

Pros:
- It requires to launch only one TopoLVM to support multiple volume groups.

Cons:
- Users need to prepare device-class setting for `lvmd`.

### Option B) multiple provisioner

This proposal provides a way to deploy multiple TopoLVMs on a single Kubernetes cluster.
Each TopoLVM handles a different volume group.

Users can use arbitrary TopoLVM by specifying a provisioner name in a storageClass as follows:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-hdd
provisioner: topolvm.cybozu.com/hdd
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
volumeBindingMode: WaitForFirstConsumer
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-ssd
provisioner: topolvm.cybozu.com/ssd
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
volumeBindingMode: WaitForFirstConsumer
```

This proposal requires launching TopoLVM components for each provisioner.
Since `lvmd` is also launched per provisoner, it will continue to target only one volume group as before.

Pros:
- It doesn't require many changes to implement.

Cons:
- Users will need a lot of work to launch multiple TopoLVMs.

### Decision Outcome

Choose options: [A) device-class](#option-a-device-class),
because option B) is complicated to launch multiple TopoLVM for users.

## Design Details

### How to expose free storage capacity of nodes

Currently `topolvm-node` exposes free storage capacity as `capacity.topolvm.cybozu.com/<deviec-calss>` annotation of each Node as follows:

```yaml
kind: Node
metadata:
  name: worker-1
  annotations:
    capacity.topolvm.cybozu.com/ssd: "1073741824"
```

This proposal will change annotation to `capacity.topolvm.cybozu.com/<device-class>` as follows 
to expose the capacity of each node:

```yaml
kind: Node
metadata:
  name: worker-1
  annotations:
    capacity.topolvm.cybozu.com/__default__: "1073741824"
    capacity.topolvm.cybozu.com/ssd: "1073741824"
    capacity.topolvm.cybozu.com/hdd: "1099511627776"
```

The default device-class is annotated without the part of the name, like `capacity.topolvm.cybozu.com`.

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

#### A-1) insert multiple resources

This proposal would insert `capacity.topolvm.cybozu.com/<device-class>` as follows:

```yaml
spec:
  containers:
  - name: testhttpd
    resources:
      requests:
        capacity.topolvm.cybozu.com/ssd: "1073741824"
        capacity.topolvm.cybozu.com/hdd: "1099511627776"
      limits:
        capacity.topolvm.cybozu.com/ssd: "1073741824"
        capacity.topolvm.cybozu.com/hdd: "1099511627776"
```

Then users should modify the scheduler policy as follows:

```json
{
    ...
    "extenders": [{
        "urlPrefix": "http://...",
        "filterVerb": "predicate/ssd",
        "prioritizeVerb": "prioritize/ssd",
        "weight": 2,
        "managedResources":
        [{
          "name": "capacity.topolvm.cybozu.com/ssd",
          "ignoredByScheduler": true
        }],
        "nodeCacheCapable": false
    },
    {
        "urlPrefix": "http://...",
        "filterVerb": "predicate/hdd",
        "prioritizeVerb": "prioritize/hdd",
        "weight": 1,
        "managedResources":
        [{
          "name": "capacity.topolvm.cybozu.com/hdd",
          "ignoredByScheduler": true
        }],
        "nodeCacheCapable": false
    }]
}
```

Users will need to add a extender setting for each device-class.
In order for the scheduler to know the device-class name, users need to pass the device-class name in verb.
Then, users can specify weight parameter for each device-class.

Pros:
- The weight of extender can be adjusted for each device-class.

Cons:
- The settings of scheduler policy are complicated and must be rewritten according to your environment.

#### A-2) insert multiple annotations

This proposal would insert `topolvm.cybozu.com/capacity` to resources and `capacity.topolvm.cybozu.com/<device-class>` annotation as follows:

```yaml
metadata:
  annotations:
    capacity.topolvm.cybozu.com/ssd: "1073741824"
    capacity.topolvm.cybozu.com/hdd: "1099511627776"
spec:
  containers:
  - name: testhttpd
    resources:
      requests:
        topolvm.cybozu.com/capacity: "1"
      limits:
        topolvm.cybozu.com/capacity: "1"
```

The values of `topolvm.cybozu.com/capacity` don't matter.

Users shouldn't modify the scheduler policy.

Pros:
- The settings of scheduler policy is simple.
- For pods that use only one volume group, scheduling can be done as usual.
- For pods that use two or more volume group, scheduling to a node with insufficient capacity can be avoided.

Cons:
- The weight of extender cannot be adjusted individually when using two or more volume group.

#### Decision outcome

Choose options: [A-2) insert multiple annotations](#a-2-insert-multiple-annotations),
because option A-1) is complicated to set scheduler policy. In most cases, option 2 works without problems.

### Setting of divisors

Currently, topolvm-scheduler calculates the score of a node by this formula:

    min(10, max(0, log2(capacity >> 30 / divisor)))

This proposal would calculate the score of each device-class by the above formula.
Then the smallest value of them will be used as the final score.

Users can specify dedicated `divisor` parameter for each device-class as follows:
 
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: topolvm-config
  namespace: topolvm-system
data:
  scheduler.yaml: |
    default-divisor: 1
    divisors:
      ssd: 5
      hdd: 10
```

### Ephemeral Inline Volume

Ephemeral Inline Volumes are not related to StorageClass.

`topolvm-node` make `lvmd` create logical volumes for ephemeral inline volumes on the default volume group.

### Device-class setting

This proposal makes use of the concept of device-class to hide volume group names that are node-local.

Therefore, `lvmd` should have a device-class setting as follows:

```yaml
device-classes:
  - name: ssd
    volume-group: ssd-vg
    spare-gb: 10
    default: true
  - name: hdd
    volume-group: hdd-vg
    spare-gb: 10
```

If StorageClass has no `topolvm.cybozu.com/device-class` parameter,
`lvmd` will use the default device-class.
`lvmd` always uses the default device-class for ephemeral inline volumes.

### Upgrade / Downgrade Strategy

Perform the following steps to upgrade:

1. Add `ConfigMap` resource for setting of divisors. (see [Setting of divisors](#setting-of-divisors))
1. Prepare a configuration file for `lvmd`. (see [Device-class setting](#device-class-setting))
1. Replace `lvmd` binary and restart `lvmd.service`.
1. Update container images for TopoLVM.
1. Add the name of device-class to StorageClass resources. (optional)

If you've run `lvmd` in the previous version as follows:

```console
$ lvmd --volume-group=myvg1 --listen=/run/topolvm/lvmd.sock --spare=10
```

You have to prepare a configuration file as follows:

```yaml
socket-name: /run/topolvm/lvmd.sock
device-classes:
  - name: ssd
    volume-group: myvg1
    default: true
    spare-gb: 10
```

And run `lvmd` with the following options:

```console
$ lvmd --config=/path/to/your-config.yaml
```

