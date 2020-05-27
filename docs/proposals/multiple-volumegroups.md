# Multiple Volume Groups

<!-- toc -->
- [Multiple Volume Groups](#multiple-volume-groups)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
  - [Proposal](#proposal)
    - [Option A) device class](#option-a-device-class)
    - [Option B) multiple provisioner](#option-b-multiple-provisioner)
    - [Decision Outcome](#decision-outcome)
  - [Design Details](#design-details)
    - [How to expose free storage capacity of nodes](#how-to-expose-free-storage-capacity-of-nodes)
    - [How to annotate resources](#how-to-annotate-resources)
      - [A-1) insert multiple resources](#a-1-insert-multiple-resources)
      - [A-2) insert multiple annotations](#a-2-insert-multiple-annotations)
      - [Decision outcome](#decision-outcome-1)
    - [Setting of divisors](#setting-of-divisors)
    - [Ephemeral Volume](#ephemeral-volume)
    - [Upgrade / Downgrade Strategy](#upgrade--downgrade-strategy)
<!-- /toc -->

## Summary

Multiple Volume Groups adds capability to use multiple arbitrary volume groups to TopoLVM.

## Motivation

In cases where a node has different types of storage devices such as HDD and SSD,
users may want to prepare and use volume groups for each storage type.

### Goals

- Create logical volume on the volume group specified in the StorageClass
- Schedule pods respecting the free storage space of the target volume group
- Create ephemeral inline volumes on the volume group specified in the volumeAttributes
- Keep backward compatibility

## Proposal

### Option A) device class

This proposal make it possible to specify a name of device class
as a parameter of a StorageClass as follows:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-hdd
provisioner: topolvm.io
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.io/device-class": "hdd"
volumeBindingMode: WaitForFirstConsumer
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-ssd
provisioner: topolvm.io
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.io/device-class": "ssd"
volumeBindingMode: WaitForFirstConsumer
```

`lvmd` receives the device class as a parameter. It has a map between device classes and volume groups, 
operates a volume group mapped a specified device class.

Pros:
- It requires to launch only one TopoLVM to support multiple volume groups.

Cons:
- It doesn't have compatibility. Users need some procedures to upgrade.

### Option B) multiple provisioner

This proposal make it possible to specify a name of volume group
as a provisioner of a StorageClass as follows:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-hdd
provisioner: topolvm.io/hdd
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
volumeBindingMode: WaitForFirstConsumer
---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner-ssd
provisioner: topolvm.io/ssd
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
volumeBindingMode: WaitForFirstConsumer
```

This proposal requires launching TopoLVM components for each provisioner.
Since `lvmd` is also launched per provisoner, it will continue to target only one volume group as before.

Pros:
- It doesn't require many changes to implement.
- It keeps compatibility.

Cons:
- Users will need a lot of work to launch multiple TopoLVMs.

### Decision Outcome

Choose options: [A) device class](#option-a-device-class),
because option B) is complicated to launch multiple TopoLVM for users.

## Design Details

### How to expose free storage capacity of nodes

Currently `topolvm-node` exposes free storage capacity as `capacity.topolvm.io/<deviec calss>` annotation of each Node as follows:

```yaml
kind: Node
metdta:
  name: wroker-1
  annotations:
    capacity.topolvm.io/ssd: "1073741824"
```

This proposal will change annotation to `capacity.topolvm.io/<device class>` as follows 
to expose the capacity of each node:

```yaml
kind: Node
metdta:
  name: wroker-1
  annotations:
    capacity.topolvm.io/ssd: "1073741824"
    capacity.topolvm.io/hdd: "1099511627776"
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

#### A-1) insert multiple resources

This proposal would insert `capacity.topolvm.io/<device class>` as follows:

```yaml
spec:
  containers:
  - name: testhttpd
    resources:
      requests:
        capacity.topolvm.io/ssd: "1073741824"
        capacity.topolvm.io/hdd: "1099511627776"
```

Then users should modify the scheduler policy as follows:

```json
{
    ...
    "extenders": [{
        "urlPrefix": "http://...",
        "filterVerb": "predicate/ssd",
        "prioritizeVerb": "prioritize/ssd",
        "managedResources":
        [{
          "name": "capacity.topolvm.io/ssd",
          "ignoredByScheduler": true
        }],
        "nodeCacheCapable": false
    },
    {
        "urlPrefix": "http://...",
        "filterVerb": "predicate/hdd",
        "prioritizeVerb": "prioritize/hdd",
        "managedResources":
        [{
          "name": "capacity.topolvm.io/hdd",
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

#### A-2) insert multiple annotations

This proposal would insert `topolvm.io/capacity` to resources and `capacity.topolvm.io/<device class>` annotation as follows:

```yaml
metdta:
  annotations:
    capacity.topolvm.io/ssd: "1073741824"
    capacity.topolvm.io/hdd: "1099511627776"
spec:
  containers:
  - name: testhttpd
    resources:
      requests:
        topolvm.io/capacity: "1"
```

The values of `topolvm.io/capacity` don't matter.

Users shouldn't modify the scheduler policy.

Currently, topolvm-scheduler calculates the score of a node by this formula:

    min(10, max(0, log2(capacity >> 30 / divisor)))

This proposal would calculate the score of each volume group by the above formula, 
and use the average of them as the final score.

Pros:
- No need to modify the scheduler policy depending on volume groups

Cons:
- Cannot calculate the score by specifying weight for each volume group

#### Decision outcome

Choose options: [A-2) insert multiple annotations](#a-2-insert-multiple-annotations),
because option A-1) is complicated to set scheduler policy. In most cases, option 2 works without problems.

### Setting of divisors

Also, the ConfigMap can contain `divisor` parameter for each volume group.
  
```yaml
apiVersion: v1
kind: ConfigMap
metaadta:
  name: topolvm-config
  namespace: topolvm-system
data:
  scheduler.yaml: |
    default-divisor: 10
    divisors:
      ssd: 5
      hdd: 10
```

### Ephemeral Volume

Ephemeral Volume doesn't have StorageClass.
However it can specify arbitrary values in `volumeAttributes`.

This proposal will allow to specify device class in `volumeAttributes`.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: ubuntu
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
  - name: ubuntu
    image: nginx4
    volumeMounts:
    - mountPath: /test1
      name: my-volume
  volumes:
  - name: my-volume
    csi:
      driver: topolvm.cybozu.com
      fsType: xfs
      volumeAttributes:
        topolvm.cybozu.com/size: "2"
        topolvm.cybozu.com/device-class: "hdd"
```


### Upgrade / Downgrade Strategy

Perform the following steps to upgrade:

1. Add `ConfigMap` resource for setting of divisors.
1. Replace `lvmd` binary and restart `lvmd.service`.
1. Update container images for TopoLVM.
1. Add the name of device class to StorageClass resources and ephemeral volumes. (optional)

If the name of device class in StorageClass Resources and ephemeral volums is empty,
`lvmd` will use the default name of volume group.
