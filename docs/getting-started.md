# Getting Started

We provide a [Helm Chart](../charts/topolvm/) to install TopoLVM on Kubernetes.
The chart depends on [cert-manager](https://cert-manager.io/). If you don't have cert-manager installed, you can install it with the Helm Chart.

## Prerequisites

A volume group must be created on all nodes where TopoLVM will run.
The default volume group name defined in the Helm Chart is `myvg1`.

## Install Instructions

First, add the TopoLVM helm repository.

```sh
helm repo add topolvm https://topolvm.github.io/topolvm
helm repo update
```

Then, install TopoLVM with the release name `topolvm`.

```sh
helm install --namespace=topolvm-system topolvm topolvm/topolvm
```

> [!NOTE]
> If you'd like to install TopoLVM in a namespace other than `topolvm-system`, you
> must set its name in the `.webhook.{pvc,pod}MutatingWebhook.ignoreNamespaces`
> field in values.yaml.  By doing so, TopoLVM's mutating webhooks, which depend on
> topolvm-controller in the installed namespace, won't be used when a Pod or a PVC
> is created in the same namespace, and TopoLVM's startup process won't get stuck
> due to a circular dependency.

If you want to install cert-manager together, use the following command instead.

```sh
CERT_MANAGER_VERSION=v1.17.4
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.crds.yaml

helm install --namespace=topolvm-system topolvm topolvm/topolvm --set cert-manager.enabled=true
```

Finally, check if TopoLVM is running. All TopoLVM pods should be running.

```sh
kubectl get pod -n topolvm-system
```

## Usage

You can create PersistentVolumes (PV) by TopoLVM after installation succeeded.
The StorageClass `topolvm-provisioner` is automatically created by the Helm Chart.

```sh
kubectl apply -f - <<EOF
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: my-pvc
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
spec:
  containers:
  - name: pause
    image: registry.k8s.io/pause:3.9
    volumeMounts:
    - mountPath: /data
      name: volume
  volumes:
  - name: volume
    persistentVolumeClaim:
      claimName: my-pvc
EOF
```
