Design notes
============

Motivation
----------

To run software such as MySQL or Elasticsearch, it would be nice to use
local fast storages and form a cluster to replicate data between servers.

TopoLVM provides a storage driver for such software running on Kubernetes.

Goals
-----

- Use LVM for flexible volume capacity management.
- Enhance the scheduler to prefer nodes having a larger storage capacity.
- Support dynamic volume provisioning from PVC.
- Support volume resizing (resizing for CSI becomes beta in Kubernetes 1.16).

### Future goals

- Prefer nodes with less IO usage.
- Support volume snapshot.

Components
----------

- `topolvm-controller`: CSI controller service.
- `topolvm-scheduler`: A [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for TopoLVM.
- `topolvm-node`: CSI node service.
- `lvmd`: gRPC service to manage LVM volumes.

### Diagram

- ![#FFFF00](https://placehold.it/15/FFFF00/000000?text=+) TopoLVM components
- ![#ADD8E6](https://placehold.it/15/ADD8E6/000000?text=+) Kubernetes components
- ![#FFC0CB](https://placehold.it/15/FFC0CB/000000?text=+) Kubernetes CSI Sidecar containers

![component diagram](http://www.plantuml.com/plantuml/svg/bPH1Zzey48Rl-HMZx0KEQ8MFU_YqNqDlj8f0fQgs74my2XQE7TaEswhQ_rudSOm9i7Gl26OUZzztFCEpiLJRfX99JOi3BH7I1TP2_QvGsXJ-900lcP9MAo5Gvw8fkTm2DP1QLIjnh6P5oARmy0E5KA_j8MCCPrXGRNgyC7nMQtNaXYk9-gTi0zHQMkoxapcNX-GjIQHY25_TnxmxrdrhTRYQWyB5kiyjA5PAhj7hT9UsTAznVYwohHhazImB0kSdXHfBRgocGH60q-JeGxD3WTQZ_fU3bhpSsq-YmOvQRhumZxXRMRZnp1W9niY5CNBN6Fc0CVBlniXzO-GzOtv6Sa4bzfVs0QZRY1yauzwQDGBrDguFmAYbEseGKbfpW_g8EbOmD25RBNe9IrNoWegD4as54nUUZbgGRxAUp54RvnkbxU5CK5wb7b9iXKOrka0FAsVirEyXETz6atYP9gSqQTDlhYTXcGooRfiS4YyMJ9I6yChJeJrUntfe4tp-vQIpTbicmuE77axF7Y6QV9WrzUm_EBC0J_2_bCfIY-VeoyDEDBXbwbMwCztyEhPSvLaoZ7o0XhBXzDCxNBHUdeiY0OthXOiZXUIA6NBT3BbcXepCa5jcxe3HJbsuEQ5N2oRZlq-OUO5kS1sKQNH67hzJM-nli-FN_01E0XvDlE_hHG67ucwlr64yKLwlB_NuhMRZzj-4aZ2pdZ5jBFGlY7PR6wH6wT3TIxM-nSyeMLE9lm00)

Blue arrows in the diagram indicate communications over unix domain sockets.
Red arrows indicate communications over TCP sockets.

Architecture
------------

TopoLVM is a storage plugin based on [CSI](https://github.com/container-storage-interface/spec/).
Therefore, the architecture basically follows the one described in
https://kubernetes-csi.github.io/docs/ .

To manage LVM, `lvmd` should be run as a system service of the node OS.
It provides gRPC services via UNIX domain socket to create/update/delete
LVM logical volumes and watch a volume group status.

`topolvm-node` implements CSI node services as well as miscellaneous control
on each Node.  It communicates with `lvmd` to watch changes in free space
of a volume group and exports the information by annotating Kubernetes
`Node` resource of the running node.  In the mean time, it adds a finalizer
to the `Node` to cleanup PersistentVolumeClaims (PVC) bound on the node.

`topolvm-node` also works as a custom Kubernetes controller to implement
dynamic volume provisioning.  Details are described in the following sections.

`topolvm-controller` implements CSI controller services.  It also works as
a custom Kubernetes controller to implement dynamic volume provisioning and
resource cleanups.

`topolvm-scheduler` is a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) to extend the
standard Kubernetes scheduler for TopoLVM.

### How the scheduler extension works

To extend the standard scheduler, TopoLVM components work together as follows:

- `topolvm-node` exposes free storage capacity as `topolvm.cybozu.com/capacity` annotation of each Node.
- `topolvm-controller` works as a [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) for new Pods.
    - It adds `topolvm.cybozu.com/capacity` resource to the first container of a pod.
    - The value is the sum of the storage capacity requests of all unbound TopoLVM PVC referenced by the pod.
- `topolvm-scheduler` filters and scores Nodes for a new pod having `topolvm.cybozu.com/capacity` resource request.
    - Nodes having less capacity than requested are filtered.
    - Nodes having larger capacity are scored higher.

### How dynamic provisioning works

To support dynamic volume provisioning, CSI controller service need to create a
logical volume on remote target nodes.  In general, CSI controller runs on a
different node from the target node of the volume.  To allow communication
between CSI controller and the target node, TopoLVM uses a custom resource
called `LogicalVolume`.

Dynamic provisioning depends on [CSI `external-provisioner`](https://kubernetes-csi.github.io/docs/external-provisioner.html) sidecar container.

1. `external-provisioner` finds a new unbound PersistentVolumeClaim (PVC) for TopoLVM.
2. `external-provisioner` calls CSI controller's `CreateVolume` with the topology key of the target node.
3. `topolvm-controller` creates a `LogicalVolume` with the topology key and capacity of the volume.
4. `topolvm-node` on the target node finds the `LogicalVolume`.
5. `topolvm-node` sends a volume create request to `lvmd`.
6. `lvmd` creates an LVM logical volume as requested.
7. `topolvm-node` updates the status of `LogicalVolume`.
8.  `topolvm-controller` finds the updated status of `LogicalVolume`.
9.  `topolvm-controller` sends the success (or failure) to `external-provisioner`.
10. `external-provisioner` creates a PersistentVolume (PV) and binds it to the PVC.

Limitations
-----------

TopoLVM depends on Kubernetes deeply.
Portability to other container orchestrators (CO) is not considered.

Packaging and deployment
------------------------

`lvmd` is provided as a single executable.
Users needs to deploy `lvmd` manually by themselves.

Other components, as well as CSI sidecar containers, are provided in a single
Docker container image, and is deployed as Kubernetes objects.
