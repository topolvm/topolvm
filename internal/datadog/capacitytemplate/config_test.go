package capacitytemplate

import (
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	withTemplate := func(mutate func(*TemplateConfig)) Config {
		template := remoteXFSTemplate
		mutate(&template)
		return Config{Templates: []TemplateConfig{remoteTemplate, template}}
	}
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{name: "multiple templates", config: multiConfig},
		{name: "empty", config: Config{}, wantErr: "at least one"},
		{
			name:    "invalid storage class",
			config:  withTemplate(func(t *TemplateConfig) { t.StorageClass = "Bad_Value" }),
			wantErr: "capacityTemplates[1]: invalid storageClassName",
		},
		{
			name:    "missing device class",
			config:  withTemplate(func(t *TemplateConfig) { t.DeviceClass = "" }),
			wantErr: "capacityTemplates[1]: invalid deviceClassName",
		},
		{
			name:    "negative spare",
			config:  withTemplate(func(t *TemplateConfig) { t.SpareGB = -1 }),
			wantErr: "capacityTemplates[1]: spareGB",
		},
		{
			name:    "unsupported handler",
			config:  withTemplate(func(t *TemplateConfig) { t.Handler = "local-lvm" }),
			wantErr: `capacityTemplates[1]: handler "local-lvm" is unsupported`,
		},
		{
			name:    "duplicate storage class",
			config:  withTemplate(func(t *TemplateConfig) { t.StorageClass = remoteTemplate.StorageClass }),
			wantErr: "capacityTemplates[1].storageClassName \"ephemeral-remote-data\" duplicates capacityTemplates[0]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
