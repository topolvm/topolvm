package lvmd

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/topolvm/topolvm"
	lvmdTypes "github.com/topolvm/topolvm/pkg/lvmd/types"
)

// ErrDeviceClassNotFound is returned when a VG or LV is not found.
var ErrDeviceClassNotFound = errors.New("device-class not found")

const (
	defaultSpareGB = 10
)

// This regexp is based on the following validation:
//
//	https://github.com/kubernetes/apimachinery/blob/v0.18.3/pkg/util/validation/validation.go#L42
var qualifiedNameRegexp = regexp.MustCompile("^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$")

// This regexp is used to check StripeSize format
var stripeSizeRegexp = regexp.MustCompile("(?i)^([0-9]*)(k|m|g|t|p|e|b|s)?$")

// GetSpare returns spare in bytes for the device-class
func GetSpare(dc *lvmdTypes.DeviceClass) uint64 {
	if dc.SpareGB == nil {
		return defaultSpareGB << 30
	}
	return *dc.SpareGB << 30
}

// ValidateDeviceClasses validates device-classes
func ValidateDeviceClasses(deviceClasses []*lvmdTypes.DeviceClass) error {
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

		// validate Type of the device-class
		switch dc.Type {
		case "", lvmdTypes.TypeThick, lvmdTypes.TypeThin:
		default:
			return fmt.Errorf("target 'type' of device-class can be one of '%[1]s' or '%[2]s' or empty to default to '%[1]s'", lvmdTypes.TypeThick, lvmdTypes.TypeThin)
		}

		name := dc.VolumeGroup

		// thinpool validation, ignore any thinpoolconfig if Type is not TypeThin
		if dc.Type == lvmdTypes.TypeThin {

			if dc.ThinPoolConfig == nil {
				return fmt.Errorf("device class type is thin but thinpool config is empty: %s", dc.Name)
			}

			if len(dc.ThinPoolConfig.Name) == 0 {
				return fmt.Errorf("thinpool name should not be empty: %s", dc.Name)
			}

			if dc.ThinPoolConfig.OverprovisionRatio < 1.0 {
				return fmt.Errorf("overprovision ratio for thin pool %s in device class %s should be greater than 1.0", dc.ThinPoolConfig.Name, dc.Name)
			}
			// combination of volumegroup and thinpool should be unique across device classes
			// so the key 'name' shouldn't appear twice to verify it's uniqueness
			name = name + "/" + dc.ThinPoolConfig.Name
		}

		if vgNames[name] {
			return fmt.Errorf("duplicate volumegroup/thinpool name: %s, %s", dc.Name, name)
		}

		dcNames[dc.Name] = true
		vgNames[name] = true
		if dc.StripeSize != "" && !stripeSizeRegexp.MatchString(dc.StripeSize) {
			return fmt.Errorf("stripe-size format is \"Size[k|UNIT]\": %s", dc.Name)
		}
	}
	if countDefault > 1 {
		return errors.New("should not have multiple default device-class")
	}
	return nil
}

// DeviceClassManager maps between device-classes and volume groups.
type DeviceClassManager struct {
	defaultDeviceClass        *lvmdTypes.DeviceClass
	deviceClassByName         map[string]*lvmdTypes.DeviceClass
	deviceClassByVGName       map[string]*lvmdTypes.DeviceClass
	deviceClassByThinPoolName map[string]*lvmdTypes.DeviceClass
}

// NewDeviceClassManager creates a new DeviceClassManager
func NewDeviceClassManager(deviceClasses []*lvmdTypes.DeviceClass) *DeviceClassManager {
	dcm := DeviceClassManager{}
	dcm.deviceClassByName = make(map[string]*lvmdTypes.DeviceClass)
	dcm.deviceClassByVGName = make(map[string]*lvmdTypes.DeviceClass)
	dcm.deviceClassByThinPoolName = make(map[string]*lvmdTypes.DeviceClass)
	for _, dc := range deviceClasses {
		if dc.Default {
			dcm.defaultDeviceClass = dc
		}
		dcm.deviceClassByName[dc.Name] = dc

		// device-class has two targets and at a time it can only be in one of
		// "deviceClassByVGName" or "deviceClassByThinPoolName" maps
		switch dc.Type {
		case "", lvmdTypes.TypeThick:
			// device-class target is volumegroup and any logical volume referring to
			// this device-class will have thick logical volumes
			dc.Type = lvmdTypes.TypeThick
			dcm.deviceClassByVGName[dc.VolumeGroup] = dc
		case lvmdTypes.TypeThin:
			// we can't store pool name alone as there can be of thinpool with same name
			// but on a different vg, so combination of vg and thinpool should be unique
			dcm.deviceClassByThinPoolName[dc.VolumeGroup+"/"+dc.ThinPoolConfig.Name] = dc
		}
	}
	return &dcm
}

// DeviceClass returns the device-class by its name
func (m DeviceClassManager) DeviceClass(dcName string) (*lvmdTypes.DeviceClass, error) {
	if dcName == topolvm.DefaultDeviceClassName && m.defaultDeviceClass != nil {
		return m.defaultDeviceClass, nil
	}
	if v, ok := m.deviceClassByName[dcName]; ok {
		return v, nil
	}
	return nil, ErrDeviceClassNotFound
}

// FindDeviceClassByVGName returns the device-class with the volume group name
func (m DeviceClassManager) FindDeviceClassByVGName(vgName string) (*lvmdTypes.DeviceClass, error) {
	if v, ok := m.deviceClassByVGName[vgName]; ok {
		return v, nil
	}
	return nil, ErrDeviceClassNotFound
}

// FindDeviceClassByThinPoolName returns the device-class with volume group and pool combination
func (m DeviceClassManager) FindDeviceClassByThinPoolName(vgName string, poolName string) (*lvmdTypes.DeviceClass, error) {
	name := vgName + "/" + poolName
	if v, ok := m.deviceClassByThinPoolName[name]; ok {
		return v, nil
	}
	return nil, ErrDeviceClassNotFound
}
