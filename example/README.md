Getting Started
===============

## Quick Start with kind

```console
cd example
make setup test
```

You can see Logical Volumes are attached to PV as follows...
```console
kubectl get pvc
kubectl get pv
lvscan
```

Clean up the generated files.
```console
make clean
```

## Step by Step Deployment

(Fig.)

In `example/`

### Components for each Node

The components for each Node are:
1. lvmd
2. lvmetrics
3. topolvm-node
4. topolvm-csi

Except for `lvmd`, you can deploy these components using Kubernetes.
A sample manifest is here and the detailed description for each component is as follows:

#### lvmd

```console
mkdir -p build
mkidr -p /tmp/topolvm
go build -o build/lvmd ../pkg/lvmd
systemd-run --unit=lvmd.service ./build/lvmd --volume-group=myvg --listen=/tmp/topolvm/lvmd.sock --spare=1
```
Now lvmd.service is running and open its API at `/tmp/topolvm/lvmd.sock`

Note: If you do not have any Volume Group, you can use loopback device for testing.
```console
sudo losetup -f build/backing_store
sudo vgcreate -y myvg $(sudo losetup -j build/backing_store | cut -d: -f1)
sudo lvcreate -y -n csi-node-test-block -L 1G myvg
sudo lvcreate -y -n csi-node-test-fs -L 1G myvg
```

#### lvmetrics

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

#### topolvm-csi (mode: node)

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


### Components for Controller

#### topolvm-csi (mode: controller)

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

#### topolvm-hook

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

ToDO: Describe description of `scheduler-config.yaml` and `scheduler-policy.cfg`
`topolvm-scheduler` can run anywhere 

```yaml
```

#### Custom Resource and StorageClass Definition

