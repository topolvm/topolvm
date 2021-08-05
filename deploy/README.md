# Deploying TopoLVM

Each of these steps are shown in depth in the following sections:

1. Deploy [lvmd][] as a `systemd` service or as a daemonset on a worker node with LVM installed.
1. Prepare [cert-manager][] for [topolvm-controller][]. You may supplement an existing instance.
1. Determine how [topolvm-scheduler][] to be run:
    - If you run with a managed control plane (such as GKE, AKS, etc), `topolvm-scheduler` should be deployed as Deployment and Service
    - `topolvm-scheduler` should otherwise be deployed as DaemonSet in unmanaged (i.e. bare metal) deployments
    - Enable [Storage Capacity Tracking](https://kubernetes.io/docs/concepts/storage/storage-capacity/) mode instead of using `topolvm-scheduler`
1. Install Helm chart
1. Configure `kube-scheduler` to use `topolvm-scheduler`.

## lvmd

[lvmd][] is a gRPC service to manage an LVM volume group.  The pre-built binary can be downloaded from [releases page](https://github.com/topolvm/topolvm/releases).
It can be built from source code by `mkdir build; go build -o build/lvmd ./pkg/lvmd`.

`lvmd` can setup as a daemon or a Kubernetes Daemonset.

### Setup `lvmd` as daemon

1. Prepare LVM volume groups.  A non-empty volume group can be used because LV names wouldn't conflict.
2. Edit [lvmd.yaml](./lvmd-config/lvmd.yaml) if you want to specify the device-class settings to use multiple volume groups. See [lvmd.md](../docs/lvmd.md) for details.

    ```yaml
    device-classes:
      - name: ssd
        volume-group: myvg1
        default: true
        spare-gb: 10
    ```

3. Install `lvmd` and `lvmd.service`, then start the service.

### Setup `lvmd` using Kubernetes DaemonSet

Also, you can setup [lvmd][] using Kubernetes DaemonSet.

Notice: The lvmd container uses `nsenter` to run some lvm commands(like `lvcreate`) as a host process, so you can't launch lvmd with DaemonSet when you're using [kind](https://kind.sigs.k8s.io/).

To setup `lvmd` with Daemonset:

1. Prepare LVM volume groups in the host. A non-empty volume group can be used because LV names wouldn't conflict.
2. Specify the following options in the values.yaml of Helm Chart:

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

### Migrate lvmd, which is running as daemon, to DaemonSet

This section describes how to switch to DaemonSet lvmd from lvmd running as daemons.

1. Install Helm Chart by configuring lvmd to act as a DaemonSet.
   **You need to set the temporal `socket-name` which is not the same as the value in lvmd running as daemon.**
   After the installation Helm Chart, DaemonSet lvmd and lvmd running as daemon exist at the same time using different sockets.

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

2. Change the options of topolvm-node to communicate with the DaemonSet lvmd instead of lvmd running as daemon.
   **You should set the temporal socket name which is not the same as in lvmd running as daemon.**

   ```yaml
   <snip>
   node:
     lvmdSocket: /run/lvmd/lvmd.sock # Change this value to to something linke `/run/lvmd/lvmd-work.sock`.
   <snip>
   ```

3. Check if you can create Pod/PVC and can access to existing PV.

4. Stop and remove lvmd running as daemon.

5. Change the `socket-name` and `--lvmd-socket` options to the original one.
   To reflect the changes of ConfigMap, restart DamonSet lvmd manually.

    ```yaml
   <snip>
   lvmd:
     socketName: /run/topolvm/lvmd-work.sock # Change this value to something like `/run/topolvm/lvmd.sock`.
   <snip>
   node:
     lvmdSocket: /run/lvmd/lvmd.sock # Change this value to something linke `/run/lvmd/lvmd.sock`.
   <snip>
    ```

## cert-manager

[cert-manager][] is used to issue self-signed TLS certificate for [topolvm-controller][].
Follow the [documentation](https://docs.cert-manager.io/en/latest/getting-started/install/kubernetes.html) to install it into your Kubernetes cluster.

### OPTIONAL: Install cert-manager with Helm Chart

Before installing the chart, you must first install the cert-manager CustomResourceDefinition resources.

```sh
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.crds.yaml
```

Set the `cert-manager.enabled=true` in the Helm Chart values.

```yaml
cert-manager:
  enabled: true
```

### OPTIONAL: Prepare the certificate without cert-manager

You can prepare the certificate manually without `cert-manager`.
When doing so, do not apply [certificates.yaml](./manifests/base/certificates.yaml).

1. Prepare PEM encoded self-signed certificate and key files.  
   The certificate must be valid for hostname `controller.topolvm-system.svc`.
2. Base64-encode the CA cert (in its PEM format
3. Create Secret in `topolvm-system` namespace as follows:

    ```console
    kubectl -n topolvm-system create secret tls mutatingwebhook \
        --cert=<CERTIFICATE FILE> --key=<KEY FILE>
    ```

4. Specify the certificate in the Helm Chart values.

    ```yaml
    <snip>
    webhook:
      caBundle: ... # Base64-encoded, PEM-encoded CA certificate that signs the server certificate
    <snip>
    ```

## Scheduing

### topolvm-scheduler

[topolvm-scheduler][] is a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for `kube-scheduler`.
It must be deployed to where `kube-scheduler` can connect.

If your Kubernetes cluster runs the control plane on Nodes, `topolvm-scheduler` should be run as DaemonSet
limited to the control plane nodes.  `kube-scheduler` then connects to the extender via loopback network device.

Otherwise, `topolvm-scheduler` should be run as Deployment and Service.
`kube-scheduler` then connects to the Service address.

#### Running topolvm-scheduler using DaemonSet

Set the `scheduler.type=daemonset` in the Helm Chart values.
The default is daemonset.

    ```yaml
    <snip>
    scheduler:
      type: daemonset
    <snip>
    ```

#### Running topolvm-scheduler using Deployment and Service

In this case, you can set the `scheduler.type=deployment` in the Helm Chart values.

    ```yaml
    <snip>
    scheduler:
      type: deployment
    <snip>
    ```

This way, `topolvm-scheduler` is exposed by LoadBalancer service.

Then edit `urlPrefix` in [scheduler-config-v1beta1.yaml](./scheduler-config/scheduler-config-v1beta1.yaml) for K8s 1.19 or later, to specify the LoadBalancer address.

#### OPTIONAL: tune the node scoring

The node scoring for Pod scheduling can be fine-tuned with the following two ways:
1. Adjust `divisor` parameter in the scoring expression
2. Change the weight for the node scoring against the default by kube-scheduler

The scoring expression in `topolvm-scheduler` is as follows:
```
min(10, max(0, log2(capacity >> 30 / divisor)))
```
For example, the default of `divisor` is `1`, then if a node has the free disk capacity more than `1024GiB`, `topolvm-scheduler` scores the node as `10`. `divisor` should be adjusted to suit each environment. It can be specified the default value and values for each device-class in [scheduler-options.yaml](./manifests/overlays/daemonset-scheduler/scheduler-options.yaml) as follows:

```yaml
default-divisor: 1
divisors:
  ssd: 1
  hdd: 10
```

Besides, the scoring weight can be passed to kube-scheduler via [scheduler-config-v1beta1.yaml](./scheduler-config/scheduler-config-v1beta1.yaml). Almost all scoring algorithms in kube-scheduler are weighted as `"weight": 1`. So if you want to give a priority to the scoring by `topolvm-scheduler`, you have to set the weight as a value larger than one like as follows:

```yaml
apiVersion: kubescheduler.config.k8s.io/v1beta1
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: true
clientConnection:
  kubeconfig: /etc/kubernetes/scheduler.conf
extenders:
- urlPrefix: "http://127.0.0.1:9251"
  filterVerb: "predicate"
  prioritizeVerb: "prioritize"
  nodeCacheCapable: false
  weight: 100 ## EDIT THIS FIELD ##
  managedResources:
  - name: "topolvm.cybozu.com/capacity"
    ignoredByScheduler: true
```

### Storage Capacity Tracking

topolvm supports [Storage Capacity Tracking](https://kubernetes.io/docs/concepts/storage/storage-capacity/).
You can enable Storage Capacity Tracking mode instead of using topolvm-scheduler.
You need to use Kubernetes Cluster v1.21 or later when using Storage Capacity Tracking with topolvm.

You can see the limitations of using Storage Capacity Tracking from [here](https://kubernetes.io/docs/concepts/storage/storage-capacity/#scheduling).

#### Use Storage Capacity Tracking

If you want to use Storage Capacity Tracking instead of using topolvm-scheduler,
you need set the `controller.storageCapacityTracking.enabled=true` and `scheduler.enabled=false` in the Helm Chart values.

    ```yaml
    <snip>
    controller:
      storageCapacityTracking:
        enabled: false
    <snip>
    scheduler:
      enabled: false
    <snip>
    ```

## Protect system namespaces from TopoLVM webhook

TopoLVM installs a mutating webhook for Pods.  It may prevent Kubernetes from bootstrapping
if the webhook pods and the system pods are both missing.

To workaround the problem, add a label to system namespaces such as `kube-system` as follows:

```console
$ kubectl label namespace kube-system topolvm.cybozu.com/webhook=ignore
```

## Configure StorageClasses

You need to create [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/) for TopoLVM.
The Helm chart creates a StorageClasses by default with the following configuration.
You can edit the Helm Chart values as needed.

   ```yaml
   <snip>
   storageClasses:
     - name: topolvm-provisioner
       storageClass:
         fsType: xfs
         isDefaultClass: false
         volumeBindingMode: WaitForFirstConsumer
         allowVolumeExpansion: true
   <snip>
   ```

## Install Helm Chart

The first step is to create a namespace and add a label.

```console
$ kubectl create namespace topolvm-system
$ kubectl label namespace topolvm-system topolvm.cybozu.com/webhook=ignore
```

> :memo: Helm does not support adding labels or other metadata when creating namespaces.
>
> refs: https://github.com/helm/helm/issues/5153, https://github.com/helm/helm/issues/3503

Install Helm Chart using the configured values.yaml.

```sh
helm upgrade --namespace=topolvm-system -f values.yaml -i topolvm topolvm/topolvm
```

## Configure kube-scheduler

`kube-scheduler` need to be configured to use `topolvm-scheduler` extender.

First you need to choose an appropriate `KubeSchdulerConfiguration` YAML file according to your Kubernetes version.

```console
cp ./deploy/scheduler-config/scheduler-config-v1beta1.yaml ./deploy/scheduler-config/scheduler-config.yaml
```

And then copy the [deploy/scheduler-config](./scheduler-config) directory to the hosts where `kube-scheduler`s run.

### For new clusters

If you are installing your cluster from scratch with `kubeadm`, you can use the following configuration:

```yaml
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
metadata:
  name: config
kubernetesVersion: v1.18.2
scheduler:
  extraVolumes:
    - name: "config"
      hostPath: /path/to/scheduler-config     # absolute path to ./scheduler-config directory
      mountPath: /var/lib/scheduler
      readOnly: true
  extraArgs:
    config: /var/lib/scheduler/scheduler-config.yaml
```

### For existing clusters

The changes to `/etc/kubernetes/manifests/kube-scheduler.yaml` that are affected by this are as follows:

1. Add a line to the `command` arguments array such as ```- --config=/var/lib/scheduler/scheduler-config.yaml```. Note that this is the location of the file **after** it is mapped to the `kube-scheduler` container, not where it exists on the node local filesystem.
2. Add a volume mapping to the location of the configuration on your node:

    ```yaml
      spec.volumes:
      - hostPath:
          path: /path/to/scheduler-config     # absolute path to ./scheduler-config directory
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

## How to use TopoLVM provisioner

See [podpvc.yaml](../example/podpvc.yaml) for how to use TopoLVM provisioner.

[lvmd]: ../docs/lvmd.md
[cert-manager]: https://github.com/jetstack/cert-manager
[topolvm-scheduler]: ../docs/topolvm-scheduler.md
[topolvm-controller]: ../docs/topolvm-controller.md
