topolvm-scheduler
=================

`topolvm-scheduler` is a Kubernetes [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for TopoLVM.

It filters and prioritizes Nodes based on the amount of free space in their volume groups.

Scheduler policy
----------------

`topolvm-scheduler` need to be configured in [scheduler policy](https://pkg.go.dev/k8s.io/kubernetes@v1.17.3/pkg/scheduler/apis/config?tab=doc#Policy) as follows:

```json
{
    ...
    "extenders": [{
        "urlPrefix": "http://...",
        "filterVerb": "predicate",
        "prioritizeVerb": "prioritize",
        "managedResources":
        [{
          "name": "topolvm.cybozu.com/capacity",
          "ignoredByScheduler": true
        }],
        "nodeCacheCapable": false
    }]
}
```

As shown, only pods that request `topolvm.cybozu.com/capacity` resource are
managed by `topolvm-scheduler`.

Verbs
-----

The extender provides two verbs:

- `predicate` to filter nodes
- `prioritize` to score nodes

### `predicate`

This verb filters out nodes whose volume groups have not enough free space.

Volume group capacity is identified from the value of `capacity.topolvm.cybozu.com/<device-class>`
annotation.

### `prioritize`

This verb scores nodes.  The score of a node is calculated by this formula:

    min(10, max(0, log2(capacity >> 30 / divisor)))

`divisor` can be changed with a configuration file.

Command-line flags
------------------

| Name      | Type    | Default                       | Description            |
| --------- | ------- | ----------------------------: | ---------------------- |
| `listen`  | string  | `:8000`                       | HTTP listening address |
| `config`  | string  | `/etc/topolvm/scheduler.yaml` | Config file path       |

Config file format
------------------

The divisor parameter can be specified in YAML file:

```yaml
default-divisor: 10
divisors:
  ssd: 5
  hdd: 10
```

| Name              | Type                 | Default | Description                                       |
| ----------------- | -------------------- | ------- | ------------------------------------------------- |
| `default-divisor` | float64              | `1`     | A default value of the variable for node scoring. |
| `divisors`        | `map[string]float64` | `{}`    | A variable for node scoring per device-class.     |

