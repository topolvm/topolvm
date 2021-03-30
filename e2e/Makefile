# parameters
TEST_KUBERNETES_TARGET ?= current
TEST_SCHEDULER_MANIFEST ?= daemonset

## Dependency versions
KIND_VERSION=0.10.0
MINIKUBE_VERSION=v1.18.1

BINDIR=$(PWD)/bin
SUDO=sudo
KIND_CLUSTER_NAME=topolvm-e2e
KIND := $(BINDIR)/kind
KUBECTL := $(BINDIR)/kubectl
KUSTOMIZE := ../bin/kustomize
GINKGO := ../bin/ginkgo

ifeq ($(TEST_KUBERNETES_TARGET),current)
TEST_KUBERNETES_VERSION=1.20
else ifeq ($(TEST_KUBERNETES_TARGET),prev)
TEST_KUBERNETES_VERSION=1.19
else ifeq ($(TEST_KUBERNETES_TARGET),prev2)
TEST_KUBERNETES_VERSION=1.18
endif

export TEST_KUBERNETES_VERSION

SCHEDULER_POLICY := scheduler-policy-daemonset.cfg
KUSTOMIZE_DIR := manifests/overlays/daemonset-scheduler
ifeq ($(TEST_SCHEDULER_MANIFEST),deployment)
SCHEDULER_POLICY=scheduler-policy-deployment.cfg
KUSTOMIZE_DIR=manifests/overlays/deployment-scheduler
endif

ifeq ($(TEST_KUBERNETES_VERSION),1.20)
KUBERNETES_VERSION=1.20.2
KUBEADM_APIVERSION=kubeadm.k8s.io/v1beta2
SCHEDULER_CONFIG=scheduler-config-v1beta1-$(TEST_SCHEDULER_MANIFEST).yaml
else ifeq ($(TEST_KUBERNETES_VERSION),1.19)
KUBERNETES_VERSION=1.19.4
KUBEADM_APIVERSION=kubeadm.k8s.io/v1beta2
SCHEDULER_CONFIG=scheduler-config-v1beta1-$(TEST_SCHEDULER_MANIFEST).yaml
else ifeq ($(TEST_KUBERNETES_VERSION),1.18)
KUBERNETES_VERSION=1.18.8
KUBEADM_APIVERSION=kubeadm.k8s.io/v1beta2
SCHEDULER_CONFIG=scheduler-config-v1alpha1.yaml
endif

GO_FILES := $(shell find .. -prune -o -path ../e2e -prune -o -name '*.go' -print)
SERVER_CERT_FILES=./certs/cert.pem ./certs/key.pem
BACKING_STORE=./build

topolvm.img: $(GO_FILES)
	rm -rf tmpbin
	mkdir -p tmpbin
	CGO_ENABLED=0 go build -o tmpbin/hypertopolvm ../pkg/hypertopolvm
	ln -s hypertopolvm ./tmpbin/lvmd
	ln -s hypertopolvm ./tmpbin/topolvm-scheduler
	ln -s hypertopolvm ./tmpbin/topolvm-node
	ln -s hypertopolvm ./tmpbin/topolvm-controller
	$(MAKE) -f ../csi-sidecars.mk OUTPUT_DIR=tmpbin
	docker build --no-cache --rm=false -f Dockerfile -t topolvm:dev tmpbin
	docker save -o $@ topolvm:dev

/tmp/topolvm/scheduler/scheduler-config.yaml: $(SCHEDULER_CONFIG)
	mkdir -p /tmp/topolvm/scheduler
	sed -e "s|@DEPLOYMENT_SCHEDULER_HOST@|topolvm-e2e-worker|" $< > $@

# The policy file will be copied in all kubernetes version, but it isn't used in kubernetes v1.19 or later.
/tmp/topolvm/scheduler/scheduler-policy.cfg: $(SCHEDULER_POLICY)
	mkdir -p /tmp/topolvm/scheduler
	sed -e "s|@DEPLOYMENT_SCHEDULER_HOST@|topolvm-e2e-worker|" $< > $@

