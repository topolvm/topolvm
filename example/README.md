Demonstration with kind
=======================

You can see a demonstration of how TopoLVM provisioner works with the following commands. This demonstration using [kind](https://github.com/kubernetes-sigs/kind) and loopback device on your host.
```console
$ cd example
$ make setup run
```

TopoLVM provisions an LVM logical volume and bind it with a PersistentVolumeClaim as follows:
```console
$ kubectl get pvc
% NAME          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
% topolvm-pvc   Bound    pvc-05df10d2-b7ee-11e9-8da2-0242ac110002   1Gi        RWO            topolvm-provisioner   23m

$ kubectl get pv
% NAME CAPACITY ACCESS MODES RECLAIM POLICY STATUS CLAIM STORAGECLASS REASON AGE
% pvc-05df10d2-b7ee-11e9-8da2-0242ac110002 1Gi RWO Delete Bound topolvm-system/topolvm-pvc topolvm-provisioner 25m

$ sudo lvscan
% ACTIVE '/dev/myvg/05e33db5-b7ee-11e9-8da2-0242ac110002' [1.00 GiB] inherit
```

Clean up the generated files.
```console
$ make clean
```
