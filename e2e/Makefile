SUDO=sudo
GOFLAGS=-mod=vendor
export GOFLAGS
GINKGO=$(GOPATH)/bin/ginkgo
KUBECTL=/usr/local/bin/kubectl
KUBERNETES_VERSION=1.14.3

GO_FILES := $(shell find .. -path ../vendor -prune -o -path ../e2e -prune -o -name '*.go' -print)
CA_FILES=./certs/ca.csr ./certs/ca.pem ./certs/ca-key.pem
SERVER_CERT_FILES=./certs/server.csr ./certs/server.pem ./certs/server-key.pem
KUBECONFIG := $(shell kind get kubeconfig-path --name=kind)
export KUBECONFIG

$(KUBECTL):
	$(SUDO) curl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	$(SUDO) chmod 755 $(KUBECTL)

topolvm.img: $(GO_FILES)
	rm -rf tmpbin
	mkdir -p tmpbin
	CGO_ENABLED=0 go build -o tmpbin/hypertopolvm ../pkg/hypertopolvm
	ln -s hypertopolvm ./tmpbin/csi-topolvm
	ln -s hypertopolvm ./tmpbin/lvmetrics
	ln -s hypertopolvm ./tmpbin/topolvm-scheduler
	ln -s hypertopolvm ./tmpbin/topolvm-node
	ln -s hypertopolvm ./tmpbin/topolvm-hook
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
	sed s/@KUBERNETES_VERSION@/$(KUBERNETES_VERSION)/ topolvm-cluster.yaml > /tmp/topolvm-cluster.yaml
	env KUBECONFIG= kind create cluster --config /tmp/topolvm-cluster.yaml --image kindest/node:v$(KUBERNETES_VERSION)

shutdown-kind:
	kind delete cluster || true
	for d in $$($(SUDO) find /tmp/topolvm -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done

start-lvmd:
	mkdir -p build
	go build -o build/lvmd ../pkg/lvmd
	if [ -f build/backing_store ]; then $(MAKE) stop-lvmd; fi
	truncate --size=20G build/backing_store
	$(SUDO) losetup -f build/backing_store
	$(SUDO) vgcreate -y myvg $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	# For csi_node_test.go
	$(SUDO) lvcreate -y -n csi-node-test-block -L 1G myvg
	$(SUDO) lvcreate -y -n csi-node-test-fs -L 1G myvg
	mkdir -p /tmp/topolvm
	$(SUDO) systemd-run --unit=lvmd.service $(shell pwd)/build/lvmd --volume-group=myvg --listen=/tmp/topolvm/lvmd.sock

stop-lvmd:
	$(MAKE) shutdown-kind
	if systemctl is-active -q lvmd.service; then $(SUDO) systemctl stop lvmd.service; fi
	if [ -f build/backing_store ]; then \
		$(SUDO) vgremove -ffy myvg; \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); \
		rm -f build/backing_store; \
	fi

test: topolvm.img hook-secret.yml
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	$(KUBECTL) apply -f namespace.yml
	$(MAKE) secret
	kind load image-archive topolvm.img
	$(KUBECTL) apply -f lvmetrics.yml -f scheduler.yml -f hook-secret.yml -f hook.yml -f provisioner.yml
	$(KUBECTL) apply -f ../topolvm-node/config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml
	$(KUBECTL) apply -f csi.yml
	$(SUDO) env \
		PATH=$(PATH) \
		GOPATH=$(GOPATH) \
		GO111MODULE=on \
		GOFLAGS=$(GOFLAGS) \
		KUBECONFIG=$(KUBECONFIG) \
		E2ETEST=1 \
		$(GINKGO) --failFast -v .

$(CA_FILES): ./certs/csr.json
	cfssl gencert -initca certs/csr.json | cfssljson -bare certs/ca

$(SERVER_CERT_FILES): $(CA_FILES) ./certs/ca-config.json ./certs/server.json
	cfssl gencert -ca=certs/ca.pem -ca-key=certs/ca-key.pem -config=certs/ca-config.json -profile=server certs/server.json | cfssljson -bare certs/server

hook-secret.yml: hook-secret.yml.template $(CA_FILES)
	./patch_capem.sh

secret: $(SERVER_CERT_FILES)
	$(KUBECTL) delete -n=topolvm-system secret topolvm-hook-certs --ignore-not-found
	$(KUBECTL) create -n=topolvm-system secret generic topolvm-hook-certs --from-file=certs/server.pem --from-file=certs/server-key.pem

setup: $(KUBECTL)
	go install github.com/cloudflare/cfssl/cmd/cfssl
	go install github.com/cloudflare/cfssl/cmd/cfssljson
	go install github.com/onsi/ginkgo/ginkgo
	cd /tmp; env GOFLAGS= GO111MODULE=on go get sigs.k8s.io/kind@v0.4.0
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs
	if apt-cache show btrfs-progs; then \
		$(SUDO) apt-get install -y btrfs-progs; \
	else \
		$(SUDO) apt-get install -y btrfs-tools; \
	fi

clean: stop-lvmd
	rm -rf $(CA_FILES) $(SERVER_CERT_FILES) hook-secret.yml topolvm.img build/

.PHONY: launch-kind shutdown-kind start-lvmd stop-lvmd test secret setup clean