launch-kind: /tmp/topolvm/scheduler/scheduler-config.yaml /tmp/topolvm/scheduler/scheduler-policy.cfg
	$(SUDO) rm -rf /tmp/topolvm/controller /tmp/topolvm/worker*
	sed -e "s|@KUBERNETES_VERSION@|$(KUBERNETES_VERSION)|" \
		-e "s|@KUBEADM_APIVERSION@|$(KUBEADM_APIVERSION)|" topolvm-cluster.yaml > /tmp/topolvm-cluster.yaml
	$(KIND) create cluster --name=$(KIND_CLUSTER_NAME) --config /tmp/topolvm-cluster.yaml --image kindest/node:v$(KUBERNETES_VERSION)

shutdown-kind:
	$(KIND) delete cluster --name=$(KIND_CLUSTER_NAME) || true
	sleep 2
	for d in $$($(SUDO) find /tmp/topolvm -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done
	for d in $$(mount | grep /lib/kubelet | cut -d ' ' -f 3); do $(SUDO) umount $$d; done

start-lvmd:
	mkdir -p build $(BACKING_STORE)
	go build -o build/lvmd ../pkg/lvmd
	if [ -f $(BACKING_STORE)/backing_store* ]; then $(MAKE) stop-lvmd; fi

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
	truncate --size=10G $(BACKING_STORE)/backing_store1_2; \
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store1_2; \
	$(SUDO) vgcreate -y node1-myvg2 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store1_2 | cut -d: -f1); \
	truncate --size=10G $(BACKING_STORE)/backing_store2_2; \
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store2_2; \
	$(SUDO) vgcreate -y node2-myvg2 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store2_2 | cut -d: -f1); \
	truncate --size=10G $(BACKING_STORE)/backing_store3_3; \
	$(SUDO) losetup -f $(BACKING_STORE)/backing_store3_3; \
	$(SUDO) vgcreate -y node3-myvg3 $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store3_3 | cut -d: -f1); \

	for i in $$(seq 3); do \
		$(SUDO) systemd-run --unit=lvmd$$i.service $(shell pwd)/build/lvmd --config=$(shell pwd)/lvmd$$i.yaml; \
	done

stop-lvmd:
	$(MAKE) shutdown-kind
	for i in $$(seq 3); do \
		if systemctl is-active -q lvmd$$i.service; then $(SUDO) systemctl stop lvmd$$i.service; fi; \
		for j in $$(seq 3); do \
			if [ -f $(BACKING_STORE)/backing_store$${i}_$${j} ]; then \
				$(SUDO) vgremove -ffy node$${i}-myvg$${j}; \
				$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_$${j} | cut -d: -f1); \
				$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store$${i}_$${j} | cut -d: -f1); \
				rm -f $(BACKING_STORE)/backing_store$${i}_$${j}; \
			fi; \
		done; \
	done

# --load_restrictor='none' allows the certificates to be loaded from a path which
# isn't a subdir of e2ebase
test: topolvm.img $(SERVER_CERT_FILES)
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	$(KIND) load image-archive --name=$(KIND_CLUSTER_NAME) topolvm.img
	$(KUSTOMIZE) build --load_restrictor='none' $(KUSTOMIZE_DIR) | $(KUBECTL) apply -f -
	$(SUDO) -E env PATH=${PATH} E2ETEST=1 BINDIR=$(BINDIR) $(GINKGO) --failFast -v .

$(SERVER_CERT_FILES):
	mkdir -p certs
	go run ../pkg/gencert -outdir certs

clean: stop-lvmd
	rm -rf \
		$(SERVER_CERT_FILES) \
		topolvm.img \
		build/ \
		$(BACKING_STORE)/backing_store* \
		/tmp/topolvm/scheduler/scheduler-config.yaml \
		/tmp/topolvm/scheduler/scheduler-policy.cfg

setup:
	cd ..; $(MAKE) setup
	mkdir -p $(BINDIR)
	curl -o $(BINDIR)/kind -sfL https://kind.sigs.k8s.io/dl/v$(KIND_VERSION)/kind-linux-amd64
	curl -o $(BINDIR)/kubectl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl
	chmod a+x $(BINDIR)/kubectl $(BINDIR)/kind

