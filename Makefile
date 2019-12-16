CSI_VERSION=1.1.0
PROTOBUF_VERSION=3.7.1
CURL=curl -Lsf
KUBEBUILDER_VERSION = 2.0.1
CTRLTOOLS_VERSION = 0.2.1

GOOS := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
GOFLAGS = -mod=vendor
export GOFLAGS

PTYPES_PKG := github.com/golang/protobuf/ptypes
GO_OUT := plugins=grpc
GO_OUT := $(GO_OUT),Mgoogle/protobuf/descriptor.proto=github.com/golang/protobuf/protoc-gen-go/descriptor
GO_OUT := $(GO_OUT),Mgoogle/protobuf/wrappers.proto=$(PTYPES_PKG)/wrappers
GO_OUT := $(GO_OUT):csi

SUDO=sudo
PACKAGES := unzip
GO_FILES=$(shell find -name '*.go' -not -name '*_test.go')
BUILD_TARGET=hypertopolvm

# CSI sidecar containers
EXTERNAL_PROVISIONER_VERSION=1.4.0
NODE_DRIVER_REGISTRAR_VERSION=1.2.0
EXTERNAL_ATTACHER_VERSION=1.2.1
LIVENESSPROBE_VERSION = 1.1.0
CSI_SIDECARS = \
	external-provisioner \
	node-driver-registrar \
	external-attacher \
	livenessprobe

all: build

external-provisioner:
	mkdir -p build
	mkdir -p $(GOPATH)/src/github.com/kubernetes-csi
	rm -rf $(GOPATH)/src/github.com/kubernetes-csi/external-provisioner
	curl -sSLf https://github.com/kubernetes-csi/external-provisioner/archive/v$(EXTERNAL_PROVISIONER_VERSION).tar.gz | \
        tar zxf - -C $(GOPATH)/src/github.com/kubernetes-csi/
	mv $(GOPATH)/src/github.com/kubernetes-csi/external-provisioner-$(EXTERNAL_PROVISIONER_VERSION) \
		$(GOPATH)/src/github.com/kubernetes-csi/external-provisioner/
	(cd $(GOPATH)/src/github.com/kubernetes-csi/external-provisioner/; GO111MODULE=off make)
	cp -f $(GOPATH)/src/github.com/kubernetes-csi/external-provisioner/bin/csi-provisioner ./build/

node-driver-registrar:
	mkdir -p build
	mkdir -p $(GOPATH)/src/github.com/kubernetes-csi
	rm -rf $(GOPATH)/src/github.com/kubernetes-csi/node-driver-registrar
	curl -sSLf https://github.com/kubernetes-csi/node-driver-registrar/archive/v$(NODE_DRIVER_REGISTRAR_VERSION).tar.gz | \
        tar zxf - -C $(GOPATH)/src/github.com/kubernetes-csi/
	mv $(GOPATH)/src/github.com/kubernetes-csi/node-driver-registrar-$(NODE_DRIVER_REGISTRAR_VERSION) \
		$(GOPATH)/src/github.com/kubernetes-csi/node-driver-registrar/
	(cd $(GOPATH)/src/github.com/kubernetes-csi/node-driver-registrar/; GO111MODULE=off make)
	cp -f $(GOPATH)/src/github.com/kubernetes-csi/node-driver-registrar/bin/csi-node-driver-registrar ./build/

external-attacher:
	mkdir -p build
	mkdir -p $(GOPATH)/src/github.com/kubernetes-csi
	rm -rf $(GOPATH)/src/github.com/kubernetes-csi/external-attacher
	curl -sSLf https://github.com/kubernetes-csi/external-attacher/archive/v$(EXTERNAL_ATTACHER_VERSION).tar.gz | \
        tar zxf - -C $(GOPATH)/src/github.com/kubernetes-csi/
	mv $(GOPATH)/src/github.com/kubernetes-csi/external-attacher-$(EXTERNAL_ATTACHER_VERSION) \
		$(GOPATH)/src/github.com/kubernetes-csi/external-attacher/
	(cd $(GOPATH)/src/github.com/kubernetes-csi/external-attacher/; GO111MODULE=off make)
	cp -f $(GOPATH)/src/github.com/kubernetes-csi/external-attacher/bin/csi-attacher ./build/

