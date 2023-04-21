KUBERNETES_VERSION := 1.26.3
HELM_VERSION := 3.11.3
CERT_MANAGER_VERSION := v1.11.1

KIND_CLUSTER_NAME := topolvm-example

SUDO := sudo
CURL := curl -sSLf
BINDIR := $(shell pwd)/bin
TMPDIR := /tmp/topolvm-example
HELM := $(BINDIR)/helm
KUBECTL := $(BINDIR)/kubectl
KIND := ../bin/kind

GO_FILES := $(shell find .. -prune -o -path ../e2e -prune -o -name '*.go' -print)
BACKING_STORE := ./build

BUILD_IMAGE := false

HELM_VALUES_FILE := values.yaml
SCHEDULER_CONFIG := scheduler-config.yaml
LOGICALVOLUME_GK_NAME := logicalvolume.topolvm.io
DOMAIN_NAME := topolvm.io

BUILD_IMAGE_FLAG :=
ifeq ($(BUILD_IMAGE),true)
BUILD_IMAGE_FLAG := --set image.tag=devel
endif

$(KUBECTL):
	mkdir -p $(BINDIR)
	$(CURL) https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	chmod 755 $(KUBECTL)

build/topolvm.img: $(GO_FILES)
	$(MAKE) -C .. image-with-sidecar IMAGE_PREFIX=ghcr.io/topolvm/
	docker save -o $@ ghcr.io/topolvm/topolvm-with-sidecar:devel

.PHONY: run
run:
	rm -f $(TMPDIR)/scheduler/scheduler-config.yaml
	$(MAKE) start-lvmd
	$(MAKE) launch-kind
	if [ "$(BUILD_IMAGE)" = "true" ]; then \
		$(MAKE) build/topolvm.img; \
		$(KIND) load image-archive --name=$(KIND_CLUSTER_NAME) build/topolvm.img; \
	fi
	$(KUBECTL) apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml
	$(KUBECTL) create namespace topolvm-system
	$(KUBECTL) label namespace topolvm-system $(DOMAIN_NAME)/webhook=ignore
	$(KUBECTL) label namespace kube-system $(DOMAIN_NAME)/webhook=ignore
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ -f $(HELM_VALUES_FILE) $(BUILD_IMAGE_FLAG)
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system -l="app.kubernetes.io/component=controller,app.kubernetes.io/name=topolvm" pod
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system certificate/topolvm-mutatingwebhook
	timeout 120 sh -c "until $(KUBECTL) apply -f podpvc.yaml; do sleep 10; done"
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n default pod -l app=example
	[ $$($(KUBECTL) get --no-headers=true $(LOGICALVOLUME_GK_NAME) | wc -l) -ne 0 ]

.PHONY: setup
setup: $(KUBECTL)
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs thin-provisioning-tools
	cd ..; $(MAKE) install-kind
	mkdir -p $(BINDIR)
	mkdir -p build
	$(CURL) https://get.helm.sh/helm-v$(HELM_VERSION)-linux-amd64.tar.gz \
	  | tar xvz -C $(BINDIR) --strip-components 1 linux-amd64/helm

.PHONY: stop-lvmd
clean: stop-lvmd
	rm -rf bin/ build/ $(TMPDIR)

$(TMPDIR)/scheduler/scheduler-config.yaml: ../deploy/scheduler-config/$(SCHEDULER_CONFIG)
	mkdir -p $(TMPDIR)/scheduler
	cp $< $@

$(TMPDIR)/lvmd/lvmd.yaml: ../deploy/lvmd-config/lvmd.yaml
	mkdir -p $(TMPDIR)/lvmd
	sed -e 's=/run/topolvm/lvmd.sock=$(TMPDIR)/lvmd/lvmd.sock=; s=spare-gb: 10=spare-gb: 1=' $< > $@

.PHONY: launch-kind
launch-kind: $(TMPDIR)/scheduler/scheduler-config.yaml
	$(MAKE) shutdown-kind
	$(SUDO) rm -rf $(TMPDIR)/controller $(TMPDIR)/worker*
	sed 's/@KUBERNETES_VERSION@/$(KUBERNETES_VERSION)/; s=@TMPDIR@=$(TMPDIR)='  kind/topolvm-cluster.yaml > $(TMPDIR)/topolvm-cluster.yaml
	$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config $(TMPDIR)/topolvm-cluster.yaml --image kindest/node:v$(KUBERNETES_VERSION)

.PHONY: shutdown-kind
shutdown-kind:
	$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME) || true
	for d in $$($(SUDO) find $(TMPDIR) -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done

.PHONY: start-lvmd
start-lvmd: $(TMPDIR)/lvmd/lvmd.yaml
	go build -o build/lvmd ../pkg/lvmd
	if [ -f $(BACKING_STORE)/backing_store ]; then $(MAKE) stop-lvmd; fi; \
	mkdir -p $(TMPDIR)/worker; \
	mkdir -p $(TMPDIR)/lvmd; \
	truncate --size=20G $(BACKING_STORE)/backing_store; \
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store; \
	$(SUDO) vgcreate -f -y myvg1 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store | cut -d: -f1); \
	$(SUDO) lvcreate -T myvg1/thinpool -L 12G; \
	$(SUDO) systemd-run --unit=lvmd.service $(shell pwd)/build/lvmd --config=$(TMPDIR)/lvmd/lvmd.yaml; \

.PHONY: stop-lvmd
stop-lvmd:
	$(MAKE) shutdown-kind
	if systemctl is-active -q lvmd.service; then $(SUDO) systemctl stop lvmd.service; fi; \
	if [ -f $(BACKING_STORE)/backing_store ]; then \
		$(SUDO) vgremove -ffy myvg1; \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store | cut -d: -f1); \
		rm -f $(BACKING_STORE)/backing_store; \
	fi
