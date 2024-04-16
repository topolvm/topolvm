# topolvm-scheduler

`topolvm-scheduler` is a Kubernetes [scheduler extender](https://github.com/kubernetes/design-proposals-archive/blob/main/scheduling/scheduler_extender.md) for TopoLVM.

It filters and prioritizes Nodes based on the amount of free space in their volume groups.

## Scheduler Policy

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
          "name": "topolvm.io/capacity",
          "ignoredByScheduler": true
        }],
        "nodeCacheCapable": false
    }]
}
```

As shown above, only pods that request `topolvm.io/capacity` resource are
managed by `topolvm-scheduler`.

## Verbs

The extender provides two verbs:

- `predicate` to filter nodes
- `prioritize` to score nodes

### `predicate`

This verb filters out nodes whose volume groups have not enough free space.

Volume group capacity is identified from the value of `capacity.topolvm.io/<device-class>`
annotation.

### `prioritize`

This verb scores nodes.  The score of a node is calculated by this formula:

$$ \mathrm{min} \left( 10, \ \mathrm{max} \left( 0, \ \log_{2}{ \left( \mathrm{capacity} \gg 30 / \mathrm{divisor} \right) } \right) \right) $$

For example, the default of `divisor` is `1`, then if a node has the free disk capacity more than `1024GiB`, `topolvm-scheduler` scores the node as `10`. `divisor` should be adjusted to suit each environment.

`divisor` can be given through the configuration file.

## Command-line Flags

| Name     | Type   | Default | Description      |
| -------- | ------ | ------- | ---------------- |
| `config` | string | ``      | Config file path |

## Config File Format

The divisor parameter can be specified in YAML file:

```yaml
default-divisor: 10
divisors:
  ssd: 5
  hdd: 10
```

| Name              | Type                 | Default | Description                                       |
| ----------------- | -------------------- | ------- | ------------------------------------------------- |
| `listen`          | string               | `:8000` | HTTP listening address                            |
| `default-divisor` | float64              | `1`     | A default value of the variable for node scoring. |
| `divisors`        | `map[string]float64` | `{}`    | A variable for node scoring per device-class.     |
