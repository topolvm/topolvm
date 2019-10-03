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
- `lvmetrics`: A sidecar container to expose storage metrics.

### Diagram

- ![#FFFF00](https://placehold.it/15/FFFF00/000000?text=+) TopoLVM components
- ![#ADD8E6](https://placehold.it/15/ADD8E6/000000?text=+) Kubernetes components
- ![#FFC0CB](https://placehold.it/15/FFC0CB/000000?text=+) Kubernetes CSI Sidecar containers

![component diagram](http://www.plantuml.com/plantuml/svg/bPHFRzem6CRl-HHMUe43gl2nXwbRs8rD4MXC4-DWubV1mh4Zsw6RfdxtEOd_X02blPMyFpzwtiUF-wmDKQQfU5AJuaXAGEa2QYx_LY1CYlub26qpAOoId8FAULCoiKD4ezJ8Ml9JDIl2D4KFlu1p-T8Uqbep2WLHkiSBpMQraYUccHIWVels0p6658VkPCx4CNbD4Y4feE-ImhmxrltL-h2Qtk5YtSyM12efrk1y8hHjwTxZ_DnagnhjTImD1kVHeOAIQQD8SDIXLW6COeKdm--XfFLkqMEp1mx6WUwNnPQiF9Wll86EMcw-qQX5eymm01m2m1S1uBi1u0y4WDyT07vl0FX-0FYj05pdSau4T9Yh6QhRBwwOsdQ7DXpKRgYF42M6x8a6b9AQQL0dK4C7FgnijUWjB6N92i8taZSLJEpdwIYgV9FrP0vAstn0c1xEE65LwY19Lw1bemfmiBIBnNlnm_bkqEpBCOvZt8vVRIRXSMgWtUliaFXGqKGg5DemzV4u7siV4_hwnu2WxUkR-6A43ATdbn0hZsRRXxrDRKVbYvXzGbrtqHCgLtbsXZMrdGutQQdFGaX332IncL5n9ERA4kT1qHzyUeEBTVPST8UlBO4lbi1Nbi3NbuYA8p7V_rjBt047Zz9lCVxtYsPk2LjKAvOf80OUVn9J76wputlv08u3FcYuI-f2mAAmFqXv75wuXrSBB_NuewRZG6z2IUYeOtJxBGGLJDjN6gp6SSZtgjikucSGjAduVm00)

Blue arrows in the diagram indicate communications over unix domain sockets.
Red arrows indicate communications over TCP sockets.

Architecture
------------

TopoLVM is a storage plugin based on [CSI](https://github.com/container-storage-interface/spec/).
Therefore, the architecture basically follows the one described in
https://kubernetes-csi.github.io/docs/ .

On each node, a program called `lvmd` runs.  It provides gRPC services via
UNIX domain socket to manage a LVM volume group.  The services include
creating, updating, deleting logical volumes on the volume group and watching
free capacity of the volume group.

`lvmetrics` is another program that runs on each node.  It collects the volume
group metrics from `lvmd` and exposes it as annotations of `Node` resources.
It also adds a finalizer to `Node` to cleanup PersistentVolumeClaims (PVC) on the node.

`topolvm-scheduler` is a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) to extend the
standard Kubernetes scheduler for TopoLVM.

`topolvm-controller` implements CSI controller service.  It also works as
a custom Kubernetes controller to implement dynamic volume provisioning and
resource cleanups.

`topolvm-node` implements CSI node service.  It also works as a custom
Kubernetes controller to implement dynamic-volume provisioning at node side.

### How the scheduler extension works

To extend the standard scheduler, TopoLVM components work together as follows:

- `lvmetrics` exposes free storage capacity as `topolvm.cybozu.com/capacity` annotation of each Node.
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
