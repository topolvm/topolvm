# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.17.1
# If you update the version, you also need to update getting-started.md.
# https://github.com/cert-manager/cert-manager/releases
CERT_MANAGER_VERSION := v1.16.1
# https://github.com/helm/chart-testing/releases
CHART_TESTING_VERSION := 3.11.0
# https://github.com/containernetworking/plugins/releases
CNI_PLUGINS_VERSION := v1.6.0
# https://github.com/GoogleContainerTools/container-structure-test/releases
CONTAINER_STRUCTURE_TEST_VERSION := 1.19.1
# https://github.com/Mirantis/cri-dockerd/releases
CRI_DOCKERD_VERSION := v0.3.15
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.31.1
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v1.61.0
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.14.2
# https://github.com/helm/helm/releases
HELM_VERSION := 3.16.2
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
# https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION := v0.24.0
# It is set by CI using the environment variable, use conditional assignment.
KUBERNETES_VERSION ?= 1.31.0
# https://github.com/kubernetes/minikube/releases
MINIKUBE_VERSION := v1.34.0
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION := 28.2

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
EXTERNAL_PROVISIONER_VERSION := 5.1.0
EXTERNAL_RESIZER_VERSION := 1.12.0
EXTERNAL_SNAPSHOTTER_VERSION := 8.1.0
LIVENESSPROBE_VERSION := 2.14.0
NODE_DRIVER_REGISTRAR_VERSION := 2.12.0

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.31.0)
	KIND_NODE_IMAGE=kindest/node:v1.31.0@sha256:53df588e04085fd41ae12de0c3fe4c72f7013bba32a20e7325357a1ac94ba865
else ifeq ($(KUBERNETES_VERSION), 1.30.4)
	KIND_NODE_IMAGE=kindest/node:v1.30.4@sha256:976ea815844d5fa93be213437e3ff5754cd599b040946b5cca43ca45c2047114
else ifeq ($(KUBERNETES_VERSION), 1.29.8)
	KIND_NODE_IMAGE=kindest/node:v1.29.8@sha256:d46b7aa29567e93b27f7531d258c372e829d7224b25e3fc6ffdefed12476d3aa
endif
