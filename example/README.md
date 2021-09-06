Demonstration with kind
=======================

This directory contains scripts to run TopoLVM in a demonstration environment.
It uses [kind](https://github.com/kubernetes-sigs/kind) to run Kubernetes
and loopback block devices to run `lvmd`.

You can try TopoLVM using the example with tag like following command. The main branch is being edited, so it may not work.

```
$ git checkout v0.9.1
```

To start the demonstration environment, run the following commands:

```console
$ make setup
$ make run
```

An LVM logical volume will be created and bound with a PersistentVolumeClaim as follows:

```console
$ kubectl get pvc
% NAME          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
% topolvm-pvc   Bound    pvc-05df10d2-b7ee-11e9-8da2-0242ac110002   1Gi        RWO            topolvm-provisioner   23m

$ kubectl get pv
% NAME CAPACITY ACCESS MODES RECLAIM POLICY STATUS CLAIM STORAGECLASS REASON AGE
% pvc-05df10d2-b7ee-11e9-8da2-0242ac110002 1Gi RWO Delete Bound topolvm-system/topolvm-pvc topolvm-provisioner 25m

$ sudo lvscan
% ACTIVE '/dev/myvg1/05e33db5-b7ee-11e9-8da2-0242ac110002' [1.00 GiB] inherit
```

To stop the demonstration environment, run:

```console
$ make clean
```

If you're not on a Linux machine, we ship a _Vagrantfile_ which sets up a Linux VM using [Vagrant](https://www.vagrantup.com/).
It requires [VirtualBox](https://www.virtualbox.org/) and the [vagrant-disksize](https://github.com/sprotheroe/vagrant-disksize) plugin.
Once Vagrant is setup, add the _vagrant-disksize_ plugin:
```console
$ vagrant plugin install vagrant-disksize
```
and bring your VM up
```console
$ vagrant up
$ vagrant ssh
$ cd /vagrant/example
```
Next, run the example as suggested. However, as Vagrant shares the host directory with the virtual machine, you need to specify where the
your volume will be created. In order to do it, just override the `BACKING_STORE` variable. For example:
```
$ make setup
$ make BACKING_STORE=/tmp run
```
Next, follow the steps previously highlighted. Once you're done with your demonstration environment, logout from your VM and run:
```console
$ vagrant destroy
```
to clean up your environment.
