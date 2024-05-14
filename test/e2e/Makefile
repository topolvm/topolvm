include ../../versions.mk

# parameters
# none/daemonset/deployment
TEST_SCHEDULER_EXTENDER_TYPE ?= deployment
# true/false
TEST_LEGACY ?= false
# systemd-service/daemonset/embedded
TEST_LVMD_TYPE ?= systemd-service

BACKING_STORE := ./build
BINDIR := $(shell pwd)/bin
CLUSTER_NAME := topolvm-e2e
CURL := curl -sSLf
GINKGO := $(BINDIR)/ginkgo-$(GINKGO_VERSION)
GINKGO_FLAGS :=
GO_FILES := $(shell find ../.. -path ../../test -prune -o -name '*.go' -print)
HELM := ../../bin/helm
KIND := ../../bin/kind
KIND_CONFIG := topolvm-cluster.yaml
KUBECTL := $(BINDIR)/kubectl-$(KUBERNETES_VERSION)
MINIKUBE_FEATURE_GATES="ReadWriteOncePod=true,CSIVolumeHealth=true"
MINIKUBE_HOME = $(BINDIR)
SUDO := sudo

export MINIKUBE_HOME

DOMAIN_NAME := topolvm.io
HELM_VALUES_FILES := manifests/values/base.yaml
SCHEDULER_CONFIG := ../../deploy/scheduler-config/scheduler-config.yaml
STORAGE_CAPACITY := true
USE_LEGACY :=

ifeq ($(TEST_SCHEDULER_EXTENDER_TYPE),daemonset)
HELM_VALUES_FILES += manifests/values/daemonset-scheduler.yaml
STORAGE_CAPACITY := false
else ifeq ($(TEST_SCHEDULER_EXTENDER_TYPE),deployment)
HELM_VALUES_FILES += manifests/values/deployment-scheduler.yaml
STORAGE_CAPACITY := false
endif

ifeq ($(TEST_LEGACY),true)
DOMAIN_NAME := topolvm.cybozu.com
HELM_VALUES_FILES += manifests/values/legacy.yaml
SCHEDULER_CONFIG := ../../deploy/scheduler-config/scheduler-config-legacy.yaml
USE_LEGACY := true
endif

ifeq ($(TEST_LVMD_TYPE),systemd-service)
HELM_VALUES_FILES += manifests/values/systemd-service-lvmd.yaml
else ifeq ($(TEST_LVMD_TYPE),embedded)
HELM_VALUES_FILES += manifests/values/embedded-lvmd.yaml
endif

topolvm.img: $(GO_FILES)
	mkdir -p tmpbin
	CGO_ENABLED=0 go build -o tmpbin/hypertopolvm ../../cmd/hypertopolvm
	$(MAKE) -f ../../csi-sidecars.mk OUTPUT_DIR=tmpbin
	docker build --no-cache --rm=false -f Dockerfile -t topolvm:dev .
	docker save -o $@ topolvm:dev

/tmp/topolvm/scheduler/scheduler-config.yaml: $(SCHEDULER_CONFIG)
	mkdir -p /tmp/topolvm/scheduler
ifeq ($(TEST_SCHEDULER_EXTENDER_TYPE),deployment)
	sed -e 's|\(urlPrefix:\).*|\1 "http://topolvm-e2e-worker:30251"|' $< > $@
else
	cp $< $@
endif

.PHONY: launch-kind
launch-kind: /tmp/topolvm/scheduler/scheduler-config.yaml $(KIND)
	$(SUDO) rm -rf /tmp/topolvm/controller /tmp/topolvm/worker*
	sed -e "s|@KUBERNETES_VERSION@|$(KUBERNETES_VERSION)|" $(KIND_CONFIG) > /tmp/$(KIND_CONFIG)
	$(KIND) create cluster --name=$(CLUSTER_NAME) --config /tmp/$(KIND_CONFIG) --image $(KIND_NODE_IMAGE)

.PHONY: shutdown-kind
shutdown-kind: $(KIND)
	$(KIND) delete cluster --name=$(CLUSTER_NAME) || true
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
	go build -o build/lvmd ../../cmd/lvmd
	if [ $$(ls -1 $(BACKING_STORE)/backing_store* 2>/dev/null | wc -l) -ne 0 ]; then $(MAKE) stop-lvmd; fi
	for i in $$(seq 3); do \
		mkdir -p /tmp/topolvm/worker$$i; \
		mkdir -p /tmp/topolvm/lvmd$$i; \
		$(MAKE) common/create-vg ID=$$i; \
		$(SUDO) systemctl reset-failed lvmd$$i.service || true; \
		$(SUDO) systemd-run --unit=lvmd$$i.service $(shell pwd)/build/lvmd --config=$(shell pwd)/lvmd$$i.yaml; \
	done

