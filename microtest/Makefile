SUDO=sudo
CONTAINER_BUILDER=podman
GOFLAGS=-mod=vendor
export GOFLAGS

lvmetrics.img:
	CGO_ENABLED=0 go build ../pkg/lvmetrics
	CGO_ENABLED=0 go build -o lvmd ./lvmd-fake
	$(CONTAINER_BUILDER) build -t docker.io/library/lvmetrics:latest .
	rm -f lvmetrics lvmd
	$(CONTAINER_BUILDER) save -o $@ docker.io/library/lvmetrics:latest

test: lvmetrics.img
	/snap/bin/microk8s.status --wait-ready
	$(SUDO) /snap/bin/microk8s.ctr -n k8s.io images import $<
	/snap/bin/microk8s.kubectl apply -f lvmetrics.yml
	./test.sh

setup:
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y snapd
ifeq ($(CONTAINER_BUILDER),podman)
	$(SUDO) apt-get install -y podman
endif
	$(SUDO) snap install microk8s --classic

.PHONY: test setup
