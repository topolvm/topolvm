# Snapshot and Restore

See also [the proposal of the functionality](https://github.com/topolvm/topolvm/blob/main/docs/proposals/thin-snapshots-restore.md).

## Getting Started

### Prerequisites

You need to create a thin pool of LVM beforehand, because TopoLVM doesn't create one. For example:
```
lvcreate -T -n pool0 -L 4G myvg1
```

You also need to install the CRDs and the controller for volume snapshots. Please follow [the official document](https://github.com/kubernetes-csi/external-snapshotter#usage) to install them.

### Set up a Device Class

Change your lvmd settings to use the thin pool you've created. For example, if you are using the Helm charts, modify your values.yaml as follows:
```yaml
lvmd:
  deviceClasses:
    # ...
    # Other device classes are here.
    # ...
    - name: "thin"
      volume-group: "myvg1"
      type: thin
      thin-pool:
        name: "pool0"
        overprovision-ratio: 5.0
```

### Set up a Storage Class

Create a storage class for the DeviceClass for the thin pool. For example, if you are using the Helm charts, modify your values.yaml as follows:
```yaml
storageClasses:
  # ...
  # Other storage classes are here.
  # ...
  - name: topolvm-provisioner-thin
    storageClass:
      fsType: xfs
      isDefaultClass: false
      volumeBindingMode: WaitForFirstConsumer
      allowVolumeExpansion: true
      additionalParameters:
        '{{ include "topolvm.pluginName" . }}/device-class': "thin"
```

### Deploy TopoLVM

Deploy TopoLVM with the settings you updated above. See also the [Getting Started](https://github.com/topolvm/topolvm/blob/main/docs/getting-started.md) guide.

Run the following command to deploy `my-pvc` and `my-pod`, which use `topolvm-provisioner-thin`.

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
  storageClassName: topolvm-provisioner-thin
---
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - name: pause
    image: ubuntu:22.04
    command:
    - bash
    - -c
    - |
      sleep infinity
    volumeMounts:
    - mountPath: /data
      name: volume
  volumes:
  - name: volume
    persistentVolumeClaim:
      claimName: my-pvc
EOF
```

Write some data to the volume:
```sh
$ kubectl exec -it my-pod -- bash
root@my-pod:/# echo hello > /data/world
root@my-pod:/# ls /data/world
/data/world
root@my-pod:/# cat /data/world
hello
```

### Take a Snapshot

Before taking snapshots, you need to prepare `VolumeSnapshotClass` as follows:

```sh
kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: topolvm-provisioner-thin
driver: topolvm.io
deletionPolicy: Delete
EOF
```

Then run the following command to take a snapshot:

```sh
kubectl apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-snapshot
spec:
  volumeSnapshotClassName: topolvm-provisioner-thin
  source:
    persistentVolumeClaimName: my-pvc
EOF
```

### Restore a PV from the Snapshot

Run the following command to restore a PV from the snapshot taken above:

```sh
kubectl apply -f - <<EOF
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: my-pvc2
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: topolvm-provisioner-thin
  dataSource:
    name: my-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
---
apiVersion: v1
kind: Pod
metadata:
  name: my-pod2
spec:
  containers:
  - name: pause
    image: ubuntu:22.04
    command:
    - bash
    - -c
    - |
      sleep infinity
    volumeMounts:
    - mountPath: /data
      name: volume
  volumes:
  - name: volume
    persistentVolumeClaim:
      claimName: my-pvc2
EOF
```

And check the content of the volume:
```sh
$ kubectl exec -it my-pod2 -- cat /data/world
hello
```

## See Also

- [The proposal of the functionality](https://github.com/topolvm/topolvm/blob/main/docs/proposals/thin-snapshots-restore.md)
- Issue [#827](https://github.com/topolvm/topolvm/issues/827)
