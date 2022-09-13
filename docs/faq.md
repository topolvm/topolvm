Frequently Asked Questions
==========================

- [Why does `lvmd` run on the host OS?](#why-does-lvmd-run-on-the-host-os)
- [Why doesn't TopoLVM use extended resources?](#why-doesnt-topolvm-use-extended-resources)

## Why does `lvmd` run on the host OS?

Because LVM is not designed for containers.

For example, LVM commands need an exclusive lock to avoid conflicts.
If the same PV/VG/LV is shared between host OS and containers, the commands would conflict.
In the worst case, the metadata will be corrupted.

If `lvmd` can use storage devices exclusively, it might be able to create
PV/VG/LV using those devices in containers.  This option is not implemented, though.

## Why doesn't TopoLVM use extended resources?

Quick answer: Using extended resources prevents PVC from being resized.

[Extended resources](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#extended-resources) are a Kubernetes feature to allow users to define arbitrary resources consumed by Pods.

What is good in extended resources is that `kube-scheduler` takes them into account for Pod scheduling.
However, using extended resources to schedule pods onto nodes with sufficient capacity has several issues.

One problem is that the resource requests need to be copied from PVC to Pods.
For example, if a Pod has two PVC requesting 10 GiB and 20 GiB storage, the Pod should request 30 GiB storage capacity.

The biggest problem appears when PVC get resized.  Suppose that a node has 100 GiB storage capacity as an extended resource, and a Pod with PVC requesting 50 GiB of storage is scheduled to the node.  If PVC is resized to 80 GiB, the remaining storage becomes 20 GiB.

To keep track of the volume _usage_, the Pod should now request 80 GiB storage.  But this is impossible because `kube-apiserver` does not allow editing Pod resource requests.  As a consequence, `kube-scheduler` fails to notice the change in storage usage.

TopoLVM, on the other hand, keeps track of the volume _free_ capacity through annotations of nodes.
TopoLVM's extended scheduler `topolvm-scheduler` ignores the current usage.  It only cares if a node has sufficient _free_ capacity for new Pods.

## Why do I get exit code 5 on `lvcreate`?

Likely because you are using read only file system, and/or `/etc/lvm` is mounted
read-only.

To mitigate this you need to set the env `LVM_SYSTEM_DIR=/tmp` on the lvmd daemon.

In the helm chart you can do this with the following:

```yaml
lvmd:
  env:
    - name: LVM_SYSTEM_DIR
      value: /tmp
```
