---
name: Update supporting Kubernetes
about: Dependencies relating to Kubernetes version upgrades
title: 'Update supporting Kubernetes'
labels: 'update kubernetes'
assignees: ''

---

## Update Procedure

- Read [this document](https://github.com/topolvm/topolvm/blob/main/docs/maintenance.md).

## Must Update Dependencies

Must update Kubernetes with each new version of Kubernetes.

- [ ] k8s.io/api
  - https://github.com/kubernetes/api/tags
    - The supported Kubernetes version is written in the description of each tag.
- [ ] k8s.io/apimachinery
  - https://github.com/kubernetes/apimachinery/tags
    - The supported Kubernetes version is written in the description of each tag.
- [ ] k8s.io/client-go
  - https://github.com/kubernetes/client-go/tags
    - The supported Kubernetes version is written in the description of each tag.
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

## Semi-required Dependencies

These are not released on the occasion of a Kubernetes version upgrade, so if there is a release that corresponds to a Kubernetes version, use it.

- [ ] node driver registrar
  - https://github.com/kubernetes-csi/node-driver-registrar/tree/master/CHANGELOG
- [ ] liveness probe
  - https://github.com/kubernetes-csi/livenessprobe/tree/master/CHANGELOG
- [ ] cri-docker
  - https://github.com/Mirantis/cri-dockerd/releases

## Checklist

- [ ] Finish implementation of the issue
- [ ] Test all functions
