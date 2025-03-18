# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.21.2
# If you update the version, you also need to update getting-started.md.
# https://github.com/cert-manager/cert-manager/releases
CERT_MANAGER_VERSION := v1.17.1
# https://github.com/helm/chart-testing/releases
CHART_TESTING_VERSION := 3.12.0
# https://github.com/containernetworking/plugins/releases
CNI_PLUGINS_VERSION := v1.6.2
# https://github.com/GoogleContainerTools/container-structure-test/releases
CONTAINER_STRUCTURE_TEST_VERSION := 1.19.3
# https://github.com/Mirantis/cri-dockerd/releases
CRI_DOCKERD_VERSION := v0.3.16
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.32.0
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v1.64.7
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.14.2
# https://github.com/helm/helm/releases
HELM_VERSION := 3.17.1
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
# https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION := v0.27.0
# It is set by CI using the environment variable, use conditional assignment.
KUBERNETES_VERSION ?= 1.32.2
# https://github.com/kubernetes/minikube/releases
MINIKUBE_VERSION := v1.35.0
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION := 30.0

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
EXTERNAL_PROVISIONER_VERSION := 5.2.0
# https://github.com/kubernetes-csi/external-resizer/releases
EXTERNAL_RESIZER_VERSION := 1.13.2
# https://github.com/kubernetes-csi/external-snapshotter/releases
EXTERNAL_SNAPSHOTTER_VERSION := 8.2.1
# https://github.com/kubernetes-csi/livenessprobe/releases
LIVENESSPROBE_VERSION := 2.15.0
# https://github.com/kubernetes-csi/node-driver-registrar/releases
NODE_DRIVER_REGISTRAR_VERSION := 2.13.0

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.32.2)
	KIND_NODE_IMAGE=kindest/node:v1.32.2@sha256:f226345927d7e348497136874b6d207e0b32cc52154ad8323129352923a3142f
else ifeq ($(KUBERNETES_VERSION), 1.31.6)
	KIND_NODE_IMAGE=kindest/node:v1.31.6@sha256:28b7cbb993dfe093c76641a0c95807637213c9109b761f1d422c2400e22b8e87
else ifeq ($(KUBERNETES_VERSION), 1.30.10)
	KIND_NODE_IMAGE=kindest/node:v1.30.10@sha256:4de75d0e82481ea846c0ed1de86328d821c1e6a6a91ac37bf804e5313670e507
endif
