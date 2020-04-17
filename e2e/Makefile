SUDO=sudo
GOFLAGS=-mod=vendor
export GOFLAGS
GINKGO=$(GOPATH)/bin/ginkgo
KUBECTL=/usr/local/bin/kubectl
KIND_VERSION=0.7.0
KUSTOMIZE_VERSION=3.5.4
KUSTOMIZE=/usr/local/bin/kustomize

# If you want to change the Kubernetes version for e2e test, specify this variable from command line.
# e.g. $ make TEST_KUBERNETES_VERSION=1.17 setup test
TEST_KUBERNETES_VERSION=1.17

ifeq ($(TEST_KUBERNETES_VERSION),1.17)
KUBERNETES_VERSION=1.17.2
KUBEADM_APIVERSION=kubeadm.k8s.io/v1beta2
KUSTOMIZE_DIR=e2ebase
else ifeq ($(TEST_KUBERNETES_VERSION),1.16)
KUBERNETES_VERSION=1.16.4
KUBEADM_APIVERSION=kubeadm.k8s.io/v1beta2
KUSTOMIZE_DIR=e2ebase
else ifeq ($(TEST_KUBERNETES_VERSION),1.15)
KUBERNETES_VERSION=1.15.7
KUBEADM_APIVERSION=kubeadm.k8s.io/v1beta2
KUSTOMIZE_DIR=legacy
endif

GO_FILES := $(shell find .. -path ../vendor -prune -o -path ../e2e -prune -o -name '*.go' -print)
CA_FILES=./certs/ca.csr ./certs/ca.pem ./certs/ca-key.pem
SERVER_CERT_FILES=./certs/server.csr ./certs/server.pem ./certs/server-key.pem

$(KUBECTL):
	$(SUDO) curl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	$(SUDO) chmod 755 $(KUBECTL)

topolvm.img: $(GO_FILES)
	rm -rf tmpbin
	mkdir -p tmpbin
	CGO_ENABLED=0 go build -o tmpbin/hypertopolvm ../pkg/hypertopolvm
	ln -s hypertopolvm ./tmpbin/topolvm-scheduler
	ln -s hypertopolvm ./tmpbin/topolvm-node
	ln -s hypertopolvm ./tmpbin/topolvm-controller
	docker build --no-cache --rm=false -f Dockerfile -t topolvm:dev tmpbin
	docker save -o $@ topolvm:dev

/tmp/topolvm/scheduler/scheduler-config.yaml: scheduler-config.yaml
	mkdir -p /tmp/topolvm/scheduler
	cp $< $@

/tmp/topolvm/scheduler/scheduler-policy.cfg: scheduler-policy.cfg
	mkdir -p /tmp/topolvm/scheduler
	cp $< $@

launch-kind: /tmp/topolvm/scheduler/scheduler-config.yaml /tmp/topolvm/scheduler/scheduler-policy.cfg
	$(SUDO) rm -rf /tmp/topolvm/controller /tmp/topolvm/worker*
	sed -e "s|@KUBERNETES_VERSION@|$(KUBERNETES_VERSION)|" \
		-e "s|@KUBEADM_APIVERSION@|$(KUBEADM_APIVERSION)|" topolvm-cluster.yaml > /tmp/topolvm-cluster.yaml
	kind create cluster --config /tmp/topolvm-cluster.yaml --image kindest/node:v$(KUBERNETES_VERSION)

shutdown-kind:
	kind delete cluster || true
	sleep 2
	for d in $$($(SUDO) find /tmp/topolvm -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done
	for d in $$(mount | grep /lib/kubelet | cut -d ' ' -f 3); do $(SUDO) umount $$d; done

start-lvmd:
	mkdir -p build
	go build -o build/lvmd ../pkg/lvmd
	if [ -f build/backing_store1 ] || [ -f build/backing_store2 ] || [ -f build/backing_store3 ]; then $(MAKE) stop-lvmd; fi

	for i in $$(seq 3); do \
		mkdir -p /tmp/topolvm/worker$$i; \
		mkdir -p /tmp/topolvm/lvmd$$i; \
		truncate --size=20G build/backing_store$$i; \
		$(SUDO) losetup -f build/backing_store$$i; \
		$(SUDO) vgcreate -y myvg$$i $$($(SUDO) losetup -j build/backing_store$$i | cut -d: -f1); \
		$(SUDO) lvcreate -y -n csi-node-test-block -L 1G myvg$$i; \
		$(SUDO) lvcreate -y -n csi-node-test-fs -L 1G myvg$$i; \
		$(SUDO) systemd-run --unit=lvmd$$i.service $(shell pwd)/build/lvmd --listen=/tmp/topolvm/lvmd$$i/lvmd.sock --spare=1; \
	done

stop-lvmd:
	$(MAKE) shutdown-kind
	for i in $$(seq 3); do \
		if systemctl is-active -q lvmd$$i.service; then $(SUDO) systemctl stop lvmd$$i.service; fi; \
		if [ -f build/backing_store$$i ]; then \
			$(SUDO) vgremove -ffy myvg$$i; \
			$(SUDO) pvremove -ffy $$($(SUDO) losetup -j build/backing_store$$i | cut -d: -f1); \
			$(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store$$i | cut -d: -f1); \
			rm -f build/backing_store$$i; \
		fi; \
	done

# --load_restrictor='none' allows the certificates to be loaded from a path which
# isn't a subdir of e2ebase
test: topolvm.img $(SERVER_CERT_FILES)
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	kind load image-archive topolvm.img
	kustomize build --load_restrictor='none' $(KUSTOMIZE_DIR) | kubectl apply -f -
	$(SUDO) env \
		PATH=$(PATH) \
		GOPATH=$(GOPATH) \
		GO111MODULE=on \
		GOFLAGS=$(GOFLAGS) \
		E2ETEST=1 \
		TEST_KUBERNETES_VERSION=$(TEST_KUBERNETES_VERSION) \
		$(GINKGO) --failFast -v .

$(CA_FILES): ./certs/csr.json
	cfssl gencert -initca certs/csr.json | cfssljson -bare certs/ca

$(SERVER_CERT_FILES): $(CA_FILES) ./certs/ca-config.json ./certs/server.json
	cfssl gencert -ca=certs/ca.pem -ca-key=certs/ca-key.pem -config=certs/ca-config.json -profile=server certs/server.json | cfssljson -bare certs/server

setup: $(KUBECTL)
	go install github.com/cloudflare/cfssl/cmd/cfssl
	go install github.com/cloudflare/cfssl/cmd/cfssljson
	go install github.com/onsi/ginkgo/ginkgo
	curl -sSLf https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize/v$(KUSTOMIZE_VERSION)/kustomize_v$(KUSTOMIZE_VERSION)_linux_amd64.tar.gz | tar xzf - > kustomize
	$(SUDO) mv kustomize $(KUSTOMIZE)
	cd /tmp; env GOFLAGS= GO111MODULE=on go get sigs.k8s.io/kind@v$(KIND_VERSION)
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs
	if apt-cache show btrfs-progs; then \
		$(SUDO) apt-get install -y btrfs-progs; \
	else \
		$(SUDO) apt-get install -y btrfs-tools; \
	fi

clean: stop-lvmd
	rm -rf $(CA_FILES) $(SERVER_CERT_FILES) topolvm.img build/

.PHONY: launch-kind shutdown-kind start-lvmd stop-lvmd test secret setup clean
