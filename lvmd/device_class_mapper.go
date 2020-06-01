package lvmd

import "github.com/cybozu-go/topolvm"

// DeviceClass maps between device classes and volume groups.
type DeviceClass struct {
	// Name for the device class name
	Name string `json:"name"`
	// Volume group name for the deice class
	VolumeGroup string `json:"volume-group"`
	// Default is a flag to indicate whether the device class is the default
	Default bool `json:"default"`
	// SpareGB is storage capacity in GiB to be spared
	SpareGB *uint64 `json:"spare-gb"`
}

const defaultSpareGB = 10

// DeviceClassMapper maps between device classes and volume groups.
type DeviceClassMapper struct {
	deviceClasses []*DeviceClass
}

// NewDeviceClassMapper creates a new DeviceClassMapper
func NewDeviceClassMapper(deviceClasses []*DeviceClass) *DeviceClassMapper {
	return &DeviceClassMapper{
		deviceClasses: deviceClasses,
	}
}

func (m DeviceClassMapper) defaultDeviceClass() *DeviceClass {
	for _, dc := range m.deviceClasses {
		if dc.Default {
			return dc
		}
	}
	return nil
}

// DeviceClass returns the device class by its name
func (m DeviceClassMapper) DeviceClass(dcName string) *DeviceClass {
	if dcName == topolvm.DefaultDeviceClassName {
		return m.defaultDeviceClass()
	}
	for _, dc := range m.deviceClasses {
		if dc.Name == dcName {
			return dc
		}
	}
	return nil
}

// DeviceClassWithVGName returns the device class with the volume group name
func (m DeviceClassMapper) DeviceClassWithVGName(vgName string) *DeviceClass {
	for _, dc := range m.deviceClasses {
		if dc.VolumeGroup == vgName {
			return dc
		}
	}
	return nil
}

// GetSpare returns spare in bytes for the device class
func (m DeviceClassMapper) GetSpare(dc string) uint64 {
	deviceClass := m.DeviceClass(dc)
	if deviceClass == nil || deviceClass.SpareGB == nil {
		return defaultSpareGB << 30
	}
	return *deviceClass.SpareGB << 30
}
