[![GitHub release](https://img.shields.io/github/v/release/topolvm/topolvm.svg?maxAge=60)][releases]
[![Main](https://github.com/topolvm/topolvm/workflows/Main/badge.svg)](https://github.com/topolvm/topolvm/actions)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/topolvm/topolvm?tab=overview)](https://pkg.go.dev/github.com/topolvm/topolvm?tab=overview)
[![Go Report Card](https://goreportcard.com/badge/github.com/topolvm/topolvm)](https://goreportcard.com/badge/github.com/topolvm/topolvm)

TopoLVM
=======

TopoLVM is a [CSI][] plugin using LVM for Kubernetes.
It can be considered as a specific implementation of [local persistent volumes](https://kubernetes.io/docs/concepts/storage/volumes/#local) using CSI and LVM.

The team presented the motivation and implementation of TopoLVM at KubeCon Europe 2020: https://kccnceu20.sched.com/event/ZerD

Join our community on Slack: [Invitation form](https://docs.google.com/forms/d/e/1FAIpQLSd2zZhqZUDTs8YUfhvKmSI_xb_iiPnz3-Hy6S7ehmHHmiifEg/viewform?embedded=true)

- **Project Status**: Testing for production
- **Conformed CSI version**: [1.4.0](https://github.com/container-storage-interface/spec/blob/v1.4.0/spec.md)

Supported environments
----------------------

- Kubernetes: 1.21, 1.20, 1.19
- Node OS: Linux with LVM2 (*1)
- Filesystems: ext4, xfs

*1 The host's Linux Kernel must be v4.9 or later which supports `rmapbt` and `reflink`, if you use xfs filesystem with an official docker image.

Features
--------

- [Dynamic provisioning](https://kubernetes-csi.github.io/docs/external-provisioner.html): Volumes are created dynamically when `PersistentVolumeClaim` objects are created.
- [Raw block volume](https://kubernetes-csi.github.io/docs/raw-block.html): Volumes are available as block devices inside containers.
- [Ephemeral inline volume](https://kubernetes.io/docs/concepts/storage/volumes/#csi-ephemeral-volumes): Volumes can be directly embedded in the Pod specification.
- [Topology](https://kubernetes-csi.github.io/docs/topology.html): TopoLVM uses CSI topology feature to schedule Pod to Node where LVM volume exists.
- Extended scheduler: TopoLVM extends the general Pod scheduler to prioritize Nodes having larger storage capacity.
- Volume metrics: Usage stats are exported as Prometheus metrics from `kubelet`.
- [Volume Expansion](https://kubernetes-csi.github.io/docs/volume-expansion.html): Volumes can be expanded by editing `PersistentVolumeClaim` objects.

### Planned features

- [Snapshot](https://kubernetes-csi.github.io/docs/snapshot-restore-feature.html): When we want it.

Programs
--------

A diagram of components is available in [docs/design.md](docs/design.md#diagram).

This repository contains these programs:

- `topolvm-controller`: CSI controller service.
- `topolvm-scheduler`: A [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for TopoLVM.
- `topolvm-node`: CSI node service.
- `lvmd`: gRPC service to manage LVM volumes.

`lvmd` is a standalone program that should be run on Node OS as a systemd service.
Other programs are packaged into [a container image](https://quay.io/organization/topolvm).

Getting started
---------------

A demonstration of TopoLVM running on [kind (Kubernetes IN Docker)][kind] is available at [example](example/) directory.

For production deployments, see [deploy/README.md](./deploy/README.md).

User manual is at [docs/user-manual.md](docs/user-manual.md).

_Deprecated: If you want to use TopoLVM on [Rancher/RKE](https://rancher.com/docs/rke/latest/en/), see [docs/deprecated/rancher.md](docs/deprecated/rancher.md)._

Documentation
-------------

[docs](docs/) directory contains documents about designs and specifications.

Docker images
-------------

Docker images are available on [Quay.io](https://quay.io/organization/topolvm)

[releases]: https://github.com/topolvm/topolvm/releases
[CSI]: https://github.com/container-storage-interface/spec
[kind]: https://github.com/kubernetes-sigs/kind
