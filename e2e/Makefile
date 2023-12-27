include ../versions.mk

# parameters
# none/daemonset/deployment
TEST_SCHEDULER_EXTENDER_TYPE ?= deployment
# true/false
TEST_LEGACY ?= false
# systemd-service/daemonset/embedded
TEST_LVMD_TYPE ?= systemd-service

BINDIR := $(shell pwd)/bin
SUDO := sudo
CURL := curl -sSLf
CLUSTER_NAME := topolvm-e2e
KIND := ../bin/kind
KUBECTL := $(BINDIR)/kubectl-$(KUBERNETES_VERSION)
HELM := ../bin/helm
GINKGO := $(BINDIR)/ginkgo-$(GINKGO_VERSION)

MINIKUBE_HOME = $(BINDIR)
export MINIKUBE_HOME

DOMAIN_NAME := topolvm.io
HELM_VALUES_FILES := manifests/values/base.yaml
SCHEDULER_CONFIG := ../deploy/scheduler-config/scheduler-config.yaml
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
SCHEDULER_CONFIG := ../deploy/scheduler-config/scheduler-config-legacy.yaml
USE_LEGACY := true
endif

ifeq ($(TEST_LVMD_TYPE),systemd-service)
HELM_VALUES_FILES += manifests/values/systemd-service-lvmd.yaml
else ifeq ($(TEST_LVMD_TYPE),embedded)
HELM_VALUES_FILES += manifests/values/embedded-lvmd.yaml
endif

KIND_CONFIG="topolvm-cluster.yaml"
MINIKUBE_FEATURE_GATES="ReadWriteOncePod=true"

GO_FILES := $(shell find .. -path ../e2e -prune -o -name '*.go' -print)
BACKING_STORE := ./build

topolvm.img: $(GO_FILES)
	mkdir -p tmpbin
	CGO_ENABLED=0 go build -o tmpbin/hypertopolvm ../pkg/hypertopolvm
	$(MAKE) -f ../csi-sidecars.mk OUTPUT_DIR=tmpbin
	docker build --no-cache --rm=false -f Dockerfile -t topolvm:dev .
	docker save -o $@ topolvm:dev

/tmp/topolvm/scheduler/scheduler-config.yaml: $(SCHEDULER_CONFIG)
	mkdir -p /tmp/topolvm/scheduler
ifeq ($(TEST_SCHEDULER_EXTENDER_TYPE),deployment)
	sed -e 's|\(urlPrefix:\).*|\1 "http://topolvm-e2e-worker:30251"|' $< > $@
else
	cp $< $@
endif

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
	go build -o build/lvmd ../pkg/lvmd
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
create-cluster: topolvm.img $(HELM) $(KIND) $(KUBECTL)
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	$(MAKE) apply-certmanager-crds
	$(MAKE) prepare-namespace
	$(KIND) load image-archive --name=$(CLUSTER_NAME) $<
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ $(patsubst %,-f %,$(HELM_VALUES_FILES))
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system -l="app.kubernetes.io/component=controller,app.kubernetes.io/name=topolvm" pod
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system certificate/topolvm-mutatingwebhook

.PHONY: prepare-test
prepare-test: $(KUBECTL)
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
	$(KUBECTL) apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v$(EXTERNAL_SNAPSHOTTER_VERSION)/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
	sed 's/@DOMAIN_NAME@/$(DOMAIN_NAME)/' manifests/volumesnapshotclass_tpl.yaml | $(KUBECTL) apply -f -

.PHONY: run-test
run-test: GINKGO_FLAGS =
run-test: $(GINKGO)
	$(SUDO) -E env \
	PATH=${PATH} \
	E2ETEST=1 \
	KUBECTL=$(KUBECTL) \
	STORAGE_CAPACITY=$(STORAGE_CAPACITY) \
	USE_LEGACY=$(USE_LEGACY) \
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
incluster-lvmd/test: topolvm.img $(GINKGO) $(HELM) $(KUBECTL)
	@if [ "$(TEST_SCHEDULER_EXTENDER_TYPE)" != none ]; then \
		echo "Using the scheduler extender is not supported"; \
		exit 1; \
	fi
	@if [ "$(TEST_LVMD_TYPE)" = systemd-service ]; then \
		echo "Running LVMd as systemd-service is not supported"; \
		exit 1; \
	fi
	$(MAKE) apply-certmanager-crds
	$(MAKE) prepare-namespace
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ $(patsubst %,-f %,$(HELM_VALUES_FILES))
	$(KUBECTL) wait --for=condition=available --timeout=120s -n topolvm-system deployments/topolvm-controller
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system -l="app.kubernetes.io/component=controller,app.kubernetes.io/name=topolvm" pod
	$(KUBECTL) wait --for=condition=ready --timeout=120s -n topolvm-system certificate/topolvm-mutatingwebhook
	$(MAKE) prepare-test
	$(MAKE) run-test

.PHONY: incluster-lvmd/clean
incluster-lvmd/clean: incluster-lvmd/delete-minikube incluster-lvmd/remove-vg

.PHONY: common/create-vg
common/create-vg:
	for i in $$(seq 2); do \
		truncate --size=20G $(BACKING_STORE)/backing_store$(ID)_$${i}; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_$${i}; \
		$(SUDO) vgcreate -y node$(ID)-myvg$${i} $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_$${i} | cut -d: -f1); \
	done

	# Create Volume Group for testing raid1 volumes.
	# This requires 2 PVs in each VG.
	truncate --size=3G $(BACKING_STORE)/backing_store$(ID)_3
	truncate --size=3G $(BACKING_STORE)/backing_store$(ID)_4
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_3
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_4
	$(SUDO) vgcreate -y node$(ID)-myvg3 \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_3 | cut -d: -f1) \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_4 | cut -d: -f1)

	# Create Volume Group for thinpool
	truncate --size=5G $(BACKING_STORE)/backing_store$(ID)_5
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_5
	$(SUDO) vgcreate -y node$(ID)-myvg4 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_5 | cut -d: -f1)
	$(SUDO) lvcreate -T -n pool0 -L 4G node$(ID)-myvg4

	# Create Volume Group for testing lvcreate-option-classes.
	# This requires 2 PVs in each VG.
	truncate --size=3G $(BACKING_STORE)/backing_store$(ID)_6
	truncate --size=3G $(BACKING_STORE)/backing_store$(ID)_7
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_6
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store$(ID)_7
	$(SUDO) vgcreate -y node$(ID)-myvg5 \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_6 | cut -d: -f1) \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_7 | cut -d: -f1)

.PHONY: common/remove-vg
common/remove-vg:
	for i in $$(seq 5); do \
		$(SUDO) vgremove -ffy node$(ID)-myvg$${i}; \
	done; \
	for i in $$(seq 7); do \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_$${i} | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$(ID)_$${i} | cut -d: -f1); \
		rm -f $(BACKING_STORE)/backing_store$(ID)_$${i}; \
	done
