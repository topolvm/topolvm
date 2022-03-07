End-to-end tests of TopoLVM using kind
=====================================

This directory contains codes for end-to-end tests of TopoLVM.
The tests run using [kind (Kubernetes IN Docker)][kind] to make an environment with multiple `lvmd` running as systemd service.
On the other hand, to test `lvmd` running as a daemonset, [minikube][minikube] is used to make a test environment in the localhost without container or VM.

Setup environment
-----------------

1. Prepare Ubuntu machine.
2. [Install Docker CE](https://docs.docker.com/install/linux/docker-ce/ubuntu/#install-using-the-repository).
3. Add yourself to `docker` group.  e.g. `sudo adduser $USER docker`
4. Run `make setup`.

How to run tests
----------------

### Run tests using kind with lvmd as a systemd service

Start `lvmd` as a systemd service as follows:

```console
make start-lvmd
```

Run the tests with the following command. Repeat it until you get satisfied.
When tests fail, use `kubectl` to inspect the Kubernetes cluster.

```console
make test
```

You can also start a cluster without running tests by the following command.

```console
make create-cluster
```

You can cleanup test environment as follows:

```
# stop Kubernetes
make shutdown-kind

# stop lvmd
make stop-lvmd
```

### Run tests using minikube with lvmd as a daemonset

Make lvm and launch Kubernetes using minikube with the following commands:

```console
make daemonset-lvmd/create-vg
make daemonset-lvmd/setup-minikube
make daemonset-lvmd/update-minikube-setting
```

Run the tests with the following command.
You can inspect the Kubernetes cluster using `kubectl` command as well as kind.

```console
make daemonset-lvmd/test
```

You can cleanup test environment as follows:

```console
make daemonset-lvmd/clean
```

[kind]: https://github.com/kubernetes-sigs/kind
[minikube]: https://github.com/kubernetes/minikube
