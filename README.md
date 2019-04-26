[![GitHub release](https://img.shields.io/github/release/cybozu-go/topolvm.svg?maxAge=60)][releases]
[![CircleCI](https://circleci.com/gh/cybozu-go/topolvm.svg?style=svg)](https://circleci.com/gh/cybozu-go/topolvm)
[![GoDoc](https://godoc.org/github.com/cybozu-go/topolvm?status.svg)][godoc]
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/topolvm)](https://goreportcard.com/report/github.com/cybozu-go/topolvm)
[![Docker Repository on Quay](https://quay.io/repository/cybozu/topolvm/status "Docker Repository on Quay")](https://quay.io/repository/cybozu/topolvm)

topolvm
====

**topolvm** is a [CSI][] plugin using LVM for Kubernetes.

**Project Status**: Initial Development

Runtime Dependencies
------------

* LVM command line tools
* Supported Kubernetes versions
  - 1.14.x
* [etcd][]: coil requires etcd v3 API, does not support v2.
* Routing Software
  - [Bird][]
  - Other software that can import a kernel routing table and advertise them via BGP, RIP, OSPF.

Features
--------

* IP address management (IPAM)

    Coil dynamically allocates IP addresses to Pods.

    Coil has a mechanism called _address pool_ so that the administrator
    can control to assign special/global IP addresses only to some Pods.

* Address pools

    An address pool is a pool of allocatable IP addresses.  In addition to
    the _default_ pool, users can define arbitrary address pools.

    Pods in a specific Kubernetes namespace take their IP addresses from
    the address pool whose name matches the namespace if such a pool exists.

    This way, only users who can create Pods in the namespace can use
    special/global IP addresses.

* Address block

    Coil divides a large subnet into small fixed size blocks (e.g. `/27`),
    and assign them to nodes.  Nodes then allocate IP addresses to Pods
    from the assigned blocks.

* Intra-node Pod routing

    Coil programs _intra_-node routing for Pods.

    As to inter-node routing, coil publishes address blocks assigned to
    the node to an unused kernel routing table as described next.

* Publish address blocks to implement inter-node Pod routing

    Coil registers address blocks assigned to a node with an unused
    kernel routing table.  The default table ID is `119`.

    The routing table can be referenced by other routing programs
    such as [BIRD][] to implement inter-node routing.

    An example BIRD configuration file that advertises address blocks
    via BGP is available at [mtest/bird.conf](mtest/bird.conf).

Programs
--------

This repository contains these programs:

* `coil`: [CNI][] plugin.
* `coilctl`: CLI tool to configure coil IPAM.
* `coild`: A background service to manage IP address.
* `coil-controller`: watches kubernetes resources for coil.
* `coil-installer`: installs `coil` and CNI configuration file.
* `hypercoil`: all-in-one binary just like `hyperkube`.

`coil` should be installed in `/opt/cni/bin` directory.

`coilctl` directly communicates with etcd.
Therefore it can be installed any host that can connect to etcd cluster.

`coild` and `coil-installer` should run as [`DaemonSet`](https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/).

`coil-controller` should be deployed as [`Deployment`](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/).

Documentation
-------------

[docs](docs/) directory contains documents about designs and specifications.

License
-------

MIT

[releases]: https://github.com/cybozu-go/coil/releases
[godoc]: https://godoc.org/github.com/cybozu-go/coil
[CNI]: https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/network-plugins/
[BIRD]: https://bird.network.cz/
[NetworkPolicy]: https://kubernetes.io/docs/concepts/services-networking/network-policies/
[etcd]: https://github.com/etcd-io/etcd
[CoreOS Container Linux]: https://coreos.com/os/docs/latest/
