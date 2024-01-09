# topolvm-controller

`topolvm-controller` provides a CSI controller service.  It also works as
a custom Kubernetes controller to cleanup stale resources.

Specifically, `topolvm-controller` watches `Node` resource deletion to
cleanup `PersistentVolumeClaim` on the deleting Nodes.

## CSI Controller Features

`topolvm-controller` implements following optional features:

- [`CREATE_DELETE_VOLUME`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#createvolume) to support dynamic volume provisioning
- [`GET_CAPACITY`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#getcapacity)
- [`EXPAND_VOLUME`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#controllerexpandvolume)

## Webhooks

`topolvm-controller` implements two webhooks:

### `/pod/mutate`

Mutate new Pods to add `capacity.topolvm.io/<device-class>` annotations to the pod
and `topolvm.io/capacity` resource request to its first container.
These annotations and the resource request will be used by
[`topolvm-scheduler`](./topolvm-scheduler.md) to filter and score Nodes.

This hook handles two classes of pods. First, pods having at least one _unbound_
PersistentVolumeClaim (PVC) for TopoLVM and _no_ bound PVC for TopoLVM. Second,
pods which have at least one generic ephemeral volume which specify using the StorageClass of TopoLVM.

For both PVCs and generic ephemeral volumes, the requested storage size for the
volume is calculated as follows:
- if the volume has no storage request, the size will be treated as 1 GiB.
- if the volume has storage request, the size is as is.

The value of the resource request is the sum of storage sizes
of unbound PVCs for TopoLVM.

The following manifest exemplifies usage of TopoLVM PVCs:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm
provisioner: topolvm.io            # topolvm-scheduler works only for StorageClass with this provisioner.
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
  "topolvm.io/device-class": "ssd"
volumeBindingMode: WaitForFirstConsumer
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: local-pvc1
  namespace: hook-test
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm                # reference the above StorageClass
---
apiVersion: v1
kind: Pod
metadata:
  name: pause
  namespace: hook-test
  labels:
    app.kubernetes.io/name: pause
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause
    volumeMounts:
    - mountPath: /test1
      name: my-volume1
  volumes:
  - name: my-volume1
    persistentVolumeClaim:
      claimName: local-pvc1                # have the above PVC
```

The hook inserts `capacity.topolvm.io/<device-class>` to the annotations
and `topolvm.io/capacity` to the first container as follows:

```yaml
metadata:
  annotations:
    capacity.topolvm.io/ssd: "1073741824"
spec:
  containers:
  - name: pause
    resources:
      limits:
        topolvm.io/capacity: "1"
      requests:
        topolvm.io/capacity: "1"
```

If the specified StorageClass does not have `topolvm.io/device-class` parameter,
it will be annotated with `capacity.topolvm.io/00default`.

Below is an example for TopoLVM generic ephemeral volumes:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pause
  labels:
    app.kubernetes.io/name: pause
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause
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
            storageClassName: topolvm # reference the above StorageClass
```

The hook inserts `capacity.topolvm.io/<device-class>` to the annotations
and `topolvm.io/capacity` to the first container as follows:

```yaml
metadata:
  annotations:
    capacity.topolvm.io/ssd: "1073741824"
spec:
  containers:
  - name: ubuntu
    resources:
      limits:
        topolvm.io/capacity: "1"
      requests:
        topolvm.io/capacity: "1"
```

### `/pvc/mutate`

Mutate new PVCs to add `topolvm.io/pvc` finalizer. This finalizer is required to delete a pod in the following scenario.

1. StatefulSet pod is deleted by `kubectl drain`. PVC is remained.
2. A pod is recreated by the StatefulSet controller but not scheduled for some reasons.
3. Delete a node resource on which the pod was running.
4. PVC related to the node is deleted by the TopoLVM controller.

At step 4, the StatefulSet pod is not deleted if the PVC finalizer does not exist.

## Controllers for Kubernetes Objects

### The Controller for Nodes

The controller adds `topolvm.io/node` finalizer.

When a Node is being deleted, the controller deletes all PVCs and LogicalVolumes for TopoLVM
on the deleting node. 

This node finalize procedure may be skipped with the `--skip-node-finalize` flag. 
When this is true, the PVCs and the LogicalVolume CRs from a deleted node must be
deleted manually by a cluster administrator.

### The Controller for PersistentVolumeClams

When a PVC for TopoLVM is being deleted, the controller waits for other
finalizers to be completed.  Once it becomes the last finalizer, it removes
the finalizer to immediately delete PVC then deletes pending pods referencing
the deleted PVC, if any.

Command-line flags
------------------

| Name                   | Type   | Default                                 | Description                                                                  |
| ---------------------- | ------ | --------------------------------------- | ---------------------------------------------------------------------------- |
| `cert-dir`             | string | `/tmp/k8s-webhook-server/serving-certs` | Directory for `tls.crt` and `tls.key` files.                                 |
| `csi-socket`           | string | `/run/topolvm/csi-topolvm.sock`         | UNIX domain socket of `topolvm-controller`.                                  |
| `metrics-bind-address` | string | `:8080`                                 | Listen address for Prometheus metrics.                                       |
| `leader-election-id`   | string | `topolvm`                               | ID for leader election by controller-runtime.                                |
| `webhook-addr`         | string | `:9443`                                 | Listen address for the webhook endpoint.                                     |
| `skip-node-finalize`   | bool   | `false`                                 | When true, skips automatic cleanup of PhysicalVolumeClaims on Node deletion. |
