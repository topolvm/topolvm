FROM quay.io/cybozu/ubuntu-debug:18.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        btrfs-progs \
        file \
        xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY tmpbin/* /
COPY build/hypertopolvm /hypertopolvm

RUN ln -sf hypertopolvm lvmd && \
	ln -sf hypertopolvm topolvm-scheduler && \
	ln -sf hypertopolvm topolvm-node && \
	ln -sf hypertopolvm topolvm-controller
