Getting Started
===============

## Quick Start with kind

You can see a demonstration of how TopoLVM provisioner works with the following commands. This demonstration using [kind](https://github.com/kubernetes-sigs/kind) and loopback device on your host.
```console
$ cd example
$ make setup run
```

You can see Logical Volumes are attached to PV as follows:
```console
$ kubectl get pvc
% NAME          STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS          AGE
% topolvm-pvc   Bound    pvc-05df10d2-b7ee-11e9-8da2-0242ac110002   1Gi        RWO            topolvm-provisioner   23m

$ kubectl get pv
% NAME CAPACITY ACCESS MODES RECLAIM POLICY STATUS CLAIM STORAGECLASS REASON AGE
% pvc-05df10d2-b7ee-11e9-8da2-0242ac110002 1Gi RWO Delete Bound topolvm-system/topolvm-pvc topolvm-provisioner 25m

$ sudo lvscan
% ACTIVE '/dev/myvg/05e33db5-b7ee-11e9-8da2-0242ac110002' [1.00 GiB] inherit
```

Clean up the generated files.
```console
$ make clean
```

## Step by Step Deployment

Component diagram including TopoLVM is as follows:

![component diagram](http://www.plantuml.com/plantuml/svg/fLLFRne_5Bplfx389R_3AegFEVmK5NgeH2C4QgeguM3ifyN2Qw_yXxIg-EwrTzVsiWH8rHlxtdZyPkmnZyOIRLqjYeRG7Qa0JMRG2FMh1cadw7U1qCjKIQkL4A0NmbLShX4nQ39TVK6vWrQWzvp2gxobXfTMDKhiw_ycwEO72A7U0eynXZEWH9kE8G0RhVRSS2L1lyfG8DOIkWNjLowut1M7ef2A0NfI3ExRPUslC9hdZ4FFLbrlHg1MSWNzw7xJWEx6lizpX-BrYSDobcQ-pqDhgBYnciGXEwXV3LPp6f7fUqJPxnHKzSY-KeRI4EorU_pERK20xR7zbuVDURMrduI35ZL__ihonYpJ30t4oK1yOY2-QY3-DmFnXmt47pOG_uM1-Bg1-8A1yR8l197GAUaAg0cLFYkauSRR0dezu0yDGxV0d3XfC6B9XXX04x2K9TUdoratornLd1Bnh8IhuTQNHmR70tpAuOWawVYMO9JJD5ot7A0MSZZcmDSvSER0cUCGN8eq1bZxX0JWwMjYe6DOHKFGvvyM90iFm6qyoEJMGEqXRR1LQdTfYvubm8xlHwWS7MpEB2fVRZR2mPgfDrd-ZzeyeGTKBHVJ8vXhVFV8rMAOwCGpeiWnEWk9GK_zk5LQ-701buCMSNbi_9uwVA8ElwCE3zLbdamnKdSM4bDuJjqLO9Q7expn_n8gaS-7fvbg81RklXDBjwF3SKq4jTsxRmtpq976Cw0YtShU9mD5p7ii3QwUnwUPKTaRd_33PdPiB2bwyWYIkLhy0G00)

You can see detailed description of each components [here](../docs/).
Almost all components can deploy via Kubernetes, except for `lvmd` which must be running at hosts where you want to create Logical Volumes.

Sample manifests are located at [example/manifest](./manifest/). The detailed description of these manifests is as follows.

### Custom Resource and Storage Class Definition

TopoLVM uses the custom resource definition of `LogicalVolume` ([manifest](./manifest/crd.yaml)) and storage class `topolvm.cybozu.com` ([manifest](./manifest/provisioner.yaml)).

### Components for each Node

The components for each Node are:
1. lvmd
2. lvmetrics
3. topolvm-node
4. topolvm-csi (mode: node)

#### lvmd

`lvmd` creates/deletes Logical Volume using given Volume Group. You can build and run it with following commands:
```console
mkdir -p build
mkidr -p /tmp/topolvm
go build -o build/lvmd ../pkg/lvmd
systemd-run --unit=lvmd.service ./build/lvmd --volume-group=<volume_group_name> --listen=/tmp/topolvm/lvmd.sock --spare=1
```
Now `lvmd.service` is running and open its API endpoint at Unix Domain Socket `/tmp/topolvm/lvmd.sock`.  Of course, you cau write systemd service definition like [this](./manifest/lvmd.service).

Note: If you do not have any Volume Group, you can use loopback device for testing.
```console
sudo truncate --size=20G build/backing_store
sudo losetup -f build/backing_store
sudo vgcreate -y myvg $(sudo losetup -j build/backing_store | cut -d: -f1)
```

#### lvmetrics

`lvmetrics` gathers metrics from `lvmd` and annotate Kubernetes `Node` with them. So you must give the API endpoint of `lvmd` and which `Node` to annotate.
```yaml
- name: lvmetrics
 image: quay.io/cybozu/topolvm:0.1.0
 command:
 - /lvmetrics
 - --socket=/run/topolvm/lvmd.sock
 env:
 - name: NODE_NAME
 valueFrom:
 fieldRef:
 fieldPath: spec.nodeName
 volumeMounts:
 - name: lvmd-socket-dir
 mountPath: /run/topolvm
```

#### topolvm-node

`topolvm-node` is a Kubernetes custom controller for Custom Resource `LogicalVolume`.  It controls Logical Volume via `lvmd`, so you must give the API endpoint of `lvmd`.
```yaml
- name: topolvm-node
 image: quay.io/cybozu/topolvm:0.1.0
 command:
 - /topolvm-node
 - --node-name=$(MY_NODE_NAME)
 - --lvmd-socket=/run/lvmd/lvmd.sock
 env:
 - name: MY_NODE_NAME
 valueFrom:
 fieldRef:
 fieldPath: spec.nodeName
 volumeMounts:
 - name: lvmd-socket-dir
 mountPath: /run/lvmd
```

#### csi-topolvm (mode: node)

`csi-topolvm` is an Unified CSI Driver for TopoLVM. With option `node`, it works as Node Service. To obtain Logical Volume information, `csi-topolvm node` need the API endpoint of `lvmd`. It also open own API endpoints with given Unix Domain Socket path. This API endpoint is registered to kubelet by registrar container `csi-registrar`, which uses [kubernetes-csi/node-driver-registrar](https://github.com/kubernetes-csi/node-driver-registrar).
```yaml
- name: node
 image: quay.io/cybozu/topolvm:0.1.0
 securityContext:
 privileged: true
 command:
 - /csi-topolvm
 - node
 - --node-name=$(MY_NODE_NAME)
 - --csi-socket-name=/run/topolvm/csi-topolvm.sock
 - --lvmd-socket-name=/run/lvmd/lvmd.sock
 env:
 - name: MY_NODE_NAME
 valueFrom:
 fieldRef:
 fieldPath: spec.nodeName
 volumeMounts:
 - name: node-plugin-dir
 mountPath: /run/topolvm
 - name: lvmd-socket-dir
 mountPath: /run/lvmd
 - name: pod-volumes-dir
 mountPath: /var/lib/kubelet/pods
 mountPropagation: "Bidirectional"
 - name: csi-plugin-dir
 mountPath: /var/lib/kubelet/plugins/kubernetes.io/csi
 mountPropagation: "Bidirectional"

- name: csi-registrar
 image: quay.io/cybozu/topolvm:0.1.0
 command:
 - /csi-node-driver-registrar
 - "--csi-address=/run/topolvm/csi-topolvm.sock"
 - "--kubelet-registration-path=/var/lib/kubelet/plugins/topolvm.cybozu.com/node/csi-topolvm.sock"
 lifecycle:
 preStop:
 exec:
 command: ["/bin/sh", "-c", "rm -rf /registration/topolvm.cybozu.com /registration/topolvm.cybozu.com-reg.sock"]
 volumeMounts:
 - name: node-plugin-dir
 mountPath: /run/topolvm
 - name: registration-dir
 mountPath: /registration
```


### Components for Control Plane

The components for Control Plane are:
1. topolvm-hook
2. topolvm-scheduler
3. topolvm-csi (mode: controller)

#### topolvm-hook

`topolvm-hook` is a Kubernetes mutating admission webhook for TopoLVM. It reads requested resource size in `PersistentVolumeClaim` and add it to spec of containers, which referred by `csi-topolvm controller` to manage Volume Group capacity.

Note that `topolvm-hook` requires TLS certification.  In sample manifests, we uses [cert-manager](https://github.com/jetstack/cert-manager) which can generate TLS certificate and inject CA certificate to Kubernetes API server.  You can also write CA certificate to [APIService manifest](./manifest/mutatingwebhooks.yaml.template) (replace `{{CA_BUNDLE}}` with base64 encoded string of the certificate).
```yaml
- name: topolvm-hook
 image: quay.io/cybozu/topolvm:0.1.0
 command:
 - /topolvm-hook
 - --listen=:9252
 - --cert=/certs/tls.crt
 - --key=/certs/tls.key
 livenessProbe:
 httpGet:
 path: /status
 port: 9252
 scheme: HTTPS
 volumeMounts:
 - name: certs
 mountPath: /certs
```

#### topolvm-scheduler

`topolvm-scheduler` is a Kubernetes scheduler extender for TopoLVM. It filters and prioritizes Nodes based on the amount of free space in their volume groups.  The API endpoint of `topolvm-scheduler` is accessed by Kubernetes API server.  You can deploy it anywhere as long as API server can access it.  The [configuration](./scheduler/scheduler-config.yaml) for TopoLVM must be applied to `kube-scheduler` with `--config` option.

The [sample manifest](./manifest/scheduler.yaml) is an example of deployment using Kubernetes.  It deploys `topolvm-scheduler` as `DaemonSet` and access it via host network.

#### topolvm-csi (mode: controller)

`topolvm-csi` with `controller` option is Controller Service of CSI Driver. It opens the API Endpoint with Unix Domain Socket and receives requirements for Logical Volume from `csi-provisioner` and `csi-attacher` containers, which use [kubernetes-csi/external-provisioner](https://github.com/kubernetes-csi/external-provisioner) and [kubernetes-csi/external-attacher](https://github.com/kubernetes-csi/external-attacher).
```yaml
- name: controller
 image: quay.io/cybozu/topolvm:0.1.0
 command:
 - /csi-topolvm
 - controller
 - --csi-socket-name=/run/topolvm/csi-topolvm.sock
 volumeMounts:
 - name: socket-dir
 mountPath: /run/topolvm

- name: csi-provisioner
 image: quay.io/cybozu/topolvm:0.1.0
 command:
 - /csi-provisioner
 - "--csi-address=/run/topolvm/csi-topolvm.sock"
 - "--feature-gates=Topology=true"
 volumeMounts:
 - name: socket-dir
 mountPath: /run/topolvm

- name: csi-attacher
 image: quay.io/cybozu/topolvm:0.1.0
 command:
 - /csi-attacher
 - "--csi-address=/run/topolvm/csi-topolvm.sock"
 volumeMounts:
 - name: socket-dir
 mountPath: /run/topolvm
```
