lvmetrics
=========

`lvmetrics` is a sidecar container of CSI node pod to update `Node` resources.

Annotations
-----------

`lvmetrics` adds `topolvm.cybozu.com/capacity` annotation.
The value is the free storage space size in bytes.

`lvmetrics` obtains the free storage space size by watching [`lvmd`](./lvmd.md).

Finalizer
---------

`lvmetrics` adds `topolvm.cybozu.com/node` finalizer to `Node`.
The finalizer will be processed by [`topolvm-controller`](./topolvm-controller.md)

Command-line flags
------------------

| Name           | Default                  | Description                        |
| ----------     | ------------------------ | -----------------------------      |
| `nodename`     |                          | `Node` resource name.              |
| `socket`       | `/run/topolvm/lvmd.sock` | UNIX domain socket of `lvmd`.      |
| `metrics-addr` | `:8080`                  | The listen address of metrics API. |

Environment variables
---------------------

- `NODE_NAME`: `Node` resource name.

If both `NODE_NAME` and `nodename` flag are given, `nodename` flag is preceded.


Prometheus metrics
------------------

`lvmetrics` exposes Prometheus metrics at `/metrics` endpoint.

| Name                           | Type                     | Description                                                                                         |
| ----------                     | ------------------------ | -----------------------------                                                                       |
| `topolvm_volumegroup_available_bytes` | Gauge                    | available bytes of LVM VG mangaged by lvmd. This value is same with node annotation by lvmetrics |
