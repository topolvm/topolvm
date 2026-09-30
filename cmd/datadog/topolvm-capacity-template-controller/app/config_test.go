package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/topolvm/topolvm/internal/datadog/capacitytemplate"
)

// Validation rules are covered by capacitytemplate.TestConfigValidate; these
// tests cover file parsing, defaulting, and that validation is applied.

func writeConfig(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "capacity-template.yaml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadControllerConfig(t *testing.T) {
	path := writeConfig(t, `capacityTemplates:
- storageClassName: ephemeral-remote-data
  deviceClassName: remote-ssd
  spareGB: 7
- storageClassName: ephemeral-remote-data-xfs
  deviceClassName: remote-ssd
  handler: remote-lvm
`)
	got, err := loadControllerConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	// The first entry omits handler and must default to remote-lvm.
	want := []capacitytemplate.TemplateConfig{
		{
			StorageClass: "ephemeral-remote-data",
			DeviceClass:  "remote-ssd",
			SpareGB:      7,
			Handler:      capacitytemplate.HandlerRemoteLVM,
		},
		{StorageClass: "ephemeral-remote-data-xfs", DeviceClass: "remote-ssd", Handler: capacitytemplate.HandlerRemoteLVM},
	}
	if !reflect.DeepEqual(got.Templates, want) {
		t.Fatalf("loadControllerConfig() = %#v, want %#v", got.Templates, want)
	}
}

func TestLoadControllerConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr string
	}{
		{
			name: "unknown field",
			data: `capacityTemplates:
- storageClass: ephemeral-remote-data
  storageClassName: ephemeral-remote-data
  deviceClassName: remote-ssd
`,
			wantErr: "storageClass",
		},
		{
			name: "validation applied",
			data: `capacityTemplates:
- storageClassName: ephemeral-local-data
  deviceClassName: ssd
  handler: local-lvm
`,
			wantErr: "unsupported",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadControllerConfig(writeConfig(t, tc.data))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("loadControllerConfig() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
