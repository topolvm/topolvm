# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.15.1
# If you update the version, you also need to update getting-started.md.
# https://github.com/cert-manager/cert-manager/releases
CERT_MANAGER_VERSION := v1.15.1
# https://github.com/helm/chart-testing/releases
CHART_TESTING_VERSION := 3.11.0
# https://github.com/containernetworking/plugins/releases
CNI_PLUGINS_VERSION := v1.5.1
# https://github.com/GoogleContainerTools/container-structure-test/releases
CONTAINER_STRUCTURE_TEST_VERSION := 1.18.1
# https://github.com/Mirantis/cri-dockerd/releases
CRI_DOCKERD_VERSION := v0.3.14
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.30.0
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v1.59.1
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.13.1
# https://github.com/helm/helm/releases
HELM_VERSION := 3.15.2
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
# https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION := v0.23.0
# It is set by CI using the environment variable, use conditional assignment.
KUBERNETES_VERSION ?= 1.30.0
# https://github.com/kubernetes/minikube/releases
MINIKUBE_VERSION := v1.33.1
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION :=  27.2

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
EXTERNAL_PROVISIONER_VERSION := 5.0.1
EXTERNAL_RESIZER_VERSION := 1.11.1
EXTERNAL_SNAPSHOTTER_VERSION := 8.0.1
LIVENESSPROBE_VERSION := 2.13.0
NODE_DRIVER_REGISTRAR_VERSION := 2.11.1

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.30.0)
	KIND_NODE_IMAGE=kindest/node:v1.30.0@sha256:047357ac0cfea04663786a612ba1eaba9702bef25227a794b52890dd8bcd692e
else ifeq ($(KUBERNETES_VERSION), 1.29.4)
	KIND_NODE_IMAGE=kindest/node:v1.29.4@sha256:3abb816a5b1061fb15c6e9e60856ec40d56b7b52bcea5f5f1350bc6e2320b6f8
else ifeq ($(KUBERNETES_VERSION), 1.28.9)
	KIND_NODE_IMAGE=kindest/node:v1.28.9@sha256:dca54bc6a6079dd34699d53d7d4ffa2e853e46a20cd12d619a09207e35300bd0
endif
