![TopoLVM logo](./docs/img/TopoLVM_logo.svg)
[![GitHub release](https://img.shields.io/github/v/release/topolvm/topolvm.svg?maxAge=60)][releases]
[![Main](https://github.com/topolvm/topolvm/workflows/Main/badge.svg)](https://github.com/topolvm/topolvm/actions)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/topolvm/topolvm?tab=overview)](https://pkg.go.dev/github.com/topolvm/topolvm?tab=overview)
[![Go Report Card](https://goreportcard.com/badge/github.com/topolvm/topolvm)](https://goreportcard.com/badge/github.com/topolvm/topolvm)

TopoLVM
=======

TopoLVM is a [CSI][] plugin using LVM for Kubernetes.
It can be considered as a specific implementation of [local persistent volumes](https://kubernetes.io/docs/concepts/storage/volumes/#local) using CSI and LVM.

- **Project Status**: Testing for production
- **Conformed CSI version**: [1.5.0](https://github.com/container-storage-interface/spec/blob/v1.5.0/spec.md)

Our supported platform are:

- Kubernetes: 1.25, 1.24, 1.23
- Node OS: Linux with LVM2 (*1)
- Filesystems: ext4, xfs, btrfs(experimental)
- lvm version 2.02.163 or later (adds JSON output support)

*1 The host's Linux Kernel must be v4.9 or later which supports `rmapbt` and `reflink`, if you use xfs filesystem with an official docker image.

Docker images are available on [ghcr.io](https://github.com/orgs/topolvm/packages).  
**Please note that [the images on quay.io](https://quay.io/organization/topolvm) are deprecated and will be removed in the future.**

Getting started
---------------

A demonstration of TopoLVM running on [kind (Kubernetes IN Docker)][kind] is available at [example](example/) directory.

For production deployments, see [deploy/README.md](./deploy/README.md).

User manual is at [docs/user-manual.md](docs/user-manual.md).

_Deprecated: If you want to use TopoLVM on [Rancher/RKE](https://rancher.com/docs/rke/latest/en/), see [docs/deprecated/rancher.md](docs/deprecated/rancher.md)._

Contributing
------------

TopoLVM project welcomes contributions from any member of our community. To get
started contributing, please see our [Contributing Guide](CONTRIBUTING.md).

Scope
-----

### In Scope

- [Dynamic provisioning](https://kubernetes-csi.github.io/docs/external-provisioner.html): Volumes are created dynamically when `PersistentVolumeClaim` objects are created.
- [Raw block volume](https://kubernetes-csi.github.io/docs/raw-block.html): Volumes are available as block devices inside containers.
- [Topology](https://kubernetes-csi.github.io/docs/topology.html): TopoLVM uses CSI topology feature to schedule Pod to Node where LVM volume exists.
- Extended scheduler: TopoLVM extends the general Pod scheduler to prioritize Nodes having larger storage capacity.
- Volume metrics: Usage stats are exported as Prometheus metrics from `kubelet`.
- [Volume Expansion](https://kubernetes-csi.github.io/docs/volume-expansion.html): Volumes can be expanded by editing `PersistentVolumeClaim` objects.
- [Storage capacity tracking](https://github.com/topolvm/topolvm/tree/main/deploy#storage-capacity-tracking): You can enable Storage Capacity Tracking mode instead of using topolvm-scheduler.

### Planned features

- [Snapshot](https://kubernetes-csi.github.io/docs/snapshot-restore-feature.html): When we want it.

### Deprecated features

- [Pod security policy](https://kubernetes.io/docs/concepts/policy/pod-security-policy/) support is deprecated and will be removed when TopoLVM removes the support of Kubernetes < v1.25. This policy is same as Kubernetes.

Communications
--------------

If you have any questions or ideas, please use [discussions](https://github.com/topolvm/topolvm/discussions).

Resources
---------

[docs](docs/) directory contains the user manual, designs and specifications, and so on.

A diagram of components is available in [docs/design.md](docs/design.md#diagram).

TopoLVM maintainers presented the motivation and implementation of TopoLVM at KubeCon Europe 2020: https://kccnceu20.sched.com/event/ZerD

License
-------

This project is licensed under [Apache License 2.0](LICENSE).

[releases]: https://github.com/topolvm/topolvm/releases
[CSI]: https://github.com/container-storage-interface/spec
[kind]: https://github.com/kubernetes-sigs/kind
