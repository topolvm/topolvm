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
- [Other documents](#other-documents)

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
  "topolvm.cybozu.com/device-class": "ssd"
volumeBindingMode: WaitForFirstConsumer
allowVolumeExpansion: true
```

`provisioner` must be `topolvm.cybozu.com`.

`parameters` are optional.
To specify a filesystem type, give `csi.storage.k8s.io/fstype` parameter.
To specify a device-class name to be used, give `topolvm.cybozu.com/device-class` parameter. 
If no `topolvm.cybozu.com/device-class` is specified, the default device-class is used.

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

### Generic Ephemeral Volume

TopoLVM supports the Generic Ephemeral Volume feature.  
You can use Generic Ephemeral Volumes with TopoLVM like following:

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

By using the Generic Ephemeral Volume function, you can use any CSI driver in the container to utilize the temporary volume.  
TopoLVM schedules also based on the capacity used by the Generic Ephemeral Volumes.  
When using Generic Ephemeral Volumes, the following processing is performed:

- When applying a Pod with a Generic Ephemeral Volume, the ephemeralController in kube-controller-manager creates a PVC for the Generic Ephemeral Volume(that is, it works the same as creating a regular PVC).
- When the pod with a Generic Ephemeral Volume is deleted, the PVC is also deleted at the same time because if PVC is created as a Generic Ephemeral Volume, the PVC's OwnerReference is set to the Pod associated.
- Since the deletion of PVs and real volumes associated with PVCs depends on ReclaimPolicy setting of StorageClass, StorageClass used in Generic Ephemeral Volume must be set to `reclaimPolicy=Delete` if you want to delete PVs and real volumes associated when delete the Pod.

You can find out more about Generic Ephemeral Volume feature [here](https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/1698-generic-ephemeral-volumes).

Other documents
---------------

- [Limitations](limitations.md).
- [Frequently Asked Questions](faq.md).
- [Monitoring with Prometheus](prometheus.md).
