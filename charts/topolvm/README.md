# TopoLVM Helm Chart

## Prerequisites

* `cert-manager` version `v1.7.0+` installed. ref: https://cert-manager.io/
* Requires at least `v3.5.0+` version of helm to support

## Installation

See [Getting Started](https://github.com/topolvm/topolvm/blob/topolvm-chart-v13.0.1/docs/getting-started.md).

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| cert-manager.enabled | bool | `false` | Install cert-manager together. # ref: https://cert-manager.io/docs/installation/kubernetes/#installing-with-helm |
| controller.affinity | string | `"podAntiAffinity:\n  requiredDuringSchedulingIgnoredDuringExecution:\n    - labelSelector:\n        matchExpressions:\n          - key: app.kubernetes.io/component\n            operator: In\n            values:\n              - controller\n          - key: app.kubernetes.io/name\n            operator: In\n            values:\n              - {{ include \"topolvm.name\" . }}\n      topologyKey: kubernetes.io/hostname\n"` | Specify affinity. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity |
| controller.args | list | `[]` | Arguments to be passed to the command. |
| controller.initContainers | list | `[]` | Additional initContainers for the controller service. |
| controller.labels | object | `{}` | Additional labels to be added to the Deployment. |
| controller.minReadySeconds | int | `nil` | Specify minReadySeconds. |
| controller.nodeFinalize.skipped | bool | `false` | Skip automatic cleanup of PhysicalVolumeClaims when a Node is deleted. |
| controller.nodeSelector | object | `{}` | Specify nodeSelector. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |
| controller.podDisruptionBudget.enabled | bool | `true` | Specify podDisruptionBudget enabled. |
| controller.podLabels | object | `{}` | Additional labels to be set on the controller pod. |
| controller.priorityClassName | string | `nil` | Specify priorityClassName. |
| controller.prometheus.podMonitor.additionalLabels | object | `{}` | Additional labels that can be used so PodMonitor will be discovered by Prometheus. |
| controller.prometheus.podMonitor.enabled | bool | `false` | Set this to `true` to create PodMonitor for Prometheus operator. |
| controller.prometheus.podMonitor.interval | string | `""` | Scrape interval. If not set, the Prometheus default scrape interval is used. |
| controller.prometheus.podMonitor.metricRelabelings | list | `[]` | MetricRelabelConfigs to apply to samples before ingestion. |
| controller.prometheus.podMonitor.namespace | string | `""` | Optional namespace in which to create PodMonitor. |
| controller.prometheus.podMonitor.relabelings | list | `[]` | RelabelConfigs to apply to samples before scraping. |
| controller.prometheus.podMonitor.scrapeTimeout | string | `""` | Scrape timeout. If not set, the Prometheus default scrape timeout is used. |
| controller.replicaCount | int | `2` | Number of replicas for CSI controller service. |
| controller.securityContext.enabled | bool | `true` | Enable securityContext. |
| controller.storageCapacityTracking.enabled | bool | `true` | Enable Storage Capacity Tracking for csi-provisioner. |
| controller.terminationGracePeriodSeconds | int | `nil` | Specify terminationGracePeriodSeconds. |
| controller.tolerations | list | `[]` | Specify tolerations. # ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| controller.updateStrategy | object | `{}` | Specify updateStrategy. |
| controller.volumes | list | `[{"emptyDir":{},"name":"socket-dir"}]` | Specify volumes. |
| env.csi_provisioner | list | `[]` | Specify environment variables for csi_provisioner container. |
| env.csi_registrar | list | `[]` | Specify environment variables for csi_registrar container. |
| env.csi_resizer | list | `[]` | Specify environment variables for csi_resizer container. |
| env.csi_snapshotter | list | `[]` | Specify environment variables for csi_snapshotter container. |
| env.liveness_probe | list | `[]` | Specify environment variables for liveness_probe container. |
| env.topolvm_controller | list | `[]` | Specify environment variables for topolvm_controller container. |
| env.topolvm_node | list | `[]` | Specify environment variables for topolvm_node container. |
| env.topolvm_scheduler | list | `[]` | Specify environment variables for topolvm_scheduler container. |
| image.csi.csiProvisioner | string | `nil` | Specify csi-provisioner image. If not specified, `ghcr.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.csiResizer | string | `nil` | Specify csi-resizer image. If not specified, `ghcr.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.csiSnapshotter | string | `nil` | Specify csi-snapshot image. If not specified, `ghcr.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.livenessProbe | string | `nil` | Specify livenessprobe image. If not specified, `ghcr.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.csi.nodeDriverRegistrar | string | `nil` | Specify csi-node-driver-registrar: image. If not specified, `ghcr.io/topolvm/topolvm-with-sidecar:{{ .Values.image.tag }}` will be used. |
| image.pullPolicy | string | `nil` | TopoLVM image pullPolicy. |
| image.pullSecrets | list | `[]` | List of imagePullSecrets. |
| image.repository | string | `"ghcr.io/topolvm/topolvm-with-sidecar"` | TopoLVM image repository to use. |
| image.tag | string | `{{ .Chart.AppVersion }}` | TopoLVM image tag to use. |
| livenessProbe.csi_registrar | object | `{"failureThreshold":null,"initialDelaySeconds":10,"periodSeconds":60,"timeoutSeconds":3}` | Specify livenessProbe. # ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/ |
| livenessProbe.lvmd | object | `{"failureThreshold":null,"initialDelaySeconds":10,"periodSeconds":60,"timeoutSeconds":3}` | Specify livenessProbe. # ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/ |
| livenessProbe.topolvm_controller | object | `{"failureThreshold":null,"initialDelaySeconds":10,"periodSeconds":60,"timeoutSeconds":3}` | Specify livenessProbe. # ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/ |
| livenessProbe.topolvm_node | object | `{"failureThreshold":null,"initialDelaySeconds":10,"periodSeconds":60,"timeoutSeconds":3}` | Specify resources. # ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/ |
| livenessProbe.topolvm_scheduler | object | `{"failureThreshold":null,"initialDelaySeconds":10,"periodSeconds":60,"timeoutSeconds":3}` | Specify livenessProbe. # ref: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/ |
| lvmd.additionalConfigs | list | `[]` | Define additional LVM Daemon configs if you have additional types of nodes. Please ensure nodeSelectors are non overlapping. |
| lvmd.affinity | object | `{}` | Specify affinity. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity |
| lvmd.args | list | `[]` | Arguments to be passed to the command. |
| lvmd.deviceClasses | list | `[{"default":true,"name":"ssd","spare-gb":10,"volume-group":"myvg1"}]` | Specify the device-class settings. |
| lvmd.env | list | `[]` | extra environment variables |
| lvmd.initContainers | list | `[]` | Additional initContainers for the lvmd service. |
| lvmd.labels | object | `{}` | Additional labels to be added to the Daemonset. |
| lvmd.lvcreateOptionClasses | list | `[]` | Specify the lvcreate-option-class settings. |
| lvmd.managed | bool | `true` | If true, set up lvmd service with DaemonSet. |
| lvmd.nodeSelector | object | `{}` | Specify nodeSelector. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |
| lvmd.podLabels | object | `{}` | Additional labels to be set on the lvmd service pods. |
| lvmd.priorityClassName | string | `nil` | Specify priorityClassName. |
| lvmd.socketName | string | `"/run/topolvm/lvmd.sock"` | Specify socketName. |
| lvmd.tolerations | list | `[]` | Specify tolerations. # ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| lvmd.updateStrategy | object | `{}` | Specify updateStrategy. |
| lvmd.volumeMounts | list | `[]` | Specify volumeMounts. |
| lvmd.volumes | list | `[]` | Specify volumes. |
| node.affinity | object | `{}` | Specify affinity. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity |
| node.args | list | `[]` | Arguments to be passed to the command. |
| node.initContainers | list | `[]` | Additional initContainers for the node service. |
| node.kubeletWorkDirectory | string | `"/var/lib/kubelet"` | Specify the work directory of Kubelet on the host. For example, on microk8s it needs to be set to `/var/snap/microk8s/common/var/lib/kubelet` |
| node.labels | object | `{}` | Additional labels to be added to the Daemonset. |
| node.lvmdEmbedded | bool | `false` | Specify whether to embed lvmd in the node container. Should not be used in conjunction with lvmd.managed otherwise lvmd will be started twice. |
| node.lvmdSocket | string | `"/run/topolvm/lvmd.sock"` | Specify the socket to be used for communication with lvmd. |
| node.metrics.annotations | object | `{"prometheus.io/port":"metrics"}` | Annotations for Scrape used by Prometheus. |
| node.metrics.enabled | bool | `true` | If true, enable scraping of metrics by Prometheus. |
| node.nodeSelector | object | `{}` | Specify nodeSelector. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |
| node.podLabels | object | `{}` | Additional labels to be set on the node pods. |
| node.priorityClassName | string | `nil` | Specify priorityClassName. |
| node.prometheus.podMonitor.additionalLabels | object | `{}` | Additional labels that can be used so PodMonitor will be discovered by Prometheus. |
| node.prometheus.podMonitor.enabled | bool | `false` | Set this to `true` to create PodMonitor for Prometheus operator. |
| node.prometheus.podMonitor.interval | string | `""` | Scrape interval. If not set, the Prometheus default scrape interval is used. |
| node.prometheus.podMonitor.metricRelabelings | list | `[]` | MetricRelabelConfigs to apply to samples before ingestion. |
| node.prometheus.podMonitor.namespace | string | `""` | Optional namespace in which to create PodMonitor. |
| node.prometheus.podMonitor.relabelings | list | `[]` | RelabelConfigs to apply to samples before scraping. |
| node.prometheus.podMonitor.scrapeTimeout | string | `""` | Scrape timeout. If not set, the Prometheus default scrape timeout is used. |
| node.securityContext.privileged | bool | `true` |  |
| node.tolerations | list | `[]` | Specify tolerations. # ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| node.updateStrategy | object | `{}` | Specify updateStrategy. |
| node.volumeMounts.topolvmNode | list | `[]` | Specify volumes. |
| node.volumes | list | `[]` | Specify volumes. |
| priorityClass.enabled | bool | `true` | Install priorityClass. |
| priorityClass.name | string | `"topolvm"` | Specify priorityClass resource name. |
| priorityClass.value | int | `1000000` |  |
| resources.csi_provisioner | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.csi_registrar | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.csi_resizer | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.csi_snapshotter | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.liveness_probe | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.lvmd | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.topolvm_controller | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.topolvm_node | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| resources.topolvm_scheduler | object | `{}` | Specify resources. # ref: https://kubernetes.io/docs/user-guide/compute-resources/ |
| scheduler.affinity | object | `{"nodeAffinity":{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"node-role.kubernetes.io/control-plane","operator":"Exists"}]}]}}}` | Specify affinity on the Deployment or DaemonSet. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity |
| scheduler.args | list | `[]` | Arguments to be passed to the command. |
| scheduler.deployment.replicaCount | int | `2` | Number of replicas for Deployment. |
| scheduler.enabled | bool | `false` | If true, enable scheduler extender for TopoLVM |
| scheduler.labels | object | `{}` | Additional labels to be added to the Deployment or Daemonset. |
| scheduler.minReadySeconds | int | `nil` | Specify minReadySeconds on the Deployment or DaemonSet. |
| scheduler.nodeSelector | object | `{}` | Specify nodeSelector on the Deployment or DaemonSet. # ref: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/ |
| scheduler.options.listen.host | string | `"localhost"` | Host used by Probe. |
| scheduler.options.listen.port | int | `9251` | Listen port. |
| scheduler.podDisruptionBudget.enabled | bool | `true` | Specify podDisruptionBudget enabled. |
| scheduler.podLabels | object | `{}` | Additional labels to be set on the scheduler pods. |
| scheduler.priorityClassName | string | `nil` | Specify priorityClassName on the Deployment or DaemonSet. |
| scheduler.schedulerOptions | object | `{}` | Tune the Node scoring. ref: https://github.com/topolvm/topolvm/blob/master/deploy/README.md |
| scheduler.service.clusterIP | string | `nil` | Specify Service clusterIP. |
| scheduler.service.nodePort | int | `nil` | Specify nodePort. |
| scheduler.service.type | string | `"LoadBalancer"` | Specify Service type. |
| scheduler.terminationGracePeriodSeconds | int | `nil` | Specify terminationGracePeriodSeconds on the Deployment or DaemonSet. |
| scheduler.tolerations | list | `[{"key":"CriticalAddonsOnly","operator":"Exists"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/control-plane"},{"effect":"NoSchedule","key":"node-role.kubernetes.io/master"}]` | Specify tolerations on the Deployment or DaemonSet. # ref: https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/ |
| scheduler.type | string | `"daemonset"` | If you run with a managed control plane (such as GKE, AKS, etc), topolvm-scheduler should be deployed as Deployment and Service. topolvm-scheduler should otherwise be deployed as DaemonSet in unmanaged (i.e. bare metal) deployments. possible values:  daemonset/deployment |
| scheduler.updateStrategy | object | `{}` | Specify updateStrategy on the Deployment or DaemonSet. |
| securityContext.runAsGroup | int | `10000` | Specify runAsGroup. |
| securityContext.runAsUser | int | `10000` | Specify runAsUser. |
| snapshot.enabled | bool | `true` | Turn on the snapshot feature. |
| storageClasses | list | `[{"name":"topolvm-provisioner","storageClass":{"additionalParameters":{},"allowVolumeExpansion":true,"annotations":{},"fsType":"xfs","isDefaultClass":false,"mountOptions":[],"reclaimPolicy":null,"volumeBindingMode":"WaitForFirstConsumer"}}]` | Whether to create storageclass(es) ref: https://kubernetes.io/docs/concepts/storage/storage-classes/ |
| useLegacy | bool | `false` | If true, the legacy plugin name and legacy custom resource group is used(topolvm.cybozu.com). |
| webhook.caBundle | string | `nil` | Specify the certificate to be used for AdmissionWebhook. |
| webhook.existingCertManagerIssuer | object | `{}` | Specify the cert-manager issuer to be used for AdmissionWebhook. |
| webhook.podMutatingWebhook.enabled | bool | `false` | Enable Pod MutatingWebhook. |
| webhook.pvcMutatingWebhook.enabled | bool | `true` | Enable PVC MutatingWebhook. |

## Generate Manifests

You can use the `helm template` command to render manifests.

```sh
helm template --include-crds --namespace=topolvm-system topolvm topolvm/topolvm
```

## About the Legacy Flag

In https://github.com/topolvm/topolvm/pull/592, the domain name which is used for CRD and other purposes was changed from topolvm.cybozu.com to topolvm.io.
Automatic domain name migration to topolvm.io is risky from the data integrity point of view, and migration to topolvm.io has a large impact on the entire TopoLVM system, including CRDs.
So we added an option to use topolvm.cybozu.com as it is.
TopoLVM users can continue to use topolvm.cybozu.com by setting `--set useLegacy=true` in their helm chart.

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
