package lvmd

type DeviceClass struct {
	Name        string `json:"name"`
	VolumeGroup string `json:"volume-group"`
	Default     bool   `json:"default"`
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

func (m DeviceClassMapper) DeviceClass(dcName string) *DeviceClass {
	if len(dcName) == 0 {
		return m.defaultDeviceClass()
	}
	for _, dc := range m.deviceClasses {
		if dc.Name == dcName {
			return dc
		}
	}
	return nil
}

func (m DeviceClassMapper) DeviceClassFrom(vgName string) *DeviceClass {
	for _, dc := range m.deviceClasses {
		if dc.VolumeGroup == vgName {
			return dc
		}
	}
	return nil
}

func (m DeviceClassMapper) GetSpare(dc string) uint64 {
	deviceClass := m.DeviceClass(dc)
	if deviceClass == nil || deviceClass.SpareGB == nil {
		return defaultSpareGB << 30
	}
	return *deviceClass.SpareGB << 30
}
