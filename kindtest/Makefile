SUDO=sudo
GOFLAGS=-mod=vendor
export GOFLAGS
GINKGO=$(GOPATH)/bin/ginkgo
KUBECTL=/usr/local/bin/kubectl
KUBERNETES_VERSION=1.14.3

GO_FILES := $(shell find .. -path ../vendor -prune -o -path ../kindtest -prune -o -name '*.go' -print)
CA_FILES=./certs/ca.csr ./certs/ca.pem ./certs/ca-key.pem
SERVER_CERT_FILES=./certs/server.csr ./certs/server.pem ./certs/server-key.pem
KUBECONFIG := $(shell kind get kubeconfig-path --name=kind)
export KUBECONFIG

$(KUBECTL):
	curl -sfL https://storage.googleapis.com/kubernetes-release/release/v$(KUBERNETES_VERSION)/bin/linux/amd64/kubectl -o $(KUBECTL)
	chmod 755 $(KUBECTL)

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

/tmp/topolvm/scheduler-config.yaml: scheduler-config.yaml
	mkdir -p /tmp/topolvm
	cp $< $@

launch-kind: /tmp/topolvm/scheduler-config.yaml
	$(SUDO) rm -rf /tmp/topolvm/kubelet*
	sed s/@KUBERNETES_VERSION@/$(KUBERNETES_VERSION)/ topolvm-cluster.yaml > /tmp/topolvm-cluster.yaml
	env KUBECONFIG= kind create cluster --config /tmp/topolvm-cluster.yaml --loglevel debug --image kindest/node:v$(KUBERNETES_VERSION)

shutdown-kind:
	kind delete cluster || true

start-lvmd:
	mkdir -p build
	CGO_ENABLED=0 go build -o build/lvmd ../pkg/lvmd
	if [ "$$($(SUDO) losetup -j build/backing_store)" ]; then $(SUDO) vgremove myvg; $(SUDO) pvremove $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); $(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); fi
	truncate --size=20G build/backing_store
	$(SUDO) losetup -f build/backing_store
	$(SUDO) pvcreate -fy $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	$(SUDO) vgcreate myvg $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	# For csi_node_test.go
	$(SUDO) lvcreate -n csi-node-test-block -L 1G myvg
	$(SUDO) lvcreate -n csi-node-test-fs -L 1G myvg
	mkdir -p /tmp/topolvm
	$(SUDO) systemd-run --unit=lvmd.service $(shell pwd)/build/lvmd --volume-group=myvg --listen=/tmp/topolvm/lvmd.sock

stop-lvmd:
	-$(SUDO) systemctl stop lvmd.service
	-$(SUDO) vgremove -ffy myvg
	$(SUDO) pvremove -ffy $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	$(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	$(SUDO) dmsetup remove_all
	rm build/backing_store

test: topolvm.img
	$(MAKE) shutdown-kind
	$(MAKE) launch-kind
	$(MAKE) secret
	kind load image-archive topolvm.img
	$(KUBECTL) delete -f lvmetrics.yml -f scheduler.yml -f hook-secret.yml -f hook.yml -f scandpv.yml --ignore-not-found
	$(KUBECTL) apply -f lvmetrics.yml -f scheduler.yml -f hook-secret.yml -f hook.yml -f scandpv.yml
	$(KUBECTL) create namespace topolvm-system
	$(KUBECTL) apply -f ../topolvm-node/config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml
	$(KUBECTL) apply -f csi.yml
	$(SUDO) env PATH=$(PATH) GOPATH=$(GOPATH) GO111MODULE=on GOFLAGS=$(GOFLAGS) KUBECONFIG=$(KUBECONFIG) $(GINKGO) --failFast -v .
	$(SUDO) $$(which csi-sanity) -ginkgo.v -test.v \
		-csi.endpoint /var/lib/kubelet/plugins/topolvm.cybozu.com/node/csi-topolvm.sock \
		-csi.controllerendpoint /var/lib/kubelet/plugins/topolvm.cybozu.com/controller/csi-topolvm.sock \
		-csi.mountdir /var/lib/kubelet/plugins/topolvm.cybozu.com/node/mountdir \
		-csi.stagingdir /var/lib/kubelet/plugins/topolvm.cybozu.com/node/stagingdir \
		-csi.testvolumesize 1073741824

$(CA_FILES): ./certs/csr.json
	cfssl gencert -initca certs/csr.json | cfssljson -bare certs/ca

$(SERVER_CERT_FILES): $(CA_FILES) ./certs/ca-config.json ./certs/server.json
	cfssl gencert -ca=certs/ca.pem -ca-key=certs/ca-key.pem -config=certs/ca-config.json -profile=server certs/server.json | cfssljson -bare certs/server

hook-secret.yml: hook-secret.yml.template $(CA_FILES)
	./patch_capem.sh

secret: $(SERVER_CERT_FILES)
	$(KUBECTL) delete -n=kube-system secret topolvm-hook-certs --ignore-not-found
	$(KUBECTL) create -n=kube-system secret generic topolvm-hook-certs --from-file=certs/server.pem --from-file=certs/server-key.pem

setup: $(KUBECTL)
	go install github.com/cloudflare/cfssl/cmd/cfssl
	go install github.com/cloudflare/cfssl/cmd/cfssljson
	go install github.com/onsi/ginkgo/ginkgo
	cd /tmp; env GOFLAGS= GO111MODULE=on go get sigs.k8s.io/kind@v0.4.0
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs

clean: stop-lvmd
	rm -rf $(CA_FILES) $(SERVER_CERT_FILES) hook-secret.yml topolvm.img build/

.PHONY: launch-kind shutdown-kind start-lvmd stop-lvmd test sanity-test setup clean
