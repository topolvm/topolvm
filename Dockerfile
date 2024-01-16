# Build topolvm
FROM --platform=$BUILDPLATFORM golang:1.20-bullseye AS build-topolvm

# Get argument
ARG TOPOLVM_VERSION
ARG TARGETARCH

COPY . /workdir
WORKDIR /workdir

RUN touch lvmd/proto/*.go
RUN make build-topolvm TOPOLVM_VERSION=${TOPOLVM_VERSION} GOARCH=${TARGETARCH}

# TopoLVM container
FROM --platform=$TARGETPLATFORM ubuntu:18.04 as topolvm

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
    btrfs-progs \
    file \
    xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build-topolvm /workdir/build/hypertopolvm /hypertopolvm

RUN ln -s hypertopolvm /lvmd \
    && ln -s hypertopolvm /topolvm-scheduler \
    && ln -s hypertopolvm /topolvm-node \
    && ln -s hypertopolvm /topolvm-controller

COPY --from=build-topolvm /workdir/LICENSE /LICENSE

ENTRYPOINT ["/hypertopolvm"]

# Build sidecars
FROM --platform=$BUILDPLATFORM build-topolvm as build-sidecars

# Get argument
ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive
RUN  apt-get update \
    && apt-get -y install --no-install-recommends \
    patch

RUN make csi-sidecars GOARCH=${TARGETARCH}


# TopoLVM container with sidecar
FROM --platform=$TARGETPLATFORM topolvm as topolvm-with-sidecar

COPY --from=gcr.io/k8s-staging-sig-storage/csi-provisioner:v3.6.0 /csi-provisioner /csi-provisioner
COPY --from=gcr.io/k8s-staging-sig-storage/csi-node-driver-registrar:v2.8.0 /csi-node-driver-registrar /csi-node-driver-registrar
COPY --from=gcr.io/k8s-staging-sig-storage/csi-resizer:v1.8.0 /csi-resizer /csi-resizer
COPY --from=gcr.io/k8s-staging-sig-storage/csi-snapshotter:v6.3.0 /csi-snapshotter /csi-snapshotter
COPY --from=gcr.io/k8s-staging-sig-storage/livenessprobe:v2.10.0 /livenessprobe /livenessprobe
