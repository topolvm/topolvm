#!/usr/bin/env bash

set -euxo pipefail

GOVERSION=1.24.3 # Choose a supported version
KUBERNETES_VERSION=1.32.2 # Choose a supported version

multipass delete -p testvm || true
multipass launch lts --name testvm --memory 8G --disk 20G --cpus 4

multipass exec testvm -- sudo apt update
multipass exec testvm -- sudo apt upgrade -y
multipass exec testvm -- sudo apt install -y make

multipass exec testvm -- curl -fsSL https://get.docker.com -o get-docker.sh
multipass exec testvm -- sudo sh ./get-docker.sh
multipass exec testvm -- sudo groupadd docker || true
multipass exec testvm -- sudo usermod -aG docker ubuntu

multipass exec testvm -- wget https://go.dev/dl/go${GOVERSION}.linux-amd64.tar.gz
multipass exec testvm -- sudo rm -rf /usr/local/go
multipass exec testvm -- sudo tar -C /usr/local -xzf go${GOVERSION}.linux-amd64.tar.gz
multipass exec testvm -- bash -c 'echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.bashrc'

multipass exec testvm -- git clone https://github.com/topolvm/topolvm.git
multipass exec testvm -- bash -c 'cd topolvm && git checkout main' # Choose your branch

multipass exec testvm -- bash -c "\
  export PATH=\$PATH:/usr/local/go/bin; \
  env \
    KUBERNETES_VERSION=${KUBERNETES_VERSION} \
    TEST_LVMD_TYPE=daemonset \
    TEST_SCHEDULER_EXTENDER_TYPE=none \
  make -C topolvm/test/e2e \
    incluster-lvmd/create-vg incluster-lvmd/setup-minikube incluster-lvmd/launch-minikube incluster-lvmd/test"
