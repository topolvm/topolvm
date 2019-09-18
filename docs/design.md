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

![component diagram](http://www.plantuml.com/plantuml/svg/fLKnRziy4DtrAmXdoJSOGJXw-11Kxb0W3OmOJL4a6HZoO1EcI86aaXQ5_tj9YU99jcEbw194T-_UyJtUqJVEC-kRIXKrupks0J9RGgtChmgqdv7V1I6FfT7U6gN0hbIpaVgPC4TcvVeBmGnPWtsL79xq9NToxarjr6lrtunS_02bp5laSSv90PnPrp54O6tDgtJIQB1FEWQOzunlmORAbTIxM9V8U6xMbQVm7EFORLyKsWWWT-7FTOjk_uk20cClTRdkSai6bT5hI893ouZkn5wZsXYSrXdBHLPwZL8jRAJpbg6q20tLuAtaFJ9_7r2cJZhA6EkFeI4uQ0_uNVC22dp2fgy0kvMRhV-a6cXHjmzV1JOMfmsasK1wP22TD93-cu7qmmRI3nj8_y90EcqWFMTWMf6LOXew9Ls0r36IDepLqWLRucGZVqoui0gYKKS9mJnxOGx833mNsVNoVjcTvTPi96VgmQYcWiiGFZHuzL1so1LO5rm5xJjmgPYiPWban3quqrWE2Mn2hvZ34RZ3zUgFaGSW5QU1JIlOGlR5Y8DESr3aeCYnZpxtD0v4-tIT6ib7boIiUqV9vvz1jG0R9h6VX6ptmAvzvjyQiyCEn-zaOecmyO35o6WoSEd9_F7YHJNmBCoRdlzFseKa2xAvNvKZM5E7xCZKOZ3Ho7D0WNluBYIDDkPJAonSdC7XfYRSG1qzfOuUgQCdrD6XEkH1YWDrxqBwDNvFtGGGvT5Utk7F8PcJi621fhJ0F5nzPvvl3oudS2LGcxtwLei07MkS0F7SbioJdQDnM63VQJITH8U3B9RH49W-HtXC4lOS21-J7bpaVKiJuA18Js4E3POguFDjPDgC7oPn5eukAzts3MZHLEeF)

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
