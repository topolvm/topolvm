# Build topolvm
FROM --platform=$BUILDPLATFORM golang:1.22-bullseye AS build-topolvm

# Get argument
ARG TOPOLVM_VERSION
ARG TARGETARCH

COPY . /workdir
WORKDIR /workdir

RUN touch pkg/lvmd/proto/*.go
RUN make build-topolvm TOPOLVM_VERSION=${TOPOLVM_VERSION} GOARCH=${TARGETARCH}

# TopoLVM container
FROM ubuntu:22.04 AS topolvm

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
FROM --platform=$BUILDPLATFORM build-topolvm AS build-sidecars

# Get argument
ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive
RUN  apt-get update \
    && apt-get -y install --no-install-recommends \
        patch

RUN make csi-sidecars GOARCH=${TARGETARCH}

# TopoLVM container with sidecar
FROM topolvm AS topolvm-with-sidecar

COPY --from=build-sidecars /workdir/build/csi-provisioner /csi-provisioner
COPY --from=build-sidecars /workdir/build/csi-node-driver-registrar /csi-node-driver-registrar
COPY --from=build-sidecars /workdir/build/csi-resizer /csi-resizer
COPY --from=build-sidecars /workdir/build/csi-snapshotter /csi-snapshotter
COPY --from=build-sidecars /workdir/build/livenessprobe /livenessprobe
