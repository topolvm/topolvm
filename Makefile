CSI_VERSION=1.1.0
PROTOBUF_VERSION=3.7.1
CURL=curl -Lsf

csi.proto:
	$(CURL) -o $@ https://raw.githubusercontent.com/container-storage-interface/spec/v$(CSI_VERSION)/csi.proto

bin/protoc:
	$(CURL) -o protobuf.zip https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-$(PROTOBUF_VERSION)-linux-x86_64.zip
	unzip protobuf.zip bin/protoc 'include/*'
	rm -f protobuf.zip

bin/protoc-gen-go:
	GOBIN=$(shell pwd)/bin go install -mod=vendor github.com/golang/protobuf/protoc-gen-go

csi/csi.pb.go: csi.proto bin/protoc bin/protoc-gen-go
	mkdir -p csi
	bin/protoc -I. --go_out=csi $<
