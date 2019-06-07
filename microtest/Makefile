SUDO=sudo
CONTAINER_BUILDER=podman
GOFLAGS=-mod=vendor
export GOFLAGS
GINKGO=$(GOPATH)/bin/ginkgo
SNAPBIN=/snap/bin
KUBECTL=$(SNAPBIN)/microk8s.kubectl
SNAPDATA=/var/snap/microk8s/current

GO_FILES=$(shell find ../ -path ../microtest -prune -o -name "*.go" -print)
CA_FILES=./certs/ca.csr ./certs/ca.pem ./certs/ca-key.pem
SERVER_CERT_FILES=./certs/server.csr ./certs/server.pem ./certs/server-key.pem

topolvm.img: $(GO_FILES)
	mkdir -p build
	CGO_ENABLED=0 go build -o build/lvmetrics ../pkg/lvmetrics
	CGO_ENABLED=0 go build -o build/topolvm-scheduler ../pkg/topolvm-scheduler
	CGO_ENABLED=0 go build -o build/topolvm-hook ../pkg/topolvm-hook
	CGO_ENABLED=0 go build -o build/csi-topolvm ../pkg/csi-topolvm
	CGO_ENABLED=0 go build -o build/topolvm-node ../pkg/topolvm-node
	CGO_ENABLED=0 go build -o build/lvmd-fake ./lvmd-fake
	$(CONTAINER_BUILDER) build -t docker.io/library/topolvm:latest .
	rm -f $@
	$(CONTAINER_BUILDER) save -o $@ docker.io/library/topolvm:latest

start-lvmd:
	mkdir -p build
	CGO_ENABLED=0 go build -o build/lvmd ../pkg/lvmd
	if [ "$$($(SUDO) losetup -j build/backing_store)" ]; then $(SUDO) vgremove myvg; $(SUDO) pvremove $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); $(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1); fi
	truncate --size=10G build/backing_store
	$(SUDO) losetup -f build/backing_store
	$(SUDO) pvcreate $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	$(SUDO) vgcreate myvg $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	mkdir -p /tmp/topolvm
	$(SUDO) systemd-run --unit=lvmd.service $(shell pwd)/build/lvmd --volume-group=myvg --listen=/tmp/topolvm/lvmd.sock

stop-lvmd:
	$(SUDO) systemctl stop lvmd.service
	$(SUDO) systemctl disable lvmd.service
	$(SUDO) rm /run/systemd/transient/lvmd.service
	$(SUDO) systemctl daemon-reload
	$(SUDO) systemctl reset-failed lvmd.service
	$(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	rm build/backing_store

test: topolvm.img secret hook.yml
	$(SNAPBIN)/microk8s.status --wait-ready
	$(SUDO) $(SNAPBIN)/microk8s.ctr -n k8s.io images import $<
	$(KUBECTL) delete -f lvmetrics.yml -f scheduler.yml -f hook.yml -f scandpv.yml --ignore-not-found
	$(KUBECTL) apply -f lvmetrics.yml -f scheduler.yml -f hook.yml -f scandpv.yml
	$(SUDO) cp scheduler-config.yaml $(SNAPDATA)/scheduler-config.yaml
	if ! grep -q scheduler-config.yaml $(SNAPDATA)/args/kube-scheduler; then echo "--config=$(SNAPDATA)/scheduler-config.yaml" | $(SUDO) tee -a $(SNAPDATA)/args/kube-scheduler; fi
	$(SUDO) systemctl restart snap.microk8s.daemon-scheduler.service
	$(GINKGO) -v .

$(CA_FILES): ./certs/csr.json
	cfssl gencert -initca certs/csr.json | cfssljson -bare certs/ca

$(SERVER_CERT_FILES): $(CA_FILES) ./certs/ca-config.json ./certs/server.json
	cfssl gencert -ca=certs/ca.pem -ca-key=certs/ca-key.pem -config=certs/ca-config.json -profile=server certs/server.json | cfssljson -bare certs/server

hook.yml: hook.yml.template $(CA_FILES)
	./patch_capem.sh

secret: $(SERVER_CERT_FILES)
	$(KUBECTL) delete -n=kube-system secret topolvm-hook-certs --ignore-not-found
	$(KUBECTL) create -n=kube-system secret generic topolvm-hook-certs --from-file=certs/server.pem --from-file=certs/server-key.pem

setup:
	go install github.com/cloudflare/cfssl/cmd/cfssl
	go install github.com/cloudflare/cfssl/cmd/cfssljson
	go install github.com/onsi/ginkgo/ginkgo
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y snapd lvm2
ifeq ($(CONTAINER_BUILDER),podman)
	$(SUDO) apt-get install -y podman
endif
	$(SUDO) snap install core
	$(SUDO) snap install microk8s --classic
	echo "--allow-privileged=true" | $(SUDO) tee -a /var/snap/microk8s/current/args/kubelet
	echo "--allow-privileged=true" | $(SUDO) tee -a /var/snap/microk8s/current/args/kube-apiserver
	$(SUDO) systemctl restart snap.microk8s.daemon-kubelet.service
	$(SUDO) systemctl restart snap.microk8s.daemon-apiserver.service

clean:
	rm -rf $(CA_FILES) $(SERVER_CERT_FILES) hook.yml topolvm.img build/

.PHONY: test setup
