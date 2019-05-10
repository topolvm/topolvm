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

- Unified CSI driver binary
- Scheduler extender for VG capacity management
- Remote LVM service for dynamic volume provisioning
    - authentication using TLS client certificate
- kubelet device plugin to expose VG capacity

### Diagram

![component diagram](http://www.plantuml.com/plantuml/svg/ZPG_pzem48VtV8fJEcV08qFrIj2XKbkXHkhoYi74LnhXE97_K53LxruxSIucD0eBudAFVS-Fd7Wpbclh6fbrlBhmCq9UMcxnfvCbsXp-P03lkrPPtKg9-Y3TkLP7u0RoNVaPfWwKgAzrXNauO8of1LPScm6D5LGUvxL2RVBiRvQfLY1yyn-RdWhVmaH_moYpBuVMdcFJAZBo8m8ys6mcdV0m5V6S89NDaaiavRL1g-jg1AcE_Iy_leg3Rc_tsASwz7qQZrpS2INQ2CGgxrk1JWu-vcVB-TbgVlPYVlPgVhQIdwtbPmlvsIn_J3cGHSEDHHrNZdUryJbG7qDbgbyed0nLUcoFdMpl3QfnKGqE4ygHXqytYqgWxkTDRnYAzmydwV0esj-g-0Zzsu4jdByVTbdqVeBeE97JIX3xU1VGPGcGh2viQU8CIlOsGfC--vy-c-cpHNtsf3-nQtUbzijKgiz6_Vc_21HaJx_Y5bvb6R6q7E3x1lq3cWs5w_n3MgQ75ha-3Oitlw4Ihf7_0000)

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

Extension of the general scheduler will be implemented as a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md).

To support dynamic volume provisioning, CSI controller service need to create a
logical volume on remote target nodes.  To accept volume creation or deletion
requests from CSI controller, each Node runs a gRPC service to manage LVM
logical volumes.

Remote LVM service is divided into two:
- LVMd: A gRPC service listening on a Unix domain socket running on Node OS. 
- LVMd-Proxy: A gRPC service listening on a TCP socket to proxy requests from 
  CSI Controller to LVMd. This is run as a DaemonSet.

LVMd-Proxy will authenticate CSI controller with TLS certificates.

LVMd accepts requests from LVMd-Proxy and lvmetrics.

### Authentication

To protect the LVM service, the gRPC service should require authentication.
Authentication will be implemented with mutual TLS.

Packaging and deployment
------------------------

LVMd is provided as a single executable.
Users need to deploy LVMd manually by themselves. 

Other components as well as CSI sidecar containers are provided as Docker 
container images, and will be deployed as Kubernetes objects.
