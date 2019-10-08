# TopoLVM container
FROM quay.io/cybozu/ubuntu:18.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        file \
        btrfs-progs \
        xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY build/hypertopolvm /hypertopolvm
RUN ln -s hypertopolvm /lvmd \
    && ln -s hypertopolvm /topolvm-scheduler \
    && ln -s hypertopolvm /topolvm-node \
    && ln -s hypertopolvm /topolvm-controller

# CSI sidecar
COPY build/csi-provisioner /csi-provisioner
COPY build/csi-node-driver-registrar /csi-node-driver-registrar
COPY build/csi-attacher /csi-attacher
COPY build/livenessprobe /livenessprobe
COPY LICENSE /LICENSE

ENTRYPOINT ["/hypertopolvm"]
