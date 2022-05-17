package topolvm

import corev1 "k8s.io/api/core/v1"

// CapacityKeyPrefix is the key prefix of Node annotation that represents VG free space.
const CapacityKeyPrefix = "capacity.topolvm.io/"

// CapacityResource is the resource name of topolvm capacity.
const CapacityResource = corev1.ResourceName("topolvm.io/capacity")

// PluginName is the name of the CSI plugin.
const PluginName = "topolvm.io"

// TopologyNodeKey is the key of topology that represents node name.
const TopologyNodeKey = "topology.topolvm.io/node"

// DeviceClassKey is the key used in CSI volume create requests to specify a device-class.
const DeviceClassKey = "topolvm.io/device-class"

// ResizeRequestedAtKey is the key of LogicalVolume that represents the timestamp of the resize request.
const ResizeRequestedAtKey = "topolvm.io/resize-requested-at"

// LogicalVolumeFinalizer is the name of LogicalVolume finalizer
const LogicalVolumeFinalizer = "topolvm.io/logicalvolume"

// NodeFinalizer is the name of Node finalizer of TopoLVM
const NodeFinalizer = "topolvm.io/node"

// PVCFinalizer is the name of PVC finalizer of TopoLVM
const PVCFinalizer = "topolvm.io/pvc"

// DefaultCSISocket is the default path of the CSI socket file.
const DefaultCSISocket = "/run/topolvm/csi-topolvm.sock"

// DefaultLVMdSocket is the default path of the lvmd socket file.
const DefaultLVMdSocket = "/run/topolvm/lvmd.sock"

// EphemeralVolumeSizeKey is the key used to obtain ephemeral inline volume size
// from the volume context
const EphemeralVolumeSizeKey = "topolvm.io/size"

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
