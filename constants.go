package topolvm

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
)

const pluginName = "topolvm.io"
const legacyPluginName = "topolvm.cybozu.com"

// GetPluginName return the name of the CSI plugin.
func GetPluginName() string {
	if os.Getenv("USE_LEGACY_PLUGIN_NAME") == "" {
		return pluginName
	} else {
		return legacyPluginName
	}
}

// GetCapacityKeyPrefix return the key prefix of Node annotation that represents VG free space.
func GetCapacityKeyPrefix() string {
	return fmt.Sprintf("capacity.%s/", GetPluginName())
}

// GetCapacityResource return the resource name of topolvm capacity.
func GetCapacityResource() corev1.ResourceName {
	return corev1.ResourceName(fmt.Sprintf("%s/capacity", GetPluginName()))
}

func GetTopologyNodeKey() string {
	return fmt.Sprintf("topology.%s/node", GetPluginName())
}

// GetDeviceClassKey return the key used in CSI volume create requests to specify a device-class.
func GetDeviceClassKey() string {
	return fmt.Sprintf("%s/device-class", GetPluginName())
}

// GetResizeRequestedAtKey return the key of LogicalVolume that represents the timestamp of the resize request.
func GetResizeRequestedAtKey() string {
	return fmt.Sprintf("%s/resize-requested-at", GetPluginName())
}

// GetLogicalVolumeFinalizer return the name of LogicalVolume finalizer
func GetLogicalVolumeFinalizer() string {
	return fmt.Sprintf("%s/logicalvolume", GetPluginName())
}

// GetNodeFinalizer return the name of Node finalizer of TopoLVM
func GetNodeFinalizer() string {
	return fmt.Sprintf("%s/node", GetPluginName())
}

// GetPVCFinalizer return the name of PVC finalizer of TopoLVM
func GetPVCFinalizer() string {
	return fmt.Sprintf("%s/pvc", GetPluginName())
}

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
