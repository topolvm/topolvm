# Advanced Setup

TopoLVM can be installed by Helm Chart as described in [Getting Started](getting-started.md).
This document describes how to install TopoLVM with advanced configurations.

<!-- Created by VSCode Markdown All in One command: Create Table of Contents -->
- [StorageClass](#storageclass)
- [Pod Priority](#pod-priority)
- [LVMd](#lvmd)
  - [Run LVMd as a Dedicated Daemonset](#run-lvmd-as-a-dedicated-daemonset)
  - [Run LVMd as a Embed Function in topolvm-node](#run-lvmd-as-a-embed-function-in-topolvm-node)
  - [Run LVMd as a Systemd Service](#run-lvmd-as-a-systemd-service)
  - [Migrate LVMd, which is running as a systemd service, to DaemonSet](#migrate-lvmd-which-is-running-as-a-systemd-service-to-daemonset)
  - [Use different LVMd configurations on different nodes](#use-different-lvmd-configurations-on-different-nodes)
- [Certificates](#certificates)
- [Scheduling](#scheduling)
  - [Using Storage Capacity Tracking](#using-storage-capacity-tracking)
  - [Using topolvm-scheduler](#using-topolvm-scheduler)

## StorageClass

You can configure the StorageClass created by the Helm Chart by editing the Helm Chart values.

`fsType` specifies the filesystem type of the volume. Supported filesystems are `ext4`, `xfs` and `btrfs`(beta).

`volumeBindingMode` can be either `WaitForFirstConsumer` or `Immediate`.
`WaitForFirstConsumer` is recommended because TopoLVM cannot schedule pods
wisely if `volumeBindingMode` is `Immediate`.

`allowVolumeExpansion` enables expanding volumes.

`additionalParameters` defines additional parameters for the StorageClass.
You can use it to set `device-class` that the StorageClass will use.
The `device-class` is described in the [LVMd](lvmd.md) document.

`reclaimPolicy` can be either `Delete` or `Retain`.
If you delete a PVC whose corresponding PV has `Retain` reclaim policy, the corresponding `LogicalVolume` resource and the LVM logical volume are *NOT* deleted. If you delete this `LogicalVolume` resource after deleting the PVC, the related LVM logical volume is also deleted.

## Pod Priority

Pods using TopoLVM should always be prioritized over other normal pods.
This is because TopoLVM pods can only be scheduled to a single node where
its volumes exist whereas normal pods can be run on any node.

The Helm Chart create a PriorityClass by default.
You can configure its priority value by editing `priorityClass.value`.

The PriorityClass is not used by default.
To apply it to pods, you need to specify the PriorityClass name in `priorityClassName` field of the pod spec as follows.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: foo
spec:
  priorityClassName: topolvm
  ...
```

## LVMd

[LVMd](lvmd.md) is a component to manage LVM.
There are three options for managing LVMd: using dedicated DaemonSet, embedding LVMd in topolvm-node, or using systemd. Below are the details of each of these options.

In general, it is recommended to run LVMd as the first or the second option.
Since these modes uses `nsenter` to run `lvm` related commands as a host process, it is necessary to enable `hostPID` and privileged mode for the Pods.
This can not be achieved by some reason or on a environment (e.g. [kind](https://kind.sigs.k8s.io/)).
In this case, you can choose to run as a systemd service.

### Run LVMd as a Dedicated Daemonset

The Helm Chart runs a LVMd as a dedicated Daemonset by default.
If you want to configure LVMd, you can edit the Helm Chart values.

```yaml
lvmd:
  managed: true
  socketName: /run/topolvm/lvmd.sock
  deviceClasses:
    - name: ssd
      volume-group: myvg1 # Change this value to your VG name.
      default: true
      spare-gb: 10
```

> [!NOTE]
> If you are using a read-only filesystem, or `/etc/lvm` is mounted read-only, LVMd will likely fail to create volumes with status code 5.
> To avoid this, you need to set an extra environment variable.

```yaml
lvmd:
  env:
    - name: LVM_SYSTEM_DIR
      value: /tmp
```

### Run LVMd as a Embed Function in topolvm-node

This is in the very early stage, so be careful to use it.

In this mode, LMVd runs as a embed function in `topolvm-node` container.
Thanks to lower consumption of resources, it is also suitable for edge computing or the IoT.

To use the mode, you need to set the Helm Chart values as follows:

```yaml
lvmd:
  managed: false
node:
  lvmdEmbedded: true
```

### Run LVMd as a Systemd Service

Before setup, you need to get LMVd binary.
We provide pre-built binaries in the [releases page](https://github.com/topolvm/topolvm/releases) for x86 architecture.
If you use other architecture or want to build it from source code, you can build it by `mkdir build; go build -o build/lvmd ./pkg/lvmd`.

To setup LVMd as a systemd service:

1. Place [lvmd.yaml](../deploy/lvmd-config/lvmd.yaml) in `/etc/topolvm/lvmd.yaml`. If you want to specify the `device-class` settings to use multiple volume groups, edit the file. See [lvmd.md](lvmd.md) for details.
2. Install LVMd binary in `/opt/sbin/lvmd` and [`lvmd.service`](../deploy/systemd/lvmd.service) in `/etc/systemd/system`, then start the service.

### Migrate LVMd, which is running as a systemd service, to DaemonSet

This section describes how to switch to DaemonSet LVMd from LVMd running as a systemd service.

1. Install Helm Chart by configuring LVMd to act as a DaemonSet.
   **You need to set the temporal `socket-name` which is not the same as the value in LVMd running as a systemd service.**
   After the installation Helm Chart, DaemonSet LVMd and LVMd running as a systemd service exist at the same time using different sockets.

   ```yaml
   <snip>
   lvmd:
     managed: true
     socketName: /run/topolvm/lvmd.sock # Change this value to something like `/run/topolvm/lvmd-work.sock`.
     deviceClasses:
       - name: ssd
         volume-group: myvg1
         default: true
         spare-gb: 10
   <snip>
   ```

2. Change the options of topolvm-node to communicate with the DaemonSet LVMd instead of LVMd running as a systemd service.
   **You should set the temporal socket name which is not the same as in LVMd running as a systemd service.**

   ```yaml
   <snip>
   node:
     lvmdSocket: /run/lvmd/lvmd.sock # Change this value to to something like `/run/lvmd/lvmd-work.sock`.
   <snip>
   ```

3. Check if you can create Pod/PVC and can access to existing PV.

4. Stop and remove LVMd running as a systemd service.

5. Change the `socket-name` and `--lvmd-socket` options to the original one.
   To reflect the changes of ConfigMap, restart DamonSet LVMd manually.

   ```yaml
   <snip>
   lvmd:
    socketName: /run/topolvm/lvmd-work.sock # Change this value to something like `/run/topolvm/lvmd.sock`.
   <snip>
   node:
    lvmdSocket: /run/lvmd/lvmd.sock # Change this value to something like `/run/lvmd/lvmd.sock`.
   <snip>
   ```

### Use different LVMd configurations on different nodes

Depending on your setup, you might want to deploy different LVMd configurations on different nodes. You can do this by using [`lvmd.additionalConfigs` in `values.yaml` file](https://github.com/topolvm/topolvm/blob/3ddfec27480ca0381c6b0ec4b9e536afdff1aad6/charts/topolvm/values.yaml#L194-L203). Check the comments there for more details.

Please note that `lvmd.additionalConfigs` will only work as expected if you use the managed (i.e., `DaemonSet`) version of LVMd. This feature won't work if you use embedded or unmanaged version of LVMd.

See also [#555](https://github.com/topolvm/topolvm/issues/555) and [#973](https://github.com/topolvm/topolvm/issues/973).

## Certificates

TopoLVM uses webhooks and its requires TLS certificates.
The default method is using cert-manager described in [Getting Started](getting-started.md).

If you don't want to use cert-manager, you can use your own certificates as follows:

1. Prepare PEM encoded self-signed certificate and key files.  
   The certificate must be valid for hostname like `topolvm-controller.topolvm-system.svc`.
2. Base64-encode the CA cert (in its PEM format)
3. Create Secret in `topolvm-system` namespace as follows:

   ```bash
   kubectl -n topolvm-system create secret tls topolvm-mutatingwebhook \
       --cert=<CERTIFICATE FILE> --key=<KEY FILE>
   ```

4. Specify the certificate in the Helm Chart values.

   ```yaml
   <snip>
   webhook:
     caBundle: ... # Base64-encoded, PEM-encoded CA certificate that signs the server certificate
   <snip>
   ```

## Scheduling

It is necessary to configure `kube-scheduler` to schedule pods to appropriate nodes which have sufficient capacity to create volumes because TopoLVM provides node local volumes.

There are two options, using Storage Capacity Tracking feature or using `topolvm-scheduler`.

The former is the default option and it is easy to setup.
The latter is complicated, but it can prioritize nodes by the free disk capacity which can not be achieved by Storage Capacity Tracking.

### Using Storage Capacity Tracking

It is built-in feature of Kubernetes and the Helm Chart uses it by default, so you don't need to do anything.

You can see the limitations of using Storage Capacity Tracking from [here](https://kubernetes.io/docs/concepts/storage/storage-capacity/#scheduling).

### Using topolvm-scheduler

[topolvm-scheduler](topolvm-scheduler.md) is a [scheduler extender](https://github.com/kubernetes/design-proposals-archive/blob/main/scheduling/scheduler_extender.md) for `kube-scheduler`.
It must be deployed to where `kube-scheduler` can connect.

If your `kube-scheduler` can't connect to pods directly, you need to run `topolvm-scheduler` as a DaemonSet on the nodes running `kube-scheduler` to ensure that `kube-scheduler` can connect to `topolvm-scheduler` via a loopback network device.

Otherwise, you can run `topolvm-scheduler` as a Deployment and create a Service to connect to it from `kube-scheduler`.

To use `topolvm-scheduler`, you need to enable it in the Helm Chart values.

```yaml
scheduler:
  enabled: false
  # you can set the type to `daemonset` or `deployment`. The default is `daemonset`.
  type: daemonset
controller:
  storageCapacityTracking:
    enabled: false
webhook:
  podMutatingWebhook:
    enabled: true
```

`kube-scheduler` needs to be configured to use `topolvm-scheduler` extender.

To configure `kube-scheduler`, copy the [scheduler-config.yaml](../deploy/scheduler-config/scheduler-config.yaml) to the hosts where `kube-scheduler`s run.

If you are using `topolvm-scheduler` as a Deployment, you need to edit the `urlPrefix` in the file to specify the LoadBalancer address.

#### For New Clusters

If you are installing your cluster from scratch with `kubeadm`, you can use the following configuration:

```yaml
apiVersion: kubeadm.k8s.io/v1beta3
kind: ClusterConfiguration
metadata:
  name: config
kubernetesVersion: v1.33.0
scheduler:
  extraVolumes:
    - name: "config"
      hostPath: /path/to/scheduler-config # absolute path to the directory containing scheduler-config.yaml
      mountPath: /var/lib/scheduler
      readOnly: true
  extraArgs:
    config: /var/lib/scheduler/scheduler-config.yaml
```

#### For Existing Clusters

To configure `kube-scheduler` installed by `kubeadm`, you need to edit the `/etc/kubernetes/manifests/kube-scheduler.yaml` as follows:

1. Add a line to the `command` arguments array such as `- --config=/var/lib/scheduler/scheduler-config.yaml`. Note that this is the location of the file **after** it is mapped to the `kube-scheduler` container, not where it exists on the node local filesystem.
2. Add a volume mapping to the location of the configuration on your node:

   ```yaml
   spec.volumes:
     - hostPath:
         path: /path/to/scheduler-config # absolute path to the directory containing scheduler-config.yaml
         type: Directory
       name: topolvm-config
   ```

3. Add a `volumeMount` for the scheduler container:

   ```yaml
   spec.containers.volumeMounts:
     - mountPath: /var/lib/scheduler
       name: topolvm-config
       readOnly: true
   ```

#### OPTIONAL: Tune The Node Scoring

The node scoring for pod scheduling can be fine-tuned with the following two ways:

1. Adjust `divisor` parameter for `topolvm-scheduler`
2. Change the weight for the node scoring against the default by `kube-scheduler`

The first method is to tune calculation of the node scoring by `topolvm-scheduler` itself.
To adjust the parameter, you can set the Helm Chart value `scheduler.schedulerOptions`.
The parameter detail is described in [topolvm-scheduler](topolvm-scheduler.md).

The second method is to change the weight of the node score from `topolvm-scheduler`.
The weight can be passed to `kube-scheduler` via [scheduler-config.yaml](../deploy/scheduler-config/scheduler-config.yaml).
Almost all scoring algorithms in `kube-scheduler` are weighted as `"weight": 1`.
So if you want to give a priority to the scoring by `topolvm-scheduler`, you have to set the weight as a value larger than one like as follows:

```yaml
apiVersion: kubescheduler.config.k8s.io/v1
kind: KubeSchedulerConfiguration
...
extenders:
  - urlPrefix: "http://127.0.0.1:9251"
    filterVerb: "predicate"
    prioritizeVerb: "prioritize"
    nodeCacheCapable: false
    weight: 100 ## EDIT THIS FIELD ##
    managedResources:
      - name: "topolvm.io/capacity"
        ignoredByScheduler: true
```
