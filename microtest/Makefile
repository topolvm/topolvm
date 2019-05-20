SUDO=sudo
CONTAINER_BUILDER=podman
GOFLAGS=-mod=vendor
export GOFLAGS
GINKGO=$(GOPATH)/bin/ginkgo
SNAPBIN=/snap/bin
KUBECTL=$(SNAPBIN)/microk8s.kubectl
SNAPDATA=/var/snap/microk8s/current

GO_FILES=$(shell find ../ -name "*.go" -print)

topolvm.img: $(GO_FILES)
	CGO_ENABLED=0 go build ../pkg/lvmetrics
	CGO_ENABLED=0 go build ../pkg/topolvm-scheduler
	CGO_ENABLED=0 go build ../pkg/topolvm-hook
	CGO_ENABLED=0 go build -o lvmd ./lvmd-fake
	$(CONTAINER_BUILDER) build -t docker.io/library/lvmetrics:latest .
	rm -f lvmetrics topolvm-scheduler topolvm-hook lvmd
	rm -f $@
	$(CONTAINER_BUILDER) save -o $@ docker.io/library/lvmetrics:latest

test: topolvm.img
	$(SNAPBIN)/microk8s.status --wait-ready
	$(SUDO) $(SNAPBIN)/microk8s.ctr -n k8s.io images import $<
	$(KUBECTL) delete -f lvmetrics.yml --ignore-not-found --wait
	$(KUBECTL) delete -f scheduler.yml --ignore-not-found --wait
	$(KUBECTL) apply -f lvmetrics.yml
	$(KUBECTL) apply -f scheduler.yml
	$(SUDO) cp scheduler-config.yaml $(SNAPDATA)/scheduler-config.yaml
	if ! grep -q scheduler-config.yaml $(SNAPDATA)/args/kube-scheduler; then echo "--config=$(SNAPDATA)/scheduler-config.yaml" | $(SUDO) tee -a $(SNAPDATA)/args/kube-scheduler; fi
	$(SUDO) systemctl restart snap.microk8s.daemon-scheduler.service
	$(GINKGO) -v .

setup:
	go install github.com/onsi/ginkgo/ginkgo
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y snapd
ifeq ($(CONTAINER_BUILDER),podman)
	$(SUDO) apt-get install -y podman
endif
	$(SUDO) snap install core
	$(SUDO) snap install microk8s --classic

.PHONY: test setup
