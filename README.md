[![GitHub release](https://img.shields.io/github/release/cybozu-go/topolvm.svg?maxAge=60)][releases]
[![CircleCI](https://circleci.com/gh/cybozu-go/topolvm.svg?style=svg)](https://circleci.com/gh/cybozu-go/topolvm)
[![GoDoc](https://godoc.org/github.com/cybozu-go/topolvm?status.svg)][godoc]
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/topolvm)](https://goreportcard.com/report/github.com/cybozu-go/topolvm)
[![Docker Repository on Quay](https://quay.io/repository/cybozu/topolvm/status "Docker Repository on Quay")](https://quay.io/repository/cybozu/topolvm)

TopoLVM
=======

TopoLVM is a [CSI][] plugin using LVM for Kubernetes.
It can be considered as a specific implementation of [local persistent volumes](https://kubernetes.io/docs/concepts/storage/volumes/#local) using CSI and LVM.

**Project Status**: Testing for production

Supported environments
----------------------

- Kubernetes
  - 1.14
  - 1.15
- Node OS
  - Linux with LVM2
- Filesystems
  - ext4
  - xfs
  - btrfs

Features
--------

- [Dynamic provisioning](https://kubernetes-csi.github.io/docs/external-provisioner.html): Volumes are created dynamically when `PersistentVolumeClaim` objects are created.
- [Raw block volume](https://kubernetes-csi.github.io/docs/raw-block.html): Volumes are available as block devices inside containers.
- [Topology](https://kubernetes-csi.github.io/docs/topology.html): TopoLVM uses CSI topology feature to schedule Pod to Node where LVM volume exist.
- Extended scheduler: TopoLVM extends the general Pod scheduler to prioritize Nodes having larger storage capacity.

### Planned features

- [Volume Expansion](https://kubernetes-csi.github.io/docs/volume-expansion.html): When support for Kubernetes 1.16 will be added.
- [Snapshot](https://kubernetes-csi.github.io/docs/snapshot-restore-feature.html): When we want it.

Programs
--------

A diagram of components is available in [docs/design.md](docs/design.md#diagram).

This repository contains these programs:

- `csi-topolvm`: Unified CSI driver.
- `lvmd`: gRPC service to manage LVM volumes.
- `lvmetrics`: A DaemonSet sidecar container to expose storage metrics as Node annotations.
- `topolvm-scheduler`: A [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for TopoLVM.
- `topolvm-node`: A sidecar to communicate with CSI controller over TopoLVM [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/).
- `topolvm-hook`: A [MutatingAdmissionWebhook](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#mutatingadmissionwebhook) for `topolvm-scheduler`.

`lvmd` is a standalone program that should be run on Node OS as a systemd service.
Other programs are packaged into container images.

Getting started
---------------

A demonstration of TopoLVM running on [kind (Kubernetes IN Docker)][kind] is available at [example](example/) directory.

For production deployments, see [deploy](deploy/) directory.

Documentation
-------------

[docs](docs/) directory contains documents about designs and specifications.

[releases]: https://github.com/cybozu-go/topolvm/releases
[godoc]: https://godoc.org/github.com/cybozu-go/topolvm
[CSI]: https://github.com/container-storage-interface/spec
[kind]: https://github.com/kubernetes-sigs/kind
