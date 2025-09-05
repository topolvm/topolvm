# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.28.0
# If you update the version, you also need to update getting-started.md.
# https://github.com/cert-manager/cert-manager/releases
CERT_MANAGER_VERSION := v1.17.4
# https://github.com/helm/chart-testing/releases
CHART_TESTING_VERSION := 3.13.0
# https://github.com/containernetworking/plugins/releases
CNI_PLUGINS_VERSION := v1.8.0
# https://github.com/GoogleContainerTools/container-structure-test/releases
CONTAINER_STRUCTURE_TEST_VERSION := 1.19.3
# https://github.com/Mirantis/cri-dockerd/releases
CRI_DOCKERD_VERSION := v0.3.20
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.33.0
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v1.64.8
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.14.2
# https://github.com/helm/helm/releases
HELM_VERSION := 3.18.6
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
# https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION := v0.30.0
# It is set by CI using the environment variable, use conditional assignment.
KUBERNETES_VERSION ?= 1.33.4
# https://github.com/kubernetes/minikube/releases
MINIKUBE_VERSION := v1.36.0
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION := 32.0

# Tools versions which are defined in go.mod
SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
CONTROLLER_RUNTIME_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-runtime/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
CONTROLLER_TOOLS_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-tools/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
GINKGO_VERSION := $(shell awk '/github.com\/onsi\/ginkgo\/v2/ {print $$2}' $(SELF_DIR)/go.mod)
PROTOC_GEN_DOC_VERSION := $(shell awk '/github.com\/pseudomuto\/protoc-gen-doc/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
PROTOC_GEN_GO_GRPC_VERSION := $(shell awk '/google.golang.org\/grpc\/cmd\/protoc-gen-go-grpc/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)

ENVTEST_BRANCH := release-$(shell echo $(CONTROLLER_RUNTIME_VERSION) | cut -d "." -f 1-2)
ENVTEST_KUBERNETES_VERSION := $(shell echo $(KUBERNETES_VERSION) | cut -d "." -f 1-2)

# CSI sidecar versions
# https://github.com/kubernetes-csi/external-provisioner/releases
EXTERNAL_PROVISIONER_VERSION := 5.3.0
# https://github.com/kubernetes-csi/external-resizer/releases
EXTERNAL_RESIZER_VERSION := 1.14.0
# https://github.com/kubernetes-csi/external-snapshotter/releases
EXTERNAL_SNAPSHOTTER_VERSION := 8.3.0
# https://github.com/kubernetes-csi/livenessprobe/releases
LIVENESSPROBE_VERSION := 2.16.0
# https://github.com/kubernetes-csi/node-driver-registrar/releases
NODE_DRIVER_REGISTRAR_VERSION := 2.14.0

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.33.4)
	KIND_NODE_IMAGE=kindest/node:v1.33.4@sha256:25a6018e48dfcaee478f4a59af81157a437f15e6e140bf103f85a2e7cd0cbbf2
else ifeq ($(KUBERNETES_VERSION), 1.32.8)
	KIND_NODE_IMAGE=kindest/node:v1.32.8@sha256:abd489f042d2b644e2d033f5c2d900bc707798d075e8186cb65e3f1367a9d5a1
else ifeq ($(KUBERNETES_VERSION), 1.31.12)
	KIND_NODE_IMAGE=kindest/node:v1.31.12@sha256:0f5cc49c5e73c0c2bb6e2df56e7df189240d83cf94edfa30946482eb08ec57d2
endif
