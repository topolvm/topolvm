# Maintenance Guide

This is the maintenance guide for TopoLVM.

## How to Upgrade Supported Kubernetes Version

TopoLVM depends on some Kubernetes repositories like `k8s.io/client-go` and should support 3 consecutive Kubernets versions at a time.
Here is the guide for how to upgrade the supported versions.
Issues and PRs related to the last upgrade task also help you understand how to upgrade the supported versions,
so checking them together with this guide is recommended when you do this task.

### Upgrade Procedure

Please write down to the Github issue of this task what kinds of changes we find in the release note and what we are going to do and NOT going to do to address the changes.
The format is up to you, but this is very important to keep track of what changes are made in this task, so please do not forget to do it.

Basically, we should pay attention to breaking changes and security fixes first.
If we find some interesting features added in new versions, please consider if we are going to use them or not and make a GitHub issue to incorporate them after the upgrading task is done.

#### Kubernetes

Choose the next version and check the [release note](https://kubernetes.io/docs/setup/release/notes/). e.g. 1.17, 1.18, 1.19 -> 1.18, 1.19, 1.20

Edit the following files.
- `docs/advanced-setup.md`
- `README.md`
- `versions.mk`
- `.github/workflows/e2e-k8s-incluster-lvmd.yaml`
- `.github/workflows/e2e-k8s-workflow.yaml`

Next, we should update `go.mod` by the following commands.
Please note that Kubernetes v1 corresponds with v0 for the release tags. For example, v1.17.2 corresponds with the `v0.17.2` tag.
```console
$ VERSION=<upgrading Kubernetes release version>
$ go get k8s.io/api@v${VERSION} k8s.io/apimachinery@v${VERSION} k8s.io/client-go@v${VERSION} k8s.io/mount-utils@v${VERSION}
```

Read the [controller-runtime's release note](https://github.com/kubernetes-sigs/controller-runtime/releases), and check which version is compatible with the Kubernetes versions.
Then, upgrade the controller-runtime's version by the following commands.

```console
$ VERSION=<upgrading controller-runtime version>
$ go get sigs.k8s.io/controller-runtime@v${VERSION}
```

Read the [`controller-tools`'s release note](https://github.com/kubernetes-sigs/controller-tools/releases), and update to the newest version that is compatible with all supported kubernetes versions. If there are breaking changes, we should decide how to manage these changes.
Then, upgrade the controller-tools's version by the following commands.

```console
$ VERSION=<upgrading controller-tools version>
$ go get sigs.k8s.io/controller-tools@v${VERSION}
```

At last, make it tidy.

```console
$ go mod tidy
```

Regenerate manifests using new controller-tools.

```console
$ make setup
$ make generate
```

These are minimal changes for the Kubernetes upgrade, but if there are some breaking changes found in the release notes, you have to handle them as well in this step.

#### Go

Choose the same version of Go [used by the latest Kubernetes](https://github.com/kubernetes/kubernetes/blob/master/go.mod) supported by TopoLVM.

Edit the following files.
- `go.mod`
- `Dockerfile`

#### CSI Sidecars

> [!NOTE]
> TopoLVM builds csi-sidecars for the cases that we want to use own-patched binaries (e.g. can't wait official binaries in case of emergency.)

TopoLVM does not use all the sidecars listed [here](https://kubernetes-csi.github.io/docs/sidecar-containers.html).
Have a look at `csi-sidecars.mk` first and understand what sidecars are actually being used.

Check the release pages of the sidecars under [kubernetes-csi](https://github.com/kubernetes-csi) one by one and choose the latest version for each sidecar which satisfies both "Minimal Kubernetes version" and "Supported CSI spec versions".

DO NOT follow the "Status and Releases" tables in this [page](https://kubernetes-csi.github.io/docs/sidecar-containers.html) and the README.md files in the sidecar repositories because they are sometimes not updated properly.

Edit `versions.mk` to change sidecars' version.

Read the change logs which are linked from the release pages.
Confirm diffs of RBAC between published files in upstream and following ones, and update it if required.
For example, see https://github.com/kubernetes-csi/external-provisioner/blob/master/deploy/kubernetes/rbac.yaml.

- `charts/topolvm/templates/controller/clusterroles.yaml`
- `charts/topolvm/templates/controller/roles.yaml`

If the `external-snapshotter` sidecar is updated, you also update `go.mod` and source code accordingly.

#### Depending Tools

The depending tools versions are specified in `versions.mk`.

The following tools do not depend on other software, use latest versions.

- [helm](https://github.com/helm/helm/releases)
- [helm-docs](https://github.com/norwoodj/helm-docs/releases)
- [protoc](https://github.com/protocolbuffers/protobuf/releases)

The following tools depend on kubernetes, use appropriate version associating to minimal supported kubernetes version by TopoLVM.

- [kind](https://github.com/kubernetes-sigs/kind/releases)
- [minikube](https://github.com/kubernetes/minikube/releases)

#### Depending Modules

Read [kubernetes go.mod](https://github.com/kubernetes/kubernetes/blob/master/go.mod), and update the `prometheus/*` and `grpc` modules.

#### Update Upstream Information

Visit [the upstream web page](https://kubernetes-csi.github.io/docs/drivers.html) to check current TopoLVM information. If some information is old, create PR to update the information

#### Final Check

`git grep <dropped kubernetes version e.g. 1.18>`, `git grep image:`, `git grep -i VERSION` and looking `versions.mk` might help us avoid overlooking necessary changes.
Please update the versions in the code and docs with great care.

## How to Upgrade Supported CSI Version

Read the [release note](https://github.com/container-storage-interface/spec/releases) and check all the changes from the current version to the latest.

Basically, CSI spec should NOT be upgraded aggressively.

Upgrade the CSI version only if new features we should cover are introduced in newer versions, or the Kubernetes versions TopoLVM is going to support does not support the current CSI version.

For updating CSI spec, update the version of `github.com/container-storage-interface/spec` in `go.mod`.
When updating CSI spec, please also upgrade the version of `github.com/kubernetes-csi/csi-test`.
