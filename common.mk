# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
KIND_VERSION=v0.20.0

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.27.3)
	KIND_NODE_IMAGE=kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72
else ifeq ($(KUBERNETES_VERSION), 1.26.6)
	KIND_NODE_IMAGE=kindest/node:v1.26.6@sha256:6e2d8b28a5b601defe327b98bd1c2d1930b49e5d8c512e1895099e4504007adb
else ifeq ($(KUBERNETES_VERSION), 1.25.11)
	KIND_NODE_IMAGE=kindest/node:v1.25.11@sha256:227fa11ce74ea76a0474eeefb84cb75d8dad1b08638371ecf0e86259b35be0c8
endif
