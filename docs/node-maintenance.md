# Node Maintenance

These steps only apply when using the TopoLVM StorageClass and PVCs.
If only generic ephemeral volumes are used, these steps are not necessary.

## Retiring Nodes

To remove a node and volumes/pods on the node from the cluster, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`.

    `--ignore-daemonsets=true` allows the command to succeed even if pods managed by Daemonset exist (e.g. topolvm-node).
    If drain stacks due to PodDisruptionBudgets or something, try `--force` option.

2. Run `kubectl delete nodes NODE`
3. TopoLVM will remove PersistentVolumeClaims on the node.
4. `StatefulSet` controller reschedules Pods and PVCs on other nodes.

## Rebooting Nodes

To reboot a node without removing volumes, follow these steps:

1. Run `kubectl drain NODE --ignore-daemonsets=true`.

   `--ignore-daemonsets=true` allows the command to succeed even if pods managed by Daemonset exist (e.g. topolvm-node).

2. Reboot the node.
3. Run `kubectl uncordon NODE` after the node comes back online.
4. After reboot, Pods will be rescheduled to the same node because PVCs remain intact.
