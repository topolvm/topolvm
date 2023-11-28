`lvmd`
======

`lvmd` is a gRPC service to manage LVM volumes.  It is composed of two services:
- VGService
    - Provide volume group information: list logical volume, list and watch free bytes
- LVService
    - Provide management of logical volumes: create, remove, resize

`lvmd` is intended to be run as a systemd service on the node OS.

Command-line options are:

| Option      | Type   | Default value            | Description                                |
| ----------- | ------ | ------------------------ | ------------------------------------------ |
| `config`    | string | `/etc/topolvm/lvmd.yaml` | Config file path for device-class settings |
| `container` | -      | not set                  | Set if lvmd runs in the container          |

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
  - name: striped
    volume-group: multi-pv-vg
    spare-gb: 10
    stripe: 2
    stripe-size: "64"
  - name: raid
    volume-group: raid-vg
    lvcreate-options:
      - --type=raid1
```

| Name             | Type                     | Default                  | Description                         |
| ---------------- | ------------------------ | ------------------------ | ----------------------------------- |
| `socket-name`    | string                   | `/run/topolvm/lvmd.sock` | Unix domain socket endpoint of gRPC |
| `device-classes` | `map[string]DeviceClass` | -                        | The device-class settings           |

The device-class settings can be specified in the following fields:

| Name               | Type     | Default | Description                                                                        |
| ------------------ | -------- | ------- | ---------------------------------------------------------------------------------- |
| `name`             | string   | -       | The name of a device-class.                                                        |
| `volume-group`     | string   | -       | The group where this device-class creates the logical volumes.                     |
| `spare-gb`         | uint64   | `10`    | Storage capacity in GiB to be spared.                                              |
| `default`          | bool     | `false` | A flag to indicate that this device-class is used by default.                      |
| `stripe`           | uint     | -       | The number of stripes in the logical volume.                                       |
| `stripe-size`      | string   | -       | The amount of data that is written to one device before moving to the next device. |
| `lvcreate-options` | []string | -       | Extra arguments to pass to `lvcreate`, e.g. `["--type=raid1"]`.                    |

Note that striping can be configured both using the dedicated options (`stripe` and `stripe-size`) and `lvcreate-options`.
Either one can be used but not together since this would lead to duplicate arguments to `lvcreate`.
This means that you should never set `lvcreate-options: ["--stripes=n"]` and `stripe: n` at the same time.
It is fine to use both as long as `lvcreate-options` are not used for striping however:
```
stripe: 2
lvcreate-options: ["--mirrors=1"]
```

After updating the configuration file, you need to restart lvmd. If you are running lvmd as a DaemonSet, restart the Pods after updating the ConfigMap. If you want to restart automatically, consider using [Reloader](https://github.com/stakater/Reloader) or similar.

Spare capacity
--------------

LVMd subtracts a certain amount from the free space of a volume group before
reporting the free space of the volume group.

The default spare capacity is 10 GiB.  This can be changed with `--spare` command-line flag.

API specification
-----------------

[See here.](./lvmd-protocol.md)
