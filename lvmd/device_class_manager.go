package lvmd

import (
	"errors"
	"regexp"
)

// ErrNotFound is returned when a VG or LV is not found.
var ErrNotFound = errors.New("device class not found")

const defaultSpareGB = 10

var qualifiedNameRegexp = regexp.MustCompile("^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$")
var lvmNameRegexp = regexp.MustCompile("^([A-Za-z0-9_.][-A-Za-z0-9_.]*)?$")

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

// GetSpare returns spare in bytes for the device class
func (c DeviceClass) GetSpare() uint64 {
	if c.SpareGB == nil {
		return defaultSpareGB << 30
	}
	return *c.SpareGB << 30
}

// ValidateDeviceClasses validates device classes
func ValidateDeviceClasses(deviceClasses []*DeviceClass) error {
	if len(deviceClasses) < 1 {
		return errors.New("should have at least one device class")
	}
	var countDefault = 0
	for _, dc := range deviceClasses {
		if len(dc.Name) == 0 {
			return errors.New("device class name should not be empty")
		} else if len(dc.Name) > 63 {
			return errors.New("device class name is too long")
		}
		if !qualifiedNameRegexp.MatchString(dc.Name) {
			return errors.New("device class name should consist of alphanumeric characters, '-', '_' or '.', and should start and end with an alphanumeric character")
		}
		if len(dc.VolumeGroup) == 0 {
			return errors.New("volume group name should not be empty")
		} else if len(dc.VolumeGroup) > 126 {
			return errors.New("volume group name is too long")
		}
		if !lvmNameRegexp.MatchString(dc.VolumeGroup) {
			return errors.New("volume group name should consist of alphanumeric characters, '-', '_' or '.', and should not start with '-'")
		}
		if dc.Default {
			countDefault++
		}
	}
	if countDefault != 1 {
		return errors.New("should have only one default device class")
	}
	return nil
}

// DeviceClassManager maps between device classes and volume groups.
type DeviceClassManager struct {
	deviceClasses []*DeviceClass
}

// NewDeviceClassManager creates a new DeviceClassManager
func NewDeviceClassManager(deviceClasses []*DeviceClass) *DeviceClassManager {
	return &DeviceClassManager{
		deviceClasses: deviceClasses,
	}
}

func (m DeviceClassManager) defaultDeviceClass() *DeviceClass {
	for _, dc := range m.deviceClasses {
		if dc.Default {
			return dc
		}
	}
	return nil
}

// DeviceClass returns the device class by its name
func (m DeviceClassManager) DeviceClass(dcName string) (*DeviceClass, error) {
	if dcName == "" {
		return m.defaultDeviceClass(), nil
	}
	for _, dc := range m.deviceClasses {
		if dc.Name == dcName {
			return dc, nil
		}
	}
	return nil, ErrNotFound
}

// FindDeviceClassByVGName returns the device class with the volume group name
func (m DeviceClassManager) FindDeviceClassByVGName(vgName string) (*DeviceClass, error) {
	for _, dc := range m.deviceClasses {
		if dc.VolumeGroup == vgName {
			return dc, nil
		}
	}
	return nil, ErrNotFound
}
