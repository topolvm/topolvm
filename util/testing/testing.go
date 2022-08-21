package testing

import (
	"os"
	"testing"
)

func DoEnvCheck(t *testing.T) {
	if os.Getenv("TEST_LEGACY") == "" {
		t.Skip("Run test using TEST_LEGACY env")
	}
}
