topolvm-hook
============

`topolvm-hook` is a Kubernetes [mutating admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) for TopoLVM.

It mutate pod with pvc volume size, it's used as metric for `topolvm-shceduler`.

Detail of mutation
------------------

`topolvm-hook` provides `mutate` verbs for pod with its required pvc volume size. When original manifest of pod is as follows:

```yaml
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
  storageClassName: topolvm
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
      claimName: local-pvc1
```

then, `topolvm-hook` inserts `resource` filed to the first container in pod spec as follows:

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

When a pod refers multiple pvc, `topolvm-hook` calculate summation of volume sizes and inserts it.
Note that you MUST create storage class using TopoLVM provisioner, and use it in the pvc.

Command-line flags
------------------

|   Name   |  Type  | Default |            Description             |
| -------- | ------ | ------: | ---------------------------------- |
| `listen` | string | `:8443` | HTTPS listening address            |
| `cert`   | string |       - | path of certification file for TLS |
| `key`    | string |       - | path of private key file for TLS   |