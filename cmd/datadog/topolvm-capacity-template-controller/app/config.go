package app

import (
	"fmt"
	"os"

	"github.com/topolvm/topolvm/internal/datadog/capacitytemplate"
	"sigs.k8s.io/yaml"
)

type fileConfig struct {
	CapacityTemplates []fileCapacityTemplate `json:"capacityTemplates"`
}

type fileCapacityTemplate struct {
	StorageClassName string `json:"storageClassName"`
	DeviceClassName  string `json:"deviceClassName"`
	SpareGB          int64  `json:"spareGB,omitempty"`
	Handler          string `json:"handler,omitempty"`
}

func loadControllerConfig(path string) (capacitytemplate.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return capacitytemplate.Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}
	config := fileConfig{}
	if err := yaml.UnmarshalStrict(data, &config); err != nil {
		return capacitytemplate.Config{}, fmt.Errorf("parse config file %q: %w", path, err)
	}
	return convertFileConfig(config)
}

// convertFileConfig maps every file entry to a template, applies defaults, and
// validates the full list against the implemented capacity handlers.
func convertFileConfig(config fileConfig) (capacitytemplate.Config, error) {
	converted := capacitytemplate.Config{
		Templates: make([]capacitytemplate.TemplateConfig, 0, len(config.CapacityTemplates)),
	}
	for _, template := range config.CapacityTemplates {
		handler := template.Handler
		if handler == "" {
			handler = capacitytemplate.HandlerRemoteLVM
		}
		converted.Templates = append(converted.Templates, capacitytemplate.TemplateConfig{
			StorageClass: template.StorageClassName,
			DeviceClass:  template.DeviceClassName,
			SpareGB:      template.SpareGB,
			Handler:      handler,
		})
	}
	if err := converted.Validate(); err != nil {
		return capacitytemplate.Config{}, err
	}
	return converted, nil
}
