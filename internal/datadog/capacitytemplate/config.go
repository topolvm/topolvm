package capacitytemplate

import (
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/util/validation"
)

// Config is the static controller configuration. Every template is reconciled
// independently for every NodeGroup and yields at most one CSIStorageCapacity.
type Config struct {
	Templates []TemplateConfig
}

// TemplateConfig binds one TopoLVM StorageClass/device-class pair to the
// capacity handler that knows how that device class is provisioned on a fresh
// node.
type TemplateConfig struct {
	StorageClass string
	DeviceClass  string
	SpareGB      int64
	Handler      string
}

// Validate checks the configuration against the implemented handlers.
func (c Config) Validate() error {
	return c.validate(defaultCapacityHandlers())
}

func (c Config) validate(handlers map[string]CapacityHandler) error {
	if len(c.Templates) == 0 {
		return fmt.Errorf("capacityTemplates must contain at least one entry")
	}
	seenStorageClasses := make(map[string]int, len(c.Templates))
	for i, template := range c.Templates {
		if err := template.validate(handlers); err != nil {
			return fmt.Errorf("capacityTemplates[%d]: %w", i, err)
		}
		// A StorageClass has exactly one device-class parameter and a
		// CSIStorageCapacity is keyed by StorageClass, so one StorageClass can
		// only ever be described by one template.
		if previous, duplicate := seenStorageClasses[template.StorageClass]; duplicate {
			return fmt.Errorf(
				"capacityTemplates[%d].storageClassName %q duplicates capacityTemplates[%d]",
				i, template.StorageClass, previous)
		}
		seenStorageClasses[template.StorageClass] = i
	}
	return nil
}

func (t TemplateConfig) validate(handlers map[string]CapacityHandler) error {
	if errs := validation.IsDNS1123Subdomain(t.StorageClass); len(errs) != 0 {
		return fmt.Errorf("invalid storageClassName %q: %v", t.StorageClass, errs)
	}
	if errs := validation.IsValidLabelValue(t.DeviceClass); t.DeviceClass == "" || len(errs) != 0 {
		return fmt.Errorf("invalid deviceClassName %q: %v", t.DeviceClass, errs)
	}
	if t.SpareGB < 0 || t.SpareGB > math.MaxInt64/gibibyte {
		return fmt.Errorf("spareGB must be between 0 and %d", math.MaxInt64/gibibyte)
	}
	if _, ok := handlers[t.Handler]; !ok {
		return fmt.Errorf("handler %q is unsupported; supported handlers: %q", t.Handler, handlerNames(handlers))
	}
	return nil
}
