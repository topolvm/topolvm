package types

type DeviceType string

const (
	TypeThin  = DeviceType("thin")
	TypeThick = DeviceType("thick")
)

// ThinPoolConfig holds the configuration of thin pool in a volume group
type ThinPoolConfig struct {
	// Name of thinpool
	Name string `json:"name"`
	// OverprovisionRatio signifies the upper bound multiplier for allowing logical volume creation in this pool
	OverprovisionRatio *float64 `json:"overprovision-ratio"`
}

// DeviceClass maps between device-classes and target for logical volume creation
// current targets are VolumeGroup for thick-lv and ThinPool for thin-lv
type DeviceClass struct {
	// Name for the device-class name
	Name string `json:"name"`
	// Volume group name for the device-class
	VolumeGroup string `json:"volume-group"`
	// Default is a flag to indicate whether the device-class is the default
	Default bool `json:"default"`
	// SpareGB is storage capacity in GiB to be spared
	SpareGB *uint64 `json:"spare-gb"`
	// Stripe is the number of stripes in the logical volume
	Stripe *uint `json:"stripe"`
	// StripeSize is the amount of data that is written to one device before moving to the next device
	StripeSize string `json:"stripe-size"`
	// LVCreateOptions are extra arguments to pass to lvcreate
	LVCreateOptions []string `json:"lvcreate-options"`
	// Type is the name of logical volume target, supports 'thick' (default) or 'thin' currently
	Type DeviceType `json:"type"`
	// ThinPoolConfig holds the configuration for thinpool in this volume group corresponding to the device-class
	ThinPoolConfig *ThinPoolConfig `json:"thin-pool"`
}

type LvcreateOptionClass struct {
	// Name for the lvcreate-option-class name
	Name string `json:"name"`
	// Options are extra arguments to pass to lvcreate
	Options []string `json:"options"`
}
