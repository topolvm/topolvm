package topolvm

import corev1 "k8s.io/api/core/v1"

// CapacityKeyPrefix is a key prefix of Node annotation that represents VG free space.
const CapacityKeyPrefix = "capacity.topolvm.io"

// CapacityKey is a key of Node annotation that represents VG free space.
func CapacityKey(dc string) string {
	if len(dc) == 0 {
		return CapacityKeyPrefix
	}
	return CapacityKeyPrefix + "/" + dc
}

// CapacityResource is the resource name of topolvm capacity.
const CapacityResource = corev1.ResourceName("topolvm.io/capacity")

// PluginName is the name of the CSI plugin.
const PluginName = "topolvm.io"

// TopologyNodeKey is a key of topology that represents node name.
const TopologyNodeKey = "topology.topolvm.io/node"

// DeviceClassKey is a key of VolumeGroup which the LogicalVolume belongs.
const DeviceClassKey = "topolvm.io/device-class"

// ResizeRequestedAtKey is a key of LogicalVolume that represents the timestamp of the resize request.
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

// EphemeralVolumeDeviceClassKey is the key to obtain device class to which the ephemeral volume belongs.
const EphemeralVolumeDeviceClassKey = "topolvm.io/device-class"

// DefaultSizeGb is the default size in GiB for  volumes (PVC or inline ephemeral volumes) w/o capacity requests.
const DefaultSizeGb = 1

// DefaultSize is DefaultSizeGb in bytes
const DefaultSize = DefaultSizeGb << 30
