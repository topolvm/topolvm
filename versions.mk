# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.33.0
# If you update the version, you also need to update getting-started.md.
# https://github.com/cert-manager/cert-manager/releases
CERT_MANAGER_VERSION := v1.17.4
# https://github.com/helm/chart-testing/releases
CHART_TESTING_VERSION := 3.14.0
# https://github.com/containernetworking/plugins/releases
CNI_PLUGINS_VERSION := v1.9.1
# https://github.com/GoogleContainerTools/container-structure-test/releases
CONTAINER_STRUCTURE_TEST_VERSION := 1.22.1
# https://github.com/Mirantis/cri-dockerd/releases
CRI_DOCKERD_VERSION := v0.4.3
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.36.0
# https://github.com/rhysd/actionlint/releases
ACTIONLINT_VERSION := v1.7.12
# https://github.com/suzuki-shunsuke/ghalint/releases
GHALINT_VERSION := v1.5.6
# https://github.com/zizmorcore/zizmor/releases
ZIZMOR_VERSION := 1.26.1
# SHA256 checksum of the zizmor release tarball for verification
ZIZMOR_SHA256 := 8556289a64e7aaf2400cd516f61a471aa91c5902cc56ad96a82fd12f90c2ef73
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v2.11.4
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.14.2
# https://github.com/helm/helm/releases
HELM_VERSION := 3.20.2
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
# https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION := v0.32.0
# It is set by CI using the environment variable, use conditional assignment.
# Use a Kubernetes version supported by the minikube version below.
# The patch version may differ from the k8s patch version in go.mod.
KUBERNETES_VERSION ?= 1.36.4
KUBERNETES_MINOR = $(shell echo $(KUBERNETES_VERSION) | cut -d '.' -f2)
# https://github.com/kubernetes/minikube/releases
MINIKUBE_VERSION := v1.39.0
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION := 34.1
# https://github.com/mikefarah/yq/releases
YQ_VERSION := 4.53.2

# Tools versions which are defined in go.mod
SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
CONTROLLER_RUNTIME_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-runtime/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
CONTROLLER_TOOLS_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-tools/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
GINKGO_VERSION := $(shell awk '/github.com\/onsi\/ginkgo\/v2/ {print $$2}' $(SELF_DIR)/go.mod)
PROTOC_GEN_DOC_VERSION := $(shell awk '/github.com\/pseudomuto\/protoc-gen-doc/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
PROTOC_GEN_GO_GRPC_VERSION := $(shell awk '/google.golang.org\/grpc\/cmd\/protoc-gen-go-grpc/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)

ENVTEST_KUBERNETES_VERSION := $(shell echo $(KUBERNETES_VERSION) | cut -d "." -f 1-2).0

# CSI sidecar versions
# https://github.com/kubernetes-csi/external-provisioner/releases
EXTERNAL_PROVISIONER_VERSION := 6.2.0
# https://github.com/kubernetes-csi/external-resizer/releases
EXTERNAL_RESIZER_VERSION := 2.1.0
# https://github.com/kubernetes-csi/external-snapshotter/releases
EXTERNAL_SNAPSHOTTER_VERSION := 8.5.0
# https://github.com/kubernetes-csi/livenessprobe/releases
LIVENESSPROBE_VERSION := 2.18.0
# https://github.com/kubernetes-csi/node-driver-registrar/releases
NODE_DRIVER_REGISTRAR_VERSION := 2.16.0

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
# NOTE: If kind does not have a prebuilt image for the exact patch version,
# we use the image from the latest available patch version for the same minor version.
ifeq ($(KUBERNETES_VERSION), 1.36.4)
	KIND_NODE_IMAGE=kindest/node:v1.36.1@sha256:3489c7674813ba5d8b1a9977baea8a6e553784dab7b84759d1014dbd78f7ebd5
else ifeq ($(KUBERNETES_VERSION), 1.35.5)
	KIND_NODE_IMAGE=kindest/node:v1.35.5@sha256:ce977ae6d65918d0b58a5f8b5e940429c2ce42fa3a5619ec2bbc60b949c0ac95
else ifeq ($(KUBERNETES_VERSION), 1.34.8)
	KIND_NODE_IMAGE=kindest/node:v1.34.8@sha256:02722c2dedddcfc00febf5d27fbeb9b7b2c14294c82109ff4a85d89ac9ba3256
endif
