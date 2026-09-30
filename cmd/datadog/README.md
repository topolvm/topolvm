# Datadog downstream commands

Commands below this directory are Datadog-specific downstream integrations.
They are not part of upstream-supported TopoLVM functionality and are not
intended for upstream submission.

The capacity-template controller is maintained on the downstream `datadog`
branch. Its capacity matrix mirrors `GetRemoteLVMCapacity` from the
k8s-nodegroups remote TopoLVM implementation, including `diskCount`,
`raidLevel`, whole-GiB cloud-disk rounding, and 8 MiB of LVM overhead per
resulting PV. Re-check that matrix whenever NodeGroup storage preparation
changes.

## Capacity-template controller configuration

The controller reads one configuration file, for example:

```yaml
capacityTemplates:
  - storageClassName: ephemeral-remote-data
    deviceClassName: remote-ssd
    spareGB: 0
    handler: remote-lvm
```

Each entry is reconciled independently for every NodeGroup and produces at most
one `CSIStorageCapacity`. An entry is published only when its StorageClass
exists, uses the `topolvm.io` provisioner with `WaitForFirstConsumer`, and has
`topolvm.io/device-class` equal to `deviceClassName`, and when its handler
reports capacity for the NodeGroup. Otherwise that entry's object is deleted
without affecting other entries. Removing an entry from the file deletes its
objects on the next reconcile.

Constraints:

- `storageClassName` must be unique across entries.
- `handler` defaults to `remote-lvm`. `remote-lvm` is the only implemented
  handler; any other value is rejected at startup. A future `local-lvm`
  handler would be added to the handler registry in
  `internal/datadog/capacitytemplate/handler.go`.
- Unknown keys in the file are rejected.

## Capacity-template controller rollback

Scale or remove the controller Deployment first, then remove only its dynamic
objects with the exact ownership label:

```sh
kubectl delete csistoragecapacities.storage.k8s.io --all-namespaces \
  -l storage.datadoghq.com/managed-by=datadog-topolvm-capacity-template-controller
```
