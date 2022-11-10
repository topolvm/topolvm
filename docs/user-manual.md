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
- [Generic ephemeral volumes](#generic-ephemeral-volumes)
- [Other documents](#other-documents)

StorageClass
------------

An example StorageClass looks like this:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm-provisioner
provisioner: topolvm.io
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.io/device-class": "ssd"
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

`provisioner` must be `topolvm.io`.

`parameters` are optional.
To specify a filesystem type, give `csi.storage.k8s.io/fstype` parameter.
To specify a device-class name to be used, give `topolvm.io/device-class` parameter. 
If no `topolvm.io/device-class` is specified, the default device-class is used.

Supported filesystems are: `ext4` and `xfs`.

`volumeBindingMode` can be either `WaitForFirstConsumer` or `Immediate`.
`WaitForFirstConsumer` is recommended because TopoLVM cannot schedule pods
wisely if `volumeBindingMode` is `Immediate`.

`allowVolumeExpansion` enables CSI drivers to expand volumes.
This feature is available for Kubernetes 1.16 and later releases.

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
only generic ephemeral volumes are used, these steps are not necessary.

### Retiring nodes

To remove a node and volumes/pods on the node from the cluster, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`.
    `--ignore-daemonsets=true` allows the command to succeed even if pods managed by daemonset exist (e.g. topolvm-node).
    If drain stacks due to PodDisruptionBudgets or something, try `--force` option.
2. Run `kubectl delete nodes NODE`
3. TopoLVM will remove Pods and PersistentVolumeClaims on the node.
4. `StatefulSet` controller reschedules Pods and PVCs on other nodes.

### Rebooting nodes

To reboot a node without removing volumes, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`.
   `--ignore-daemonsets=true` allows the command to succeed even if pods managed by daemonset exist (e.g. topolvm-node).
2. Reboot the node.
3. Run `kubectl uncordon NODE` after the node comes back online.
4. After reboot, Pods will be rescheduled to the same node because PVCs remain intact.

Generic ephemeral volumes
----------------

TopoLVM supports the generic ephemeral volume feature.  
You can use generic ephemeral volumes with TopoLVM like the following:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod-ephemeral
  namespace: default
  labels:
    app.kubernetes.io/name: my-pod-ephemeral
    app: example
spec:
  containers:
  - name: ubuntu
    image: quay.io/cybozu/ubuntu:20.04
    command: ["/usr/local/bin/pause"]
    volumeMounts:
    - mountPath: /test1
      name: my-volume
  volumes:
  - name: my-volume
    ephemeral:
      volumeClaimTemplate:
        spec:
          accessModes:
          - ReadWriteOnce
          resources:
            requests:
              storage: 1Gi
          storageClassName: topolvm-provisioner
```

By using the generic ephemeral volume function, you can use any CSI driver in the container to utilize the temporary volume.  
TopoLVM also schedules based on the capacity used by the generic ephemeral volumes.  
When using generic ephemeral volumes, the following processing is performed:

- When applying a Pod with a generic ephemeral volume, the ephemeralController in kube-controller-manager creates a PVC for the generic ephemeral volume (that is, it works the same as creating a regular PVC).
- When the pod with a generic ephemeral volume is deleted, the PVC is also deleted at the same time because if PVC is created as a generic ephemeral volume, the PVC's OwnerReference is set to the Pod associated.
- Since the deletion of PVs and real volumes associated with PVCs depends on ReclaimPolicy setting of StorageClass, StorageClass used in generic ephemeral volume must be set to `reclaimPolicy=Delete` if you want to delete PVs and real volumes associated when deleting the Pod.

You can find out more about generic ephemeral volume feature [here](https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/1698-generic-ephemeral-volumes).

Other documents
---------------

- [Limitations](limitations.md).
- [Frequently Asked Questions](faq.md).
- [Monitoring with Prometheus](prometheus.md).
