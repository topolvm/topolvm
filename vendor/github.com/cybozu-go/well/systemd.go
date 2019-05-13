package well

import (
	"bufio"
	"os"
	"runtime"
	"strings"
)

// IsSystemdService returns true if the program runs as a systemd service.
func IsSystemdService() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// https://www.freedesktop.org/software/systemd/man/systemd.exec.html#%24JOURNAL_STREAM
	if len(os.Getenv("JOURNAL_STREAM")) > 0 {
		return true
	}

	f, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	isService := false
	for sc.Scan() {
		fields := strings.Split(sc.Text(), ":")
		if len(fields) < 3 {
			continue
		}
		if fields[1] != "name=systemd" {
			continue
		}
		isService = strings.HasSuffix(fields[2], ".service")
		break
	}
	if err := sc.Err(); err != nil {
		return false
	}

	return isService
}
