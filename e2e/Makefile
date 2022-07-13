# parameters
KUBERNETES_VERSION ?= 1.23.3
TEST_SCHEDULER_MANIFEST ?= daemonset
STORAGE_CAPACITY ?= false
SKIP_NODE_FINALIZE ?= false

## Dependency versions
MINIKUBE_VERSION := v1.26.0
CERT_MANAGER_VERSION := v1.7.0
KUSTOMIZE_VERSION := v4.5.5

BINDIR := $(shell pwd)/bin
SUDO := sudo
CURL := curl -sSLf
KIND_CLUSTER_NAME := topolvm-e2e
KIND := ../bin/kind
KUBECTL := $(BINDIR)/kubectl
HELM := ../bin/helm
GINKGO := ../bin/ginkgo

MINIKUBE_HOME = $(BINDIR)
export MINIKUBE_HOME

READ_WRITE_ONCE_POD=true
ifeq ($(KUBERNETES_VERSION),1.21.2)
READ_WRITE_ONCE_POD=false
endif

KIND_NODE_IMAGE=kindest/node:v$(KUBERNETES_VERSION)
export TEST_KUBERNETES_VERSION=$(shell echo $(KUBERNETES_VERSION) | grep -o '^[0-9]*\.[0-9]*')

HELM_VALUES_FILE := manifests/values/daemonset-scheduler.yaml
ifeq ($(TEST_SCHEDULER_MANIFEST),deployment)
HELM_VALUES_FILE := manifests/values/deployment-scheduler.yaml
endif
ifeq ($(STORAGE_CAPACITY),true)
HELM_VALUES_FILE := manifests/values/storage-capacity.yaml
endif

KIND_CONFIG="topolvm-cluster.yaml"
MINIKUBE_FEATURE_GATES=""
ifneq ($(KUBERNETES_VERSION),1.21.2)
KIND_CONFIG="topolvm-cluster-single-writer.yaml"
MINIKUBE_FEATURE_GATES="ReadWriteOncePod=true"
endif

HELM_VALUES_FILE_LVMD := manifests/values/daemonset-lvmd-storage-capacity.yaml

ifeq ($(SKIP_NODE_FINALIZE),true)
HELM_SKIP_NODE_FINALIZE_FLAG := --set controller.nodeFinalize.skipped=true
else
HELM_SKIP_NODE_FINALIZE_FLAG :=
endif

ifeq ($(KUBERNETES_VERSION),1.21.2)
SCHEDULER_CONFIG := scheduler-config-v1beta1-$(TEST_SCHEDULER_MANIFEST).yaml
else
SCHEDULER_CONFIG := scheduler-config-v1beta2-$(TEST_SCHEDULER_MANIFEST).yaml
endif

ifeq ($(KUBERNETES_VERSION),1.21.2)
SCHEDULER_DEPLOYMENT_CONFIG := scheduler-config-v1beta1-deployment.yaml
else
SCHEDULER_DEPLOYMENT_CONFIG := scheduler-config-v1beta2-deployment.yaml
endif

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
	sed -e "s|@DEPLOYMENT_SCHEDULER_HOST@|topolvm-e2e-worker|" $< > $@

.PHONY: prepare-namespace
prepare-namespace:
	$(KUBECTL) create namespace topolvm-system
	$(KUBECTL) label namespace topolvm-system topolvm.cybozu.com/webhook=ignore
	$(KUBECTL) label namespace kube-system topolvm.cybozu.com/webhook=ignore

.PHONY: apply-certmanager
apply-certmanager:
	$(KUBECTL) apply -f https://github.com/jetstack/cert-manager/releases/download/$(CERT_MANAGER_VERSION)/cert-manager.crds.yaml

.PHONY: launch-kind
launch-kind: /tmp/topolvm/scheduler/scheduler-config.yaml
	$(SUDO) rm -rf /tmp/topolvm/controller /tmp/topolvm/worker*
	sed -e "s|@KUBERNETES_VERSION@|$(KUBERNETES_VERSION)|" $(KIND_CONFIG) > /tmp/$(KIND_CONFIG)
	$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config /tmp/$(KIND_CONFIG) --image $(KIND_NODE_IMAGE)

