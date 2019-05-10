Design notes
============

Motivation
----------

To run softwares such as MySQL or Elasticsearch, it would be nice to use
local fast storages and form a cluster to replicate data between servers.

TopoLVM provides a storage driver for such softwares running on Kubernetes.

Goals
-----

- Use LVM for flexible volume capacity management
- Enhance the scheduler to prefer nodes having larger storage capacity
- Support dynamic volume provisioning from PVC

### Future goals

- Prefer nodes with less IO usage.
- Support volume resizing (resizing for CSI is alpha as of Kubernetes 1.14)
- Support volume snapshot
- Authentication between CSI controller and Remote LVM service.

Components
----------

- `csi-topolvm`: Unified CSI driver.
- `lvmd`: gRPC service to manage LVM volumes
- `lvmetrics`: A kubelet [device plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/) to expose metrics needed for scheduling
- `topolvm-scheduler`: A [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for TopoLVM
- `topolvm-node`: A sidecar to communicate with CSI controller over TopoLVM [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

### Diagram

![component diagram](http://www.plantuml.com/plantuml/svg/ZPG_Rzim4CLtVOg3khI349XZXo20tA50MWGHcWw11vgyEa8eKk2Vj4NHxnvI54MRi60lXddyyVZUuq5tnb9jtI2gaR9NUa2eEqij_dAYGrl3lmhWkLNIQYK4wWVX3htS0JLGjzrNo3lGeNxh6TRXW36cvyYeDX6TLy0yJdk9sjJL_wgIYYDyzp-RYGh_meqLG1ApxUBMbkAWvAB8Nn7Wg6QJvOJY8YHoV96oMTKIIV4seDKzIs1QftshHkLhGVHCp9Kcb0BaBNdkrUrHe9sPULLFM_TFttZkpaz38EiXa8iXa2j38DU6G2w6GAu8WQo4G8eGajM9R0PgBEfv-EZO6sy6zIwi37j1xaHSSMhfdXf_32tZIohjupQslrU7db7nf8GRyAClVue2Zo_SuYZKtm5b6dQ3yN2NRe8lVonAZdfIcgS4ckSFU-5mnzLlsTp2zBZm6GwF3ssLXg24yl547YMQwTDkaDjb8B-znm9r3ieFdw1iW9mpG0e3PDK0EMCWqvUw7UjVeGe391aelqRGW7zQtwFLI2tU_AIMlHInkZ51q-xDZv5xj92-0bNBp4t8BPXGMtJDnMLCZxUoZr9qido7ahj1l0C0)

Blue arrows in the diagram indicate communications over unix domain sockets.
Red arrows indicate communications over TCP sockets.

Architecture
------------

TopoLVM adopts [CSI](https://github.com/container-storage-interface/spec/).
Therefore, the architecture basically follows the one described in
https://kubernetes-csi.github.io/docs/ .

Other than that, TopoLVM extends the general Pod scheduler of Kubernetes to
reference node-local metrics such as VG free capacity or IO usage.  To expose
these metrics, TopoLVM installs a [device plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
called *lvmetrics* into `kubelet`.

Extension of the general scheduler will be implemented as a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) called `topolvm-scheduler`.

To support dynamic volume provisioning, CSI controller service need to create a
logical volume on remote target nodes.  In general, CSI controller runs on a
different node from the target node of the volume.  To allow communication
between CSI controller and the target node, TopoLVM uses a custom resource
(CRD) called `LogicalVolume`.

1. `external-provisioner` calls CSI controller's `CreateVolume` with the topology key of the target node.
2. CSI controller creates a `LogicalVolume` with the topology key and capacity of the volume.
3. `topolvm-node` on the target node watches `LogicalVolume` for the target node.
4. `topolvm-node` sends a volume create request to `lvmd` over UNIX domain socket.
5. `lvmd` creates an LVM logical volume as requested.
6. `topolvm-node` updates the status of `LogicalVolume`.
7. CSI controller watches the status of `LogicalVolume` until an LVM logical volume is getting created.

`lvmd` accepts requests from `topolvm-node` and `lvmetrics`.

Limitations
-----------

The CSI driver of TopoLVM depends on Kubernetes because the controller service creates CRD object in Kubernetes.
This limitation can be removed if the controller service uses etcd instead of Kubernetes.

Packaging and deployment
------------------------

`lvmd` is provided as a single executable.
Users need to deploy `lvmd` manually by themselves. 

Other components as well as CSI sidecar containers are provided as Docker 
container images, and will be deployed as Kubernetes objects.
