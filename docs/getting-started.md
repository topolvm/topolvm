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

TopoLVM uses webhooks. To work webhooks properly, add a label to the target namespace. We also recommend to use a dedicated namespace.

```sh
kubectl label namespace topolvm-system topolvm.io/webhook=ignore
kubectl label namespace kube-system topolvm.io/webhook=ignore
```

Then, install TopoLVM with the release name `topolvm`.

```sh
helm install --namespace=topolvm-system topolvm topolvm/topolvm
```

If you want to install cert-manager together, use the following command instead.

```sh
kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/${VERSION}/cert-manager.crds.yaml

helm install --namespace=topolvm-system topolvm topolvm/topolvm --set cert-manager.enabled=true
```

Finally, check if TopoLVM is running. All TopoLVM pods should be running.

```sh
kubectl get pod -n topolvm-system
```

## Configure node startup taint

There are potential race conditions on node startup (especially when a node is first joining the cluster) where pods/processes that rely on the a CSI Driver can act on a node before the CSI Driver is able to startup up and become fully ready. To combat this, the TopoLVM contains a feature to automatically remove a taint from the node on startup. Users can taint their nodes when they join the cluster and/or on startup, to prevent other pods from running and/or being scheduled on the node prior to the CSI Driver becoming ready.

This feature is activated by default, and cluster administrators should use the taint `topolvm.io/agent-not-ready:NoExecute` (any effect will work, but `NoExecute` is recommended).

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
    image: registry.k8s.io/pause
    volumeMounts:
    - mountPath: /data
      name: volume
  volumes:
  - name: volume
    persistentVolumeClaim:
      claimName: my-pvc
EOF
```
