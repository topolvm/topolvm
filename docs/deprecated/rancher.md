How to use TopoLVM with Rancher/RKE
===================================

This document is a brief introduction of how to use TopoLVM on [Rancher/RKE](https://rancher.com/docs/rke/latest/en/).

Rancher/RKE will be deployed on the following 4 instances of Google Compute Engine (GCE).

| Hostname  | Machine Type    | Role              | Requirement      |
| --------- | --------------- | ----------------- | ---------------- |
| `rancher` | `n1-standard-2` | Rancher Server    | Allow HTTP/HTTPS |
| `master`  | `n1-standard-2` | Kubernetes Master |                  |
| `worker1` | `n1-standard-2` | Kubernetes Worker | Mount 1 SSD      |
| `worker2` | `n1-standard-2` | Kubernetes Worker | Mount 1 SSD      |

If the `gcloud` command is not installed on your PC, please refer to [this document](https://cloud.google.com/sdk/docs/quickstarts) and install Google Cloud SDK beforehand.

## 1. Run Rancher Server

### Create GCE instance

Create a GCE instance for Rancher Server. This document uses the `asia-northeast1-c` zone, but you can choose any other zone you want.

```bash
ZONE=asia-northeast1-c
gcloud compute instances create rancher \
  --zone ${ZONE} \
  --machine-type n1-standard-2 \
  --image-project ubuntu-os-cloud \
  --image-family ubuntu-1804-lts \
  --boot-disk-size 200GB
```

Then, allow HTTP/HTTPS with the following commands.

1. Go to `VM instances` on the GCE dashboard and open the configuration page of `rancher`
2. Click `EDIT` at the top of the page
3. Enable `Allow HTTP traffic` and `Allow HTTPS traffic` under `Firewalls`
4. Click `Save` at the bottom of the page

### Install Docker

Run the installation script.

```bash
gcloud compute ssh --zone ${ZONE} rancher -- "curl -sSLf https://get.docker.com | sudo sh"
```

### Start Rancher

```bash
gcloud compute ssh --zone ${ZONE} rancher -- "sudo docker run -d --restart=unless-stopped -p 80:80 -p 443:443 rancher/rancher:v2.3.4"
```

Go to the external IP address of `rancher` which appears on the GCE dashboard with your favorite browser.

For simplicity, TLS certification is not prepared in this example.
So, just allow insecure access and proceed next.

## 2. Deploy Kubernetes cluster

### Create GCE instances for Master & Worker Nodes

Create `master`, `worker1` and `worker2`.
`worker1` and `worker2` mounts SSD at `/dev/nvme0` to provision TopoLVM volumes.

```bash
gcloud compute instances create master \
  --zone ${ZONE} \
  --machine-type n1-standard-2 \
  --image-project ubuntu-os-cloud \
  --image-family ubuntu-1804-lts \
  --boot-disk-size 200GB

gcloud compute instances create worker1 \
  --zone ${ZONE} \
  --machine-type n1-standard-2 \
  --local-ssd interface=nvme \
  --image-project ubuntu-os-cloud \
  --image-family ubuntu-1804-lts

gcloud compute instances create worker2 \
  --zone ${ZONE} \
  --machine-type n1-standard-2 \
  --local-ssd interface=nvme \
  --image-project ubuntu-os-cloud \
  --image-family ubuntu-1804-lts
```

### Install Docker

Run the installation script.

```bash
gcloud compute ssh --zone ${ZONE} master -- "curl -sSLf https://get.docker.com | sudo sh"
gcloud compute ssh --zone ${ZONE} worker1 -- "curl -sSLf https://get.docker.com | sudo sh"
gcloud compute ssh --zone ${ZONE} worker2 -- "curl -sSLf https://get.docker.com | sudo sh"
```

### Deploy Kubernetes components with Rancher

Go to the Rancher dashboard and click `Add Cluster` -> `From existing nodes (Custom)`
to see the configuration page.  Overwrite some default values as follows.

- Cluster Name: Write your cluster name
- Cluster Options:
  - Kubernetes Version: `v1.16.4-rancher1-1`
  - Node Options:
    - Master:
      - Check `Control Plane` and `etcd`
      - Run the commands which will be displyed on the screen
    - Worker:
      - Check `Worker`
      - Run the commands which will be displyed on the screen

After finishing the configuration, click `Done` and wait for the cluster status to become `Active`.

## 3. Deploy cert-manager

You can run the `kubectl` command by downloading `Kubeconfig File` from the top right of the cluster dashboard.

Then, deploy cert-manager on the Kubernetes cluster.

```bash
kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.12.0/cert-manager.yaml
```

Add a label to `Namespace` resources for the TopoLVM webhook to avoid unnecessary validation.

```bash
kubectl label namespace kube-system topolvm.cybozu.com/webhook=ignore
kubectl label namespace cert-manager topolvm.cybozu.com/webhook=ignore
```

## 4. Install `lvmd`

Create VG (VolumeGroup) on `worker1` and `worker2`.

```bash
gcloud compute ssh --zone ${ZONE} worker1 -- sudo vgcreate myvg1 /dev/nvme0n1
gcloud compute ssh --zone ${ZONE} worker2 -- sudo vgcreate myvg1 /dev/nvme0n1
```

Install `lvmd` on `worker1` and `worker2`.

```bash
gcloud compute ssh --zone ${ZONE} worker1

# Install lvmd
TOPOLVM_VERSION=0.6.0
sudo mkdir -p /opt/sbin
curl -sSLf https://github.com/topolvm/topolvm/releases/download/v${TOPOLVM_VERSION}/lvmd-${TOPOLVM_VERSION}.tar.gz | sudo tar xzf - -C /opt/sbin

# Put configuration file
sudo mkdir -p /etc/topolvm
sudo curl -sSL -o /etc/topolvm/lvmd.yaml https://raw.githubusercontent.com/topolvm/topolvm/v${TOPOLVM_VERSION}/deploy/lvmd-config/lvmd.yaml

# Register service
sudo curl -sSL -o /etc/systemd/system/lvmd.service https://raw.githubusercontent.com/topolvm/topolvm/v${TOPOLVM_VERSION}/deploy/systemd/lvmd.service
sudo systemctl enable lvmd
sudo systemctl start lvmd
exit
```

```bash
gcloud compute ssh --zone ${ZONE} worker2

# Install lvmd
TOPOLVM_VERSION=0.6.0
sudo mkdir -p /opt/sbin
curl -sSLf https://github.com/topolvm/topolvm/releases/download/v${TOPOLVM_VERSION}/lvmd-${TOPOLVM_VERSION}.tar.gz | sudo tar xzf - -C /opt/sbin

# Put configuration file
sudo mkdir -p /etc/topolvm
sudo curl -sSL -o /etc/topolvm/lvmd.yaml https://raw.githubusercontent.com/topolvm/topolvm/v${TOPOLVM_VERSION}/deploy/lvmd-config/lvmd.yaml

# Register service
sudo curl -sSL -o /etc/systemd/system/lvmd.service https://raw.githubusercontent.com/topolvm/topolvm/v${TOPOLVM_VERSION}/deploy/systemd/lvmd.service
sudo systemctl enable lvmd
sudo systemctl start lvmd
exit
```

## 5. Deploy TopoLVM

Before deploying TopoLVM, install `kustomize` by following the link below.
https://kubernetes-sigs.github.io/kustomize/installation/

```bash
TOPOLVM_VERSION=0.6.0
kustomize build https://github.com/topolvm/topolvm/deploy/manifests/overlays/daemonset-scheduler?ref=v${TOPOLVM_VERSION} | kubectl apply -f -
kubectl apply -f https://raw.githubusercontent.com/topolvm/topolvm/v${TOPOLVM_VERSION}/deploy/manifests/base/certificates.yaml
```

## 6. Configure `topolvm-scheduler`

###  Update `topolvm-scheduler` manifest

First, `master` has the following label and taint.

- Label
    1. `node-role.kubernetes.io/controlplane=true`
    2. `node-role.kubernetes.io/etcd=true`
- Taints
    1. `node-role.kubernetes.io/controlplane=true:NoSchedule`
    2. `node-role.kubernetes.io/etcd=true:NoExecute`

To locate `topolvm-scheduler` onto `master`, update node affinity and toleration.

```bash
$ kubectl edit daemonset topolvm-scheduler -n topolvm-system
# Edit as follows
 apiVersion: apps/v1
 kind: DaemonSet
...
 spec:
...
   template:
...
     spec:
       affinity:
         nodeAffinity:
           requiredDuringSchedulingIgnoredDuringExecution:
             nodeSelectorTerms:
             - matchExpressions:
-              - key: node-role.kubernetes.io/master
+              - key: node-role.kubernetes.io/controlplane
                 operator: Exists
...
       tolerations:
       - key: CriticalAddonsOnly
         operator: Exists
-      - effect: NoSchedule
-        key: node-role.kubernetes.io/master
+      - key: node-role.kubernetes.io/controlplane
+        operator: Exists
+      - key: node-role.kubernetes.io/etcd
+        operator: Exists
...
```

### Adding the scheduler extender

Download the scheduler extender configuration files on the `master` instance.

They must be placed under `/etc/kubernetes` on `master` because `kube-scheduler`, deployed with Rancher, is configured to mount the directory.

```bash
gcloud compute ssh --zone ${ZONE} master

TOPOLVM_VERSION=0.6.0
sudo mkdir -p /etc/kubernetes/scheduler
sudo curl -sSL -o /etc/kubernetes/scheduler/scheduler-policy.cfg https://raw.githubusercontent.com/topolvm/topolvm/v${TOPOLVM_VERSION}/deploy/scheduler-config/scheduler-policy.cfg
sudo curl -sSL -o /etc/kubernetes/scheduler/scheduler-config.yaml https://raw.githubusercontent.com/topolvm/topolvm/v${TOPOLVM_VERSION}/docs/rancher/scheduler-config.yaml
exit
```

On the Rancher dashboard, click `Edit` and update `Cluster Options` with `Edit as YAML` to tell Kubernetes where the scheduler extension configuration is.

```yaml
   services:
     ...
     kube-api:
       always_pull_images: false
       pod_security_policy: false
       service_node_port_range: 30000-32767
     kube-controller: {}
     kubelet:
       fail_swap_on: false
       generate_serving_certificate: false
     kubeproxy: {}
# Add extra_args
-    scheduler: {}
+    scheduler:
+      extra_args:
+        config: /etc/kubernetes/scheduler/scheduler-config.yaml
   ssh_agent_auth: false
```

Then click `Save` to finish the configuration.

## 7. Provision volume with TopoLVM

Congratulations!! You finally deployed TopoLVM on RKE.

To confirm TopoLVM is working, create PVC and mount it on a `Pod`.

```yaml
kubectl apply -f - << EOF
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: topolvm-pvc
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner
---
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  labels:
    app.kubernetes.io/name: my-pod
spec:
  containers:
  - name: ubuntu
    image: quay.io/cybozu/ubuntu:18.04
    command: ["/usr/local/bin/pause"]
    volumeMounts:
    - mountPath: /test1
      name: my-volume
  volumes:
    - name: my-volume
      persistentVolumeClaim:
        claimName: topolvm-pvc
EOF
```
## 8. Cleanup

Do not forget to delete GCE instances.

```bash
gcloud --quiet compute instances delete rancher --zone ${ZONE}
gcloud --quiet compute instances delete master --zone ${ZONE}
gcloud --quiet compute instances delete worker1 --zone ${ZONE}
gcloud --quiet compute instances delete worker2 --zone ${ZONE}
```
