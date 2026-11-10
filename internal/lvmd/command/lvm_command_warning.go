package command

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

// DefaultLVMCommandWarningThreshold is how long an LVM command that changes
// LVM state may run before a warning is logged, unless
// SetLVMCommandWarningThreshold is called.
const DefaultLVMCommandWarningThreshold = 5 * time.Minute

var lvmCommandWarningThreshold = DefaultLVMCommandWarningThreshold

// SetLVMCommandWarningThreshold sets how long an LVM command that changes LVM
// state may run before a warning is logged. The warning is repeated at the
// same interval until the command finishes. Zero or less disables it.
func SetLVMCommandWarningThreshold(threshold time.Duration) {
	lvmCommandWarningThreshold = threshold
}

// warnWhileRunning logs a warning every threshold while cmd runs, until done
// is closed.
//
// Commands that change LVM state are not stopped when they hang, so this
// warning is how an administrator finds them. It names the lvm process and
// its /proc/<pid>/wchan, the kernel function the process waits in.
func warnWhileRunning(logger logr.Logger, cmd *exec.Cmd, threshold time.Duration, done <-chan struct{}) {
	if threshold <= 0 {
		return
	}
	start := time.Now()
	ticker := time.NewTicker(threshold)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			pid := lvmProcess(cmd.Process.Pid)
			logger.Error(nil, "LVM command is still running",
				"args", cmd.Args,
				"pid", pid,
				"wchan", wchan(pid),
				"elapsed", time.Since(start).Round(time.Second).String(),
			)
		}
	}
}

// lvmProcess returns the PID of the lvm process of a command started as pid.
// With the default prefix, pid is nsenter, which forks before it execs lvm, so
// this follows the children of pid down. Like stopOnCancel, finding lvm
// depends on hostPID: true. Where /proc tells nothing, it returns pid.
func lvmProcess(pid int) int {
	for range 10 {
		child := childProcess(pid)
		if child == 0 {
			break
		}
		pid = child
	}
	return pid
}

// childProcess returns a child of pid, or 0 if there is none or /proc does not
// tell.
func childProcess(pid int) int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}
	want := strconv.Itoa(pid)
	for _, e := range entries {
		child, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		// The command name is in parentheses and may contain spaces, so count
		// the fields after the last ')': state, ppid, ...
		s := string(stat)
		fields := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
		if len(fields) >= 2 && fields[1] == want {
			return child
		}
	}
	return 0
}

// wchan returns the kernel function that a process waits in, or "unknown".
func wchan(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "wchan"))
	if err != nil || len(data) == 0 {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}
