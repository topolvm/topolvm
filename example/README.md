Demonstration with kind
=======================

This directory contains scripts to run TopoLVM in a demonstration environment.
It uses [kind](https://github.com/kubernetes-sigs/kind) to run Kubernetes
and loopback block devices to run `lvmd`.

To start the demonstration environment, run the following commands:

```console
$ make setup
$ make run
```

An LVM logical volume will be created and bound with a PersistentVolumeClaim as follows:

```console
$ export KUBECONFIG=$(kind get kubeconfig-path)
$ kubectl get pvc
% NAME          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
% topolvm-pvc   Bound    pvc-05df10d2-b7ee-11e9-8da2-0242ac110002   1Gi        RWO            topolvm-provisioner   23m

$ kubectl get pv
% NAME CAPACITY ACCESS MODES RECLAIM POLICY STATUS CLAIM STORAGECLASS REASON AGE
% pvc-05df10d2-b7ee-11e9-8da2-0242ac110002 1Gi RWO Delete Bound topolvm-system/topolvm-pvc topolvm-provisioner 25m

$ sudo lvscan
% ACTIVE '/dev/myvg/05e33db5-b7ee-11e9-8da2-0242ac110002' [1.00 GiB] inherit
```

To stop the demonstration environment, run:

```console
$ make clean
```
