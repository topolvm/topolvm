package topolvm

import corev1 "k8s.io/api/core/v1"

// CapacityKey is a key of Node annotation that represents VG free space.
const CapacityKey = "capacity.topolvm.io/"

// CapacityResource is the resource name of topolvm capacity.
func CapacityResource(vgName string) corev1.ResourceName {
	return corev1.ResourceName(CapacityKey + vgName)
}

// PluginName is the name of the CSI plugin.
const PluginName = "topolvm.cybozu.com"

// TopologyNodeKey is a key of topology that represents node name.
const TopologyNodeKey = "topology.topolvm.cybozu.com/node"

// VolumeGroupKey is a key of VolumeGroup which the LogicalVolume belongs.
const VolumeGroupKey = "topolvm.io/volume-group"

// ResizeRequestedAtKey is a key of LogicalVolume that represents the timestamp of the resize request.
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

// SizeVolConKey is the key used to obtain ephemeral inline volume size
// from the volume context
const SizeVolConKey = "topolvm.cybozu.com/size"

// DefaultSizeGb is the default size in GiB for  volumes (PVC or inline ephemeral volumes) w/o capacity requests.
const DefaultSizeGb = 1

// DefaultSize is DefaultSizeGb in bytes
const DefaultSize = DefaultSizeGb << 30
