# Build topolvm
FROM --platform=$BUILDPLATFORM golang:1.20.13-alpine3.19 AS build-topolvm

# Get argument
ARG TOPOLVM_VERSION
ARG TARGETARCH

RUN apk add --no-cache \
    make bash

COPY . /workdir
WORKDIR /workdir

RUN touch lvmd/proto/*.go
RUN make build-topolvm TOPOLVM_VERSION=${TOPOLVM_VERSION} GOARCH=${TARGETARCH}

# Build sidecars
FROM --platform=$BUILDPLATFORM build-topolvm as build-sidecars

# Get argument
ARG TARGETARCH

RUN apk add --no-cache \
    make patch curl

RUN make csi-sidecars GOARCH=${TARGETARCH}

# TopoLVM container base without sidecars
FROM --platform=$TARGETPLATFORM alpine:3.19 as topolvm

COPY --from=build-topolvm /workdir/LICENSE /LICENSE

RUN apk add --no-cache \
    nvme-cli \
    lvm2 \
    jq \
    bash \
    btrfs-progs \
    xfsprogs \
    e2fsprogs \
    file 

COPY --from=build-topolvm /workdir/build/hypertopolvm /hypertopolvm

ENTRYPOINT ["/hypertopolvm"]

RUN ln -s hypertopolvm /lvmd \
    && ln -s hypertopolvm /topolvm-scheduler \
    && ln -s hypertopolvm /topolvm-node \
    && ln -s hypertopolvm /topolvm-controller


# TopoLVM container with sidecar
FROM --platform=$TARGETPLATFORM topolvm as topolvm-with-sidecar

COPY --from=build-sidecars /workdir/build/csi-provisioner /csi-provisioner
COPY --from=build-sidecars /workdir/build/csi-node-driver-registrar /csi-node-driver-registrar
COPY --from=build-sidecars /workdir/build/csi-resizer /csi-resizer
COPY --from=build-sidecars /workdir/build/csi-snapshotter /csi-snapshotter
COPY --from=build-sidecars /workdir/build/livenessprobe /livenessprobe
