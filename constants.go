package topolvm

import corev1 "k8s.io/api/core/v1"

// CapacityKeyPrefix is the key prefix of Node annotation that represents VG free space.
const CapacityKeyPrefix = "capacity.topolvm.io/" // DONE TODO podはhookでmutateされるだけなので不要そう

// CapacityResource is the resource name of topolvm capacity.
const CapacityResource = corev1.ResourceName("topolvm.io/capacity")

// PluginName is the name of the CSI plugin.
const PluginName = "topolvm.io" // DONE 対応不要

// TopologyNodeKey is the key of topology that represents node name.
// 1. external-provionerのsegmentはCSINodeの.drivers[].topologyKeyをsegment情報として利用する
// 2. segment情報の更新をキーの1つとしてsegment*StorageClassを単位としてexternal-provisionerはCSICapacityを更新する
// 3. CSINodeの.drivers[].topologyKeyはNodeServiceのGetNodeInfoの結果が保存される
// 4. TopologyNodeKeyは不要CSI Controller/Node内部でのみ利用され、CSIDriverがdriverとして登録される際にtopologyKeyが更新されるため、対応は不要と思われる
const TopologyNodeKey = "topology.topolvm.io/node" // DONE ドライバー内部だけでの利用なので対応不要そう でも一応レガシーのも使えるように対応しておく？ -> 上記理由により対応不要

// DeviceClassKey is the key used in CSI volume create requests to specify a device-class.
const DeviceClassKey = "topolvm.io/device-class" // TODO can not update StorageClass parameters

// ResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
const ResizeRequestedAtKey = "topolvm.io/resize-requested-at" // DONE LogicalVolumeリソースのアノテーションなのでリソースごとアップデートする

// LogicalVolumeFinalizer is the name of LogicalVolume finalizer
const LogicalVolumeFinalizer = "topolvm.io/logicalvolume" // DONE LogicalVolumeリソースのfinalizerなのでリソースごとアップデートする

// NodeFinalizer is the name of Node finalizer of TopoLVM
const NodeFinalizer = "topolvm.io/node" // DONE

// PVCFinalizer is the name of PVC finalizer of TopoLVM
const PVCFinalizer = "topolvm.io/pvc" // DONE

// DefaultCSISocket is the default path of the CSI socket file.
const DefaultCSISocket = "/run/topolvm/csi-topolvm.sock"

// DefaultLVMdSocket is the default path of the lvmd socket file.
const DefaultLVMdSocket = "/run/topolvm/lvmd.sock"

// EphemeralVolumeSizeKey is the key used to obtain ephemeral inline volume size
// from the volume context
const EphemeralVolumeSizeKey = "topolvm.io/size" // DONE これはpodの .spec.volumes[].csi.volumeAttributes["topolvm.io/size"] にサイズを指定するものなのでマイグレーションは不要だが、古いものと新しいものが両方利用できるようにしておく必要がある

// DefaultDeviceClassAnnotationName is the part of annotation name for the default device-class.
const DefaultDeviceClassAnnotationName = "00default"

// DefaultDeviceClassName is the name for the default device-class.
const DefaultDeviceClassName = ""

// DefaultSizeGb is the default size in GiB for  volumes (PVC or inline ephemeral volumes) w/o capacity requests.
const DefaultSizeGb = 1

// DefaultSize is DefaultSizeGb in bytes
const DefaultSize = int64(DefaultSizeGb << 30)

// Label key that indicates The controller/user who created this resource
// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
const CreatedbyLabelKey = "app.kubernetes.io/created-by"

// Label value that indicates The controller/user who created this resource
const CreatedbyLabelValue = "topolvm-controller"

// Legacy Parameters

// LegacyCapacityKeyPrefix is the key prefix of Node annotation that represents VG free space.
const LegacyCapacityKeyPrefix = "capacity.topolvm.cybozu/" // TODO podはhookでmutateされるだけなので不要そう

// LegacyResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
const LegacyResizeRequestedAtKey = "topolvm.io/resize-requested-at"

// LegacyLogicalVolumeFinalizer is the name of LogicalVolume finalizer
const LegacyLogicalVolumeFinalizer = "topolvm.io/logicalvolume"

// LegacyTopologyNodeKey is the key of topology that represents node name.
const LegacyTopologyNodeKey = "topology.topolvm.cybozu.com/node"

// PVCFinalizer is the name of PVC finalizer of TopoLVM
const LegacyPVCFinalizer = "topolvm.cybozu.com/pvc"

// LegacyNodeFinalizer is the name of Node finalizer of TopoLVM
const LegacyNodeFinalizer = "topolvm.cybozu.com/node"

// LegacyEphemeralVolumeSizeKey is the key used to obtain ephemeral inline volume size
// from the volume context
const LegacyEphemeralVolumeSizeKey = "topolvm.cybozu.com/size"
