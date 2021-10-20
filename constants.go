package topolvm

import corev1 "k8s.io/api/core/v1"

// CapacityKeyPrefix is the key prefix of Node annotation that represents VG free space.
const CapacityKeyPrefix = "capacity.topolvm.cybozu.com/"

// CapacityResource is the resource name of topolvm capacity.
const CapacityResource = corev1.ResourceName("topolvm.cybozu.com/capacity")

// PluginName is the name of the CSI plugin.
const PluginName = "topolvm.cybozu.com"

// TopologyNodeKey is the key of topology that represents node name.
const TopologyNodeKey = "topology.topolvm.cybozu.com/node"

// DeviceClassKey is the key used in CSI volume create requests to specify a device-class.
const DeviceClassKey = "topolvm.cybozu.com/device-class"

// ResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
const ResizeRequestedAtKey = "topolvm.cybozu.com/resize-requested-at"

// LogicalVolumeFinalizer is the name of LogicalVolume finalizer
const LogicalVolumeFinalizer = "topolvm.cybozu.com/logicalvolume"

// NodeFinalizer is the name of Node finalizer of TopoLVM
const NodeFinalizer = "topolvm.cybozu.com/node"

// PVCFinalizer is the name of PVC finalizer of TopoLVM
const PVCFinalizer = "topolvm.cybozu.com/pvc"

// DefaultCSISocket is the default path of the CSI socket file.
const DefaultCSISocket = "/run/topolvm/csi-topolvm.sock"

// DefaultLVMdSocket is the default path of the lvmd socket file.
const DefaultLVMdSocket = "/run/topolvm/lvmd.sock"

// EphemeralVolumeSizeKey is the key used to obtain ephemeral inline volume size
// from the volume context
const EphemeralVolumeSizeKey = "topolvm.cybozu.com/size"

// DefaultDeviceClassAnnotationName is the part of annotation name for the default device-class.
const DefaultDeviceClassAnnotationName = "00default"

// DefaultDeviceClassName is the name for the default device-class.
const DefaultDeviceClassName = ""

// DefaultSizeGb is the default size in GiB for  volumes (PVC or inline ephemeral volumes) w/o capacity requests.
const DefaultSizeGb = 1

// DefaultSize is DefaultSizeGb in bytes
const DefaultSize = DefaultSizeGb << 30

// Label key that indicates The controller/user who created this resource
// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
const CreatedbyLabelKey = "app.kubernetes.io/created-by"

// Label value that indicates The controller/user who created this resource
const CreatedbyLabelValue = "topolvm-controller"

const LVSnapshotSourceVol = "topolvm.cybozu.com/snapshot-source-volume"
const LVParentID = "topolvm.cybozu.com/parent-id"
const LVVolumeMode = "topolvm.cybozu.com/volume-mode"
const LVVolumeFsType = "topolvm.cybozu.com/fs-type"
