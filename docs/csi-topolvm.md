csi-topolvm
===========

`csi-topolvm` is an Unified CSI driver for TopoLVM. It runs as CSI Controller service and CSI Node service.

## Sub commands

### `controller [--csi-socket-name=] [--namespace=]`

Run as CSI Controller service mode. It creates/watches/removes `LogicalVolume`.

### `node --node-name=NODENAME [--csi-socket-name=] [--lvmd-socket-name=]`

Run as CSI Node service mode.

Command-line flags
------------------

| Name               | Type   | Default                         | Description                                                        |
| ------------------ | ------ | ------------------------------- | ------------------------------------------------------------------ |
| `node-name`        | string | -                               | [node mode] The name of the node hosting csi-topolvm node service. |
| `csi-socket-name`  | string | `/run/topolvm/csi-topolvm.sock` | The socket name for CSI gRPC server.                               |
| `lvmd-socket-name` | string | `/run/topolvm/lvmd.sock`        | [node mode] The socket name for LVMD gRPC server.                  |
| `namespace`        | string | `topolvm-system`                | [controller mode] Namespace for LogicalVolume CRD.                 |
