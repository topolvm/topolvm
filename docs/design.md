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

### Future goals

- Prefer nodes with less IO usage.
- Support volume resizing (resizing for CSI is alpha as of Kubernetes 1.14).
- Support volume snapshot.
- Authentication between the CSI controller and Remote LVM service.

Components
----------

- `csi-topolvm`: Unified CSI driver.
- `lvmd`: gRPC service to manage LVM volumes.
- `lvmetrics`: A DaemonSet sidecar container to expose storage metrics as Node annotations.
- `topolvm-scheduler`: A [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for TopoLVM.
- `topolvm-node`: A sidecar to communicate with CSI controller over TopoLVM [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).
- `topolvm-hook`: A [MutatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook) for `topolvm-scheduler`.
- `topolvm-controller`: A sidecar contoller for cleanup resources.

### Diagram

- ![#FFFF00](https://placehold.it/15/FFFF00/000000?text=+) TopoLVM components
- ![#ADD8E6](https://placehold.it/15/ADD8E6/000000?text=+) Kubernetes components
- ![#FFC0CB](https://placehold.it/15/FFC0CB/000000?text=+) Kubernetes CSI Sidecar containers
- CSI Controller service `csi-topolvm` can be deployed as `Deployment` after fix https://github.com/cybozu-go/topolvm/issues/40

![component diagram](http://www.plantuml.com/plantuml/svg/fLN1Rjiw4BphAmZbadSG0lFemqDGkOS2QI34g8iYyO4X5p9XYXH8ocbH-DyhIXGfoKcKehTeEJkRcUNGjyOIRPjA93MXHr82IZTG2_Mh0cbJz3j1w35Nqceb1EX2D2MNUqGCIlFj5nHFq1RqYLDuajVKyCogMebJzL-Ahdw04Eh5yHXZE0DAjEaP7B3MwiGDLnBqatG5OYsX1z1jPy7bqVLvCXg6zUs-dCLwd7PE4gaOe7l5ODMXkx-Se9PnaBhPzcSR0fMIM_22sv4EtOjTHRMkkApJjREWT1MbQYYviPf4QGxQTeFLd0x8y3sZz9Eaap7LxqfZyDb9V3mspo30Ugp_Qc5tl3pOJwA13dMt-yhYOFQwWJWOY2yCn8i6udyp47_OGFnX0_5V68YN3SJl6UWZKWWYeLVI5r3jAZvXfEO6z0dqVipl8aCFm1fnSS289S_404h1KXPSpnys_VzofqfEYTWfqLHmtPRd1XUxl4SMe0qt5gJjmRaWl4eTA6pvFYPec1Hs_016DPh2QXzJrdUVfnnucGD73XmvpixAWSO0wrud1xnyg0pq3Dl1DVJvfmN9tDjoFMmxM3gov1m679GwlZV2ipZOz1AvocJxCTWeUxnw5WticNgHsV-e2nrQe_AXo2UmfcvF9wucuzW7dXb5pDgR47zzuxaV5HlNx-1Y6Zrkf0w_fWvVgCElgCDZTP4dKipKlGaPelgMkWLWb8S7UVEVF9IXDaGjJQw1MBZvJYmzp9RJr0EeExtvLhCMEj6u1A9XBCgSdVFhi8vpbz5u4LtiylN66G994cBX5sKWyQzIA8tkVeiFfWMwW9ySXfNDFHsMSWkIDPNu0m00)

Blue arrows in the diagram indicate communications over unix domain sockets.
Red arrows indicate communications over TCP sockets.

Architecture
------------

TopoLVM adopts [CSI](https://github.com/container-storage-interface/spec/).
Therefore, the architecture basically follows the one described in
https://kubernetes-csi.github.io/docs/ .

Other than that, TopoLVM extends the general Pod scheduler of Kubernetes to
reference node-local metrics such as VG free capacity or IO usage.  To expose
these metrics, TopoLVM runs a sidecar container called *lvmetrics* on each node.

`lvmetrics` watches the capacity of LVM volume group and exposes it as Node
annotations. Also, it adds finalizer to the `Node` resource for cleanup.

Extension of the general scheduler is implemented as a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) called `topolvm-scheduler`.

`topolvm-hook` mutates pods using TopoLVM PVC to add a resource `topolvm.cybozu.com/capacity`.
Only pods with this resource is passed to the extended scheduler.

To support dynamic volume provisioning, CSI controller service need to create a
logical volume on remote target nodes.  In general, CSI controller runs on a
different node from the target node of the volume.  To allow communication
between CSI controller and the target node, TopoLVM uses a custom resource
(CRD) called `LogicalVolume`.

1. `external-provisioner` calls CSI controller's `CreateVolume` with the topology key of the target node.
2. CSI controller creates a `LogicalVolume` with the topology key and capacity of the volume.
3. `topolvm-node` on the target node watches `LogicalVolume` for the target node.
4. `topolvm-node` sends a volume to create request to `lvmd` over UNIX domain socket.
5. `lvmd` creates an LVM logical volume as requested.
6. `topolvm-node` updates the status of `LogicalVolume`.
7. CSI controller watches the status of `LogicalVolume` until an LVM logical volume is getting created.

`lvmd` accepts requests from `topolvm-node` and `lvmetrics`.

Limitations
-----------

The CSI driver of TopoLVM depends on Kubernetes because the controller service creates a CRD object in Kubernetes.
This limitation can be removed if the controller service uses etcd instead of Kubernetes.

Packaging and deployment
------------------------

`lvmd` is provided as a single executable.
Users needs to deploy `lvmd` manually by themselves.

Other components, as well as CSI sidecar containers, are provided as Docker
container images, and is deployed as Kubernetes objects.
