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

You can also start a cluster without running tests by the following command.

```bash
make create-cluster
```

Then, you can test specific suite.

```bash
make prepare-test
make run-test GINKGO_FLAGS="--focus hook"
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
make incluster-lvmd/test
```

By default, this will run lvmd as a DaemonSet separate from `topolvm-node`.
You can also configure the test to use an embedded version of lvmd inside `topolvm-node`.

```bash
make incluster-lvmd/test HELM_VALUES_FILE_LVMD="manifests/values/embedded-lvmd-storage-capacity.yaml"
`````

You can cleanup test environment as follows:

```bash
make incluster-lvmd/clean
```

[kind]: https://github.com/kubernetes-sigs/kind
[minikube]: https://github.com/kubernetes/minikube
