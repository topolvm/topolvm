KUBERNETES_VERSION := 1.23.3
TOPOLVM_VERSION := migrator
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

.PHONY: test
test: run migrate

.PHONY: run
run:
	$(MAKE) stop-lvmd
	$(MAKE) $(TMPDIR)/scheduler/scheduler-config.yaml
	$(MAKE) start-lvmd
	$(MAKE) launch-kind

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

	$(KUBECTL) apply -f podpvc.yaml
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n default pod -l app=example

.PHONY: migrate
migrate:
	$(MAKE) -C ../../example build/topolvm.img TOPOLVM_VERSION=$(TOPOLVM_VERSION)
	$(KIND) load image-archive --name=$(KIND_CLUSTER_NAME) ../../example/build/topolvm.img
	$(HELM) repo update
	$(HELM) dependency build ../../charts/topolvm/
	# $(HELM) upgrade --namespace=topolvm-system topolvm ../../charts/topolvm/ -f ./values-migrator.yaml
	# $(KUBECTL) apply -f ../../charts/topolvm/crds/topolvm.io_logicalvolumes.yaml
	# $(KUBECTL) -n topolvm-system delete pod -l app.kubernetes.io/component=controller
	# $(KUBECTL) -n topolvm-system delete pod -l app.kubernetes.io/component=node
	# $(KUBECTL) wait --for=condition=ready --timeout=180s -n topolvm-system pod -l app.kubernetes.io/component=controller
	# $(KUBECTL) wait --for=condition=ready --timeout=180s -n topolvm-system pod -l app.kubernetes.io/component=node

	rm $(TMPDIR)/scheduler/scheduler-config.yaml
	cp scheduler-config.yaml $(TMPDIR)/scheduler/scheduler-config.yaml
	# $(KUBECTL) -n kube-system delete pod -l component=kube-scheduler
	docker exec -it topolvm-migrator-control-plane systemctl restart kubelet
	docker exec -it topolvm-migrator-control-plane mv /etc/kubernetes/manifests/kube-scheduler.yaml /etc/kubernetes
	$(KUBECTL) -n kube-system delete pod -l component=kube-scheduler
	sleep 15
	docker exec -it topolvm-migrator-control-plane mv /etc/kubernetes/kube-scheduler.yaml /etc/kubernetes/manifests/
	docker exec -it topolvm-migrator-control-plane systemctl restart kubelet
	# $(KUBECTL) -n kube-system delete pod -l component=kube-scheduler
	# $(KUBECTL) wait --for=condition=ready --timeout=120s -n kube-system pod -l component=kube-scheduler

	$(KUBECTL) apply -f podpvc2.yaml
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n default pod -l app=example

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