# parameters
KUBERNETES_VERSION ?= 1.27.3
TEST_SCHEDULER_MANIFEST ?= daemonset
STORAGE_CAPACITY ?= false
EXTERNAL_SNAPSHOTTER_VERSION ?= v6.3.0
USE_LEGACY ?= ""

## Dependency versions
MINIKUBE_VERSION := v1.31.2
CERT_MANAGER_VERSION := v1.13.1
GINKGO_VERSION := $(shell awk '/github.com\/onsi\/ginkgo\/v2/ {print $$2}' ../go.mod)

BINDIR := $(shell pwd)/bin
SUDO := sudo
CURL := curl -sSLf
KIND_CLUSTER_NAME := topolvm-e2e
KIND := ../bin/kind
KUBECTL := $(BINDIR)/kubectl-$(KUBERNETES_VERSION)
HELM := ../bin/helm
GINKGO := $(BINDIR)/ginkgo-$(GINKGO_VERSION)

MINIKUBE_HOME = $(BINDIR)
export MINIKUBE_HOME

export TEST_KUBERNETES_VERSION=$(shell echo $(KUBERNETES_VERSION) | grep -o '^[0-9]*\.[0-9]*')

HELM_VALUES_FILE := manifests/values/daemonset-scheduler.yaml
ifeq ($(TEST_SCHEDULER_MANIFEST),deployment)
HELM_VALUES_FILE := manifests/values/deployment-scheduler.yaml
endif
ifeq ($(STORAGE_CAPACITY),true)
HELM_VALUES_FILE := manifests/values/storage-capacity.yaml
endif


KIND_CONFIG="topolvm-cluster.yaml"
MINIKUBE_FEATURE_GATES="ReadWriteOncePod=true"

HELM_VALUES_FILE_LVMD ?= manifests/values/daemonset-lvmd-storage-capacity.yaml

SCHEDULER_CONFIG := scheduler-config-$(TEST_SCHEDULER_MANIFEST).yaml

DOMAIN_NAME := topolvm.io
PROVISINER_MANIFEST := manifests/provisioner.yaml
ifeq ($(USE_LEGACY),true)
HELM_VALUES_FILE := manifests/values/daemonset-scheduler-legacy.yaml
SCHEDULER_CONFIG := scheduler-config-daemonset-legacy.yaml
DOMAIN_NAME := topolvm.cybozu.com
PROVISINER_MANIFEST := manifests/provisioner_legacy.yaml
endif


GO_FILES := $(shell find .. -path ../e2e -prune -o -name '*.go' -print)
BACKING_STORE := ./build

include ../common.mk

topolvm.img: $(GO_FILES)
	mkdir -p tmpbin
	CGO_ENABLED=0 go build -o tmpbin/hypertopolvm ../pkg/hypertopolvm
	$(MAKE) -f ../csi-sidecars.mk OUTPUT_DIR=tmpbin
	docker build --no-cache --rm=false -f Dockerfile -t topolvm:dev .
	docker save -o $@ topolvm:dev

/tmp/topolvm/scheduler/scheduler-config.yaml: $(SCHEDULER_CONFIG)
	mkdir -p /tmp/topolvm/scheduler
	sed -e "s|@DEPLOYMENT_SCHEDULER_HOST@|topolvm-e2e-worker|" $< > $@

.PHONY: prepare-namespace
prepare-namespace: $(KUBECTL)
	$(KUBECTL) create namespace topolvm-system
	$(KUBECTL) label namespace topolvm-system $(DOMAIN_NAME)/webhook=ignore
	$(KUBECTL) label namespace kube-system $(DOMAIN_NAME)/webhook=ignore

.PHONY: apply-certmanager-crds
apply-certmanager-crds: $(KUBECTL)
	$(KUBECTL) apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml

.PHONY: launch-kind
launch-kind: /tmp/topolvm/scheduler/scheduler-config.yaml $(KIND)
	$(SUDO) rm -rf /tmp/topolvm/controller /tmp/topolvm/worker*
	sed -e "s|@KUBERNETES_VERSION@|$(KUBERNETES_VERSION)|" $(KIND_CONFIG) > /tmp/$(KIND_CONFIG)
	$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config /tmp/$(KIND_CONFIG) --image $(KIND_NODE_IMAGE)

