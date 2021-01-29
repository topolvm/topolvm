How to upgrade supported Kubernetes version
===========================================

TopoLVM depends on some Kubernetes repositories like `k8s.io/client-go` and should support 3 consecutive Kubernets versions at a time.
Here is the guide for how to upgrade the supported versions.
Issues and PRs related to the last upgrade task also help you understand how to upgrade the supported versions,
so checking them together with this guide is recommended when you do this task.

## Check release notes

First of all, we should have a look at the release notes in the order below.

Please clarify on the issue on GitHub what kinds of changes we find in the release note and what we are going to do and NOT going to do to address the changes.
The format is up to you, but this is very important to keep track of what changes are made in this task, so please do not forget to do it.

Basically, we should pay attention for breaking changes and security fixes first.
If we find some interesting features added in new versions, please consider if we actually use them and make an GitHub issue to incorporate them after the upgrading task is done.

1. Kubernetes
  - Choose the next version and check the [release note](https://kubernetes.io/docs/setup/release/notes/). e.g. 1.17, 1.18, 1.19 -> 1.18, 1.19, 1.20
2. controller-runtime
  - Read the [release note](https://github.com/kubernetes-sigs/controller-runtime/releases), and check which version is compatible with the Kubernetes versions.
3. CSI spec
  - Read the [release note](https://github.com/container-storage-interface/spec/blob/master/spec.md) and check all the changes from the current version to the latest.
  - Basically, CSI spec should NOT be upgraded progressively in this task.
    Upgrade the CSI version only if new features we should cover are introduced in newer versions or we have to upgrade it because Kubernetes does not support the current CSI version.
4. CSI sidecars
  - TopoLVM does not use all the sidecars listed [here](https://kubernetes-csi.github.io/docs/sidecar-containers.html).
    Have a look at `csi-sidecars.mk` first and check what sidecars are actually used.
  - Check the release pages of the sidecars under [kubernetes-csi](https://github.com/kubernetes-csi) one by one and choose the latest version for each sidecar which satisfies both "Minimal Kubernetes version" and "Supported CSI spec versions".
    DO NOT follow the version compatibility tables in this [page](https://kubernetes-csi.github.io/docs/sidecar-containers.html) and the README.md files of the sidecar repositories because they are sometimes not updated properly.
  - Read the change logs which are linked from the release pages.

## Update written versions

Once we decide the versions we are going to upgrade, we should update the versions written in the following files manually.
`git grep 1.18`, `git grep image:`, and `git grep VERSION` might help us avoid overlooking necessary changes.
Please update the versions in the code and docs with great care.

- `README.md`: Documentation which indicates what versions are supported by TopoLVM
- `deploy/README.md`: Documentation which instructs how to deploy TopoLVM on various versions of Kubernetes
- `csi-sidecars.mk`: Makefile for building CSI sidecars
- `Makefile`: Makefile for running e2e tests
- `e2e/Makefile`: Makefile for running e2e tests
- `example/Makefile`: Makefile for running example
- `deploy/manifests/overlays/daemonset-scheduler/kustomization.yaml`: Kustomization for overwriting the TopoLVM image version
- `deploy/manifests/overlays/deployment-scheduler/kustomization.yaml`: Kustomization for overwriting the TopoLVM image version

## Update CSI spec

In case we update the version of the CSI spec written in `Makefile`, we should regenerate the gRPC schema with the protobuf file corresponding to the CSI spec version.
First, we might have to upgrade the `protoc` version defined in `Makefile` with checking the Go section in [release note](https://github.com/protocolbuffers/protobuf/releases).
Then, please execute the following commands to update the Go code.

```bash
cd ${GOPATH}/src/github.com/topolvm/topolvm
make setup
make csi/csi.pb.go
make csi/csi_grpc.pb.go
```

## Update dependencies

Next, we should update `go.mod` by the following commands.

```bash
VERSION=<upgrading Kubernetes version>
go get k8s.io/api@v${VERSION} k8s.io/apimachinery@v${VERSION} k8s.io/client-go@v${VERSION}
```

If we need to upgrade the `controller-runtime` version, do the following as well.

```bash
VERSION=<upgrading controller-runtime version>
go get sigs.k8s.io/controller-runtime@v${VERSION}
```

Then, please tidy up the dependencies.

```bash
go mod tidy
```

These are minimal changes for the Kubernetes upgrade, but if there are some breaking changes found in the release notes, you have to handle them as well in this step.

## Release the changes

We should follow [RELEASE.md](../RELEASE.md) and upgrade the minor version.

## Prepare for the next upgrade

We should create an issue for the next upgrade. Besides, Please update this document if we find something to be updated.
