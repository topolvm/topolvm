USE_LEGACY ?= ""

KUBERNETES_VERSION := 1.24.2
HELM_VERSION := 3.9.0
TOPOLVM_VERSION := 0.15.1
CERT_MANAGER_VERSION := v1.7.0

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
LABEL_KEY := topolvm.io
ifneq ($(USE_LEGACY),"")
HELM_VALUES_FILE := values_legacy.yaml
SCHEDULER_CONFIG := scheduler-config-legacy.yaml
LOGICALVOLUME_GK_NAME := logicalvolume.topolvm.cybozu.com
LABEL_KEY := topolvm.cybozu.com
endif

$(KUBECTL):
	mkdir -p $(BINDIR)
	$(CURL) https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	chmod 755 $(KUBECTL)

build/topolvm.img: $(GO_FILES)
	$(MAKE) -C .. image IMAGE_PREFIX=ghcr.io/topolvm/
	$(MAKE) -C .. tag IMAGE_PREFIX=ghcr.io/topolvm/ IMAGE_TAG=$(TOPOLVM_VERSION)
	docker save -o $@ ghcr.io/topolvm/topolvm-with-sidecar:$(TOPOLVM_VERSION)

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
	$(KUBECTL) label namespace topolvm-system $(LABEL_KEY)/webhook=ignore
	$(KUBECTL) label namespace kube-system $(LABEL_KEY)/webhook=ignore
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ -f $(HELM_VALUES_FILE)
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
	sed -e 's=/run/topolvm/lvmd.sock=$(TMPDIR)/lvmd/lvmd.sock=' $< > $@

.PHONY: launch-kind
launch-kind: $(TMPDIR)/scheduler/scheduler-config.yaml kind/topolvm-cluster.yaml
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
	$(SUDO) lvcreate -T myvg1/thinpool -L 2G; \
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

$(TMPDIR)/scheduler/kube-scheduler.yaml: migrate/kube-scheduler.yaml
	mkdir -p $(TMPDIR)/scheduler
	cp $< $@

.PHONY: migration-test
migration-test: build/topolvm.io_logicalvolumes.yaml
	$(MAKE) run BUILD_IMAGE=true USE_LEGACY=true

	# Stopping legacy topolvm
	$(KUBECTL) -n topolvm-system delete deploy topolvm-controller topolvm-cert-manager topolvm-cert-manager-cainjector topolvm-cert-manager-webhook
	$(KUBECTL) -n topolvm-system delete ds topolvm-node topolvm-scheduler
	$(KUBECTL) delete storageclass topolvm-provisioner topolvm-provisioner-thin
	$(KUBECTL) delete csidriver topolvm.cybozu.com
	$(KUBECTL) delete clusterrole \
		topolvm-cert-manager-cainjector \
		topolvm-cert-manager-controller-approve:cert-manager-io \
		topolvm-cert-manager-controller-certificates \
		topolvm-cert-manager-controller-certificatesigningrequests \
		topolvm-cert-manager-controller-challenges \
		topolvm-cert-manager-controller-clusterissuers \
		topolvm-cert-manager-controller-ingress-shim \
		topolvm-cert-manager-controller-issuers \
		topolvm-cert-manager-controller-orders \
		topolvm-cert-manager-edit \
		topolvm-cert-manager-view \
		topolvm-cert-manager-webhook:subjectaccessreviews \
		topolvm-external-provisioner-runner \
		topolvm-external-resizer-runner \
		topolvm-external-snapshotter-runner \
		topolvm-system:controller \
		topolvm-system:node
	$(KUBECTL) delete clusterrolebinding \
		topolvm-cert-manager-cainjector \
		topolvm-cert-manager-controller-approve:cert-manager-io \
		topolvm-cert-manager-controller-certificates \
		topolvm-cert-manager-controller-certificatesigningrequests \
		topolvm-cert-manager-controller-challenges \
		topolvm-cert-manager-controller-clusterissuers \
		topolvm-cert-manager-controller-ingress-shim \
		topolvm-cert-manager-controller-issuers \
		topolvm-cert-manager-controller-orders \
		topolvm-cert-manager-webhook:subjectaccessreviews \
		topolvm-csi-provisioner-role \
		topolvm-csi-resizer-role \
		topolvm-csi-snapshotter-role \
		topolvm-system:controller \
		topolvm-system:node
	$(KUBECTL) -n kube-system delete role \
		topolvm-cert-manager-cainjector:leaderelection \
		topolvm-cert-manager:leaderelection
	$(KUBECTL) -n kube-system delete rolebinding \
		topolvm-cert-manager-cainjector:leaderelection \
		topolvm-cert-manager:leaderelection
	$(KUBECTL) delete mutatingwebhookconfiguration \
		topolvm-cert-manager-webhook \
		topolvm-hook
	$(KUBECTL) delete validatingwebhookconfiguration topolvm-cert-manager-webhook
	$(KUBECTL) delete priorityclass topolvm

	# Installing the new TopoLVM
	$(KUBECTL) -n kube-system delete pod -l component=kube-scheduler,tier=control-plane
	$(KUBECTL) create namespace topolvm-system2
	$(KUBECTL) label namespace topolvm-system2 topolvm.io/webhook=ignore
	$(HELM) install --namespace=topolvm-system2 topolvm ../charts/topolvm/ -f values.yaml
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system2 deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system2 -l="app.kubernetes.io/component=controller,app.kubernetes.io/name=topolvm" pod
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system2 certificate/topolvm-mutatingwebhook
	cp ../deploy/scheduler-config/scheduler-config.yaml $(TMPDIR)/scheduler/scheduler-config.yaml
	docker exec -it $$(docker ps -f name=topolvm-example-control-plane --format="{{.ID}}") cp /mnt/host/scheduler/kube-scheduler.yaml /etc/kubernetes/manifests/kube-scheduler.yaml

	# Migration resources
	$(KUBECTL) -n topolvm-system2 patch ds topolvm-node -p '{"spec": {"template": {"spec": {"nodeSelector": {"unschedule": "true"}}}}}'
	go run cmd/migration-example/main.go
	$(KUBECTL) delete -f ../config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml
	$(KUBECTL) delete ns topolvm-system
	$(KUBECTL) -n topolvm-system2 patch ds topolvm-node --type json -p='[{"op": "remove", "path": "/spec/template/spec/nodeSelector/unschedule"}]'
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system2 -l="app.kubernetes.io/component=node,app.kubernetes.io/name=topolvm" pod
	sleep 30

	# Checking if the new TopoLVM operating correctly(existing volumes)
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n default pod -l app=example
	$(KUBECTL) exec my-pod -- touch /test1/file
	$(KUBECTL) exec my-pod -- ls -la /test1/file
	$(KUBECTL) delete -n default pod my-pod
	timeout 120 sh -c "until $(KUBECTL) apply -f podpvc.yaml; do sleep 10; done"
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n default pod my-pod

	# Checking if the new TopoLVM operating correctly(new volumes)
	timeout 120 sh -c "until $(KUBECTL) apply -f migrate/podpvc.yaml; do sleep 10; done"
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n default pod -l app=example
