# End-to-end Tests of TopoLVM Using Kind

This directory contains codes for end-to-end tests of TopoLVM.
The tests run using [kind (Kubernetes IN Docker)][kind] to make an environment with multiple `lvmd` running as systemd service.
On the other hand, to test `lvmd` running as a Daemonset, [minikube][minikube] is used to make a test environment in the localhost without container or VM.

## Setup Environment

1. Prepare Ubuntu machine.
2. [Install Docker CE](https://docs.docker.com/install/linux/docker-ce/ubuntu/#install-using-the-repository).
3. Add yourself to `docker` group.  e.g. `sudo adduser $USER docker`

## How to Run Tests

### Run Tests Using Kind with LVMd as a Systemd Service

Start `lvmd` as a systemd service as follows:

```bash
make start-lvmd
```

Run the tests with the following command. Repeat it until you get satisfied.
When tests fail, use `kubectl` to inspect the Kubernetes cluster.

```bash
make test
```

> [!NOTE]
> If launching kind failed, please check [the known issues](https://kind.sigs.k8s.io/docs/user/known-issues).

You can also start a cluster without running tests by the following command.

```bash
make create-cluster
```

Then, you can test specific suite.

```bash
make common/test GINKGO_FLAGS="--focus hook"
```

You can cleanup test environment as follows:

```bash
# stop Kubernetes
make shutdown-kind

# stop lvmd
make stop-lvmd
```

### Run Tests Using Minikube with LVMd as a Daemonset

Before launching the test, install the following tools.
- [crictl](https://github.com/kubernetes-sigs/cri-tools)
- [cri-dockerd](https://github.com/Mirantis/cri-dockerd)

Make lvm and launch Kubernetes using minikube with the following commands:

```bash
make incluster-lvmd/create-vg
make incluster-lvmd/setup-minikube
make incluster-lvmd/launch-minikube
```

Run the tests with the following command.
You can inspect the Kubernetes cluster using `kubectl` command as well as kind.

```bash
make incluster-lvmd/test TEST_SCHEDULER_EXTENDER_TYPE=none TEST_LVMD_TYPE=<daemonset/embedded>
```

You can cleanup test environment as follows:

```bash
make incluster-lvmd/clean
```

[kind]: https://github.com/kubernetes-sigs/kind
[minikube]: https://github.com/kubernetes/minikube

### Run Tests Using Minikube on a VM

You can use the following script to run the tests using Minikube on [Multipass](https://canonical.com/multipass):

```bash
#!/usr/bin/env bash

set -euxo pipefail

GOVERSION=1.24.3 # Choose a supported version
KUBERNETES_VERSION=1.34.3 # Choose a supported version

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
```