.PHONY: shutdown-kind
shutdown-kind:
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
		$(SUDO) lvcreate -y -n csi-node-test-block -L 1G node$${i}-myvg1; \
		$(SUDO) lvcreate -y -n csi-node-test-fs -L 1G node$${i}-myvg1; \
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

	for i in $$(seq 3); do \
		$(SUDO) systemd-run --unit=lvmd$$i.service $(shell pwd)/build/lvmd --config=$(shell pwd)/lvmd$$i.yaml; \
	done

.PHONY: stop-lvmd
stop-lvmd:
	$(MAKE) shutdown-kind
	for i in $$(seq 3); do \
		if systemctl is-active -q lvmd$$i.service; then $(SUDO) systemctl stop lvmd$$i.service; fi; \
		for j in $$(seq 4); do \
			# We have only 3 VGs per node, but 4 PVs, since node{i}-myvg3 uses both \
			# backing_store{i}_3 and backing_store{i}_4. \
			# I.e. there is no node{i}-myvg4. \
			if [ "$${j}" -ne 4 ]; then \
				$(SUDO) vgremove -ffy node$${i}-myvg$${j}; \
			fi; \
			$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_$${j} | cut -d: -f1); \
			$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_$${j} | cut -d: -f1); \
			rm -f $(BACKING_STORE)/backing_store$${i}_$${j}; \
		done; \
	done

.PHONY: create-cluster
create-cluster: topolvm.img
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	$(MAKE) apply-certmanager
	$(MAKE) prepare-namespace
	$(KIND) load image-archive --name=$(KIND_CLUSTER_NAME) $<
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ -f $(HELM_VALUES_FILE) --set "controller.nodeSelector.kubernetes\.io/hostname"=$(KIND_CLUSTER_NAME)-worker $(HELM_SKIP_NODE_FINALIZE_FLAG)

.PHONY: test
test: create-cluster
	$(KUBECTL) apply -f manifests/common/
	$(SUDO) -E env PATH=${PATH} E2ETEST=1 BINDIR=$(BINDIR) STORAGE_CAPACITY=$(STORAGE_CAPACITY) SKIP_NODE_FINALIZE=$(SKIP_NODE_FINALIZE) READ_WRITE_ONCE_POD=$(READ_WRITE_ONCE_POD) GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn $(GINKGO) -skip='snapshot' -skip='CreateSnapshot' -skip='DeleteSnapshot' -skip='source volume' --failFast -v .

.PHONY: clean
clean: stop-lvmd
	rm -rf \
		topolvm.img \
		build/ \
		tmpbin/ \
		/tmp/topolvm/scheduler/scheduler-config.yaml

.PHONY: setup
setup:
	cd ..; $(MAKE) setup
	mkdir -p $(BINDIR)
	$(CURL) -o $(BINDIR)/kubectl https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl
	chmod a+x $(BINDIR)/kubectl
	$(CURL) https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F$(KUSTOMIZE_VERSION)/kustomize_$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar zxf - -C $(BINDIR)

.PHONY: daemonset-lvmd/create-vg
daemonset-lvmd/create-vg:
	mkdir -p build $(BACKING_STORE)
	if [ $$(ls -1 $(BACKING_STORE)/backing_store_lvmd* 2>/dev/null | wc -l) -ne 0 ]; then $(MAKE) $(@D)/remove-vg; fi

	for i in $$(seq 3); do \
		truncate --size=20G $(BACKING_STORE)/backing_store_lvmd_$${i}; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_$${i}; \
		$(SUDO) vgcreate -y node-myvg$${i} $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
		$(SUDO) lvcreate -y -n csi-node-test-block -L 1G node-myvg$${i}; \
		$(SUDO) lvcreate -y -n csi-node-test-fs -L 1G node-myvg$${i}; \
	done

	# Create Volume Group for testing raid1 volumes.
	# This requires 2 PVs in each VG.
	truncate --size=3G $(BACKING_STORE)/backing_store_lvmd_4
	truncate --size=3G $(BACKING_STORE)/backing_store_lvmd_5
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_4
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_5
	$(SUDO) vgcreate -y node-myvg4 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_4 | cut -d: -f1) \
		$$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_5 | cut -d: -f1)

