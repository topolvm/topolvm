Deploying TopoLVM
=================

An overview of setup is as follows:

1. Deploy [lvmd][] as a systemd service on Node OS.
2. Prepare [cert-manager][] for your Kubernetes cluster.  This is for [topolvm-controller][].
3. Determine how [topolvm-scheduler][] to be run:
   - If your Kubernetes have control plane nodes, `topolvm-scheduler` should be deployed as DaemonSet.
   - Otherwise, `topolvm-scheduler` should be deployed as Deployment and Service.
4. Add `topolvm.cybozu.com/webhook: ignore` label to system namespaces such as `kube-system`.
5. Apply manifests for TopoLVM.
6. Configure `kube-scheduler` to use `topolvm-scheduler`.
7. Adjust CSIDriver config for Kubernetes 1.15 and earlier
8. Prepare StorageClasses for TopoLVM.

Example configuration files are included in the following sub directories:

- `lvmd-config/`: Configuration file for `lvmd`
- `manifests/`: Manifests for Kubernetes.
- `scheduler-config/`: Configurations to extend `kube-scheduler` with `topolvm-scheduler`.
- `systemd/`: A systemd unit file for `lvmd`.

These configuration files may need to be modified for your environment.
Read carefully the following descriptions.

lvmd
----

[lvmd][] is a gRPC service to manage an LVM volume group.  The pre-built binary can be downloaded from [releases page](https://github.com/topolvm/topolvm/releases).
It can be built from source code by `GO111MODULE=on go build ./pkg/lvmd`.

To setup `lvmd`:

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

cert-manager
------------

[cert-manager][] is used to issue self-signed TLS certificate for [topolvm-controller][].
Follow the [documentation](https://docs.cert-manager.io/en/latest/getting-started/install/kubernetes.html) to install it into your Kubernetes cluster.

### Prepare the certificate without cert-manager

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

4. Edit `MutatingWebhookConfiguration` in [webhooks.yaml](./manifests/base/mutating/webhooks.yaml) as follows:

    ```yaml
    apiVersion: admissionregistration.k8s.io/v1beta1
    kind: MutatingWebhookConfiguration
    metadata:
      name: topolvm-hook
    # snip
    webhooks:
      - name: pvc-hook.topolvm.cybozu.com
        # snip
        clientConfig:
          caBundle: |  # Base64-encoded, PEM-encoded CA certificate that signs the server certificate
            ...
      - name: pod-hook.topolvm.cybozu.com
        # snip
        clientConfig:
          caBundle: |  # The same CA certificate as above
            ...
    ```

topolvm-scheduler
-----------------

[topolvm-scheduler][] is a [scheduler extender](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/scheduling/scheduler_extender.md) for `kube-scheduler`.
It must be deployed to where `kube-scheduler` can connect.

If your Kubernetes cluster runs the control plane on Nodes, `topolvm-scheduler` should be run as DaemonSet
limited to the control plane nodes.  `kube-scheduler` then connects to the extender via loopback network device.

Otherwise, `topolvm-scheduler` should be run as Deployment and Service.
`kube-scheduler` then connects to the Service address.

### Running topolvm-scheduler using DaemonSet

The [example manifest](./manifests/overlays/daemonset-scheduler/scheduler.yaml) can be used almost as is.
You may need to change the taint key or label name of the DaemonSet.

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  namespace: topolvm-system
  name: topolvm-scheduler
spec:
  # snip
      hostNetwork: true               # If kube-scheduler does not use host network, change this false.
      tolerations:                    # Add tolerations needed to run pods on control plane nodes.
        - key: CriticalAddonsOnly
          operator: Exists
        - effect: NoSchedule
          key: node-role.kubernetes.io/master
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: node-role.kubernetes.io/master    # match the control plane node specific labels
                    operator: Exists
```

### Running topolvm-scheduler using Deployment and Service

In this case, you can use [deployment-scheduler/scheduler.yaml](./manifests/overlays/deployment-scheduler/scheduler.yaml) instead of [daemonset-scheduler/scheduler.yaml](./manifests/overlays/daemonset-scheduler/scheduler.yaml).

This way, `topolvm-scheduler` is exposed by LoadBalancer service.

Then edit `urlPrefix` in [scheduler-policy.cfg](./scheduler-config/scheduler-policy.cfg) to specify the LoadBalancer address.

### How to tune the node scoring

The node scoring for Pod scheduling can be fine-tuned with the following two ways:
1. Adjust `divisor` parameter in the scoring expression
2. Change the weight for the node scoring against the default by kube-scheduler

The scoring expression in `topolvm-scheduler` is as follows:
```
min(10, max(0, log2(capacity >> 30 / divisor)))
```
For example, the default of `divisor` is `1`, then if a node has the free disk capacity more than `1024GiB`, `topolvm-scheduler` scores the node as `10`. `divisor` should be adjusted to suit each environment. It can be specified the default value and values for each device-class in [scheduler-options.yaml](./manifests/base/scheduler-options.yaml) as follows:

```yaml
default-divisor: 1
divisors:
  ssd: 1
  hdd: 10
```

Besides, the scoring weight can be passed to kube-scheduler via [scheduler-policy.cfg](./scheduler-config/scheduler-policy.cfg). Almost all scoring algorithms in kube-scheduler are weighted as `"weight": 1`. So if you want to give a priority to the scoring by `topolvm-scheduler`, you have to set the weight as a value larger than one like as follows:
```json
{
  "kind" : "Policy",
  "apiVersion" : "v1",
  "extenders" :
    [{
      "urlPrefix": "http://127.0.0.1:9251",
      "filterVerb": "predicate",
      "prioritizeVerb": "prioritize",
      "nodeCacheCapable": false,
      "weight": 100, ## EDIT THIS FIELD ##
      "managedResources":
      [{
        "name": "topolvm.cybozu.com/capacity",
        "ignoredByScheduler": true
      }]
    }]
}
```


Protect system namespaces from TopoLVM webhook
---------------------------------------------

TopoLVM installs a mutating webhook for Pods.  It may prevent Kubernetes from bootstrapping
if the webhook pods and the system pods are both missing.

To workaround the problem, add a label to system namespaces such as `kube-system` as follows.

```console
$ kubectl label ns kube-system topolvm.cybozu.com/webhook=ignore
```

Apply manifests for TopoLVM
---------------------------

Once you finish editing manifests, apply them as follows.

If you want to apply `topolvm-scheduler` as a DaemonSet, run the following command: 

```console
kustomize build ./deploy/manifests/overlays/daemonset-scheduler | kubectl apply -f -
```

Instead, if you want to apply `topolvm-scheduler` as a Deployment, run the following command: 

```console
kustomize build ./deploy/manifests/overlays/deployment-scheduler | kubectl apply -f -
```

If you use `cert-manager`, run the following command in addition:

```console
kubectl apply -f ./deploy/manifests/base/certificates.yaml
```

Configure kube-scheduler
------------------------

`kube-scheduler` need to be configured to use `topolvm-scheduler` extender.

If your Kubernetes cluster was installed with `kubeadm`, then reconfigure it as follows:

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

Otherwise, consult the manual of your Kubernetes cluster distribution.

Adjust CSIDriver config for Kubernetes 1.15 and earlier
----------------------

If you are running Kubernetes 1.15 or prior, modify the `CSIDriver` as follows
to account for features not supported on earlier versions:

```yaml
apiVersion: storage.k8s.io/v1beta1
kind: CSIDriver
metadata:
  name: topolvm.cybozu.com
spec:
  attachRequired: true
```

Prepare StorageClasses
----------------------

Finally, you need to create [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/) for TopoLVM.

An example is available in [provisioner.yaml](./manifests/base/provisioner.yaml).

See [podpvc.yaml](../example/podpvc.yaml) for how to use TopoLVM provisioner.

[lvmd]: ../docs/lvmd.md
[cert-manager]: https://github.com/jetstack/cert-manager
[topolvm-scheduler]: ../docs/topolvm-scheduler.md
[topolvm-controller]: ../docs/topolvm-controller.md
