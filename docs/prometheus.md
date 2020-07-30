Monitoring with Prometheus
==========================

This document describes how to monitor TopoLVM and its volume metrics by [Prometheus](https://prometheus.io/).

## TopoLVM components

The following programs export their metrics under `/metrics` REST API endpoint.

- `topolvm-controller`
- `topolvm-node`
- `topolvm-scheduler`

In addition to the standard metrics of Go programs, `topolvm-node` provides available bytes of each volume group.
See [topolvm-node.md](https://github.com/topolvm/topolvm/blob/master/docs/topolvm-node.md#prometheus-metrics) for details.

An example scrape config looks like:

```yaml
scrape_configs:
  - job_name: "topolvm"
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ["topolvm-system"]
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        action: keep
        regex: node
      - source_labels: [__address__, __meta_kubernetes_pod_annotation_prometheus_io_port]
        action: replace
        regex: ([^:]+)(?::\d+)?;(\d+)
        replacement: ${1}:${2}
        target_label: __address__
      - source_labels: [__address__]
        action: replace
        regex: ([^:]+)(?::\d+)?
        replacement: ${1}
        target_label: instance
```

## PV filesystem usage

For TopoLVM filesystem volumes, their usage metrics are available via `kubelet`.
See [this StackOverflow article](https://stackoverflow.com/a/47117776/1493661) for details.

Following is an example scrape config to collect `kubelet` metrics.

```yaml
scrape_configs:
  - job_name: 'kubernetes-nodes'
    scheme: https
    tls_config:
      ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
    kubernetes_sd_configs:
      - role: node
    relabel_configs:
      - target_label: __address__
        replacement: kubernetes.default.svc:443
      - source_labels: [__meta_kubernetes_node_name]
        regex: (.+)
        target_label: __metrics_path__
        replacement: /api/v1/nodes/${1}/proxy/metrics
      - source_labels: [__name__]
        action: drop
        regex: kubelet_runtime_operations_duration_seconds.*
```