livenessprobe:
	mkdir -p build
	mkdir -p $(GOPATH)/src/github.com/kubernetes-csi
	rm -rf $(GOPATH)/src/github.com/kubernetes-csi/livenessprobe
	curl -sSLf https://github.com/kubernetes-csi/livenessprobe/archive/v$(LIVENESSPROBE_VERSION).tar.gz | \
        tar zxf - -C $(GOPATH)/src/github.com/kubernetes-csi/
	mv $(GOPATH)/src/github.com/kubernetes-csi/livenessprobe-$(LIVENESSPROBE_VERSION) \
		$(GOPATH)/src/github.com/kubernetes-csi/livenessprobe/
	(cd $(GOPATH)/src/github.com/kubernetes-csi/livenessprobe/; GO111MODULE=off make)
	cp -f $(GOPATH)/src/github.com/kubernetes-csi/livenessprobe/bin/livenessprobe ./build/

csi.proto:
	$(CURL) -o $@ https://raw.githubusercontent.com/container-storage-interface/spec/v$(CSI_VERSION)/csi.proto

bin/protoc:
	rm -rf include
	$(CURL) -o protobuf.zip https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOBUF_VERSION)/protoc-$(PROTOBUF_VERSION)-linux-x86_64.zip
	unzip protobuf.zip bin/protoc 'include/*'
	rm -f protobuf.zip

bin/protoc-gen-go:
	GOBIN="$(shell pwd)/bin" go install -mod=vendor github.com/golang/protobuf/protoc-gen-go

bin/protoc-gen-doc:
	$(CURL) https://github.com/pseudomuto/protoc-gen-doc/releases/download/v1.3.0/protoc-gen-doc-1.3.0.linux-amd64.go1.11.2.tar.gz | tar xzf - -C bin --strip-components=1

csi/csi.pb.go: csi.proto bin/protoc bin/protoc-gen-go
	mkdir -p csi
	PATH="$(shell pwd)/bin:$(PATH)" bin/protoc -I. --go_out=$(GO_OUT) $<

lvmd/proto/lvmd.pb.go: lvmd/proto/lvmd.proto bin/protoc bin/protoc-gen-go
	PATH="$(shell pwd)/bin:$(PATH)" bin/protoc -I. --go_out=plugins=grpc:. $<

docs/lvmd-protocol.md: lvmd/proto/lvmd.proto bin/protoc bin/protoc-gen-doc
	PATH="$(shell pwd)/bin:$(PATH)" bin/protoc -I. --doc_out=./docs --doc_opt=markdown,$@ $<

test:
	test -z "$$(gofmt -s -l . | grep -v '^vendor' | tee /dev/stderr)"
	test -z "$$(golint $$(go list ./... | grep -v /vendor/) | tee /dev/stderr)"
	ineffassign .
	go install ./...
	go test -race -v ./...
	go vet ./...
	test -z "$$(go vet ./... | grep -v '^vendor' | tee /dev/stderr)"

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	controller-gen \
		crd:trivialVersions=true \
		rbac:roleName=topolvm-controller \
		webhook \
		paths="./api/...;./controllers;./hook;./driver/k8s" \
		output:crd:artifacts:config=config/crd/bases
	rm -f deploy/manifests/crd.yaml
	cp config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml deploy/manifests/crd.yaml

generate: csi/csi.pb.go lvmd/proto/lvmd.pb.go docs/lvmd-protocol.md
	controller-gen object:headerFile=./hack/boilerplate.go.txt paths="./api/..."

build: $(BUILD_TARGET) $(CSI_SIDECARS) build/lvmd

build/lvmd:
	mkdir -p build
	CGO_ENABLED=0 go build -o $@ ./pkg/lvmd

$(BUILD_TARGET): $(GO_FILES)
	mkdir -p build
	go build -o ./build/$@ ./pkg/$@

clean:
	rm -rf build/

setup:
	$(SUDO) apt-get update
	$(SUDO) apt-get -y install --no-install-recommends $(PACKAGES)
	curl -sL https://go.kubebuilder.io/dl/$(KUBEBUILDER_VERSION)/$(GOOS)/$(GOARCH) | tar -xz -C /tmp/
	$(SUDO) rm -rf /usr/local/kubebuilder
	$(SUDO) mv /tmp/kubebuilder_$(KUBEBUILDER_VERSION)_$(GOOS)_$(GOARCH) /usr/local/kubebuilder
	$(SUDO) curl -o /usr/local/kubebuilder/bin/kustomize -sL https://go.kubebuilder.io/kustomize/$(GOOS)/$(GOARCH)
	$(SUDO) chmod a+x /usr/local/kubebuilder/bin/kustomize
	cd /tmp; GO111MODULE=on GOFLAGS= go get sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CTRLTOOLS_VERSION)

.PHONY: all test manifests generate build setup $(CSI_SIDECARS)