.PHONY: stop-lvmd
stop-lvmd:
	$(MAKE) shutdown-kind
	for i in $$(seq 3); do \
		if systemctl is-active -q lvmd$$i.service; then $(SUDO) systemctl stop lvmd$$i.service; fi; \
		$(MAKE) common/remove-vg ID=$$i; \
	done

.PHONY: create-cluster
create-cluster: topolvm.img $(KIND)
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	$(KIND) load image-archive --name=$(CLUSTER_NAME) $<
	$(MAKE) common/setup-components

.PHONY: test
test:
	$(MAKE) create-cluster
	$(MAKE) common/test

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
	$(MAKE) -C ../.. install-helm

$(KIND):
	$(MAKE) -C ../.. install-kind

$(KUBECTL): | $(BINDIR)
	$(CURL) -o $@ https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl
	chmod a+x $@

.PHONY: incluster-lvmd/create-vg
incluster-lvmd/create-vg:
	mkdir -p build $(BACKING_STORE)
	if [ $$(ls -1 $(BACKING_STORE)/backing_store_lvmd* 2>/dev/null | wc -l) -ne 0 ]; then $(MAKE) $(@D)/remove-vg; fi
	$(MAKE) common/create-vg ID=1

.PHONY: incluster-lvmd/remove-vg
incluster-lvmd/remove-vg:
	$(MAKE) common/remove-vg ID=1

.PHONY: incluster-lvmd/setup-minikube
incluster-lvmd/setup-minikube:
	mkdir -p $(BINDIR)
	$(SUDO) apt-get update
	DEBIAN_FRONTEND=noninteractive $(SUDO) apt-get install -y --no-install-recommends conntrack
	$(CURL) -o $(BINDIR)/minikube https://github.com/kubernetes/minikube/releases/download/$(MINIKUBE_VERSION)/minikube-linux-amd64
	chmod a+x $(BINDIR)/minikube

	# These tools are required to use minikube for Kubernetes v1.24+
	# For CNI plugins, see https://github.com/Mirantis/cri-dockerd/blob/v0.2.6/README.md#important
	$(CURL) -o cni-plugins.tgz https://github.com/containernetworking/plugins/releases/download/$(CNI_PLUGINS_VERSION)/cni-plugins-linux-amd64-$(CNI_PLUGINS_VERSION).tgz
	$(SUDO) mkdir -p /opt/cni/bin
	$(SUDO) tar -C /opt/cni/bin -xf cni-plugins.tgz

	# Install cri-docker
	$(CURL) -o cri-dockerd.deb https://github.com/Mirantis/cri-dockerd/releases/download/$(CRI_DOCKERD_VERSION)/cri-dockerd_$(CRI_DOCKERD_VERSION:v%=%).3-0.ubuntu-focal_amd64.deb
	$(SUDO) dpkg -i cri-dockerd.deb

	$(CURL) -o crictl.tar.gz https://github.com/kubernetes-sigs/cri-tools/releases/download/$(CRICTL_VERSION)/crictl-$(CRICTL_VERSION)-linux-amd64.tar.gz
	$(SUDO) tar -C /usr/local/bin -xf crictl.tar.gz

.PHONY: incluster-lvmd/launch-minikube
incluster-lvmd/launch-minikube:
	$(SUDO) -E $(BINDIR)/minikube start \
		--vm-driver=none \
		--kubernetes-version=v$(KUBERNETES_VERSION) \
		--extra-config=kubelet.read-only-port=10255 \
		--extra-config=kubeadm.node-name=$(CLUSTER_NAME)-worker \
		--extra-config=kubelet.hostname-override=$(CLUSTER_NAME)-worker \
		--feature-gates=$(MINIKUBE_FEATURE_GATES) \
		--cni=calico
	$(SUDO) chown -R $$USER $$HOME/.kube $(MINIKUBE_HOME)/.minikube
	$(SUDO) chmod -R a+r $$HOME/.kube $(MINIKUBE_HOME)/.minikube
	$(SUDO) find $(MINIKUBE_HOME)/.minikube -name id_rsa -exec chmod 600 {} ';'

.PHONY: incluster-lvmd/delete-minikube
incluster-lvmd/delete-minikube:
	$(SUDO) -E $(BINDIR)/minikube delete || true

