# Build topolvm
FROM --platform=$BUILDPLATFORM golang:1.20-bullseye AS build-topolvm

# Get argument
ARG TOPOLVM_VERSION
ARG TARGETARCH

COPY . /workdir
WORKDIR /workdir

RUN touch internal/lvmd/proto/*.go
RUN make build-topolvm TOPOLVM_VERSION=${TOPOLVM_VERSION} GOARCH=${TARGETARCH}

# TopoLVM container
FROM --platform=$TARGETPLATFORM ubuntu:22.04 as topolvm

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

COPY --from=build-sidecars /workdir/build/csi-provisioner /csi-provisioner
COPY --from=build-sidecars /workdir/build/csi-node-driver-registrar /csi-node-driver-registrar
COPY --from=build-sidecars /workdir/build/csi-resizer /csi-resizer
COPY --from=build-sidecars /workdir/build/csi-snapshotter /csi-snapshotter
COPY --from=build-sidecars /workdir/build/livenessprobe /livenessprobe
