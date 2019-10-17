`lvmd`
======

`lvmd` is a gRPC service to manage LVM volumes.  It is composed of two services:
- VGService
    - Provide volume group information: list logical volume, list and watch free bytes
- LVService
    - Provide management of logical volumes: create, remove, resize

`lvmd` is intended to be run as a systemd service on the node OS.

Command-line options are:

| Option         | Type   | Default value            | Description                          |
| -------------- | ------ | ------------------------ | ------------------------------------ |
| `volume-group` | string | ""                       | target volume group name             |
| `listen`       | string | `/run/topolvm/lvmd.sock` | unix domain socket endpoint of gRPC  |
| `spare`        | uint64 | 10                       | Storage capacity in GiB to be spared |

Spare capacity
--------------

LVMd subtracts a certain amount from the free space of a volume group before
reporting the free space of the volume group.

The default spare capacity is 10 GiB.  This can be changed with `--spare` command-line flag.

API specification
-----------------

[See here.](./lvmd-protocol.md)
