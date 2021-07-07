
KUBERNETES_VERSION := 1.21.1
KUSTOMIZE_VERSION := 3.8.9
TOPOLVM_VERSION := 0.8.3

KIND_CLUSTER_NAME := topolvm-example

SUDO := sudo
BINDIR := $(shell pwd)/bin
TMPDIR := /tmp/topolvm
KUSTOMIZE := $(BINDIR)/kustomize
KUBECTL := $(BINDIR)/kubectl
KIND := ../bin/kind

GO_FILES := $(shell find .. -prune -o -path ../e2e -prune -o -name '*.go' -print)
SERVER_CERT_FILES := ./certs/cert.pem ./certs/key.pem
BACKING_STORE := ./build

BUILD_IMAGE := false

$(KUBECTL):
	mkdir -p $(BINDIR)
	curl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	chmod 755 $(KUBECTL)

build/topolvm.img: $(GO_FILES)
	$(MAKE) -C .. image IMAGE_PREFIX=quay.io/topolvm/
	$(MAKE) -C .. tag IMAGE_PREFIX=quay.io/topolvm/ IMAGE_TAG=$(TOPOLVM_VERSION)
	docker save -o $@ quay.io/topolvm/topolvm-with-sidecar:$(TOPOLVM_VERSION)

run: $(SERVER_CERT_FILES)
	$(MAKE) start-lvmd
	$(MAKE) launch-kind
	if [ "$(BUILD_IMAGE)" = "true" ]; then \
		$(MAKE) build/topolvm.img; \
		$(KIND) load image-archive --name=$(KIND_CLUSTER_NAME) build/topolvm.img; \
	fi
	$(KUSTOMIZE) build --enable_kyaml=false . | $(KUBECTL) apply -f -
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/controller
	$(KUBECTL) apply -f podpvc.yaml

setup: $(KUBECTL)
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs
	cd ..; $(MAKE) install-kind
	mkdir -p $(BINDIR)
	mkdir -p build
	curl -sSLf https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar -xz -C $(BINDIR)
	chmod 755 $(KUSTOMIZE)

clean: stop-lvmd
	rm -rf bin/ build/

$(TMPDIR)/scheduler/scheduler-config.yaml: ../deploy/scheduler-config/scheduler-config-v1beta1.yaml
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

$(SERVER_CERT_FILES):
	mkdir -p certs
	go run ../pkg/gencert -outdir ./certs

.PHONY: launch-kind shutdown-kind start-lvmd stop-lvmd run setup clean
