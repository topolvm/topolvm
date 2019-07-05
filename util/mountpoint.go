package util

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

// IsMountpoint returns true iff the given directory is a mount point.
// If dir does not exist, an error is returned.
func IsMountpoint(dir string) (bool, error) {
	dir2, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return false, fmt.Errorf("IsMountpoint: dir=%s: err=%v", dir, err)
	}
	absdir, err := filepath.Abs(dir2)
	if err != nil {
		return false, fmt.Errorf("IsMountpoint: dir=%s: err=%v", dir, err)
	}

	data, err := ioutil.ReadFile("/proc/mounts")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if absdir == fields[1] {
			return true, nil
		}
	}
	return false, nil
}
