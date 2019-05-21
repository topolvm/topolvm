SUDO=sudo
CONTAINER_BUILDER=podman
GOFLAGS=-mod=vendor
export GOFLAGS
GINKGO=$(GOPATH)/bin/ginkgo
SNAPBIN=/snap/bin
KUBECTL=$(SNAPBIN)/microk8s.kubectl
SNAPDATA=/var/snap/microk8s/current

GO_FILES=$(shell find ../ -name "*.go" -print)
CA_FILES=./certs/ca.csr ./certs/ca.pem ./certs/ca-key.pem
SERVER_CERT_FILES=./certs/server.csr ./certs/server.pem ./certs/server-key.pem

topolvm.img: $(GO_FILES)
	CGO_ENABLED=0 go build ../pkg/lvmetrics
	CGO_ENABLED=0 go build ../pkg/topolvm-scheduler
	CGO_ENABLED=0 go build ../pkg/topolvm-hook
	CGO_ENABLED=0 go build -o lvmd ./lvmd-fake
	$(CONTAINER_BUILDER) build -t docker.io/library/lvmetrics:latest .
	rm -f lvmetrics topolvm-scheduler topolvm-hook lvmd
	rm -f $@
	$(CONTAINER_BUILDER) save -o $@ docker.io/library/lvmetrics:latest

test: topolvm.img certs secret hook.yml
	$(SNAPBIN)/microk8s.status --wait-ready
	$(SUDO) $(SNAPBIN)/microk8s.ctr -n k8s.io images import $<
	$(KUBECTL) delete -f lvmetrics.yml -f scheduler.yml -f hook.yml -f scandpv.yml --ignore-not-found --wait
	$(KUBECTL) apply -f lvmetrics.yml -f scheduler.yml -f hook.yml -f scandpv.yml
	$(SUDO) cp scheduler-config.yaml $(SNAPDATA)/scheduler-config.yaml
	if ! grep -q scheduler-config.yaml $(SNAPDATA)/args/kube-scheduler; then echo "--config=$(SNAPDATA)/scheduler-config.yaml" | $(SUDO) tee -a $(SNAPDATA)/args/kube-scheduler; fi
	$(SUDO) systemctl restart snap.microk8s.daemon-scheduler.service
	$(GINKGO) -v .

$(CA_FILES): ./certs/csr.json
	cfssl gencert -initca certs/csr.json | cfssljson -bare certs/ca

$(SERVER_CERT_FILES): $(CA_FILES) ./certs/ca-config.json ./certs/server.json
	cfssl gencert -ca=certs/ca.pem -ca-key=certs/ca-key.pem -config=certs/ca-config.json -profile=server certs/server.json | cfssljson -bare certs/server

certs: $(CA_FILES) $(SERVER_CERT_FILES)

hook.yml: hook.yml.template $(CA_FILES)
	./patch_capem.sh

secret: $(SERVER_CERT_FILES)
	$(KUBECTL) create -n=kube-system secret generic topolvm-hook-certs --from-file=certs/server.pem --from-file=certs/server-key.pem

setup:
	go install github.com/cloudflare/cfssl/cmd/cfssl
	go install github.com/cloudflare/cfssl/cmd/cfssljson
	go install github.com/onsi/ginkgo/ginkgo
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y snapd
ifeq ($(CONTAINER_BUILDER),podman)
	$(SUDO) apt-get install -y podman
endif
	$(SUDO) snap install core
	$(SUDO) snap install microk8s --classic

clean:
	rm -f $(CA_FILES) $(SERVER_CERT_FILES) hook.yml topolvm.img
	$(KUBECTL) -n=kube-system delete secret topolvm-hook-certs

.PHONY: test setup certs
