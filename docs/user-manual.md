User manual
===========

This is the user manual for TopoLVM.
For deployment, please read [../deploy/README.md](../deploy/README.md).

**Table of contents**

- [StorageClass](#storageclass)
- [Node maintenance](#node-maintenance)
  - [Retiring nodes](#retiring-nodes)
  - [Rebooting nodes](#rebooting-nodes)
- [Limitations](#limitations)
- [Trouble shooting](#trouble-shooting)
  - [StatefulSet pod is not rescheduled after node deletion](#statefulset-pod-is-not-rescheduled-after-node-deletion)

StorageClass
------------

(TBD)

Node maintenance
----------------

### Retiring nodes

To remove a node and volumes/pods on the node from the cluster, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`.
    `--ignore-daemonsets=true` avoids `topolvm-node` deletion.
    If drain stacks due to PodDisruptionBudgets or something, try `--force` option.
2. Run `kubectl delete nodes NODE`
3. TopoLVM will remove Pods and PersistentVolumeClaims on the node.
4. `StatefulSet` controller reschedules Pods and PVCs on other nodes.

### Rebooting nodes

To reboot a node without removing volumes, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`
   `--ignore-daemonsets=true` avoids `topolvm-node` deletion.
2. Reboot the node.
3. Run `kubectl uncordon NODE` after the node comes back online.
4. After reboot, Pods will be rescheduled to the same node because PVCs remain intact.

Limitations
-----------

See [limitations.md](limitations.md).

Trouble shooting
---------------

### StatefulSet pod is not rescheduled after node deletion

If you forget to mark the deleting node unschedulable by either
`kubectl drain NODE` or `kubectl cordon NODE`, a pending pod may remain
unschedulable.

The reason and how to recover from this are described in [limitations.md](limitations.md).
