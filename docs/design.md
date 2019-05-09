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
- Authentication between CSI controller and Remote LVM service.

Components
--------------

- Unified CSI driver binary
- Scheduler extender for VG capacity management
- Remote LVM service for dynamic volume provisioning
    - authentication using TLS client certificate
- kubelet device plugin to expose VG capacity

### Diagram

![component diagram](http://www.plantuml.com/plantuml/svg/ZLCzRy8m5DpzAvwoP-2D7H0ITAZKLgYHkbGCRlnA8nmdyWS5LVtljObn4b042-Bvxdntyil2MAwjgoLhURdZMuAiiDpIbvC5sGn-6S37ib5MDrAINakthTG6k85iMJn1Zq11Ub-Lb0M1CQOIL79jEcgSeFHqNYdI9cD_ZAb64Bpwdzc95Vu5HmPm3hCgEcZ5gMvKIkGj0hbBC-lZXCCKfEE956KsbIKovRucgwloJ4npn9_VNyHQDuTZnDCSSD_6KtRkaoJPI8XJnbXK3uJZ_ZZT7s_snplxuxtzyKP_lDKV9_hZHV_OicFcDJUMT5mvtbR6zo2z2PCflqauwQUHR4MjR8urAHjLXZg7uio7nuCb9KYV_FeNXbmqFogVm-bPu06sR-ibYu5xD0aIfLy2c5RFOR2T0riSsUA54AzjySeMOLezj4N6Bh_QtZnrY7VSmyN4JOyH70tvWxHYaBlM7wR76q5pk7A9Lov82LVBVm00)

Architecture
-------------

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
----------------------------

LVMd is provided as a single executable.
Users need to deploy LVMd manually by themselves. 

Other components as well as CSI sidecar containers are provided as Docker 
container images, and will be deployed as Kubernetes objects.
