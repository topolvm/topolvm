SUDO=sudo
GOFLAGS=-mod=vendor
export GOFLAGS
GINKGO=$(GOPATH)/bin/ginkgo
KUBECTL=/usr/local/bin/kubectl
KUBERNETES_VERSION=1.14.3

GO_FILES := $(shell find .. -path ../vendor -prune -o -path ../e2e -prune -o -name '*.go' -print)
CA_FILES=./build/certs/ca.csr ./build/certs/ca.pem ./build/certs/ca-key.pem
SERVER_CERT_FILES=./build/certs/server.csr ./build/certs/server.pem ./build/certs/server-key.pem
KUBECONFIG := $(shell kind get kubeconfig-path --name=kind)
CA_PEM := $(shell cat certs/ca.pem | base64 -w 0)
export KUBECONFIG

$(KUBECTL):
	$(SUDO) curl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	$(SUDO) chmod 755 $(KUBECTL)

run: mutatingwebhooks.yaml $(SERVER_CERT_FILES)
	$(MAKE) start-lvmd
	$(MAKE) launch-kind
	cp ../deploy/manifests/*.yaml build/
	rm build/certificates.yaml build/mutatingwebhooks.yaml
	$(MAKE) mutatingwebhooks.yaml
	$(KUBECTL) apply -f build/namespace.yaml
	$(KUBECTL) create -n=topolvm-system secret generic topolvm-hook-certs --from-file=tls.crt=build/certs/server.pem --from-file=tls.key=build/certs/server-key.pem
	$(KUBECTL) apply -f build

setup: $(KUBECTL)
	cd /tmp; env GOFLAGS= GO111MODULE=on go get sigs.k8s.io/kind@v0.4.0
	go install github.com/cloudflare/cfssl/cmd/cfssl
	go install github.com/cloudflare/cfssl/cmd/cfssljson
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs
	if apt-cache show btrfs-progs; then \
		$(SUDO) apt-get install -y btrfs-progs; \
	else \
		$(SUDO) apt-get install -y btrfs-tools; \
	fi

clean: stop-lvmd
	rm -rf build/

/tmp/topolvm/scheduler/scheduler-config.yaml: ../deploy/scheduler-config/scheduler-config.yaml
	mkdir -p /tmp/topolvm/scheduler
	cp $< $@

/tmp/topolvm/scheduler/scheduler-policy.cfg: ../deploy/scheduler-config/scheduler-policy.cfg
	mkdir -p /tmp/topolvm/scheduler
	cp $< $@

launch-kind: /tmp/topolvm/scheduler/scheduler-config.yaml /tmp/topolvm/scheduler/scheduler-policy.cfg kind/topolvm-cluster.yaml
	$(MAKE) shutdown-kind
	$(SUDO) rm -rf /tmp/topolvm/controller /tmp/topolvm/worker*
	sed s/@KUBERNETES_VERSION@/$(KUBERNETES_VERSION)/ kind/topolvm-cluster.yaml > /tmp/topolvm-cluster.yaml
	env KUBECONFIG= kind create cluster --config /tmp/topolvm-cluster.yaml --image kindest/node:v$(KUBERNETES_VERSION)

shutdown-kind:
	kind delete cluster || true
	for d in $$($(SUDO) find /tmp/topolvm -type d); do \
		if $(SUDO) mountpoint -q $$d; then \
			$(SUDO) umount $$d; \
		fi; \
	done

start-lvmd:
	go build -o build/lvmd ../pkg/lvmd
	if [ -f build/backing_store ]; then $(MAKE) stop-lvmd; fi
	mkdir -p /tmp/topolvm/worker; \
	mkdir -p /tmp/topolvm/lvmd; \
	truncate --size=20G build/backing_store; \
	$(SUDO) losetup -f build/backing_store; \
	$(SUDO) vgcreate -y myvg $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); \
	$(SUDO) lvcreate -y -n csi-node-test-block -L 1G myvg; \
	$(SUDO) lvcreate -y -n csi-node-test-fs -L 1G myvg; \
	$(SUDO) systemd-run --unit=lvmd.service $(shell pwd)/build/lvmd --volume-group=myvg --listen=/tmp/topolvm/lvmd/lvmd.sock --spare=1; \

stop-lvmd:
	$(MAKE) shutdown-kind
	if systemctl is-active -q lvmd.service; then $(SUDO) systemctl stop lvmd.service; fi; \
	if [ -f build/backing_store ]; then \
		$(SUDO) vgremove -ffy myvg; \
		$(SUDO) pvremove -ffy $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); \
		$(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); \
		rm -f build/backing_store; \
	fi; \

$(CA_FILES): ./certs/csr.json
		mkdir -p build/certs
		cfssl gencert -initca certs/csr.json | cfssljson -bare ./build/certs/ca

$(SERVER_CERT_FILES): $(CA_FILES) ./certs/ca-config.json ./certs/server.json
		cfssl gencert -ca=build/certs/ca.pem -ca-key=build/certs/ca-key.pem -config=certs/ca-config.json -profile=server certs/server.json | cfssljson -bare build/certs/server

mutatingwebhooks.yaml: mutatingwebhooks.yaml.template $(CA_FILES)
		cat manifest/mutatingwebhooks.yaml.template | sed -e "s|{{CA_BUNDLE}}|${CA_PEM}|g" > build/mutatingwebhooks.yaml

.PHONY: launch-kind shutdown-kind start-lvmd stop-lvmd run setup clean
