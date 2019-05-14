SUDO=sudo
GOFLAGS=-mod=vendor
export GOFLAGS

lvmetrics.img:
	CGO_ENABLED=0 go build ../pkg/lvmetrics
	CGO_ENABLED=0 go build -o lvmd ./lvmd-fake
	podman build -t docker.io/library/lvmetrics:latest .
	rm -f lvmetrics lvmd
	podman save -o $@ docker.io/library/lvmetrics:latest

test: lvmetrics.img
	microk8s.status --wait-ready
	$(SUDO) microk8s.ctr -n k8s.io images import $<


setup:
	$(SUDO) add-apt-repository -y ppa:projectatomic/ppa
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y podman snapd
	$(SUDO) snap install microk8s --classic

.PHONY: test setup
