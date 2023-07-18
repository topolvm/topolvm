package topolvm

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	testingutil "github.com/topolvm/topolvm/util/testing"
)

func TestUseLegacy(t *testing.T) {
	testingutil.DoEnvCheck(t)
	tests := []struct {
		name     string
		envval   string
		expected bool
	}{
		{
			name:     "return false if USE_LEGACY env value is empty",
			envval:   "",
			expected: false,
		},
		{
			name:     "return true if USE_LEGACY env value is not empty",
			envval:   "true",
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("USE_LEGACY", tt.envval)
			if UseLegacy() != tt.expected {
				t.Fatalf("return value is not %s", strconv.FormatBool(tt.expected))
			}
		})
	}
}

func TestGetPluginName(t *testing.T) {
	testingutil.DoEnvCheck(t)
	tests := []struct {
		name     string
		envval   string
		expected string
	}{
		{
			name:     fmt.Sprintf("return %q if USE_LEGACY env value is empty", pluginName),
			envval:   "",
			expected: pluginName,
		},
		{
			name:     fmt.Sprintf("return %q if USE_LEGACY env value is not empty", legacyPluginName),
			envval:   "true",
			expected: legacyPluginName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("USE_LEGACY", tt.envval)
			if GetPluginName() != tt.expected {
				t.Fatalf("return value is not %s", tt.expected)
			}
		})
	}
}

func TestGetCapacityKeyPrefix(t *testing.T) {
	testingutil.DoEnvCheck(t)
	doContainTest(t, GetCapacityKeyPrefix)
}

func TestGetCapacityResource(t *testing.T) {
	testingutil.DoEnvCheck(t)
	doContainTest(t, func() string {
		return string(GetCapacityResource())
	})
}

func TestGetTopologyNodeKey(t *testing.T) {
	testingutil.DoEnvCheck(t)
	doContainTest(t, GetTopologyNodeKey)
}

func TestGetDeviceClassKey(t *testing.T) {
	testingutil.DoEnvCheck(t)
	doContainTest(t, GetDeviceClassKey)
}

func TestGetResizeRequestedAtKey(t *testing.T) {
	testingutil.DoEnvCheck(t)
	doContainTest(t, GetResizeRequestedAtKey)
}

func TestGetLVPendingDeletionKey(t *testing.T) {
	testingutil.DoEnvCheck(t)
	doContainTest(t, GetLVPendingDeletionKey)
}

func TestGetLogicalVolumeFinalizer(t *testing.T) {
	testingutil.DoEnvCheck(t)
	doContainTest(t, GetLogicalVolumeFinalizer)
}

func TestGetNodeFinalizer(t *testing.T) {
	testingutil.DoEnvCheck(t)
	doContainTest(t, GetNodeFinalizer)
}

func doContainTest(t *testing.T, f func() string) {
	tests := []struct {
		name      string
		envval    string
		contained string
	}{
		{
			name:      fmt.Sprintf("return strings containing %q if USE_LEGACY env value is empty", pluginName),
			envval:    "",
			contained: pluginName,
		},
		{
			name:      fmt.Sprintf("return strings containing %q if USE_LEGACY env value is not empty", legacyPluginName),
			envval:    "true",
			contained: legacyPluginName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("USE_LEGACY", tt.envval)
			val := f()
			if !strings.Contains(val, tt.contained) {
				t.Fatalf("return value %q does not contain strings: %s", val, tt.contained)
			}
		})
	}
}