.PHONY: incluster-lvmd/test
# topolvm.img is to trigger the build of docker image
incluster-lvmd/test: topolvm.img
	@if [ "$(TEST_SCHEDULER_EXTENDER_TYPE)" != none ]; then \
		echo "Using the scheduler extender is not supported"; \
		exit 1; \
	fi
	@if [ "$(TEST_LVMD_TYPE)" = systemd-service ]; then \
		echo "Running LVMd as systemd-service is not supported"; \
		exit 1; \
	fi
	$(MAKE) common/setup-components
	$(MAKE) common/test

.PHONY: incluster-lvmd/clean
incluster-lvmd/clean: incluster-lvmd/delete-minikube incluster-lvmd/remove-vg

.PHONY: common/create-vg
common/create-vg:
	for i in $$(seq 2); do \
		truncate --size=20G $(BACKING_STORE)/backing_store$(ID)_$${i}; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_$${i}; \
		$(SUDO) vgcreate -y node$(ID)-thick$${i} $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_$${i} | cut -d: -f1); \
	done

	# Create Volume Groups for testing raid1 volumes and lvcreate-option-classes.
	# This requires 2 PVs in each VG.
	for i in $$(seq 2); do \
		truncate --size=3G $(BACKING_STORE)/backing_store$(ID)_$$((i*2+1)); \
		truncate --size=3G $(BACKING_STORE)/backing_store$(ID)_$$((i*2+2)); \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_$$((i*2+1)); \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_$$((i*2+2)); \
		$(SUDO) vgcreate -y node$(ID)-raid1-$${i} \
			$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_$$((i*2+1)) | cut -d: -f1) \
			$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_$$((i*2+2)) | cut -d: -f1); \
	done

	# Create Volume Group for thinpool
	truncate --size=5G $(BACKING_STORE)/backing_store$(ID)_7
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_7
	$(SUDO) vgcreate -y node$(ID)-thin1 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_7 | cut -d: -f1)
	$(SUDO) lvcreate -T -n pool0 -L 4G node$(ID)-thin1
	# Create Volume Group for Volume Health Monitoring Failure Check
	truncate --size=2G $(BACKING_STORE)/backing_store$(ID)_8
	truncate --size=2G $(BACKING_STORE)/backing_store$(ID)_9
	# Setup loop devices
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_8; \
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_9; \
	echo test | $(SUDO) cryptsetup open --type plain \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_8 | cut -d: -f1) crypt-$(ID)
	$(SUDO) vgcreate -y node$(ID)-volume-health \
		/dev/mapper/crypt-$(ID) \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_9 | cut -d: -f1)

.PHONY: common/remove-vg
common/remove-vg:
	for name in thick1 thick2 raid1-1 raid1-2 thin1 volume-health; do \
		$(SUDO) vgremove -ffy node$(ID)-$${name}; \
	done; \
	$(SUDO) dmsetup remove -f /dev/mapper/crypt-$(ID) || true
	for i in $$(seq 9); do \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_$${i} | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_$${i} | cut -d: -f1); \
		rm -f $(BACKING_STORE)/backing_store$(ID)_$${i}; \
	done

.PHONY: common/setup-components
common/setup-components: $(HELM) $(KUBECTL)
	@echo apply certmanager CRDs
	$(KUBECTL) apply -f https://github.com/cert-manager/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml

	@echo install TopoLVM
	$(KUBECTL) create namespace topolvm-system
	$(KUBECTL) label namespace topolvm-system $(DOMAIN_NAME)/webhook=ignore
	$(KUBECTL) label namespace kube-system $(DOMAIN_NAME)/webhook=ignore
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../../charts/topolvm/ $(patsubst %,-f %,$(HELM_VALUES_FILES))
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system -l="app.kubernetes.io/component=controller,app.kubernetes.io/name=topolvm" pod
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system certificate/topolvm-mutatingwebhook

	@echo apply VolumeSnapshot manifests
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
	sed 's/@DOMAIN_NAME@/$(DOMAIN_NAME)/' manifests/volumesnapshotclass_tpl.yaml | $(KUBECTL) apply -f -

.PHONY: common/test
common/test: $(GINKGO)
	$(SUDO) -E env \
	PATH=${PATH} \
	E2ETEST=1 \
	KUBECTL=$(KUBECTL) \
	STORAGE_CAPACITY=$(STORAGE_CAPACITY) \
	USE_LEGACY=$(USE_LEGACY) \
	$(GINKGO) --fail-fast -v $(GINKGO_FLAGS) .
