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

![component diagram](http://www.plantuml.com/plantuml/svg/bPJFZjem4CRlUOfHzh8Sq0eVzr1j6tgZLGGgLRNbOE9Hi73io7RO_j6-Uvt4CIR0qhsiDZC_ZxzlFCEJiLJRfX99JOizBH7IETP2_QvGsXJ-9W3FcP9MAo5Gvw8fkTm0DP1QLIjngAP5oAPmzmE5K2_j8MCCPrXGRNgyC7nQQtNWXYk9-gTi0zHQMko6Bus6_-dAv5pkazSaaOeXV7L_PbsDxhzML08mo9sl-joSOgNa2hrefw2bUy6pKyLjrQ2rPrbGwzbUJycDrJGe0d2Q7BrljYZGUjH_EMZ1ovtz91higCNw2_E8kvM56q-CaM2Cd1aZDusHTnWZ_s-Ct3P6tZBc1oONL69_QH-0ketugJB53baZK6_Y-W2CMhgb1Y6bDJUe3wXZ1KCJikMybx1G9I-eM2lHL7ZlmfDH2_9rrfCvQkDyexGzd0dAgzH3YYtHg4ONw67bZ1txFIHdcsWIpzFac2Pj-jNr96oMGTQjbaFYBODxfI6yycHeZzUn6je4dtyvwQnTbllXmKCF9oUF44q-J9jw-W5EBC0ZV9HIMMhn57s-sue6DqozI7Uccr_7biiyIuQH3z1GDfn-V8EBrijpKIIWSRrmSQIGN313BfiXbyoGaHbopSmDCDgfCpT7z6B1PFnR2bClq0skWpADpiZ3TsgBtNLs_9hlm4d0eudtVJqhu53SxfJQZoVAw_Mb7hxLRDp-gr0IPjOpfes5_WNHRck3r3WTkdkcrhVu2ILhAl4F)

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
`Node` resource of the running node.  In the meantime, it adds a finalizer
to the `Node` to clean up PersistentVolumeClaims (PVC) bound on the node.

`topolvm-node` also works as a custom Kubernetes controller to implement
dynamic volume provisioning.  Details are described in the following sections.

`topolvm-controller` implements CSI controller services.  It also works as
a custom Kubernetes controller to implement dynamic volume provisioning and
resource cleanups.

`topolvm-scheduler` is a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) to extend the
standard Kubernetes scheduler for TopoLVM.

### How the scheduler extension works

To extend the standard scheduler, TopoLVM components work together as follows:

- `topolvm-node` exposes free storage capacity as `capacity.topolvm.io/<volume group name>` annotation of each Node.
- `topolvm-controller` works as a [mutating webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) for new Pods.
    - It adds `capacity.topolvm.io/<volume group name>` annotation to a pod and `topolvm.io/capacity` resource to the first container of a pod.
    - The value of the annotation is the sum of the storage capacity requests of unbound TopoLVM PVCs for each volume group referenced by the pod.
- `topolvm-scheduler` filters and scores Nodes for a new pod having `topolvm.io/capacity` resource request.
    - Nodes having less capacity in given volume group than requested are filtered.
    - Nodes having larger capacity in given volume group are scored higher.

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

### How volume expansion works

When the requested size of PVC is expanded, `ControllerExpandVolume` of `topolvm-controller` is called to
change the `.spec.size` of the corresponding `LogicalVolume` resource.

If there is a difference between `logicalvolume.spec.size` and `logicalvolume.status.currentSize`,
it means that the logical volume corresponding to the `LogicalVolume` resource should be expanded.
So in that case, `topolvm-node` sends `ResizeLV` request to `lvmd`.
If it receives a successful response, `topolvm-node` updates `logicalvolume.status.currentSize`.
If it receives an erroneous response, it updates the `.status.code` and `.status.message` field with the error.

Then, if the logical volume is not a block device, `topolvm-node` resizes the filesystem of the logical volume
via `NodeExpandVolume` or `NodePublishVolume`.
If the filesystem requires offline resizing, the administrator should make `LogicalVolume` offline beforehand.
The resizing is performed in `NodePublishVolume` in this case.
If the filesystem is resized online, the resizing is performed in `NodeExpandVolume`.
Currently, all supported filesystems can be resized online, so `NodePublishVolume` is not involved with resizing.

Limitations
-----------

TopoLVM depends on Kubernetes deeply.
Portability to other container orchestrators (CO) is not considered.

Packaging and deployment
------------------------

`lvmd` is provided as a single executable.
Users need to deploy `lvmd` manually by themselves.

Other components, as well as CSI sidecar containers, are provided in a single
Docker container image, and is deployed as Kubernetes objects.
