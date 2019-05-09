Design notes
========

Motivation
-----------

To run softwares such as MySQL or Elasticsearch, it would be nice to use
local fast storages and clustering multiple instances.  TopoLVM provides
a storage driver for such softwares running on Kubernetes.

Goals
------

- Use LVM for flexible volume capacity management
- Prefer to schedule to nodes having larger free spaces
- Support dynamic volume provisioning from PVC

### Future goals

- Prefer nodes with less IO usage.
- Support volume resizing (resizing for CSI is alpha as of Kubernetes 1.14)
- Support volume snapshot

Components
--------------

- Unified CSI driver binary
- Scheduler extender for VG capacity management
- Remote LVM service for dynamic volume provisioning
    - authentication using TLS client certificate
- kubelet device plugin to expose VG capacity

Architecture
-------------

TopoLVM adopts [CSI](https://github.com/container-storage-interface/spec/).
Therefore, the architecture basically follows the one described in
https://kubernetes-csi.github.io/docs/ .

Other than that, TopoLVM extends the general Pod scheduler of Kubernetes to
reference node-local metrics such as VG free capacity or IO usage.  To expose
these metrics, TopoLVM installs a [device plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/) into `kubelet`.

Extension of the general scheduler will be implemented as a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md).

To support dynamic volume provisioning, CSI controller service need to create a
logical volume on remote target nodes.  To accept volume creation or deletion
requests from CSI controller, each Node runs a gRPC service to manage LVM
logical volumes.

To protect the LVM service, the gRPC service should require authentication.

Packaging and deployment
----------------------------
