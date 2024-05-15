# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.14.0
# https://github.com/cert-manager/cert-manager/releases
CERT_MANAGER_VERSION := v1.14.5
# https://github.com/helm/chart-testing/releases
CHART_TESTING_VERSION := 3.11.0
# https://github.com/containernetworking/plugins/releases
CNI_PLUGINS_VERSION := v1.4.1
# https://github.com/GoogleContainerTools/container-structure-test/releases
CONTAINER_STRUCTURE_TEST_VERSION := 1.18.1
# https://github.com/Mirantis/cri-dockerd/releases
CRI_DOCKERD_VERSION := v0.3.13
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.30.0
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v1.58.1
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.13.1
# https://github.com/helm/helm/releases
HELM_VERSION := 3.14.4
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
# https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION := v0.22.0
# It is set by CI using the environment variable, use conditional assignment.
KUBERNETES_VERSION ?= 1.29.2
# https://github.com/kubernetes/minikube/releases
MINIKUBE_VERSION := v1.33.0
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION :=  26.1

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
EXTERNAL_PROVISIONER_VERSION := 4.0.1
EXTERNAL_RESIZER_VERSION := 1.10.1
EXTERNAL_SNAPSHOTTER_VERSION := 7.0.2
LIVENESSPROBE_VERSION := 2.12.0
NODE_DRIVER_REGISTRAR_VERSION := 2.10.1

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.29.2)
	KIND_NODE_IMAGE=kindest/node:v1.29.2@sha256:51a1434a5397193442f0be2a297b488b6c919ce8a3931be0ce822606ea5ca245
else ifeq ($(KUBERNETES_VERSION), 1.28.7)
	KIND_NODE_IMAGE=kindest/node:v1.28.7@sha256:9bc6c451a289cf96ad0bbaf33d416901de6fd632415b076ab05f5fa7e4f65c58
else ifeq ($(KUBERNETES_VERSION), 1.27.11)
	KIND_NODE_IMAGE=kindest/node:v1.27.11@sha256:681253009e68069b8e01aad36a1e0fa8cf18bb0ab3e5c4069b2e65cafdd70843
endif
