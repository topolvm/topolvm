# TopoLVM Helm Chart
----------------------------------------

## Prerequisites

* Kubernetes 1.18+
* Configure `kube-scheduler` on the underlying nodes, ref: https://github.com/topolvm/topolvm/tree/master/deploy#configure-kube-scheduler
* `cert-manager` version `v1.0.0+` installed. ref: https://cert-manager.io/
* Requires at least `v3.5.0+` version of helm to support

## :warning: Migration from kustomize to Helm

See [MIGRATION.md](./MIGRATION.md)

## How to use TopoLVM Helm repository

You need to add this repository to your Helm repositories:

```sh
helm repo add topolvm https://topolvm.github.io/topolvm
helm repo update
```

## Dependencies

| Repository | Name	| Version |
| ---------- | ---- | ------- |
| https://charts.jetstack.io | cert-manager | 1.3.1 |

## Quick start

By default, the [topolvm-scheduler](../../deploy/README.md#topolvm-scheduler) runs in a DaemonSet.
It can alternatively run inside a Deployment.
Also, [lvmd](../../deploy/README.md#lvmd) is run in a DaemonSet by default.

### Installing the Chart

> :memo: NOTE: This installation method requires cert-manger to be installed beforehand.

To install the chart with the release name `topolvm` using a dedicated namespace(recommended):

```sh
helm install --create-namespace --namespace=topolvm-system topolvm topolvm/topolvm
```

Specify parameters using `--set key=value[,key=value]` argument to `helm install`.

Alternatively a YAML file that specifies the values for the parameters can be provided like this:

```sh
helm upgrade --create-namespace --namespace=topolvm-system -f values.yaml -i topolvm topolvm/topolvm
```

### Install together with cert-manager

Before installing the chart, you must first install the cert-manager CustomResourceDefinition resources.

```sh
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.3.1/cert-manager.crds.yaml
```

Set the `cert-manager.enabled=true` parameter when installing topolvm chart:

```sh
helm install --create-namespace --namespace=topolvm-system topolvm topolvm/topolvm --set cert-manager.enabled=true
```

## Configure kube-scheduler

The current Chart does not provide an option to make kube-scheduler configurable.
You need to configure kube-scheduler to use topolvm-scheduler extender by referring to the following document.

[deploy/README.md#configure-kube-scheduler](../../deploy/README.md#configure-kube-scheduler)

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| cert-manager.enabled | bool | `false` | Install cert-manager together. |
| controller.affinity | object | `{"podAntiAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchExpressions":[{"key":"app.kubernetes.io/name","operator":"In","values":["topolvm-controller"]}]},"topologyKey":"kubernetes.io/hostname"}]}}` | Specify affinity. |
| controller.minReadySeconds | int | `nil` | Specify minReadySeconds. |
| controller.nodeSelector | object | `{}` | Specify nodeSelector. |
| controller.replicaCount | int | `2` | Number of replicas for CSI controller service. |
| controller.resources | object | `{}` | Specify resources. |
| controller.securityContext.enabled | bool | `true` | Enable securityContext. |
| controller.storageCapacityTracking.enabled | bool | `false` | Enable Storage Capacity Tracking for csi-provisoner. |
| controller.terminationGracePeriodSeconds | int | `nil` | Specify terminationGracePeriodSeconds. |
| controller.tolerations | list | `[]` | Specify tolerations. |
| controller.updateStrategy | object | `{}` | Specify updateStrategy. |
| controller.volumes | list | `[{"emptyDir":{},"name":"socket-dir"}]` | Specify volumes. |
| image.csi.csiProvisioner | string | `nil` | Specify csi-provisioner image. If not specified, `quay.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.csiResizer | string | `nil` | Specify csi-resizer image. If not specified, `quay.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.livenessProbe | string | `nil` | Specify livenessprobe image. If not specified, `quay.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.nodeDriverRegistrar | string | `nil` | Specify csi-node-driver-registrar: image. If not specified, `quay.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.pullPolicy | string | `nil` | TopoLVM image pullPolicy. |
| image.repository | string | `"quay.io/topolvm/topolvm-with-sidecar"` | TopoLVM image repository to use. |
| image.tag | string | `{{ .Chart.AppVersion }}` | TopoLVM image tag to use. |
| lvmd.additionalConfigs | list | `[]` | Define additional LVM Daemon configs if you have additional types of nodes. Please ensure nodeSelectors are non overlapping. |
| lvmd.deviceClasses | list | `[{"default":true,"name":"ssd","spare-gb":10,"volume-group":"myvg1"}]` | Specify the device-class settings. |
| lvmd.managed | bool | `true` | If true, set up lvmd service with DaemonSet. |
| lvmd.nodeSelector | object | `{}` | Specify nodeSelector. |
| lvmd.psp.allowedHostPaths | list | `[{"pathPrefix":"/run/topolvm","readOnly":false}]` | Specify allowedHostPaths. |
| lvmd.resources | object | `{}` | Specify resources. |
| lvmd.socketName | string | `"/run/topolvm/lvmd.sock"` | Specify socketName. |
| lvmd.tolerations | list | `[]` | Specify tolerations. |
| lvmd.volumeMounts | list | `[{"mountPath":"/run/topolvm","name":"lvmd-socket-dir"}]` | Specify volumeMounts. |
| lvmd.volumes | list | `[{"hostPath":{"path":"/run/topolvm","type":"DirectoryOrCreate"},"name":"lvmd-socket-dir"}]` | Specify volumes. |
| node.lvmdSocket | string | `"/run/lvmd/lvmd.sock"` | Specify the socket to be used for communication with lvmd. |
| node.metrics.annotations | object | `{"prometheus.io/port":"8080"}` | Annotations for Scrape used by Prometheus.. |
| node.metrics.enabled | bool | `true` | If true, enable scraping of metrics by Prometheus. |
| node.nodeSelector | object | `{}` | Specify nodeSelector. |
| node.resources | object | `{}` | Specify resources. |
| node.securityContext.privileged | bool | `true` |  |
| node.tolerations | list | `[]` | Specify tolerations. |
| node.volumeMounts.topolvmNode | list | `[{"mountPath":"/run/topolvm","name":"node-plugin-dir"},{"mountPath":"/run/lvmd","name":"lvmd-socket-dir"},{"mountPath":"/var/lib/kubelet/pods","mountPropagation":"Bidirectional","name":"pod-volumes-dir"},{"mountPath":"/var/lib/kubelet/plugins/kubernetes.io/csi","mountPropagation":"Bidirectional","name":"csi-plugin-dir"}]` | Specify volumeMounts for topolvm-node container. |
| node.volumes | list | `[{"hostPath":{"path":"/var/lib/kubelet/plugins_registry/","type":"Directory"},"name":"registration-dir"},{"hostPath":{"path":"/var/lib/kubelet/plugins/topolvm.cybozu.com/node","type":"DirectoryOrCreate"},"name":"node-plugin-dir"},{"hostPath":{"path":"/var/lib/kubelet/plugins/kubernetes.io/csi","type":"DirectoryOrCreate"},"name":"csi-plugin-dir"},{"hostPath":{"path":"/var/lib/kubelet/pods/","type":"DirectoryOrCreate"},"name":"pod-volumes-dir"},{"hostPath":{"path":"/run/topolvm","type":"Directory"},"name":"lvmd-socket-dir"}]` | Specify volumes. |
| podSecurityPolicy.create | bool | `true` | Enable pod security policy. |
| scheduler.affinity | object | `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]},{"matchExpressions":[{"key":"node-role.kubernetes.io/master","operator":"Exists"}]}]}}}` | Specify affinity on the Deployment or DaemonSet. |
| scheduler.deployment.replicaCount | int | `2` | Number of replicas for Deployment. |
| scheduler.minReadySeconds | int | `nil` | Specify minReadySeconds on the Deployment or DaemonSet. |
| scheduler.nodeSelector | object | `{}` | Specify nodeSelector on the Deployment or DaemonSet. |
| scheduler.options.listen.host | string | `"localhost"` | Host used by Probe. |
| scheduler.options.listen.port | int | `9251` | Listen port. |
| scheduler.resources | object | `{}` | Specify resources on the TopoLVM scheduler extender container. |
| scheduler.schedulerOptions | object | `{}` | Tune the Node scoring. ref: https://github.com/topolvm/topolvm/blob/master/deploy/README.md |
| scheduler.service.nodePort | int | `nil` | Specify nodePort. |
| scheduler.service.type | string | `"LoadBalancer"` | Specify Service type. |
| scheduler.terminationGracePeriodSeconds | int | `nil` | Specify terminationGracePeriodSeconds on the Deployment or DaemonSet. |
| scheduler.tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/master"}]` | Specify tolerations on the Deployment or DaemonSet. |
| scheduler.type | string | `"daemonset"` | If you run with a managed control plane (such as GKE, AKS, etc), topolvm-scheduler should be deployed as Deployment and Service. topolvm-scheduler should otherwise be deployed as DaemonSet in unmanaged (i.e. bare metal) deployments. possible values:  daemonset/deployment |
| scheduler.updateStrategy | object | `{}` | Specify updateStrategy on the Deployment or DaemonSet. |
| securityContext.runAsGroup | int | `10000` | Specify runAsGroup. |
| securityContext.runAsUser | int | `10000` | Specify runAsUser. |
| storageClasses | list | `[{"name":"topolvm-provisioner","storageClass":{"additionalParameters":{},"allowVolumeExpansion":true,"annotations":{},"fsType":"xfs","isDefaultClass":false,"reclaimPolicy":null,"volumeBindingMode":"WaitForFirstConsumer"}}]` | Whether to create storageclass(s) ref: https://kubernetes.io/docs/concepts/storage/storage-classes/ |
| webhook.caBundle | string | `nil` | Specify the certificate to be used for AdmissionWebhook. |
| webhook.podMutatingWebhook.enabled | bool | `true` | Enable Pod MutatingWebhook. |
| webhook.pvcMutatingWebhook.enabled | bool | `true` | Enable PVC MutatingWebhook. |

## Generate Manifests

You can use the `helm template` command to render manifests.

```sh
helm template --include-crds --namespace=topolvm-system topolvm topolvm/topolvm
```

## Update README

The `README.md` for this chart is generated by [helm-docs](https://github.com/norwoodj/helm-docs).
To update the README, edit the `README.md.gotmpl` file and run the helm-docs command.

```console
# path to topolvm repository root
$ make setup
$ ./bin/helm-docs
INFO[2021-06-13T21:43:55+09:00] Found Chart directories [charts/topolvm]
INFO[2021-06-13T21:43:55+09:00] Generating README Documentation for chart /path/to/dir/topolvm/topolvm/charts/topolvm
```

## Release Chart

See [RELEASE.md](../../RELEASE.md)
