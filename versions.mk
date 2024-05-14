# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.12.1
# https://github.com/cert-manager/cert-manager/releases
CERT_MANAGER_VERSION := v1.13.3
# https://github.com/helm/chart-testing/releases
CHART_TESTING_VERSION := 3.10.1
# https://github.com/containernetworking/plugins/releases
CNI_PLUGINS_VERSION := v1.4.0
# https://github.com/GoogleContainerTools/container-structure-test/releases
CONTAINER_STRUCTURE_TEST_VERSION := 1.16.0
# https://github.com/Mirantis/cri-dockerd/releases
CRI_DOCKERD_VERSION := v0.3.9
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.29.0
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v1.55.2
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.12.0
# https://github.com/helm/helm/releases
HELM_VERSION := 3.14.0
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
# https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION := v0.20.0
# It is set by CI using the environment variable, use conditional assignment.
KUBERNETES_VERSION ?= 1.28.0
# https://github.com/kubernetes/minikube/releases
MINIKUBE_VERSION := v1.32.0
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION :=  25.2

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
EXTERNAL_PROVISIONER_VERSION := 4.0.0
EXTERNAL_RESIZER_VERSION := 1.9.3
EXTERNAL_SNAPSHOTTER_VERSION := 6.3.3
LIVENESSPROBE_VERSION := 2.12.0
NODE_DRIVER_REGISTRAR_VERSION := 2.10.0

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.28.0)
	KIND_NODE_IMAGE=kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31
else ifeq ($(KUBERNETES_VERSION), 1.27.3)
	KIND_NODE_IMAGE=kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72
else ifeq ($(KUBERNETES_VERSION), 1.26.6)
	KIND_NODE_IMAGE=kindest/node:v1.26.6@sha256:6e2d8b28a5b601defe327b98bd1c2d1930b49e5d8c512e1895099e4504007adb
endif