.PHONY: shutdown-kind
shutdown-kind: $(KIND)
	$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME) || true
	sleep 2
	for d in $$($(SUDO) find /tmp/topolvm -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done
	for d in $$(mount | grep /lib/kubelet | cut -d ' ' -f 3); do $(SUDO) umount $$d; done

.PHONY: start-lvmd
start-lvmd:
	mkdir -p build $(BACKING_STORE)
	go build -o build/lvmd ../pkg/lvmd
	if [ $$(ls -1 $(BACKING_STORE)/backing_store* 2>/dev/null | wc -l) -ne 0 ]; then $(MAKE) stop-lvmd; fi

	for i in $$(seq 3); do \
		mkdir -p /tmp/topolvm/worker$$i; \
		mkdir -p /tmp/topolvm/lvmd$$i; \
		truncate --size=20G $(BACKING_STORE)/backing_store$${i}_1; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$${i}_1; \
		$(SUDO) vgcreate -y node$${i}-myvg1 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_1 | cut -d: -f1); \
	done

	# Create additional Volume Groups
	for i in $$(seq 3); do \
		truncate --size=10G $(BACKING_STORE)/backing_store$${i}_2; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$${i}_2; \
		$(SUDO) vgcreate -y node$${i}-myvg2 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_2 | cut -d: -f1); \
	done

	# Create Volume Group for testing raid1 volumes.
	# This requires 2 PVs in each VG.
	# node{i}-myvg3: backing_store{i}_3, backing_store{i}_4
	for i in $$(seq 3); do \
		truncate --size=3G $(BACKING_STORE)/backing_store$${i}_3; \
		truncate --size=3G $(BACKING_STORE)/backing_store$${i}_4; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$${i}_3; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$${i}_4; \
		$(SUDO) vgcreate -y node$${i}-myvg3 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_3 | cut -d: -f1) \
			$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_4 | cut -d: -f1); \
	done

	# Create thin-p Volume Groups and thinpool
	for i in $$(seq 3); do \
		truncate --size=5G $(BACKING_STORE)/backing_store$${i}_5; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$${i}_5; \
		$(SUDO) vgcreate -y node$${i}-myvg4 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_5 | cut -d: -f1); \
		$(SUDO) lvcreate -T -n pool0 -L 4G node$${i}-myvg4; \
	done

	# Create Volume Group for testing lvcreate-option-classes.
	# This requires 2 PVs in each VG.
	# node{i}-myvg5: backing_store{i}_6, backing_store{i}_7
	for i in $$(seq 3); do \
		truncate --size=3G $(BACKING_STORE)/backing_store$${i}_6; \
		truncate --size=3G $(BACKING_STORE)/backing_store$${i}_7; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$${i}_6; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$${i}_7; \
		$(SUDO) vgcreate -y node$${i}-myvg5 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_6 | cut -d: -f1) \
			$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_7 | cut -d: -f1); \
	done

	for i in $$(seq 3); do \
		$(SUDO) systemctl reset-failed lvmd$$i.service || true; \
		$(SUDO) systemd-run --unit=lvmd$$i.service $(shell pwd)/build/lvmd --config=$(shell pwd)/lvmd$$i.yaml; \
	done

.PHONY: stop-lvmd
stop-lvmd:
	$(MAKE) shutdown-kind
	for i in $$(seq 3); do \
		if systemctl is-active -q lvmd$$i.service; then $(SUDO) systemctl stop lvmd$$i.service; fi; \
		for j in $$(seq 7); do \
			if [ "$${j}" -lt 6 ]; then \
				$(SUDO) vgremove -ffy node$${i}-myvg$${j}; \
			fi; \
			$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_$${j} | cut -d: -f1); \
			$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_$${j} | cut -d: -f1); \
			rm -f $(BACKING_STORE)/backing_store$${i}_$${j}; \
		done; \
	done

.PHONY: create-cluster
create-cluster: topolvm.img $(HELM) $(KIND) $(KUBECTL)
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	$(MAKE) apply-certmanager-crds
	$(MAKE) prepare-namespace
	$(KIND) load image-archive --name=$(KIND_CLUSTER_NAME) $<
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ -f $(HELM_VALUES_FILE) --set "controller.nodeSelector.kubernetes\.io/hostname"=$(KIND_CLUSTER_NAME)-worker
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system -l="app.kubernetes.io/component=controller,app.kubernetes.io/name=topolvm" pod
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system certificate/topolvm-mutatingwebhook

.PHONY: prepare-test
prepare-test: $(KUBECTL)
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
	$(KUBECTL) apply -f $(PROVISINER_MANIFEST)

.PHONY: run-test
run-test: GINKGO_FLAGS =
run-test: $(GINKGO)
	$(SUDO) -E env \
	PATH=${PATH} \
	E2ETEST=1 \
	KUBECTL=$(KUBECTL) \
	STORAGE_CAPACITY=$(STORAGE_CAPACITY) \
	$(GINKGO) --fail-fast -v $(GINKGO_FLAGS) .

.PHONY: test
test:
	$(MAKE) create-cluster
	$(MAKE) prepare-test
	$(MAKE) run-test

.PHONY: clean
clean: stop-lvmd
	rm -rf \
		topolvm.img \
		build/ \
		tmpbin/ \
		/tmp/topolvm/scheduler/scheduler-config.yaml

.PHONY: setup
setup:
	$(MAKE) $(GINKGO)
	$(MAKE) $(HELM)
	$(MAKE) $(KIND)
	$(MAKE) $(KUBECTL)

$(BINDIR):
	mkdir -p $@

$(GINKGO): | $(BINDIR)
	GOBIN=$(BINDIR) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)
	mv $(BINDIR)/ginkgo $@

