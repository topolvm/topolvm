SUDO=sudo
GOFLAGS=-mod=vendor
export GOFLAGS
KUBECTL=kubectl

GO_FILES=$(shell find ../ -path ../microtest -prune -o -name "*.go" -print)
CA_FILES=./certs/ca.csr ./certs/ca.pem ./certs/ca-key.pem
SERVER_CERT_FILES=./certs/server.csr ./certs/server.pem ./certs/server-key.pem

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

/tmp/topolvm/scheduler-config.yaml: scheduler-config.yaml
	mkdir -p /tmp/topolvm
	cp scheduler-config.yaml /tmp/topolvm/

launch-kind: /tmp/topolvm/scheduler-config.yaml
	env KUBECONFIG= kind create cluster --config topolvm-cluster.yaml --loglevel debug --image kindest/node:v1.14.1

shutdown-kind:
	kind delete cluster

start-lvmd:
	mkdir -p build
	CGO_ENABLED=0 go build -o build/lvmd ../pkg/lvmd
	if [ "$$($(SUDO) losetup -j build/backing_store)" ]; then $(SUDO) vgremove myvg; $(SUDO) pvremove $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); $(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); fi
	truncate --size=20G build/backing_store
	$(SUDO) losetup -f build/backing_store
	$(SUDO) pvcreate -fy $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	$(SUDO) vgcreate myvg $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	mkdir -p /tmp/topolvm
	$(SUDO) systemd-run --unit=lvmd.service $(shell pwd)/build/lvmd --volume-group=myvg --listen=/tmp/topolvm/lvmd.sock

stop-lvmd:
	-$(SUDO) systemctl stop lvmd.service
	-$(SUDO) vgremove -ffy myvg
	$(SUDO) pvremove -ffy $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	$(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	$(SUDO) dmsetup remove_all
	rm build/backing_store

test: secret
	kind load docker-image topolvm:dev
	$(KUBECTL) delete -f lvmetrics.yml -f scheduler.yml -f hook-secret.yml -f hook.yml -f scandpv.yml --ignore-not-found
	$(KUBECTL) apply -f lvmetrics.yml -f scheduler.yml -f hook-secret.yml -f hook.yml -f scandpv.yml
	docker cp ./kube-scheduler.yaml kind-control-plane:/etc/kubernetes/manifests/kube-scheduler.yaml
	# restart kube-scheduler
	$(KUBECTL) delete -n kube-system pod kube-scheduler-kind-control-plane
	$(KUBECTL) create namespace topolvm-system
	$(KUBECTL) apply -f ../topolvm-node/config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml
	$(KUBECTL) apply -f csi.yml

$(CA_FILES): ./certs/csr.json
	cfssl gencert -initca certs/csr.json | cfssljson -bare certs/ca

$(SERVER_CERT_FILES): $(CA_FILES) ./certs/ca-config.json ./certs/server.json
	cfssl gencert -ca=certs/ca.pem -ca-key=certs/ca-key.pem -config=certs/ca-config.json -profile=server certs/server.json | cfssljson -bare certs/server

hook-secret.yml: hook-secret.yml.template $(CA_FILES)
	./patch_capem.sh

secret: $(SERVER_CERT_FILES)
	$(KUBECTL) delete -n=kube-system secret topolvm-hook-certs --ignore-not-found
	$(KUBECTL) create -n=kube-system secret generic topolvm-hook-certs --from-file=certs/server.pem --from-file=certs/server-key.pem

setup:
	go install github.com/cloudflare/cfssl/cmd/cfssl
	go install github.com/cloudflare/cfssl/cmd/cfssljson
	go install github.com/onsi/ginkgo/ginkgo
	cd /tmp; env GOFLAGS= GO111MODULE=on go get sigs.k8s.io/kind@v0.4.0
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y lvm2 xfsprogs

clean: stop-lvmd
	rm -rf $(CA_FILES) $(SERVER_CERT_FILES) hook-secret.yml topolvm.img build/

.PHONY: launch-kind shutdown-kind start-lvmd stop-lvmd test setup clean
