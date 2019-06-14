FROM quay.io/cybozu/ubuntu-debug:18.04

# csi-topolvm node requires file command
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        file \
    && rm -rf /var/lib/apt/lists/*

COPY build/lvmetrics /lvmetrics
COPY build/topolvm-scheduler /topolvm-scheduler
COPY build/topolvm-hook /topolvm-hook
COPY build/csi-topolvm /csi-topolvm
COPY build/topolvm-node /topolvm-node
COPY build/lvmd-fake /lvmd
# CSI sidecar
COPY build/csi-provisioner /csi-provisioner
COPY build/csi-node-driver-registrar /csi-node-driver-registrar
COPY build/csi-attacher /csi-attacher
