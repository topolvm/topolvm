# Build Container
FROM golang:1.17-buster AS build-env

# Get argment
ARG TOPOLVM_VERSION

COPY . /workdir
WORKDIR /workdir

RUN touch csi/*.go lvmd/proto/*.go \
    && make build-topolvm TOPOLVM_VERSION=${TOPOLVM_VERSION}

# TopoLVM container
FROM ubuntu:18.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        btrfs-progs \
        file \
        xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build-env /workdir/build/hypertopolvm /hypertopolvm

RUN ln -s hypertopolvm /lvmd \
    && ln -s hypertopolvm /topolvm-scheduler \
    && ln -s hypertopolvm /topolvm-node \
    && ln -s hypertopolvm /topolvm-controller \
    && ln -s hypertopolvm /topolvm-migrator-controller \
    && ln -s hypertopolvm /topolvm-migrator-node

COPY --from=build-env /workdir/LICENSE /LICENSE

ENTRYPOINT ["/hypertopolvm"]
