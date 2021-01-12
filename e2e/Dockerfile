FROM quay.io/cybozu/ubuntu-debug:18.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        file \
        xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY . /
