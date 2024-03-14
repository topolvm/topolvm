package driver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ControllerAllocationSettings contains all allocation settings for the controller.
// These will be used to determine the correct sizing for the volumes.
type ControllerAllocationSettings struct {
	Minimum MinimumAllocationSettings `json:"minimum" ,yaml:"minimum"`
}

// MinimumAllocationSettings contains the minimum allocation settings for the controller.
// It contains the default settings and settings for specific device classes.
// The device classes take precedence over the default settings.
type MinimumAllocationSettings struct {
	Default       AllocationSettings            `json:"default" ,yaml:"default"`
	ByDeviceClass map[string]AllocationSettings `json:"deviceClasses" ,yaml:"deviceClasses"`
}

// AllocationSettings contains a set of quantities for the filesystem and block PVCs.
type AllocationSettings struct {
	Filesystem map[string]Quantity `json:"filesystem" ,yaml:"filesystem"`
	Block      Quantity            `json:"block" ,yaml:"block"`
}

type Quantity resource.Quantity

// UnmarshalText implements the encoding.TextUnmarshaler interface.
// It parses a string into a resource.Quantity.
func (s *Quantity) UnmarshalText(data []byte) error {
	quantity, err := resource.ParseQuantity(string(data))
	if err != nil {
		return err
	}
	*s = Quantity(quantity)
	return nil
}

// MinMaxAllocationsFromSettings returns the minimum and maximum allocations based on the settings.
// It uses the required and limit bytes from the CSI Call and the device class and capabilities from the StorageClass.
// It then returns the minimum and maximum allocations in bytes that can be used for that context.
func (settings ControllerAllocationSettings) MinMaxAllocationsFromSettings(
	required, limit int64,
	deviceClass string,
	capabilities []*csi.VolumeCapability,
) (int64, int64) {
	minimumSize := settings.GetMinimumAllocationSize(deviceClass, capabilities)

	if minimumSize.CmpInt64(required) > 0 {
		ctrlLogger.Info("required size is less than minimum size, "+
			"using minimum size as required size", "required", required, "minimum", minimumSize.Value())
		required = minimumSize.Value()
	}

	return required, limit
}

// GetMinimumAllocationSize returns the minimum size to be allocated from the parameters derived from the StorageClass.
// it uses either the filesystem or block key to get the minimum allocated size.
// it determines which key to use based on the capabilities.
// If no key is found or neither capability exists, it returns 0.
// If the value is not a valid quantity, it returns an error.
func (settings ControllerAllocationSettings) GetMinimumAllocationSize(
	deviceClass string,
	capabilities []*csi.VolumeCapability,
) resource.Quantity {
	var quantity resource.Quantity

	for _, capability := range capabilities {
		if capability.GetBlock() != nil {
			if size, ok := settings.Minimum.ByDeviceClass[deviceClass]; ok {
				quantity = resource.Quantity(size.Block)
			} else {
				quantity = resource.Quantity(settings.Minimum.Default.Block)
			}

			break
		}

		if capability.GetMount() != nil {
			size, deviceClassSettingsExist := settings.Minimum.ByDeviceClass[deviceClass]
			if !deviceClassSettingsExist {
				size = settings.Minimum.Default
			}

			quantity = resource.Quantity(size.Filesystem[capability.GetMount().FsType])

			break
		}
	}

	if quantity.Sign() < 0 {
		return resource.MustParse("0")
	}

	return quantity
}
