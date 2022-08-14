package testing

import (
	"os"
	"testing"
)

func DoEnvCheck(t *testing.T) {
	if os.Getenv("TEST_WITH_USE_LEGACY_PLUGIN_NAME") == "" {
		t.Skip("Run test using USE_LEGACY_PLUGIN_NAME env")
	}
}
