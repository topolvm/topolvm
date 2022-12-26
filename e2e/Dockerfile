FROM ubuntu:18.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        curl \
        btrfs-progs \
        file \
        xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY tmpbin/* /

RUN ln -sf hypertopolvm lvmd && \
	ln -sf hypertopolvm topolvm-scheduler && \
	ln -sf hypertopolvm topolvm-node && \
	ln -sf hypertopolvm topolvm-controller
