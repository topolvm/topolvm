# Demonstration with Kind

This directory contains scripts to run TopoLVM in a demonstration environment.
It uses [kind](https://github.com/kubernetes-sigs/kind) to run Kubernetes
and loopback block devices to run `lvmd`.

You can try to use TopoLVM with a specific tag as follows. The demonstration is not guaranteed to work correctly with the main branch.

```console
$ git checkout topolvm-chart-v15.7.0
```

To start the demonstration environment, run the following commands:

```console
$ make setup
$ make run
```

LVM logical volumes will be created and bound with a PersistentVolumeClaim as follows:

```console
$ kubectl get pvc
NAME                         STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS               AGE
my-pod-ephemeral-my-volume   Bound    pvc-c2bf0862-e976-4ebc-a404-75b18589020f   1Gi        RWO            topolvm-provisioner        73s
topolvm-pvc                  Bound    pvc-8e1d85b1-b563-4e2d-9b20-d806fb51bb54   1Gi        RWO            topolvm-provisioner        73s
topolvm-pvc-thin             Bound    pvc-ec55e53b-2b6c-455d-a44c-22bec00049cd   10Gi       RWO            topolvm-provisioner-thin   73s

$ kubectl get pv
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM                                STORAGECLASS               REASON   AGE
pvc-8e1d85b1-b563-4e2d-9b20-d806fb51bb54   1Gi        RWO            Delete           Bound    default/topolvm-pvc                  topolvm-provisioner                 99s
pvc-c2bf0862-e976-4ebc-a404-75b18589020f   1Gi        RWO            Delete           Bound    default/my-pod-ephemeral-my-volume   topolvm-provisioner                 97s
pvc-ec55e53b-2b6c-455d-a44c-22bec00049cd   10Gi       RWO            Delete           Bound    default/topolvm-pvc-thin             topolvm-provisioner-thin            98s

$ sudo lvscan
  ...
  ACTIVE            '/dev/myvg1/thinpool' [12.00 GiB] inherit
  ACTIVE            '/dev/myvg1/4596133d-ffdd-40ab-af18-88520e886e98' [1.00 GiB] inherit
  ACTIVE            '/dev/myvg1/dd21065d-ed1f-48c6-aa33-167370c2c58f' [10.00 GiB] inherit
  ACTIVE            '/dev/myvg1/282c490c-b4db-4cd4-a046-4e52a435df5a' [1.00 GiB] inherit
  ...
```

To stop the demonstration environment, run:

```console
$ make clean
```
