## LVMd

LVMd is a gRPC service to manage LVM volumes.  It is composed of two services:
- VGService
    - Provide volume group information: list logical volume, list and watch free bytes
- LVService
    - Provide management of logical volumes: create, remove, resize

## Command-line Flags

| Option      | Type   | Default value            | Description                                |
| ----------- | ------ | ------------------------ | ------------------------------------------ |
| `config`    | string | `/etc/topolvm/lvmd.yaml` | Config file path for device-class settings |
| `container` | -      | not set                  | Set if LVMd runs in the container          |

## Config File Format

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

> [!NOTE]
> Striping can be configured both using the dedicated options (`stripe` and `stripe-size`) and `lvcreate-options`. Either one can be used but not together since this would lead to duplicate arguments to `lvcreate`. This means that you should never set `lvcreate-options: ["--stripes=n"]` and `stripe: n` at the same time. It is fine to use both as long as `lvcreate-options` are not used for striping:
> ```
> stripe: 2
> lvcreate-options: ["--mirrors=1"]
> ```

> [!NOTE]
> After changing the configuration file, you need to restart LVMd to reflect this change. If LVMd is deployed as a DaemonSet, pod restart is needed after changing the corresponding ConfigMap. If you want to restart LVMd automatically after changing configuration, please use 3rd party tools like [Reloader](https://github.com/stakater/Reloader).

## Spare Capacity

LVMd subtracts a certain amount from the free space of a volume group before
reporting the free space of the volume group.

The default spare capacity is 10 GiB.  This can be changed with `--spare` command-line flag.

## API Specification

[See here.](./lvmd-protocol.md)
