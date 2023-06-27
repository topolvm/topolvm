# kind node image version is related to kind version.
# if you change kind version, also change kind node image version.
KIND_VERSION=v0.18.0

# The container version of kind must be with the digest.
# ref. https://github.com/kubernetes-sigs/kind/releases
ifeq ($(KUBERNETES_VERSION), 1.26.3)
	KIND_NODE_IMAGE=kindest/node:v1.26.3@sha256:61b92f38dff6ccc29969e7aa154d34e38b89443af1a2c14e6cfbd2df6419c66f
else ifeq ($(KUBERNETES_VERSION), 1.25.8)
	KIND_NODE_IMAGE=kindest/node:v1.25.8@sha256:00d3f5314cc35327706776e95b2f8e504198ce59ac545d0200a89e69fce10b7f
else ifeq ($(KUBERNETES_VERSION), 1.24.12)
	KIND_NODE_IMAGE=kindest/node:v1.24.12@sha256:1e12918b8bc3d4253bc08f640a231bb0d3b2c5a9b28aa3f2ca1aee93e1e8db16
endif
