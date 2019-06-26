FROM quay.io/cybozu/ubuntu-debug:18.04

# csi-topolvm node requires file command
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        file \
        btrfs-progs \
        xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY . /
