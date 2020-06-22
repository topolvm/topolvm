`lvmd`
======

`lvmd` is a gRPC service to manage LVM volumes.  It is composed of two services:
- VGService
    - Provide volume group information: list logical volume, list and watch free bytes
- LVService
    - Provide management of logical volumes: create, remove, resize

`lvmd` is intended to be run as a systemd service on the node OS.

Command-line options are:

| Option         | Type   | Default value            | Description                                |
| -------------- | ------ | ------------------------ | ------------------------------------------ |
| `config`       | string | `/etc/topolvm/lvmd.yaml` | Config file path for device-class settings |

The device-class settings can be specified in YAML file:

```yaml
socket-name: /run/topolvm/lvmd.sock
device-classes:
  - name: ssd
    volume-group: ssd-vg
    spare-gb: 10
    default: true
  - name: hdd
    volume-group: hdd-vg
    spare-gb: 10
```

| Name             | Type                     | Default                  | Description                         |
| ---------------- | ------------------------ | ------------------------ | ----------------------------------- |
| `socket-name`    | string                   | `/run/topolvm/lvmd.sock` | Unix domain socket endpoint of gRPC |
| `device-classes` | `map[string]DeviceClass` | -                        | The device-class settings           |

The device-class settings can be specified in the following fields:

| Name           | Type   | Default | Description                                                    |
| -------------- | ------ | ------- | -------------------------------------------------------------- |
| `name`         | string | -       | The name of a device-class.                                    |
| `volume-group` | string | -       | The group where this device-class creates the logical volumes. |
| `spare-gb`     | uint64 | `10`    | Storage capacity in GiB to be spared.                          |
| `default`      | bool   | `false` | A flag to indicate that this device-class is used by default.  |


Spare capacity
--------------

LVMd subtracts a certain amount from the free space of a volume group before
reporting the free space of the volume group.

The default spare capacity is 10 GiB.  This can be changed with `--spare` command-line flag.

API specification
-----------------

[See here.](./lvmd-protocol.md)
