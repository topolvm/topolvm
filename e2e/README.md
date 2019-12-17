End-to-end tests of TopoLVM using kind
=====================================

This directory contains codes for end-to-end tests of TopoLVM.
Since the tests make use of [kind (Kubernetes IN Docker)][kind], this is called "e2e" test.

Setup environment
-----------------

1. Prepare Ubuntu machine.
2. [Install Docker CE](https://docs.docker.com/install/linux/docker-ce/ubuntu/#install-using-the-repository).
3. Add yourself to `docker` group.  e.g. `sudo adduser $USER docker`
4. Run `make setup`.

How to run tests
----------------

Start `lvmd` as a systemd service as follows:

```console
make start-lvmd
```

Finally, run `make test`.  Repeat it until you get satisfied.

When tests fail, use `kubectl` to inspect the Kubernetes cluster.

Cleanup
-------

To stop Kubernetes, run `make shutdown-kind`.

To stop `lvmd`, run `make stop-lvmd`.

[kind]: https://github.com/kubernetes-sigs/kind
