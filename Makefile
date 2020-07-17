CSI_VERSION=1.1.0
PROTOBUF_VERSION=3.11.4
CURL=curl -Lsf
KUBEBUILDER_VERSION = 2.3.0
CTRLTOOLS_VERSION = 0.2.7

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

TOPOLVM_VERSION ?= devel
IMAGE_TAG ?= latest

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
	test -z "$$(nilerr ./... 2>&1 | tee /dev/stderr)"
	test -z "$$(custom-checker -restrictpkg.packages=html/template,log $$(go list -tags='$(GOTAGS)' ./... | grep -v /vendor/ ) 2>&1 | tee /dev/stderr)"
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
	rm -f deploy/manifests/base/crd.yaml
	cp config/crd/bases/topolvm.cybozu.com_logicalvolumes.yaml deploy/manifests/base/crd.yaml

generate: csi/csi.pb.go lvmd/proto/lvmd.pb.go docs/lvmd-protocol.md
	controller-gen object:headerFile=./hack/boilerplate.go.txt paths="./api/..."

check-uncommitted:
	$(MAKE) manifests
	$(MAKE) generate
	diffs=$$(git diff --name-only); if [ "$$diffs" != "" ]; then printf "\n>>> Uncommited changes \n%s\n\n" "$$diffs"; exit 1; fi

build: build/hypertopolvm build/lvmd csi-sidecars

build/hypertopolvm: $(GO_FILES)
	mkdir -p build
	go build -o $@ -ldflags "-X github.com/topolvm/topolvm.Version=$(TOPOLVM_VERSION)" ./pkg/hypertopolvm

build/lvmd:
	mkdir -p build
	CGO_ENABLED=0 go build -o $@ -ldflags "-X github.com/topolvm/topolvm.Version=$(TOPOLVM_VERSION)" ./pkg/lvmd

csi-sidecars:
	mkdir -p build
	make -f csi-sidecars.mk OUTPUT_DIR=build

image:
	docker build -t $(IMAGE_PREFIX)topolvm:devel .

tag:
	docker tag $(IMAGE_PREFIX)topolvm:devel $(IMAGE_PREFIX)topolvm:$(IMAGE_TAG)

push:
	docker push $(IMAGE_PREFIX)topolvm:$(IMAGE_TAG)

clean:
	rm -rf build/
	rm -rf bin/
	rm -rf include/

tools:
	cd /tmp; env GOFLAGS= GO111MODULE=on go get golang.org/x/tools/cmd/goimports
	cd /tmp; env GOFLAGS= GO111MODULE=on go get golang.org/x/lint/golint
	cd /tmp; env GOFLAGS= GO111MODULE=on go get github.com/gordonklaus/ineffassign
	cd /tmp; env GOFLAGS= GO111MODULE=on go get github.com/gostaticanalysis/nilerr/cmd/nilerr
	cd /tmp; env GOFLAGS= GO111MODULE=on go get github.com/cybozu/neco-containers/golang/analyzer/cmd/...

setup: tools
	$(SUDO) apt-get update
	$(SUDO) apt-get -y install --no-install-recommends $(PACKAGES)
	curl -sL https://go.kubebuilder.io/dl/$(KUBEBUILDER_VERSION)/$(GOOS)/$(GOARCH) | tar -xz -C /tmp/
	$(SUDO) rm -rf /usr/local/kubebuilder
	$(SUDO) mv /tmp/kubebuilder_$(KUBEBUILDER_VERSION)_$(GOOS)_$(GOARCH) /usr/local/kubebuilder
	$(SUDO) curl -o /usr/local/kubebuilder/bin/kustomize -sL https://go.kubebuilder.io/kustomize/$(GOOS)/$(GOARCH)
	$(SUDO) chmod a+x /usr/local/kubebuilder/bin/kustomize
	cd /tmp; env GOFLAGS= GO111MODULE=on go get sigs.k8s.io/controller-tools/cmd/controller-gen@v$(CTRLTOOLS_VERSION)

.PHONY: all test manifests generate check-uncommitted build csi-sidecars image tools setup
