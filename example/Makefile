KUBERNETES_VERSION := 1.24.2
HELM_VERSION := 3.9.0
TOPOLVM_VERSION := 0.14.1
CERT_MANAGER_VERSION := v1.7.0

KIND_CLUSTER_NAME := topolvm-example

SUDO := sudo
CURL := curl -sSLf
BINDIR := $(shell pwd)/bin
TMPDIR := /tmp/topolvm
HELM := $(BINDIR)/helm
KUBECTL := $(BINDIR)/kubectl
KIND := ../bin/kind

GO_FILES := $(shell find .. -prune -o -path ../e2e -prune -o -name '*.go' -print)
BACKING_STORE := ./build

BUILD_IMAGE := false

$(KUBECTL):
	mkdir -p $(BINDIR)
	$(CURL) https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	chmod 755 $(KUBECTL)

build/topolvm.img: $(GO_FILES)
	$(MAKE) -C .. image IMAGE_PREFIX=ghcr.io/topolvm/
	$(MAKE) -C .. tag IMAGE_PREFIX=ghcr.io/topolvm/ IMAGE_TAG=$(TOPOLVM_VERSION)
	docker save -o $@ ghcr.io/topolvm/topolvm-with-sidecar:$(TOPOLVM_VERSION)

run:
	$(MAKE) start-lvmd
	$(MAKE) launch-kind
	if [ "$(BUILD_IMAGE)" = "true" ]; then \
		$(MAKE) build/topolvm.img; \
		$(KIND) load image-archive --name=$(KIND_CLUSTER_NAME) build/topolvm.img; \
	fi
	$(KUBECTL) apply -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml
	$(KUBECTL) create namespace topolvm-system
	$(KUBECTL) label namespace topolvm-system topolvm.cybozu.com/webhook=ignore
	$(KUBECTL) label namespace kube-system topolvm.cybozu.com/webhook=ignore
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ -f ./values.yaml
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system certificate/topolvm-mutatingwebhook
	timeout 120 sh -c "until $(KUBECTL) apply -f podpvc.yaml; do sleep 10; done"
	$(KUBECTL) wait --for=condition=ready --timeout=60s -n default pod -l app=example

setup: $(KUBECTL)
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs thin-provisioning-tools
	cd ..; $(MAKE) install-kind
	mkdir -p $(BINDIR)
	mkdir -p build
	$(CURL) https://get.helm.sh/helm-v$(HELM_VERSION)-linux-amd64.tar.gz \
	  | tar xvz -C $(BINDIR) --strip-components 1 linux-amd64/helm

clean: stop-lvmd
	rm -rf bin/ build/

$(TMPDIR)/scheduler/scheduler-config.yaml: ../deploy/scheduler-config/scheduler-config-v1beta2.yaml
	mkdir -p $(TMPDIR)/scheduler
	cp $< $@

$(TMPDIR)/lvmd/lvmd.yaml: ../deploy/lvmd-config/lvmd.yaml
	mkdir -p $(TMPDIR)/lvmd
	sed -e 's=/run/topolvm/lvmd.sock=$(TMPDIR)/lvmd/lvmd.sock=' $< > $@

launch-kind: $(TMPDIR)/scheduler/scheduler-config.yaml kind/topolvm-cluster.yaml
	$(MAKE) shutdown-kind
	$(SUDO) rm -rf $(TMPDIR)/controller $(TMPDIR)/worker*
	sed 's/@KUBERNETES_VERSION@/$(KUBERNETES_VERSION)/; s=@TMPDIR@=$(TMPDIR)='  kind/topolvm-cluster.yaml > $(TMPDIR)/topolvm-cluster.yaml
	$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config $(TMPDIR)/topolvm-cluster.yaml --image kindest/node:v$(KUBERNETES_VERSION)

shutdown-kind:
	$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME) || true
	for d in $$($(SUDO) find $(TMPDIR) -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done

start-lvmd: $(TMPDIR)/lvmd/lvmd.yaml
	go build -o build/lvmd ../pkg/lvmd
	if [ -f $(BACKING_STORE)/backing_store ]; then $(MAKE) stop-lvmd; fi; \
	mkdir -p $(TMPDIR)/worker; \
	mkdir -p $(TMPDIR)/lvmd; \
	truncate --size=20G $(BACKING_STORE)/backing_store; \
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store; \
	$(SUDO) vgcreate -f -y myvg1 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store | cut -d: -f1); \
	$(SUDO) lvcreate -T myvg1/thinpool -L 2G; \
	$(SUDO) systemd-run --unit=lvmd.service $(shell pwd)/build/lvmd --config=$(TMPDIR)/lvmd/lvmd.yaml; \

stop-lvmd:
	$(MAKE) shutdown-kind
	if systemctl is-active -q lvmd.service; then $(SUDO) systemctl stop lvmd.service; fi; \
	if [ -f $(BACKING_STORE)/backing_store ]; then \
		$(SUDO) vgremove -ffy myvg1; \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store | cut -d: -f1); \
		rm -f $(BACKING_STORE)/backing_store; \
	fi

.PHONY: launch-kind shutdown-kind start-lvmd stop-lvmd run setup clean
