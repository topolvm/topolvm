# Migration from Kustomize to Helm

## List of renamed resources

By becoming a Helm Chart, the Release Name is given to the `.metadata.name` of some resources.
As a result, there are some differences with the resources of TopoLVM installed by Kustomize.

Old resources need to be deleted manually.

Helm template `{{template "topolvm.fullname" .}}` outputs Helm release name and chart name.
If release name contains chart name it will be used as a full name.

for example:

| Template | Release Name | Output |
| -------- | ------------ | ------ |
| `{{ template "topolvm.fullname" . }}-controller` | foo | **foo-topolvm-controller** |
| `{{ template "topolvm.fullname" . }}-controller` | topolvm | **topolvm-controller** |
| `{{ template "topolvm.fullname" . }}-controller` | bar-topolvm | **bar-topolvm-controller** |
| `{{ template "topolvm.fullname" . }}-controller` | topolvm-baz | **topolvm-baz-controller** |

### List

| Kind | Kustomize Name | Helm Name |
| ---- | -------------- | --------- |
| Issuer             | [webhook-selfsign](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/certificates.yaml#L1-L9) | `{{ template "topolvm.fullname" . }}-webhook-selfsign` |
| Certificate        | [webhook-ca](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/certificates.yaml#L11-L27) | `{{ template "topolvm.fullname" . }}-webhook-ca` |
| Issuer             | [webhook-ca](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/certificates.yaml#L29-L37) | `{{ template "topolvm.fullname" . }}-webhook-ca` |
| Certificate        | [mutatingwebhook](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/certificates.yaml#L39-L58) | `{{ template "topolvm.fullname" . }}-mutatingwebhook` |
| CSIDriver          | [topolvm.cybozu.com](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L2-L11) | **NO CHANGED** |
| ServiceAccount     | [controller](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L14-L18) | `{{ template "topolvm.fullname" . }}-controller` |
| ClusterRole        | [topolvm-system:controller](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L20-L39) | `{{ .Release.Namespace }}:controller` |
| ClusterRoleBinding | [topolvm-system:controller](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L41-L52) | `{{ .Release.Namespace }}:controller` |
| Role               | [leader-election](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L54-L80) | **NO CHANGED** |
| RoleBinding        | [leader-election](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L82-L94) | **NO CHANGED** |
| ClusterRole        | [topolvm-external-provisioner-runner](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L96-L127) | **NO CHANGED** |
| ClusterRoleBinding | [topolvm-csi-provisioner-role](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L129-L150) | **NO CHANGED** |
| RoleBinding        | [csi-provisioner-role-cfg](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L152-L164) | **NO CHANGED** |
| ClusterRole        | [topolvm-external-attacher-runner](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L166-L185) | **NO CHANGED** |
| ClusterRoleBinding | [topolvm-csi-attacher-role](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L187-L198) | **NO CHANGED** |
| RoleBinding        | [csi-attacher-role-cfg](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L210-L222) | **NO CHANGED** |
| ClusterRole        | [topolvm-external-resizer-runner](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L224-L240) | **NO CHANGED** |
| ClusterRoleBinding | [topolvm-csi-resizer-role](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L242-L253) | **NO CHANGED** |
| Role               | [external-resizer-cfg](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L255-L263) | **NO CHANGED** |
| RoleBinding        | [csi-resizer-role-cfg](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L265-L277) | **NO CHANGED** |
| Service            | [controller](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L280-L291) | `{{ template "topolvm.fullname" . }}-controller` |
| Deployment         | [controller](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/controller.yaml#L293-L399) | `{{ template "topolvm.fullname" . }}-controller` |
| MutatingWebhookConfiguration | [topolvm-hook](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/mutatingwebhooks.yaml#L1-L63) | `{{ template "topolvm.fullname" . }}-hook` |
| ServiceAccount     | [node](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/node.yaml#L1-L5) | `{{ template "topolvm.fullname" . }}-node` |
| ClusterRole        | [topolvm-system:node](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/node.yaml#L7-L24) | `{{ .Release.Namespace }}:node` |
| ClusterRoleBinding | [topolvm-system:node](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/node.yaml#L26-L37) | `{{ .Release.Namespace }}:node` |
| DaemonSet          | [node](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/node.yaml#L40-L139) | `{{ template "topolvm.fullname" . }}-node` |
| StorageClass       | [topolvm-provisioner](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/provisioner.yaml#L1-L9) | **NO CHANGED** |
| PodSecurityPolicy  | [topolvm-node](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/psp.yaml#L1-L27) | `{{ template "topolvm.fullname" . }}-node` |
| PodSecurityPolicy  | [topolvm-scheduler](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/psp.yaml#L29-L55) | `{{ template "topolvm.fullname" . }}-scheduler` |
| ServiceAccount     | [topolvm-system](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/scheduler.yaml#L2-L6) | `{{ template "topolvm.fullname" . }}-scheduler` |
| Role               | [psp:topolvm-scheduler](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/scheduler.yaml#L8-L17) | `psp:{{ template "topolvm.fullname" . }}-scheduler` |
| RoleBinding        | [topolvm-scheduler:psp:topolvm-scheduler](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/base/scheduler.yaml#L19-L31) | `{{ template "topolvm.fullname" . }}-scheduler:psp:{{ template "topolvm.fullname" . }}-scheduler` |
| DaemonSet          | [topolvm-scheduler](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/overlays/daemonset-scheduler/scheduler.yaml#L1-L54) | `{{ template "topolvm.fullname" . }}-scheduler` |
| Deployment         | [topolvm-scheduler](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/overlays/deployment-scheduler/scheduler.yaml#L1-L36) | `{{ template "topolvm.fullname" . }}-scheduler` |
| Service            | [topolvm-scheduler](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/overlays/deployment-scheduler/scheduler.yaml#L38-L49) | `{{ template "topolvm.fullname" . }}-scheduler` |
| ServiceAccount     | [lvmd](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/lvmd/lvmd.yaml#L2-L6) | `{{ template "topolvm.fullname" . }}-lvmd` |
| PodSecurityPolicy  | [lvmd](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/lvmd/lvmd.yaml#L8-L30) | `{{ template "topolvm.fullname" . }}-lvmd` |
| Role               | [psp:lvmd](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/lvmd/lvmd.yaml#L32-L41) | `psp:{{ template "topolvm.fullname" . }}-lvmd` |
| RoleBinding        | [lvmd:psp:lvmd](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/lvmd/lvmd.yaml#L43-L55) | `{{ template "topolvm.fullname" . }}-lvmd:psp:{{ template "topolvm.fullname" . }}-lvmd` |
| DaemonSet          | [lvmd](https://github.com/topolvm/topolvm/blob/v0.8.3/deploy/manifests/lvmd/lvmd.yaml#L57-L94) | `{{ template "topolvm.fullname" . }}-lvmd-{{ $lvmdidx }}` |
