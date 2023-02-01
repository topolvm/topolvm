package topolvm

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
)

const (
	pluginName       = "topolvm.io"
	legacyPluginName = "topolvm.cybozu.com"
)

func UseLegacy() bool {
	return os.Getenv("USE_LEGACY") != ""
}

// GetPluginName returns the name of the CSI plugin.
func GetPluginName() string {
	if UseLegacy() {
		return legacyPluginName
	} else {
		return pluginName
	}
}

// GetCapacityKeyPrefix returns the key prefix of Node annotation that represents VG free space.
func GetCapacityKeyPrefix() string {
	return fmt.Sprintf("capacity.%s/", GetPluginName())
}

// GetCapacityResource returns the resource name of topolvm capacity.
func GetCapacityResource() corev1.ResourceName {
	return corev1.ResourceName(fmt.Sprintf("%s/capacity", GetPluginName()))
}

// TopologyNodeKey returns the key of topology that represents node name.
func GetTopologyNodeKey() string {
	return fmt.Sprintf("topology.%s/node", GetPluginName())
}

// GetDeviceClassKey returns the key used in CSI volume create requests to specify a device-class.
func GetDeviceClassKey() string {
	return fmt.Sprintf("%s/device-class", GetPluginName())
}

// GetDeviceClassKey returns the key used in CSI volume create requests to specify a lvcreate-option-class.
func GetLvcreateOptionClassKey() string {
	return fmt.Sprintf("%s/lvcreate-option-class", GetPluginName())
}

// GetResizeRequestedAtKey returns the key of LogicalVolume that represents the timestamp of the resize request.
func GetResizeRequestedAtKey() string {
	return fmt.Sprintf("%s/resize-requested-at", GetPluginName())
}

// GetLogicalVolumeFinalizer returns the name of LogicalVolume finalizer
func GetLogicalVolumeFinalizer() string {
	return fmt.Sprintf("%s/logicalvolume", GetPluginName())
}

// GetNodeFinalizer returns the name of Node finalizer of TopoLVM
func GetNodeFinalizer() string {
	return fmt.Sprintf("%s/node", GetPluginName())
}

// PVCFinalizer is a finalizer of PVC.
const PVCFinalizer = pluginName + "/pvc"

// LegacyPVCFinalizer is a legacy finalizer of PVC.
const LegacyPVCFinalizer = legacyPluginName + "/pvc"

// DefaultCSISocket is the default path of the CSI socket file.
const DefaultCSISocket = "/run/topolvm/csi-topolvm.sock"

// DefaultLVMdSocket is the default path of the lvmd socket file.
const DefaultLVMdSocket = "/run/topolvm/lvmd.sock"

// DefaultDeviceClassAnnotationName is the part of annotation name for the default device-class.
const DefaultDeviceClassAnnotationName = "00default"

// DefaultDeviceClassName is the name for the default device-class.
const DefaultDeviceClassName = ""

// DefaultSizeGb is the default size in GiB for volumes (PVC or generic ephemeral volumes) w/o capacity requests.
const DefaultSizeGb = 1

// DefaultSize is DefaultSizeGb in bytes
const DefaultSize = int64(DefaultSizeGb << 30)

// Label key that indicates The controller/user who created this resource
// https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/#labels
const CreatedbyLabelKey = "app.kubernetes.io/created-by"

// Label value that indicates The controller/user who created this resource
const CreatedbyLabelValue = "topolvm-controller"
