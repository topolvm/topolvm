`lvmd`
======

`lvmd` is a gRPC service to manage LVM volumes.  It is composed with two services:
- VGService
    - Provide volume group information: list logical volume, list and watch free bytes
- LVService
    - Provide management of logical volumes: create, remove, resize

`lvmd` is intended to be run as a systemd service on the node OS.

Command-line options are:

| Option        | Default value            | Description                         |
| ------------- | ------------------------ | ----------------------------------- |
| `volumegroup` | ""                       | target volume group name            |
| `listen`      | `/run/topolvm/lvmd.sock` | unix domain socket endpoint of gRPC |

API specification
-----------------

[See here.](./lvmd-protocol.md)
