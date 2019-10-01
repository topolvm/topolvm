package topolvm

import corev1 "k8s.io/api/core/v1"

// CapacityKey is a key of Node annotation that represents VG free space.
const CapacityKey = "topolvm.cybozu.com/capacity"

// CapacityResource is the resource name of topolvm capacity.
const CapacityResource = corev1.ResourceName("topolvm.cybozu.com/capacity")

// PluginName is the name of the CSI plugin.
const PluginName = "topolvm.cybozu.com"

// TopologyNodeKey is a key of topology that represents node name.
const TopologyNodeKey = "topology.topolvm.cybozu.com/node"

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
