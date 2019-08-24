topolvm-hook
============

`topolvm-hook` is a Kubernetes [mutating admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) for TopoLVM.

It mutates pods having PersistentVolumeClaims for TopoLVM to add resource requirements in its first container.
The added resource will be referenced by `topolvm-scheduler` to choose and score nodes for the pod.

Mutation example
----------------

Suppose the following manifest is to be applied:

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

`topolvm-hook` inserts `topolvm.cybozu.com/capacity` resources to the first container as follows:

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

Specifications
--------------

`topolvm-hook` ignores pods having PVCs already bound to a TopoLVM volume.

The requested storage size of a PVC is calculated as follows:
- if PVC has no storage request, the size will be treated as 1 GiB.
- if PVC has storage request, the size will be rounded up to GiB unit.

When a pod has multiple unbound PVC for TopoLVM, `topolvm-hook` totals the rounded up size of each PVC.

Command-line flags
------------------

| Name             | Type   | Default                                 | Description                                  |
| ---------------- | ------ | --------------------------------------- | -------------------------------------------- |
| `--cert-dir`     | string | `/tmp/k8s-webhook-server/serving-certs` | Directory for `tls.crt` and `tls.key` files. |
| `--metrics-addr` | string | `:8080`                                 | Listen address for metrics.                  |
| `--webhook-addr` | string | `:8443`                                 | Listen address for the webhook endpoint.     |
