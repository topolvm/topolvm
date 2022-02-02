# CSI sidecar versions
EXTERNAL_PROVISIONER_VERSION = 3.0.0
EXTERNAL_RESIZER_VERSION = 1.3.0
NODE_DRIVER_REGISTRAR_VERSION = 2.3.0
LIVENESSPROBE_VERSION = 2.4.0

GOPATH ?= $(shell go env GOPATH)

SRC_ROOT = $(GOPATH)/src/github.com/kubernetes-csi
EXTERNAL_PROVISIONER_SRC  = $(SRC_ROOT)/external-provisioner
NODE_DRIVER_REGISTRAR_SRC = $(SRC_ROOT)/node-driver-registrar
EXTERNAL_RESIZER_SRC      = $(SRC_ROOT)/external-resizer
LIVENESSPROBE_SRC         = $(SRC_ROOT)/livenessprobe

OUTPUT_DIR ?= .

.PHONY: build
build: $(OUTPUT_DIR)/csi-provisioner
build: $(OUTPUT_DIR)/csi-resizer
build: $(OUTPUT_DIR)/csi-node-driver-registrar
build: $(OUTPUT_DIR)/livenessprobe

$(OUTPUT_DIR)/csi-provisioner:
	rm -rf $(EXTERNAL_PROVISIONER_SRC)
	mkdir -p $(EXTERNAL_PROVISIONER_SRC)
	curl -sSLf https://github.com/kubernetes-csi/external-provisioner/archive/v$(EXTERNAL_PROVISIONER_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(EXTERNAL_PROVISIONER_SRC)
	make -C $(EXTERNAL_PROVISIONER_SRC)
	cp -f $(EXTERNAL_PROVISIONER_SRC)/bin/csi-provisioner $(OUTPUT_DIR)/

$(OUTPUT_DIR)/csi-resizer:
	rm -rf $(EXTERNAL_RESIZER_SRC)
	mkdir -p $(EXTERNAL_RESIZER_SRC)
	curl -sSLf https://github.com/kubernetes-csi/external-resizer/archive/v$(EXTERNAL_RESIZER_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(EXTERNAL_RESIZER_SRC)
	make -C $(EXTERNAL_RESIZER_SRC)
	cp -f $(EXTERNAL_RESIZER_SRC)/bin/csi-resizer $(OUTPUT_DIR)/

$(OUTPUT_DIR)/csi-node-driver-registrar:
	rm -rf $(NODE_DRIVER_REGISTRAR_SRC)
	mkdir -p $(NODE_DRIVER_REGISTRAR_SRC)
	curl -sSLf https://github.com/kubernetes-csi/node-driver-registrar/archive/v$(NODE_DRIVER_REGISTRAR_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(NODE_DRIVER_REGISTRAR_SRC)
	make -C $(NODE_DRIVER_REGISTRAR_SRC)
	cp -f $(NODE_DRIVER_REGISTRAR_SRC)/bin/csi-node-driver-registrar $(OUTPUT_DIR)/

$(OUTPUT_DIR)/livenessprobe:
	rm -rf $(LIVENESSPROBE_SRC)
	mkdir -p $(LIVENESSPROBE_SRC)
	curl -sSLf https://github.com/kubernetes-csi/livenessprobe/archive/v$(LIVENESSPROBE_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(LIVENESSPROBE_SRC)
	make -C $(LIVENESSPROBE_SRC)
	cp -f $(LIVENESSPROBE_SRC)/bin/livenessprobe $(OUTPUT_DIR)/
