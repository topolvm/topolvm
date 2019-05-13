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
- `lvmetrics`: A DaemonSet sidecar container to expose storage metrics as Node annotations
- `topolvm-scheduler`: A [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for TopoLVM
- `topolvm-node`: A sidecar to communicate with CSI controller over TopoLVM [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).

### Diagram

![component diagram](http://www.plantuml.com/plantuml/svg/ZPJFYjim48VlVeh1lRG7iqJ77ig2qqDXMvQORWzB3eeqpPgLBQC_QKlfkrUI9IlsnYIN46c-ZB_vPV2zDbGPsubYeEoL7X7Anb23Fwreq9Jmjm1uhcLlb1G2rQEmnxRV0zLGriqNo1LeK9rQXgN_WTQwvSYeqYCQJy0SJjiUbIwBVqNHIuxmpNri0kM_-IUw3abcsuobBSLEzfEHUuI7HvjDrl6NMIHmV5BPhBb4KfmwDAfb2KpdL3ToaExEIqSYtbJ-oaDk9CUzsWCAD969fpAK7fw-yjoTpqCWwo6Ggo6G6qCWjuP0heP0RWY1h8H05Y6aKGDPTLGRL77vwD1gDmogVTWizeBSYl74gQ47gX5AD8nFgTIxxTZ-GHvRHiMJ5BR3z-xwGvbpsw6MLh7qNuOrl50ckKp2U3FTBGv2_kcmDz5MuyWtoHC-_pROSrHXphnZK3s_EmYBUov_zTKd2Ai17-6uUwndc1rSTIPSd6_Yr6VU0egOUGPYexGnZlJW2fSt9d5PYbno9s_SoGLtSkwU-onQfELPKxy2PdUIt9TlCAZ0hODhKoka1kz-KCDU5hdQ8K6XUlTzu0vT3B025TEUnX2qlvkye8h9JSjzNiofBBNJtFVOSzk9_m00)

Blue arrows in the diagram indicate communications over unix domain sockets.
Red arrows indicate communications over TCP sockets.

Architecture
------------

TopoLVM adopts [CSI](https://github.com/container-storage-interface/spec/).
Therefore, the architecture basically follows the one described in
https://kubernetes-csi.github.io/docs/ .

Other than that, TopoLVM extends the general Pod scheduler of Kubernetes to
reference node-local metrics such as VG free capacity or IO usage.  To expose
these metrics, TopoLVM run sidecar containers called *lvmetrics* on each node.

`lvmetrics` watches the capacity of LVM volume group and exposes it as Node
annotations.

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
