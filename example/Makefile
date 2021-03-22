
KUBERNETES_VERSION=1.20.2
KUSTOMIZE_VERSION= 3.8.9
KIND_VERSION=0.10.0
TOPOLVM_VERSION=0.8.1

KIND_CLUSTER_NAME=topolvm-example

SUDO=sudo
BINDIR=$(PWD)/bin
KUSTOMIZE=$(BINDIR)/kustomize
KUBECTL=$(BINDIR)/kubectl

GO_FILES := $(shell find .. -prune -o -path ../e2e -prune -o -name '*.go' -print)
SERVER_CERT_FILES=./certs/cert.pem ./certs/key.pem
BACKING_STORE=./build

BUILD_IMAGE?=false

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
		kind load image-archive --name=$(KIND_CLUSTER_NAME) build/topolvm.img; \
	fi
	$(KUSTOMIZE) build --enable_kyaml=false . | $(KUBECTL) apply -f -
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/controller
	$(KUBECTL) apply -f podpvc.yaml

setup: $(KUBECTL)
	go install sigs.k8s.io/kind@v$(KIND_VERSION)
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs
	mkdir -p $(BINDIR)
	mkdir -p build
	curl -sSLf https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar -xz -C $(BINDIR)
	chmod 755 $(KUSTOMIZE)

clean: stop-lvmd
	rm -rf bin/ build/

/tmp/topolvm/scheduler/scheduler-config.yaml: ../deploy/scheduler-config/scheduler-config-v1beta1.yaml
	mkdir -p /tmp/topolvm/scheduler
	cp $< $@

/tmp/topolvm/lvmd/lvmd.yaml: ../deploy/lvmd-config/lvmd.yaml
	mkdir -p /tmp/topolvm/lvmd
	sed -e 's/\/run\/topolvm\/lvmd.sock/\/tmp\/topolvm\/lvmd\/lvmd.sock/' $< > $@

launch-kind: /tmp/topolvm/scheduler/scheduler-config.yaml kind/topolvm-cluster.yaml
	$(MAKE) shutdown-kind
	$(SUDO) rm -rf /tmp/topolvm/controller /tmp/topolvm/worker*
	sed s/@KUBERNETES_VERSION@/$(KUBERNETES_VERSION)/ kind/topolvm-cluster.yaml > /tmp/topolvm-cluster.yaml
	kind create cluster --name=$(KIND_CLUSTER_NAME) --config /tmp/topolvm-cluster.yaml --image kindest/node:v$(KUBERNETES_VERSION)

shutdown-kind:
	kind delete cluster --name=$(KIND_CLUSTER_NAME) || true
	for d in $$($(SUDO) find /tmp/topolvm -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done

start-lvmd: /tmp/topolvm/lvmd/lvmd.yaml
	go build -o build/lvmd ../pkg/lvmd
	if [ -f $(BACKING_STORE)/backing_store ]; then $(MAKE) stop-lvmd; fi; \
	mkdir -p /tmp/topolvm/worker; \
	mkdir -p /tmp/topolvm/lvmd; \
	truncate --size=20G $(BACKING_STORE)/backing_store; \
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store; \
	$(SUDO) vgcreate -f -y myvg1 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store | cut -d: -f1); \
	$(SUDO) systemd-run --unit=lvmd.service $(shell pwd)/build/lvmd --config=/tmp/topolvm/lvmd/lvmd.yaml; \

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
