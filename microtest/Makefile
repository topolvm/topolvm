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
	rm -rf tmpbin
	mkdir -p tmpbin
	CGO_ENABLED=0 go build -o tmpbin/hypertopolvm ../pkg/hypertopolvm
	CGO_ENABLED=0 go build -o tmpbin/lvmd ./lvmd-fake
	ln -s hypertopolvm ./tmpbin/csi-topolvm
	ln -s hypertopolvm ./tmpbin/lvmetrics
	ln -s hypertopolvm ./tmpbin/topolvm-scheduler
	ln -s hypertopolvm ./tmpbin/topolvm-node
	ln -s hypertopolvm ./tmpbin/topolvm-hook
	$(CONTAINER_BUILDER) build --no-cache --rm=false -f Dockerfile -t docker.io/library/topolvm:latest tmpbin
	rm -f $@
	$(CONTAINER_BUILDER) save -o $@ docker.io/library/topolvm:latest

install-microk8s:
	$(SUDO) snap install microk8s --classic
	echo "--allow-privileged=true" | $(SUDO) tee -a /var/snap/microk8s/current/args/kubelet
	echo "--allow-privileged=true" | $(SUDO) tee -a /var/snap/microk8s/current/args/kube-apiserver
	$(SUDO) systemctl restart snap.microk8s.daemon-kubelet.service
	$(SUDO) systemctl restart snap.microk8s.daemon-apiserver.service

uninstall-microk8s:
	# Disable automatic snapshot to prevent saving previous volume data
	$(SUDO) snap set system snapshots.automatic.retention=no
	$(SUDO) snap remove microk8s

start-lvmd: install-microk8s
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

stop-lvmd: uninstall-microk8s
	-$(SUDO) systemctl stop lvmd.service
	-$(SUDO) vgremove -ffy myvg
	-$(SUDO) pvremove -ffy $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	-$(SUDO) losetup -d $$($(SUDO) losetup -j build/backing_store | cut -d: -f1)
	-$(SUDO) dmsetup remove_all
	-rm build/backing_store

test: start-lvmd topolvm.img secret hook.yml
	$(SNAPBIN)/microk8s.status --wait-ready
	$(SUDO) $(SNAPBIN)/microk8s.ctr -n k8s.io images import topolvm.img
	$(KUBECTL) delete -f lvmetrics.yml -f scheduler.yml -f hook.yml -f scandpv.yml --ignore-not-found
	$(KUBECTL) apply -f lvmetrics.yml -f scheduler.yml -f hook.yml -f scandpv.yml
	$(SUDO) cp scheduler-config.yaml $(SNAPDATA)/scheduler-config.yaml
	if ! grep -q scheduler-config.yaml $(SNAPDATA)/args/kube-scheduler; then echo "--config=$(SNAPDATA)/scheduler-config.yaml" | $(SUDO) tee -a $(SNAPDATA)/args/kube-scheduler; fi
	$(SUDO) systemctl restart snap.microk8s.daemon-scheduler.service
	$(SUDO) bash -c "export PATH=$(PATH) GOPATH=$(GOPATH) GO111MODULE=on GOFLAGS=$(GOFLAGS); $(GINKGO) -v ."
	$(SUDO) $$(which csi-sanity) -ginkgo.v -test.v \
		-csi.endpoint /var/snap/microk8s/common/var/lib/kubelet/plugins/topolvm.cybozu.com/node/csi-topolvm.sock \
		-csi.controllerendpoint /var/snap/microk8s/common/var/lib/kubelet/plugins/topolvm.cybozu.com/controller/csi-topolvm.sock \
		-csi.mountdir /var/snap/microk8s/common/var/lib/kubelet/plugins/topolvm.cybozu.com/node/mountdir \
		-csi.stagingdir /var/snap/microk8s/common/var/lib/kubelet/plugins/topolvm.cybozu.com/node/stagingdir \
		-csi.testvolumesize 1073741824

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
	$(SUDO) snap install core
	go install github.com/cloudflare/cfssl/cmd/cfssl
	go install github.com/cloudflare/cfssl/cmd/cfssljson
	go install github.com/onsi/ginkgo/ginkgo
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y snapd lvm2 xfsprogs
ifeq ($(CONTAINER_BUILDER),podman)
	$(SUDO) apt-get install -y podman
endif
	GO111MODULE=off go get -d github.com/kubernetes-csi/csi-test/cmd/csi-sanity
	(cd $(GOPATH)/src/github.com/kubernetes-csi/csi-test/cmd/csi-sanity; make install)

clean: stop-lvmd
	rm -rf $(CA_FILES) $(SERVER_CERT_FILES) hook.yml topolvm.img build/

.PHONY: install-microk8s uninstall-microk8s start-lvmd stop-lvmd test setup clean