$(HELM):
	$(MAKE) -C .. install-helm

$(KIND):
	$(MAKE) -C .. install-kind

$(KUBECTL): | $(BINDIR)
	$(CURL) -o $@ https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl
	chmod a+x $@

.PHONY: incluster-lvmd/create-vg
incluster-lvmd/create-vg:
	mkdir -p build $(BACKING_STORE)
	if [ $$(ls -1 $(BACKING_STORE)/backing_store_lvmd* 2>/dev/null | wc -l) -ne 0 ]; then $(MAKE) $(@D)/remove-vg; fi

	for i in $$(seq 3); do \
		truncate --size=20G $(BACKING_STORE)/backing_store_lvmd_$${i}; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_$${i}; \
		$(SUDO) vgcreate -y node-myvg$${i} $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
	done

	# Create Volume Group for testing raid1 volumes.
	# This requires 2 PVs in each VG.
	truncate --size=3G $(BACKING_STORE)/backing_store_lvmd_4
	truncate --size=3G $(BACKING_STORE)/backing_store_lvmd_5
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_4
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_5
	$(SUDO) vgcreate -y node-myvg4 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_4 | cut -d: -f1) \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_5 | cut -d: -f1)

	# Create Volume Group for thinpool
	truncate --size=5G $(BACKING_STORE)/backing_store_lvmd_6
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_6
	$(SUDO) vgcreate -y node-myvg5 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_6 | cut -d: -f1)
	$(SUDO) lvcreate -T -n pool0 -L 4G node-myvg5

	# Create Volume Group for testing lvcreate-option-classes.
	# This requires 2 PVs in each VG.
	truncate --size=3G $(BACKING_STORE)/backing_store_lvmd_7
	truncate --size=3G $(BACKING_STORE)/backing_store_lvmd_8
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_7
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_8
	$(SUDO) vgcreate -y node-myvg6 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_7 | cut -d: -f1) \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_8 | cut -d: -f1)


.PHONY: incluster-lvmd/remove-vg
incluster-lvmd/remove-vg:
	for i in $$(seq 8); do \
		if [ "$${j}" -lt 7 ]; then \
			$(SUDO) vgremove -ffy node-myvg$${i}; \
		fi; \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
		rm -f $(BACKING_STORE)/backing_store_lvmd_$${i}; \
	done

.PHONY: incluster-lvmd/setup-minikube
incluster-lvmd/setup-minikube:
	mkdir -p $(BINDIR)
	$(SUDO) apt-get update
	DEBIAN_FRONTEND=noninteractive $(SUDO) apt-get install -y --no-install-recommends conntrack
	$(CURL) -o $(BINDIR)/minikube https://github.com/kubernetes/minikube/releases/download/$(MINIKUBE_VERSION)/minikube-linux-amd64
	chmod a+x $(BINDIR)/minikube

.PHONY: incluster-lvmd/launch-minikube
incluster-lvmd/launch-minikube:
	@if [ "$(STORAGE_CAPACITY)" = false ]; then echo "Only storage capacity mode is supported."; exit 1; fi
	$(SUDO) -E $(BINDIR)/minikube start \
		--vm-driver=none \
		--kubernetes-version=v$(KUBERNETES_VERSION) \
		--extra-config=kubelet.read-only-port=10255 \
		--feature-gates=$(MINIKUBE_FEATURE_GATES) \
		--cni=calico
	$(SUDO) chown -R $$USER $$HOME/.kube $(MINIKUBE_HOME)/.minikube
	$(SUDO) chmod -R a+r $$HOME/.kube $(MINIKUBE_HOME)/.minikube
	$(SUDO) find $(MINIKUBE_HOME)/.minikube -name id_rsa -exec chmod 600 {} ';'

.PHONY: incluster-lvmd/delete-minikube
incluster-lvmd/delete-minikube:
	$(SUDO) -E $(BINDIR)/minikube delete || true

.PHONY: incluster-lvmd/test
incluster-lvmd/test: topolvm.img $(GINKGO) $(HELM) $(KUBECTL)
	$(MAKE) apply-certmanager-crds
	$(MAKE) prepare-namespace
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ -f $(HELM_VALUES_FILE_LVMD)
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system -l="app.kubernetes.io/component=controller,app.kubernetes.io/name=topolvm" pod
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system certificate/topolvm-mutatingwebhook
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/$(EXTERNAL_SNAPSHOTTER_VERSION)/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
	$(KUBECTL) apply -f manifests/provisioner.yaml
	$(SUDO) -E env \
	PATH=${PATH} \
	E2ETEST=1 \
	KUBECTL=$(KUBECTL) \
	STORAGE_CAPACITY=$(STORAGE_CAPACITY) \
	DAEMONSET_LVMD=true \
	$(GINKGO) --fail-fast -v .

.PHONY: incluster-lvmd/clean
incluster-lvmd/clean: incluster-lvmd/delete-minikube incluster-lvmd/remove-vg
