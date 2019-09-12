User manual
===========

This is the user manual for TopoLVM.
For deployment, please read [../deploy/README.md](../deploy/README.md).

**Table of contents**

- [StorageClass](#storageclass)
- [Node maintenance](#node-maintenance)
  - [Retiring nodes](#retiring-nodes)
  - [Rebooting nodes](#rebooting-nodes)

StorageClass
------------

(TBD)

Node maintenance
----------------

### Retiring nodes

To remove a Node safely from the cluster, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`.
    `--ignore-daemonsets=true` avoids `topolvm-node` deletion.
    If drain stacks due to PodDisruptionBudgets or something, try `--force` option.
2. Run `kubectl delete nodes NODE`
3. TopoLVM will remove Pods and PersistentVolumeClaims for `Node`.
4. `StetefulSet` controller reschedules Pods and PVCs.

### Rebooting nodes

1. Run `kubectl drain NODE --ignore-daemonsets=true`
   `--ignore-daemonsets=true` avoids `topolvm-node` deletion.
2. Reboot the node.
3. Run `kubectl uncordon NODE` after node is online.   
4. After reboot, Pods will be rescheduled to the same node because PVCs remain intact.
