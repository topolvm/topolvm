package testutils

import (
	"os"
	"testing"
)

const (
	envSkipTestsUsingRoot = "SKIP_TESTS_USING_ROOT"
)

func RequireRoot(t *testing.T) {
	t.Helper()

	if os.Getuid() == 0 {
		return
	}
	if os.Getenv(envSkipTestsUsingRoot) == "1" {
		t.Skipf("this test requires root but %s is set to 1", envSkipTestsUsingRoot)
	}
	t.Fatalf("run as root or set environment variable %s to 1", envSkipTestsUsingRoot)
}
