lvmetrics
=========

`lvmetrics` is a Kubernetes application to update `Node` by annotating
with LVM volume group metrics.

The metrics is obtained from [`lvmd`](./lvmd.md) running on the same node.
`lvmetrics` communicates with `lvmd` via UNIX domain socket.

Command-line flags
-----------------

| Name       | Default                  | Description                   |
| ---------- | ------------------------ | ----------------------------- |
| `nodename` |                          | `Node` resource name.         |
| `socket`   | `/run/topolvm/lvmd.sock` | UNIX domain socket of `lvmd`. |

Environment variables
--------------------

- `NODE_NAME`: `Node` resource name.

If both `NODE_NAME` and `nodename` flags are given, `nodename` flag value is used.

Annotations
-----------

`lvmetrics` adds following annotations to `Node` resource.
If RBAC is enabled, the service account running `lvmetrics` must be granted to edit `Node`s.

- `topolvm.cybozu.com/capacity`: The amount of free capacity of LVM volume group in bytes.
