package lvmd

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/topolvm/topolvm"
)

// ErrNotFound is returned when a VG or LV is not found.
var ErrNotFound = errors.New("device-class not found")

const defaultSpareGB = 10

// This regexp is based on the following validation:
//   https://github.com/kubernetes/apimachinery/blob/v0.18.3/pkg/util/validation/validation.go#L42
var qualifiedNameRegexp = regexp.MustCompile("^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$")

// DeviceClass maps between device-classes and volume groups.
type DeviceClass struct {
	// Name for the device-class name
	Name string `json:"name"`
	// Volume group name for the deice class
	VolumeGroup string `json:"volume-group"`
	// Default is a flag to indicate whether the device-class is the default
	Default bool `json:"default"`
	// SpareGB is storage capacity in GiB to be spared
	SpareGB *uint64 `json:"spare-gb"`
	// Stripe is the number of stripes in the logical volume
	Stripe *uint `json:"stripe"`
}

// GetSpare returns spare in bytes for the device-class
func (c DeviceClass) GetSpare() uint64 {
	if c.SpareGB == nil {
		return defaultSpareGB << 30
	}
	return *c.SpareGB << 30
}

// ValidateDeviceClasses validates device-classes
func ValidateDeviceClasses(deviceClasses []*DeviceClass) error {
	if len(deviceClasses) < 1 {
		return errors.New("should have at least one device-class")
	}
	var countDefault = 0
	dcNames := make(map[string]bool)
	vgNames := make(map[string]bool)
	for _, dc := range deviceClasses {
		if len(dc.Name) == 0 {
			return errors.New("device-class name should not be empty")
		} else if len(dc.Name) > 63 {
			return fmt.Errorf("device-class name is too long: %s", dc.Name)
		}
		if !qualifiedNameRegexp.MatchString(dc.Name) {
			return fmt.Errorf("device-class name should consist of alphanumeric characters, '-', '_' or '.', and should start and end with an alphanumeric character: %s", dc.Name)
		}
		if len(dc.VolumeGroup) == 0 {
			return fmt.Errorf("volume group name should not be empty: %s", dc.Name)
		}
		if dc.Default {
			countDefault++
		}
		if dcNames[dc.Name] {
			return fmt.Errorf("duplicate device-class name: %s", dc.Name)
		}
		if vgNames[dc.VolumeGroup] {
			return fmt.Errorf("duplicate volume group name: %s, %s", dc.Name, dc.VolumeGroup)
		}
		dcNames[dc.Name] = true
		vgNames[dc.VolumeGroup] = true
	}
	if countDefault != 1 {
		return errors.New("should have only one default device-class")
	}
	return nil
}

// DeviceClassManager maps between device-classes and volume groups.
type DeviceClassManager struct {
	defaultDeviceClass  *DeviceClass
	deviceClassByName   map[string]*DeviceClass
	deviceClassByVGName map[string]*DeviceClass
}

// NewDeviceClassManager creates a new DeviceClassManager
func NewDeviceClassManager(deviceClasses []*DeviceClass) *DeviceClassManager {
	dcm := DeviceClassManager{}
	dcm.deviceClassByName = make(map[string]*DeviceClass)
	dcm.deviceClassByVGName = make(map[string]*DeviceClass)
	for _, dc := range deviceClasses {
		if dc.Default {
			dcm.defaultDeviceClass = dc
		}
		dcm.deviceClassByName[dc.Name] = dc
		dcm.deviceClassByVGName[dc.VolumeGroup] = dc
	}
	return &dcm
}

// DeviceClass returns the device-class by its name
func (m DeviceClassManager) DeviceClass(dcName string) (*DeviceClass, error) {
	if dcName == topolvm.DefaultDeviceClassName {
		return m.defaultDeviceClass, nil
	}
	if v, ok := m.deviceClassByName[dcName]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}

// FindDeviceClassByVGName returns the device-class with the volume group name
func (m DeviceClassManager) FindDeviceClassByVGName(vgName string) (*DeviceClass, error) {
	if v, ok := m.deviceClassByVGName[vgName]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}
