topolvm-controller
==================

`topolvm-controller` provides a CSI controller service.  It also works as
a custom Kubernetes controller to cleanup stale resources.

Specifically, `topolvm-controller` watches `Node` resource deletion to
cleanup `PersistentVolumeClaim` on the deleting Nodes.

CSI controller features
-----------------------

`topolvm-controller` implements following optional features:

- [`CREATE_DELETE_VOLUME`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#createvolume) to support dynamic volume provisioning
- [`GET_CAPACITY`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#getcapacity)
- [`EXPAND_VOLUME`](https://github.com/container-storage-interface/spec/blob/v1.1.0/spec.md#controllerexpandvolume)

Webhooks
--------

`topolvm-controller` implements two webhooks:

### `/pod/mutate`

Mutate new Pods to add `capacity.topolvm.io/<volume group name>` resource request to
its first container.  This resource request will be used by
[`topolvm-scheduler`](./topolvm-scheduler.md) to filter and score Nodes.

This hook handles two classes of pods. First, pods having at least one _unbound_
PersistentVolumeClaim (PVC) for TopoLVM and _no_ bound PVC for TopoLVM. Second,
pods which have at least one inline ephemeral volume which specify using the CSI driver
type `topolvm.cybozu.com`.

For both PVCs and inline ephemeral volumes,the requested storage size for the
volume is calculated as follows:
- if the volume has no storage request, the size will be treated as 1 GiB.
- if the volume has storage request, the size will be rounded up to GiB unit.

The value of the resource request is the sum of rounded storage size
of unbound PVCs for TopoLVM.

The following manifest exemplifies usage of TopoLVM PVCs:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: topolvm
provisioner: topolvm.cybozu.com            # topolvm-scheduler works only for StorageClass with this provisioner.
parameters:
  "csi.storage.k8s.io/fstype": "xfs"
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
  name: testhttpd
  namespace: hook-test
  labels:
    app.kubernetes.io/name: testhttpd
spec:
  containers:
  - name: testhttpd
    image: quay.io/cybozu/testhttpd:0
    volumeMounts:
    - mountPath: /test1
      name: my-volume1
  volumes:
  - name: my-volume1
    persistentVolumeClaim:
      claimName: local-pvc1                # have the above PVC
```

The hook inserts `capacity.topolvm.io/<volume group name>` to the first container as follows:

```yaml
spec:
  containers:
  - name: testhttpd
    resources:
      limits:
        capacity.topolvm.io/myvg1: "1073741824"
      requests:
        capacity.topolvm.io/myvg1: "1073741824"
```

Below is an example for TopoLVM inline ephemeral volumes:
```yaml
kind: Pod
metadata:
  name: ubuntu
  labels:
    app.kubernetes.io/name: ubuntu
spec:
  containers:
  - name: ubuntu
    image: quay.io/cybozu/ubuntu:18.04
    command: ["/usr/local/bin/pause"]
    volumeMounts:
    - mountPath: /test1
      name: my-volume
  volumes:
  - name: my-volume
    csi:
      driver: topolvm.cybozu.com
```

The hook inserts `capacity.topolvm.io/<volume group name>` to the ubuntu container as follows:

```yaml
spec:
  containers:
  - name: ubuntu
    resources:
      limits:
        capacity.topolvm.io/myvg1: "1073741824"
```

### `/pvc/mutate`

Mutate new PVCs to add `topolvm.cybozu.com/pvc` finalizer.

Controllers
-----------

### Node finalizer

`topolvm-metrics` adds `topolvm.cybozu.com/node` finalizer.

When a Node is being deleted, the controller deletes all PVCs for TopoLVM
on the deleting node.

### PVC finalizer

When a PVC for TopoLVM is being deleted, the controller waits for other
finalizers to be completed.  Once it becomes the last finalizer, it removes
the finalizer to immediately delete PVC then deletes pending pods referencing
the deleted PVC, if any.

### Delete stale LogicalVolumes

[`LogicalVolume`](./crd-logical-volume.md) may be left without completing
its finalization when the node dies.

To delete such LogicalVolumes, the controller deletes them periodically by
running finalization by on behalf of `topolvm-node`.

By default, it deletes LogicalVolumes whose deletionTimestamp is behind `24h`
from the current time every `cleanup-interval` which is `10m`.

Command-line flags
------------------

| Name                 | Type     | Default                                 | Description                                                   |
| -------------------- | -------- | --------------------------------------- | ------------------------------------------------------------- |
| `cert-dir`           | string   | `/tmp/k8s-webhook-server/serving-certs` | Directory for `tls.crt` and `tls.key` files.                  |
| `cleanup-interval`   | Duration | `10m`                                   | Cleaning up interval for `LogicalVolume`.                     |
| `csi-socket`         | string   | `/run/topolvm/csi-topolvm.sock`         | UNIX domain socket of `topolvm-controller`.                   |
| `metrics-addr`       | string   | `:8080`                                 | Listen address for Prometheus metrics.                        |
| `leader-election-id` | string   | `topolvm`                               | ID for leader election by controller-runtime.                 |
| `stale-period`       | Duration | `24h`                                   | Deleting LogicalVolume is considered stale after this period. |
| `webhook-addr`       | string   | `:8443`                                 | Listen address for the webhook endpoint.                      |
