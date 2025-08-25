---
name: Update supporting Kubernetes
about: Dependencies relating to Kubernetes version upgrades
title: 'Update supporting Kubernetes'
labels: 'update kubernetes'
assignees: ''

---

## Update Procedure

- Read [this document](https://github.com/topolvm/topolvm/blob/main/docs/maintenance.md).

## Before Check List

There is a check list to confirm depending libraries or tools are released. The release notes for Kubernetes should also be checked.

### Must Update Dependencies

Must update Kubernetes with each new version of Kubernetes.

- [ ] k8s.io/mount-utils
  - https://github.com/kubernetes/mount-utils/tags
    - The supported Kubernetes version is written in the description of each tag.
- [ ] sigs.k8s.io/controller-runtime
  - https://github.com/kubernetes-sigs/controller-runtime/releases
- [ ] sigs.k8s.io/controller-tools
  - https://github.com/kubernetes-sigs/controller-tools/releases
- [ ] external provisioner
  - https://github.com/kubernetes-csi/external-provisioner/tree/master/CHANGELOG
- [ ] external resizer
  - https://github.com/kubernetes-csi/external-resizer/tree/master/CHANGELOG
- [ ] external snapshotter
  - https://github.com/kubernetes-csi/external-snapshotter/tree/master/CHANGELOG
- [ ] kind
  - https://github.com/kubernetes-sigs/kind/releases
- [ ] minikube
  - https://github.com/kubernetes/minikube/releases
- [ ] crictl
  - https://github.com/kubernetes-sigs/cri-tools/releases

### Release notes check

- [ ] Read the necessary release notes for Kubernetes.

## Checklist

- [ ] Finish implementation of the issue
- [ ] Test all functions
