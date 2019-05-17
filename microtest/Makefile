SUDO=sudo
CONTAINER_BUILDER=podman
GOFLAGS=-mod=vendor
export GOFLAGS

topolvm.img:
	CGO_ENABLED=0 go build ../pkg/lvmetrics
	CGO_ENABLED=0 go build ../pkg/topolvm-scheduler
	CGO_ENABLED=0 go build -o lvmd ./lvmd-fake
	$(CONTAINER_BUILDER) build -t docker.io/library/lvmetrics:latest .
	rm -f lvmetrics topolvm-scheduler lvmd
	$(CONTAINER_BUILDER) save -o $@ docker.io/library/lvmetrics:latest

test: topolvm.img
	/snap/bin/microk8s.status --wait-ready
	$(SUDO) /snap/bin/microk8s.ctr -n k8s.io images import $<
	/snap/bin/microk8s.kubectl apply -f lvmetrics.yml
	./test.sh

test-scheduler: topolvm.img
	/snap/bin/microk8s.status --wait-ready
	$(SUDO) /snap/bin/microk8s.ctr -n k8s.io images import $<
	/snap/bin/microk8s.kubectl apply -f scheduler.yml
	$(SUDO) cp scheduler-config.yaml /var/snap/microk8s/current/scheduler-config.yaml
	echo "--config=/var/snap/microk8s/current/scheduler-config.yaml" | $(SUDO) tee -a /var/snap/microk8s/current/args/kube-scheduler
	$(SUDO) systemctl restart snap.microk8s.daemon-scheduler.service
	/snap/bin/microk8s.kubectl apply -f nginx.yml

setup:
	$(SUDO) apt-get update
	$(SUDO) apt-get install -y snapd
ifeq ($(CONTAINER_BUILDER),podman)
	$(SUDO) apt-get install -y podman
endif
	$(SUDO) snap install core
	$(SUDO) snap install microk8s --classic

.PHONY: test setup
