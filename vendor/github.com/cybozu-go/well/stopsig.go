// +build !windows

package well

import (
	"os"
	"syscall"
)

var stopSignals = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
