KUBERNETES_VERSION := 1.23.3
TOPOLVM_VERSION := v0.14.1
KIND_CLUSTER_NAME := "topolvm-migrator"
CERT_MANAGER_VERSION := v1.7.0

# GINKGO_VERSION := $(shell awk '/github.com\/onsi\/ginkgo/ {print substr($$2, 2)}' ../../go.mod)

BINDIR := $(shell pwd)/../../example/bin
TMPDIR := /tmp/topolvm
HELM := $(BINDIR)/helm
KUBECTL := $(BINDIR)/kubectl
KIND := $(shell pwd)/../../bin/kind

.PHONY: setup
setup:
	$(MAKE) -C ../../example $@

$(TMPDIR)/scheduler/scheduler-config.yaml: legacy-scheduler-config.yaml
	mkdir -p $(TMPDIR)/scheduler
	cp $< $@

.PHONY: run
run:
	$(MAKE) stop-lvmd
	$(MAKE) $(TMPDIR)/scheduler/scheduler-config.yaml
	$(MAKE) start-lvmd
	$(MAKE) launch-kind

	# $(KUBECTL) apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.yaml
	$(KUBECTL) apply -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml
	$(KUBECTL) create namespace topolvm-system
	$(KUBECTL) label namespace topolvm-system topolvm.io/webhook=ignore
	$(KUBECTL) label namespace kube-system topolvm.io/webhook=ignore
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo add topolvm https://topolvm.github.io/topolvm
	$(HELM) repo update
	$(HELM) install --namespace=topolvm-system --version=8.0.1 -f ./values.yaml topolvm topolvm/topolvm

	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system certificate/topolvm-mutatingwebhook

.PHONY: start-lvmd
start-lvmd:
	$(MAKE) -C ../../example $@ KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME)

.PHONY: launch-kind
launch-kind:
	$(MAKE) -C ../../example $@ KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME) KUBERNETES_VERSION=$(KUBERNETES_VERSION)

.PHONY: shutdown-kind
shutdown-kind:
	$(MAKE) -C ../../example $@ KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME)

.PHONY: stop-lvmd
stop-lvmd:
	$(MAKE) -C ../../example $@ KIND_CLUSTER_NAME=$(KIND_CLUSTER_NAME)
	rm -fr $(TMPDIR)