.PHONY: daemonset-lvmd/remove-vg
daemonset-lvmd/remove-vg:
	for i in $$(seq 5); do \
		# We have only 4 VGs, but 5 PVs, since node-myvg4 uses both \
		# backing_store_lvmd_4 and backing_store_lvmd_5. \
		# I.e. there is no node-myvg5. \
		if [ "$${i}" -ne 5 ]; then \
			$(SUDO) vgremove -ffy node-myvg$${i}; \
		fi; \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
		rm -f $(BACKING_STORE)/backing_store_lvmd_$${i}; \
	done

.PHONY: daemonset-lvmd/setup-minikube
daemonset-lvmd/setup-minikube:
	mkdir -p $(BINDIR)
	$(SUDO) apt-get update
	DEBIAN_FRONTEND=noninteractive $(SUDO) apt-get install -y --no-install-recommends conntrack
	$(CURL) -o $(BINDIR)/minikube https://github.com/kubernetes/minikube/releases/download/$(MINIKUBE_VERSION)/minikube-linux-amd64
	chmod a+x $(BINDIR)/minikube

.PHONY: daemonset-lvmd/launch-minikube
daemonset-lvmd/launch-minikube:
	@if [ "$(STORAGE_CAPACITY)" = false ]; then echo "Only storage capacity mode is supported."; exit 1; fi
	$(SUDO) -E $(BINDIR)/minikube start \
		--vm-driver=none \
		--kubernetes-version=v$(KUBERNETES_VERSION) \
		--extra-config=kubelet.read-only-port=10255 \
		--feature-gates=$(MINIKUBE_FEATURE_GATES)
	$(SUDO) chown -R $$USER $$HOME/.kube $(MINIKUBE_HOME)/.minikube
	$(SUDO) chmod -R a+r $$HOME/.kube $(MINIKUBE_HOME)/.minikube
	$(SUDO) find $(MINIKUBE_HOME)/.minikube -name id_rsa -exec chmod 600 {} ';'

.PHONY: daemonset-lvmd/delete-minikube
daemonset-lvmd/delete-minikube:
	$(SUDO) -E $(BINDIR)/minikube delete || true

.PHONY: daemonset-lvmd/test
daemonset-lvmd/test: topolvm.img
	$(MAKE) apply-certmanager
	$(MAKE) prepare-namespace
	$(HELM) repo add jetstack https://charts.jetstack.io
	$(HELM) repo update
	$(HELM) dependency build ../charts/topolvm/
	$(HELM) install --namespace=topolvm-system topolvm ../charts/topolvm/ -f $(HELM_VALUES_FILE_LVMD) $(HELM_SKIP_NODE_FINALIZE_FLAG)
	$(KUBECTL) apply -f manifests/common/
	$(SUDO) -E env PATH=${PATH} E2ETEST=1 BINDIR=$(BINDIR) STORAGE_CAPACITY=$(STORAGE_CAPACITY) SKIP_NODE_FINALIZE=$(SKIP_NODE_FINALIZE) READ_WRITE_ONCE_POD=$(READ_WRITE_ONCE_POD) DAEMONSET_LVMD=true GOLANG_PROTOBUF_REGISTRATION_CONFLICT=warn $(GINKGO) -skip='snapshot' -skip='CreateSnapshot' -skip='DeleteSnapshot'  -skip='source volume' --failFast -v .

.PHONY: daemonset-lvmd/clean
daemonset-lvmd/clean: daemonset-lvmd/delete-minikube daemonset-lvmd/remove-vg
