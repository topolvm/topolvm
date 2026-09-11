# https://github.com/docker/buildx/releases
BUILDX_VERSION := 0.37.0
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
CRI_DOCKERD_VERSION := v0.4.4
# https://github.com/kubernetes-sigs/cri-tools/releases
CRICTL_VERSION := v1.36.0
# https://github.com/rhysd/actionlint/releases
ACTIONLINT_VERSION := v1.7.12
# https://github.com/suzuki-shunsuke/ghalint/releases
GHALINT_VERSION := v1.5.6
# https://github.com/zizmorcore/zizmor/releases
ZIZMOR_VERSION := 1.30.1
# SHA256 checksum of the zizmor release tarball for verification
ZIZMOR_SHA256 := e65324f4430c2717591937edcec90ccbefaf14c174f8ec9415e03ca875b46e1a
# https://github.com/golangci/golangci-lint/releases
GOLANGCI_LINT_VERSION := v2.13.2
# https://github.com/norwoodj/helm-docs/releases
HELM_DOCS_VERSION := 1.14.2
# https://github.com/helm/helm/releases
HELM_VERSION := 4.3.0
# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
# https://github.com/kubernetes-sigs/kind/releases
KIND_VERSION := v0.33.0
# It is set by CI using the environment variable, use conditional assignment.
# Use a Kubernetes version supported by the minikube version below.
# The patch version may differ from the k8s patch version in go.mod.
KUBERNETES_VERSION ?= 1.36.4
# https://github.com/kubernetes/minikube/releases
MINIKUBE_VERSION := v1.39.0
# https://github.com/protocolbuffers/protobuf/releases
PROTOC_VERSION := 36.1
# https://github.com/mikefarah/yq/releases
YQ_VERSION := 4.53.6

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
EXTERNAL_PROVISIONER_VERSION := 6.3.0
# https://github.com/kubernetes-csi/external-resizer/releases
EXTERNAL_RESIZER_VERSION := 2.2.1
# https://github.com/kubernetes-csi/external-snapshotter/releases
EXTERNAL_SNAPSHOTTER_VERSION := 8.6.0
# https://github.com/kubernetes-csi/livenessprobe/releases
LIVENESSPROBE_VERSION := 2.20.0
# https://github.com/kubernetes-csi/node-driver-registrar/releases
NODE_DRIVER_REGISTRAR_VERSION := 2.18.0

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
# NOTE: If kind does not have a prebuilt image for the exact patch version,
# we use the image from the latest available patch version for the same minor version.
ifeq ($(KUBERNETES_VERSION), 1.36.4)
	KIND_NODE_IMAGE=kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed
else ifeq ($(KUBERNETES_VERSION), 1.35.8)
	KIND_NODE_IMAGE=kindest/node:v1.35.8@sha256:07b2536e30b803ed61d1677a79df6115f798ce64c80f9e22f6ed45afd09323c0
else ifeq ($(KUBERNETES_VERSION), 1.34.11)
	KIND_NODE_IMAGE=kindest/node:v1.34.11@sha256:44e222ee2132dab25ff87301682f89eb82c7880ea3a1bf543bfe9708fd08d67d
endif
