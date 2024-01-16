BUILDX_VERSION := 0.11.2
CERT_MANAGER_VERSION := v1.13.1
CHART_TESTING_VERSION := 3.9.0
CNI_PLUGINS_VERSION := v1.3.0
CONTAINER_STRUCTURE_TEST_VERSION := 1.16.0
CRI_DOCKERD_VERSION := v0.3.4
CRICTL_VERSION := v1.28.0
HELM_DOCS_VERSION := 1.11.2
HELM_VERSION := 3.13.0
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
KIND_VERSION := v0.20.0
# It is set by CI using the environment variable, use conditional assignment.
KUBERNETES_VERSION ?= 1.27.3
MINIKUBE_VERSION := v1.31.2
PROTOC_VERSION :=  24.4

ENVTEST_KUBERNETES_VERSION := $(shell echo $(KUBERNETES_VERSION) | cut -d "." -f 1-2)

# Tools versions which are defined in go.mod
SELF_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
CONTROLLER_RUNTIME_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-runtime/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
CONTROLLER_TOOLS_VERSION := $(shell awk '/sigs\.k8s\.io\/controller-tools/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
GINKGO_VERSION := $(shell awk '/github.com\/onsi\/ginkgo\/v2/ {print $$2}' $(SELF_DIR)/go.mod)
PROTOC_GEN_DOC_VERSION := $(shell awk '/github.com\/pseudomuto\/protoc-gen-doc/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
PROTOC_GEN_GO_GRPC_VERSION := $(shell awk '/google.golang.org\/grpc\/cmd\/protoc-gen-go-grpc/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)
PROTOC_GEN_GO_VERSION := $(shell awk '/google.golang.org\/protobuf/ {print substr($$2, 2)}' $(SELF_DIR)/go.mod)

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.27.3)
	KIND_NODE_IMAGE=kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72
else ifeq ($(KUBERNETES_VERSION), 1.26.6)
	KIND_NODE_IMAGE=kindest/node:v1.26.6@sha256:6e2d8b28a5b601defe327b98bd1c2d1930b49e5d8c512e1895099e4504007adb
else ifeq ($(KUBERNETES_VERSION), 1.25.11)
	KIND_NODE_IMAGE=kindest/node:v1.25.11@sha256:227fa11ce74ea76a0474eeefb84cb75d8dad1b08638371ecf0e86259b35be0c8
endif
