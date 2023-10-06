# CSI sidecar versions
EXTERNAL_PROVISIONER_VERSION = 3.6.0
EXTERNAL_RESIZER_VERSION = 1.8.0
NODE_DRIVER_REGISTRAR_VERSION = 2.8.0
LIVENESSPROBE_VERSION = 2.10.0
EXTERNAL_SNAPSHOTTER_VERSION = 6.3.0
CSI_SIDECARS = \
	external-provisioner \
	external-snapshotter \
	external-resizer \
	node-driver-registrar \
	livenessprobe

GOPATH ?= $(shell go env GOPATH)

SRC_ROOT = $(GOPATH)/src/github.com/kubernetes-csi
EXTERNAL_PROVISIONER_SRC  = $(SRC_ROOT)/external-provisioner
NODE_DRIVER_REGISTRAR_SRC = $(SRC_ROOT)/node-driver-registrar
EXTERNAL_RESIZER_SRC      = $(SRC_ROOT)/external-resizer
EXTERNAL_SNAPSHOTTER_SRC  = $(SRC_ROOT)/external-snapshotter
LIVENESSPROBE_SRC         = $(SRC_ROOT)/livenessprobe

MAKEFILE_DIR := $(dir $(firstword $(MAKEFILE_LIST)))
OUTPUT_DIR ?= .

CURL := curl -sSLf

build: $(CSI_SIDECARS)

external-provisioner: $(OUTPUT_DIR)/.csi-provisioner-$(EXTERNAL_PROVISIONER_VERSION)
	cp -f $< $(OUTPUT_DIR)/csi-provisioner

$(OUTPUT_DIR)/.csi-provisioner-$(EXTERNAL_PROVISIONER_VERSION):
	rm -rf $(EXTERNAL_PROVISIONER_SRC)
	mkdir -p $(EXTERNAL_PROVISIONER_SRC)
	$(CURL) https://github.com/kubernetes-csi/external-provisioner/archive/v$(EXTERNAL_PROVISIONER_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(EXTERNAL_PROVISIONER_SRC)
	patch -d $(EXTERNAL_PROVISIONER_SRC)/release-tools < $(MAKEFILE_DIR)/cache-packages.patch
	make -C $(EXTERNAL_PROVISIONER_SRC)
	cp -f $(EXTERNAL_PROVISIONER_SRC)/bin/csi-provisioner $@

external-snapshotter: $(OUTPUT_DIR)/.csi-snapshotter-$(EXTERNAL_SNAPSHOTTER_VERSION)
	cp -f $< $(OUTPUT_DIR)/csi-snapshotter

$(OUTPUT_DIR)/.csi-snapshotter-$(EXTERNAL_SNAPSHOTTER_VERSION):
	rm -rf $(EXTERNAL_SNAPSHOTTER_SRC)
	mkdir -p $(EXTERNAL_SNAPSHOTTER_SRC)
	curl -sSLf https://github.com/kubernetes-csi/external-snapshotter/archive/v$(EXTERNAL_SNAPSHOTTER_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(EXTERNAL_SNAPSHOTTER_SRC)
	patch -d $(EXTERNAL_SNAPSHOTTER_SRC)/release-tools < $(MAKEFILE_DIR)/cache-packages.patch
	make -C $(EXTERNAL_SNAPSHOTTER_SRC)
	cp -f $(EXTERNAL_SNAPSHOTTER_SRC)/bin/csi-snapshotter $@

external-resizer: $(OUTPUT_DIR)/.csi-resizer-$(EXTERNAL_RESIZER_VERSION)
	cp -f $< $(OUTPUT_DIR)/csi-resizer

$(OUTPUT_DIR)/.csi-resizer-$(EXTERNAL_RESIZER_VERSION):
	rm -rf $(EXTERNAL_RESIZER_SRC)
	mkdir -p $(EXTERNAL_RESIZER_SRC)
	$(CURL) https://github.com/kubernetes-csi/external-resizer/archive/v$(EXTERNAL_RESIZER_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(EXTERNAL_RESIZER_SRC)
	patch -d $(EXTERNAL_RESIZER_SRC)/release-tools < $(MAKEFILE_DIR)/cache-packages.patch
	make -C $(EXTERNAL_RESIZER_SRC)
	cp -f $(EXTERNAL_RESIZER_SRC)/bin/csi-resizer $@

node-driver-registrar: $(OUTPUT_DIR)/.csi-node-driver-registrar-$(NODE_DRIVER_REGISTRAR_VERSION)
	cp -f $< $(OUTPUT_DIR)/csi-node-driver-registrar

$(OUTPUT_DIR)/.csi-node-driver-registrar-$(NODE_DRIVER_REGISTRAR_VERSION):
	rm -rf $(NODE_DRIVER_REGISTRAR_SRC)
	mkdir -p $(NODE_DRIVER_REGISTRAR_SRC)
	$(CURL) https://github.com/kubernetes-csi/node-driver-registrar/archive/v$(NODE_DRIVER_REGISTRAR_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(NODE_DRIVER_REGISTRAR_SRC)
	patch -d $(NODE_DRIVER_REGISTRAR_SRC)/release-tools < $(MAKEFILE_DIR)/cache-packages.patch
	make -C $(NODE_DRIVER_REGISTRAR_SRC)
	cp -f $(NODE_DRIVER_REGISTRAR_SRC)/bin/csi-node-driver-registrar $@

livenessprobe: $(OUTPUT_DIR)/.livenessprobe-$(LIVENESSPROBE_VERSION)
	cp -f $< $(OUTPUT_DIR)/livenessprobe

$(OUTPUT_DIR)/.livenessprobe-$(LIVENESSPROBE_VERSION):
	rm -rf $(LIVENESSPROBE_SRC)
	mkdir -p $(LIVENESSPROBE_SRC)
	$(CURL) https://github.com/kubernetes-csi/livenessprobe/archive/v$(LIVENESSPROBE_VERSION).tar.gz | \
        tar zxf - --strip-components 1 -C $(LIVENESSPROBE_SRC)
	patch -d $(LIVENESSPROBE_SRC)/release-tools < $(MAKEFILE_DIR)/cache-packages.patch
	make -C $(LIVENESSPROBE_SRC)
	cp -f $(LIVENESSPROBE_SRC)/bin/livenessprobe $@

.PHONY: build $(CSI_SIDECARS)
