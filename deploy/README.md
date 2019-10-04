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
7. Prepare StorageClasses for TopoLVM.

Example configuration files are included in the following sub directories:

- `manifests/`: Manifests for Kubernetes.
- `scheduler-config/`: Configurations to extend `kube-scheduler` with `topolvm-scheduler`.
- `systemd/`: A systemd unit file for `lvmd`.

These configuration files may need to be modified for your environment.
Read carefully the following descriptions.

lvmd
----

[lvmd][] is a gRPC service to manage an LVM volume group.  The pre-built binary can be downloaded from [releases page](https://github.com/cybozu-go/topolvm/releases).
It can be built from source code by `GO111MODULE=on go build ./pkg/lvmd`.

To setup `lvmd`:

1. Prepare an LVM volume group.  A non-empty volume group can be used.
2. Edit the following line in [lvmd.service](./systemd/lvmd.service) if the volume group name is not `myvg`.

    ```
    ExecStart=/opt/sbin/lvmd --volume-group=myvg --listen=/run/topolvm/lvmd.sock
    ```

3. Install `lvmd` and `lvmd.service`, then start the service.

cert-manager
------------

[cert-manager][] is used to issue self-signed TLS certificate for [topolvm-controller][].
Follow the [documentation](https://docs.cert-manager.io/en/latest/getting-started/install/kubernetes.html) to install it into your Kubernetes cluster.

### Prepare the certificate without cert-manager

You can prepare the certificate manually without `cert-manager`.
When doing so, do not apply [./manifests/certificates.yaml](./manifests/certificates.yaml).

1. Prepare PEM encoded self-signed certificate and key files.  
    The certificate must be valid for hostname `controller.topolvm-system.svc`.
2. Create Secret in `topolvm-system` namespace as follows:

    ```console
    kubectl -n topolvm-system create secret tls mutatingwebhook \
        --cert=<CERTIFICATE FILE> --key=<KEY FILE>
    ```

3. Edit `MutatingWebhookConfiguration` in [./manifests/mutating/webhooks.yaml](./manifests/mutating/webhooks.yaml) as follows:

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
          caBundle: |  # PEM encoded CA certificate that signs the server certificate
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

The [example manifest](./manifests/scheduler.yaml) can be used almost as is.
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

In this case, DaemonSet in [./manifests/scheduler.yaml](./manifests/scheduler.yaml) must be removed.

Instead, add the following resources:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  namespace: topolvm-system
  name: topolvm-scheduler
spec:
  replicas: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: topolvm-scheduler
  template:
    metadata:
      labels:
        app.kubernetes.io/name: topolvm-scheduler
    spec:
      securityContext:
        runAsUser:  10000
        runAsGroup: 10000
      serviceAccountName: topolvm-scheduler
      containers:
        - name: topolvm-scheduler
          image: quay.io/cybozu/topolvm:0.1.2
          command:
            - /topolvm-scheduler
            - --listen=:9251
          livenessProbe:
            httpGet:
              port: 9251
              path: /status
---
apiVersion: v1
kind: Service
metadata:
  namespace: topolvm-system
  name: topolvm-scheduler
spec:
  type: LoadBalancer
  selector:
    app.kubernetes.io/name: topolvm-scheduler
  ports:
  - protocol: TCP
    port: 9251
```

This way, `topolvm-scheduler` is exposed by LoadBalancer service.

Then edit `urlPrefix` in [./scheduler-config/scheduler-policy.cfg](./scheduler-config/scheduler-policy.cfg) to specify the LoadBalancer address.

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

Once you finish editing manifests, apply them in the following order:

1. [namespace.yaml](./manifests/namespace.yaml)
2. [crd.yaml](./manifests/crd.yaml)
3. [psp.yaml](./manifests/psp.yaml)
4. [certificates.yaml](./manifests/certificates.yaml) if `cert-manager` is installed
5. [scheduler.yaml](./manifests/scheduler.yaml)
6. [mutatingwebhooks.yaml](./manifests/mutatingwebhooks.yaml)
7. [controller.yaml](./manifests/controller.yaml)
8. [node.yaml](./manifests/node.yaml)

Configure kube-scheduler
------------------------

`kube-scheduler` need to be configured to use `topolvm-scheduler` extender.

If your Kubernetes cluster was installed with `kubeadm`, then reconfigure it as follows:

```yaml
apiVersion: kubeadm.k8s.io/v1beta2
kind: ClusterConfiguration
metadata:
  name: config
kubernetesVersion: v1.15.3
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

Prepare StorageClasses
----------------------

Finally, you need to create [StorageClasses](https://kubernetes.io/docs/concepts/storage/storage-classes/) for TopoLVM.

An example is available in [./manifests/provisioner.yaml](./manifests/provisioner.yaml).

See [example/podpvc.yaml](../example/podpvc.yaml) for how to use TopoLVM provisioner.

[lvmd]: ../docs/lvmd.md
[cert-manager]: https://github.com/jetstack/cert-manager
[topolvm-scheduler]: ../docs/topolvm-scheduler.md
[topolvm-controller]: ../docs/topolvm-controller.md
