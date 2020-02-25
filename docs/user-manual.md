User manual
===========

This is the user manual for TopoLVM.
For deployment, please read [../deploy/README.md](../deploy/README.md).

**Table of contents**

- [StorageClass](#storageclass)
- [Pod priority](#pod-priority)
- [Node maintenance](#node-maintenance)
  - [Retiring nodes](#retiring-nodes)
  - [Rebooting nodes](#rebooting-nodes)
- [Inline Ephemeral Volumes](#inline-ephemeral-volumes)
- [Limitations](#limitations)

StorageClass
------------

An example StorageClass looks like this:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner
provisioner: topolvm.cybozu.com
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

`provisioner` must be `topolvm.cybozu.com`.

`parameters` are optional.  To specify a filesystem type, give
`csi.storage.k8s.io/fstype` parameter.

Supported filesystems are: `ext4`, `xfs`, and `btrfs`.
TopoLVM sets the following mount options regardless of `mountOptions`.
- `lazytime` for all filesystems
- `ssd` for `btrfs`
- `wsync` for `xfs`

`volumeBindingMode` can be either `WaitForFirstConsumer` or `Immediate`.
`WaitForFirstConsumer` is recommended because TopoLVM cannot schedule pods
wisely if `volumeBindingMode` is `Immediate`.

`allowVolumeExpansion` enables CSI drivers to extend volumes.
This feature became beta in Kubernetes 1.16 and is not tested in 1.14 and 1.15.

Pod priority
------------

Pods using TopoLVM should always be prioritized over other normal pods.
This is because TopoLVM pods can only be scheduled to a single node where
its volumes exist whereas normal pods can be run on any node.

To give TopoLVM pods high priority, first create a [PriorityClass](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass):

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: topolvm
value: 1000000
globalDefault: false
description: "Pods using TopoLVM volumes should use this class."
```

and specify that PriorityClass in `priorityClassName` field as follows:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test
spec:
  priorityClassName: topolvm
  containers:
  - name: test
    image: nginx
    volumeMounts:
    - mountPath: /test1
      name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: topolvm-pvc
```

Node maintenance
----------------

These steps only apply when using the TopoLVM storage class and PVCs. If
only inline ephemeral volumes are used, these steps are not necessary.

### Retiring nodes

To remove a node and volumes/pods on the node from the cluster, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`.
    `--ignore-daemonsets=true` avoids `topolvm-node` deletion.
    If drain stacks due to PodDisruptionBudgets or something, try `--force` option.
2. Run `kubectl delete nodes NODE`
3. TopoLVM will remove Pods and PersistentVolumeClaims on the node.
4. `StatefulSet` controller reschedules Pods and PVCs on other nodes.

### Rebooting nodes

To reboot a node without removing volumes, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`
   `--ignore-daemonsets=true` avoids `topolvm-node` deletion.
2. Reboot the node.
3. Run `kubectl uncordon NODE` after the node comes back online.
4. After reboot, Pods will be rescheduled to the same node because PVCs remain intact.

Inline Ephemeral Volumes
------------------------

An example use of a TopoLVM inline ephemeral volume is as follows:

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
```

`driver` must be `topolvm.cybozu.com`.

`fsType` is optional.  To specify a filesystem type, give
`csi.storage.k8s.io/fstype` parameter. If no type is specified, the default
of ext4 will be used.
`volumeAttributes` are optional.  To specify a volume size, give
`topolvm.cybozu.com/size` parameter. If no size is specified, the default of
1 GiB will be used.

Supported filesystems are: `ext4`, `xfs`, and `btrfs`.

Limitations
-----------

See [limitations.md](limitations.md).