daemonset-lvmd/create-vg:
	mkdir -p build $(BACKING_STORE)
	if [ -f $(BACKING_STORE)/backing_store_lvmd* ]; then $(MAKE) $(@D)/remove-vg; fi

	for i in $$(seq 3); do \
		truncate --size=20G $(BACKING_STORE)/backing_store_lvmd_$${i}; \
		$(SUDO) losetup -f $(BACKING_STORE)/backing_store_lvmd_$${i}; \
		$(SUDO) vgcreate -y node-myvg$${i} $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
		$(SUDO) lvcreate -y -n csi-node-test-block -L 1G node-myvg$${i}; \
		$(SUDO) lvcreate -y -n csi-node-test-fs -L 1G node-myvg$${i}; \
	done

daemonset-lvmd/remove-vg:
	for i in $$(seq 3); do \
		$(SUDO) vgremove -ffy node-myvg$${i}; \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j $(BACKING_STORE)/backing_store_lvmd_$${i} | cut -d: -f1); \
		rm -f $(BACKING_STORE)/backing_store_lvmd_$${i}; \
	done

daemonset-lvmd/setup-minikube:
	mkdir -p $(BINDIR)
	$(SUDO) apt-get update
	DEBIAN_FRONTEND=noninteractive $(SUDO) apt-get install -y --no-install-recommends conntrack
	curl -LO https://github.com/kubernetes/minikube/releases/download/$(MINIKUBE_VERSION)/minikube-linux-amd64
	chmod a+x minikube-linux-amd64
	mv minikube-linux-amd64 $(BINDIR)/minikube

daemonset-lvmd/launch-minikube:
	$(BINDIR)/minikube start \
		--vm-driver=none \
		--kubernetes-version=v$(KUBERNETES_VERSION) \
		--extra-config=kubelet.read-only-port=10255

daemonset-lvmd/delete-minikube:
	$(BINDIR)/minikube delete || true

# Set scheduler configs manually because minikube can't edit scheduler configs.
daemonset-lvmd/update-minikube-setting: daemonset-lvmd/delete-minikube daemonset-lvmd/launch-minikube
	$(SUDO) mkdir -p /var/lib/scheduler
	$(SUDO) sh -c 'sed -e "s|@DEPLOYMENT_SCHEDULER_HOST@|127.0.0.1|" $(SCHEDULER_POLICY) > /var/lib/scheduler/scheduler-policy.cfg'
	$(SUDO) sh -c 'sed -e "s|@DEPLOYMENT_SCHEDULER_HOST@|127.0.0.1|" $(SCHEDULER_CONFIG) > /var/lib/scheduler/scheduler-config.yaml'
	$(SUDO) sh -c 'sed -e "s|@KUBERNETES_VERSION@|$(KUBERNETES_VERSION)|" kube-scheduler.yaml > /etc/kubernetes/manifests/kube-scheduler.yaml'

daemonset-lvmd/test: topolvm.img $(SERVER_CERT_FILES)
	$(KUSTOMIZE) build --load_restrictor='none' manifests/overlays/daemonset-lvmd | $(KUBECTL) apply -f -
	$(SUDO) -E env PATH=${PATH} E2ETEST=1 BINDIR=$(BINDIR) DAEMONSET_LVMD=true $(GINKGO) --failFast -v .

daemonset-lvmd/clean: daemonset-lvmd/delete-minikube daemonset-lvmd/remove-vg

.PHONY: \
	launch-kind \
	shutdown-kind \
	start-lvmd \
	stop-lvmd \
	test \
	clean \
	setup \
	daemonset-lvmd/create-vg \
	daemonset-lvmd/remove-vg \
	daemonset-lvmd/setup-minikube \
	daemonset-lvmd/launch-minikube \
	daemonset-lvmd/delete-minikube \
	daemonset-lvmd/update-minikube-setting \
	daemonset-lvmd/test \
	daemonset-lvmd/clean
