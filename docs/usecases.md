Use cases
=========

It states procedures for use-cases.

Retiring node used by `StatefulSet` pods
----------------------------------------

1. Run `kubectl drain NODE --ignore-daemonsets=true`, `--ignore-daemonsets=true` avoids `topolvm-node` deletion.
2. Pods on the specified NODE are evicted. if the eviction is stacked, try to run `kubectl drain` with `--force`.
3. Re-scheduled Pods are `Pending` because PVCs for Pods still exist.
4. Run `kubectl delete nodes NODE`
5. Node finalizer deletes PVC and Pods. After finished, Node resource is deleted.
6. `StetefulSet` controller reschedules Pods and PVCs.

Rebooting node
--------------

1. Run `kubectl drain NODE --ignore-daemonsets=true`, `--ignore-daemonsets=true` avoids `topolvm-node` deletion.
2. Pods on the specified NODE are evicted. if the eviction is stacked, try to run `kubectl drain` with `--force`.
3. Re-scheduled Pods are `Pending` because PVCs for Pods still exist.
4. Reboot the node.
5. `LogicalVolume` and PVC exist, the data can be reused with a new pod of `StatefulSet`.
6. Run `kubectl uncordon NODE` after node is online.
7. A new pod is `Running` and existing PVC is used.
