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
- `topolvm-hook`: A [MutatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook) for `topolvm-scheduler`.

### Diagram

![component diagram](http://www.plantuml.com/plantuml/svg/ZPHFZzCm4CNl-HIZS853j5eVEQ2LMWv82rerOZciUd3jeOtgn97_B234TyUE_xWf3UsbYcT-cRnvUUc3DbGPsujgfEn8zmXrQwZ1xrQqQ6huNG6yhEHWb1G2rPEm-sxO0jLGhzfFK3hGedhj6DR-1grrnv5HfGCQJy0SJhi1bQwhFrKrI8xmnVtSJvI_yazq4vAOTHjQQugz7B8YzmWF1pNtHOulPY61urdA_PAMI8hN7etgM0BpEVQD7AMhUT6HY9N6bppaLdBSqUvGe8bCFDLJw_7vCo_J-Tm4icm2kMe2kT44Sgi9vAe9v0OJo889vCo4k6eWcvvgWog6ZuwTTikWsax9OWVaLeGJfuRkg7QteM4ykxBQhCFuHxdl61NFKjWUtxhokxhuja4jhMBfNKunlD0cfKt2UYToDN8SXVmLNizqsUEGFXkDuTusQOQFFmqE79NRESyuI7dytvJehyVcXllAP5u9BZGlRtR2ufRB7qFp0QQymNlORvvMCyoEhZjpeJgDTvup7m5LodO6qg0OGqTwS84hCKnS4KKkQIV_Q2SNj9DJxMNEsOYKo2NfLy2YdIJjvt-Bq83Bc1kpKaWDtdsZXXtEVBLZWgRktTUHEtIsm29KvV174pHM-Uk8fPEqE3vhXYQwclbPCpjslbBov047Rdln5m00)

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

`topolvm-hook` mutates pods using TopoLVM PVC to add a resource `topolvm.cybozu.com/capacity`.
Only pods with this resource will be passed to the extended scheduler.

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
