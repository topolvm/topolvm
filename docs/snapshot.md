Snapshot Design
==================



limitation
------------
 - volume expansion is prohibited when snapshots exist
 - user should wait restore pvc's phase be bound then could use it
 - both snapshot and the restored volume are in the same node as the original volume
 - snapshot is in the same volume group as the original volume
 

CSI API Implementations
------------
```
func CreateSnapshot(context.Context, *CreateSnapshotRequest) (*CreateSnapshotResponse, error)
func DeleteSnapshot(context.Context, *DeleteSnapshotRequest) (*DeleteSnapshotResponse, error)
func CreateVolume(ctx context.Context, *CreateVolumeRequest) (*CreateVolumeResponse, error)
```


Create Snapshot
----------
![component diagram](img/snapshot-create.svg)

a volumsnapshot(CR) map a logicalvolume(CR), add original logical volume information to snapshot logical volume.

```yaml
apiVersion: topolvm.cybozu.com/v1
kind: LogicalVolume
metadata:
  name: snapcontent-b083470e-8293-47cc-810d-9561bd1754e6 # volumesnapshotcontent name
  annotations:
    topolvm.cybozu.com/source: df4b5a3e-8285-4554-b964-8aec23339be1 # source-pv.csi.volumeHandle
spec:
  name: snapcontent-b083470e-8293-47cc-810d-9561bd1754e6
  nodeName: 192.168.26.40
  size: 1Gi
```

### logicalvolume service

add api

```
func (s *LogicalVolumeService) CreateSnapshot(ctx context.Context, snapName string, sourceVolume *topolvmv1.LogicalVolume) (string, error)

```

### logicalvolume-controller

check logicalvolume(CR) has annotation(topolvm.cybozu.com/source) or not. if yes, call lvmd rpc api (CreateSnapshot)

### lvmd

add CreateSnapshot RPC api

```

type CreateSnapshotRequest struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
     
    Name        string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`                    // The logical volume name.
    SizeGb      uint64   `protobuf:"varint,2,opt,name=size_gb,json=sizeGb,proto3" json:"size_gb,omitempty"` // Volume size in GiB.
    Tags        []string `protobuf:"bytes,3,rep,name=tags,proto3" json:"tags,omitempty"`                    // Tags to add to the volume during creation
    DeviceClass string   `protobuf:"bytes,4,opt,name=device_class,json=deviceClass,proto3" json:"device_class,omitempty"`
    Source      string   `protobuf:"bytes,5,opt,name=source,json=source,proto3" json:"source,omitempty"` // source lv name
}
```

lvmd will call lvcreate to create snapshot

```shell
 lvcreate --snapshot --permission r --size {SizeGb} --name {name} {source}
```



Delete Snapshot
----------
![component diagram](img/snapshot-delete.svg)

volumesnapshot(CR) delete operation triggers logicalvolume deletion.  


Restore Snapshot
-----------
![component diagram](img/snapshot-restore.svg)


the size and the volume group of restore lv is the same with origin lv.

pvc restore from snapshot, the StorageClass's volumeBindingMode must be WaitForFirstConsumer. make sure restored logicalvolume and snapshot logicalvolume in the same node.
so the pvc controller will check pvc has datasource or not. if source is snapshot, pvc controller will get the node of snapshot and patch the annotation of selected node to pvc.And then wait for the restore done.  

```yaml
apiVersion: topolvm.cybozu.com/v1
kind: LogicalVolume
metadata:
  name: pvc-42041aee-79a2-4184-91aa-5e1df6068b9f # restore pv name
  annotations:
    topolvm.cybozu.com/snapshot: df4b5a3e-8285-4554-b964-8aec23339be1 # snapshotVolID
    topolvm.cybozu.com/volumemode: Filesystem   # Filesystem or Block
    topolvm.cybozu.com/fstype: xfs # xfs or ext4
spec:
  name: pvc-42041aee-79a2-4184-91aa-5e1df6068b9f
  nodeName: 192.168.26.40
  size: 1Gi
```

### lvmd
add api

```
func RestoreLV(ctx context.Context, req *proto.RestoreLVRequest) (*proto.RestoreLVResponse, error) 
```

```
type RestoreLVRequest struct {
    state         protoimpl.MessageState
    sizeCache     protoimpl.SizeCache
    unknownFields protoimpl.UnknownFields
     
    Name        string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`                    // The logical volume name.
    SizeGb      uint64   `protobuf:"varint,2,opt,name=size_gb,json=sizeGb,proto3" json:"size_gb,omitempty"` // Volume size in GiB.
    Tags        []string `protobuf:"bytes,3,rep,name=tags,proto3" json:"tags,omitempty"`                    // Tags to add to the volume during creation
    DeviceClass string   `protobuf:"bytes,4,opt,name=device_class,json=deviceClass,proto3" json:"device_class,omitempty"`
    Snapshot    string   `protobuf:"bytes,5,opt,name=snapshot,json=snapshot,proto3" json:"snapshot,omitempty"` // snapshot lv name
    VolumeMode  string   `protobuf:"bytes,6,opt,name=volumemode,json=volumemode,proto3" json:"volumemode,omitempty"` // source lv is filesystem or raw device
    FsType      string   `protobuf:"bytes,7,opt,name=fstype,json=fstype,proto3" json:"fstype,omitempty"` // if source lv is filesystem, should specify fstype
}
```

lvmd restore operation

```shell
lvcreate -L {SizeGb} --name {name}
dd if=/path/to/snapshot-lv of=/path/to/restore-lv bs=1M conv=fsync
```

if volumemode is filesystem, overwrite the uuid of fs depend on fstype

```shell
# xfs
xfs_admin -U generate /path/to/restore-lv
# ext
tune2fs -U random /path/to/restore-lv
```



