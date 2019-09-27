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
- `topolvm-controller`: A sidecar controller for cleanup resources.

### Diagram

- ![#FFFF00](https://placehold.it/15/FFFF00/000000?text=+) TopoLVM components
- ![#ADD8E6](https://placehold.it/15/ADD8E6/000000?text=+) Kubernetes components
- ![#FFC0CB](https://placehold.it/15/FFC0CB/000000?text=+) Kubernetes CSI Sidecar containers

![component diagram](http://www.plantuml.com/plantuml/svg/fPN1Rjmi58NtVWeqsUHVM55Opk9NL4yNbTB8ogYfgYGB0bSUD1Wim4chQjwzO6EmFPbngcwY63xEzJc-bxanbcZRrY9h2DsJ2j1g0urGlsgGTeL-PmWz5afQhOG0NOgsul8P4ODMnVOBIZje2_gLKtYIbzJmtAf6YTVwlnMw-052g3UlOupX32ZHfbVmOAFLApTSIT1FqYyGQmdTWNOdIoxt_bmGex5OVpmivsazLJjacLGCq9txSztHtN_Ua5CSh6ws_Tw6GAta5e9XLzBJlTdhvDOlBllqnrbqUfsiQgYuiPeaQnvrfy5gJWSoFiyaGoNfRKpz-wKnxBmxVj--W00RsF3ai5jUxUmdqK97tJvyPQamUpz070F4Hm7YnG3nlmM8FnmW_d20-2y2nCi1udC1XX4f1P7GE-aNKEDNmeIHXXiNY-liiRuV6JSAh1L76unOya8Ce1LOocBgnVscVvTRLN5An8CIRGsNRdaUSFbGz6G9shXKGTeUBWJXzIf0Yjs3KQsc463bQlczr09tQRo6ruWD40w7XWqZs267RAA1bpLmqi19u-1p7cGCiExgyk3nBMQ2X-qGAVhyqzbc_kAv75eXvZtAwn0Bx9JQdoiHL3msxJ2_CccDDFKeVxnu4IqyC_Kcy_zHDv5eZQhxifXWRUDs9wbcevPEEZE9D8WdaT3RQJ-KIWVNPxWqge4RkSFlkSD7xl0xxl3ONTA94dDt9v5XZa-vMm2JFJpOc_yUeXov2NCoXGPYuUexiNGt-pXjCq3TxjMtXZbqwd41eh4ioaESatPOfxhBD5watOkiDpuGM7uFTvi4zXoKRfC1pkAyXGumaCMxC2oDXihnyMj4sSXFDcCl77siyBLlGBAs5dy0)

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
