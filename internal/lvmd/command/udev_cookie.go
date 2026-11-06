package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// When an LVM command has to be stopped, lvm is usually waiting for a udev
// cookie: a SysV semaphore that the udev rules decrement, through
// "dmsetup udevcomplete", once they have handled the device's event. If
// systemd-udevd killed the worker for that event, that never happens.
//
// Killing lvm then would leave its cookie behind for good, since only the
// waiter deletes it, and a node has a limited number of semaphore sets
// (kernel.sem, 32000 by default). So the cookie our lvm waits for is deleted
// instead, as "dmsetup udevcomplete_all" does: libdm treats the deletion
// (EIDRM) as a completed wait, so lvm finishes and exits on its own. Only if it
// does not exit in time is the process group killed, and then the cookies its
// processes left behind are deleted.

// dmCookieKeyPrefix is how ipcs prints the keys of device-mapper udev cookies,
// which carry DM_COOKIE_MAGIC (0x0D4D) in their upper 16 bits.
const dmCookieKeyPrefix = "0x0d4d"

var (
	// cookieReleaseGrace is how long lvm may take to exit after the udev cookie
	// it waits for is released.
	cookieReleaseGrace = 10 * time.Second

	// hostCommandTimeout bounds each ipcs or ipcrm call.
	hostCommandTimeout = 30 * time.Second

	// hostCommand returns the command line that runs a helper such as ipcs where
	// LVM commands run. Tests replace it.
	hostCommand = defaultHostCommand
)

// defaultHostCommand reuses lvmCommandPrefix without the lvm binary at its
// end, so "nsenter ... /sbin/lvm" becomes "nsenter ... ipcs". A prefix that
// does not end with the lvm binary is used as it is.
func defaultHostCommand(name string, args ...string) []string {
	prefix := lvmCommandPrefix
	if n := len(prefix); n > 0 && strings.HasPrefix(filepath.Base(prefix[n-1]), "lvm") {
		prefix = prefix[:n-1]
	}
	return slices.Concat(prefix, []string{name}, args)
}

// stopProcessGroupOnCancel makes cancellation stop the command's whole process
// group: it releases the udev cookie the group waits for, if any, and kills the
// group if that is not enough.
//
// The default prefix runs lvm through "nsenter -p", and nsenter forks before
// it execs lvm when it enters a PID namespace. exec.CommandContext alone kills
// only nsenter, so lvm keeps running: it still holds the VG lock and the write
// end of our stdout pipe, and commandReadCloser.Close blocks draining stdout.
//
// The kill must be SIGKILL: lvm blocks all other signals while it holds a VG
// lock, and libdm retries the cookie wait on EINTR. The VG lock is a flock, so
// it is released when lvm dies.
func stopProcessGroupOnCancel(ctx context.Context, cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return stopProcessGroup(log.FromContext(ctx), cmd.Process.Pid)
	}
}

func stopProcessGroup(logger logr.Logger, pgid int) error {
	members := processGroupMembers(pgid)
	if len(members) == 0 {
		return killProcessGroup(pgid)
	}

	cookies, err := listUdevCookies()
	if err != nil {
		logger.Error(err, "failed to list udev cookies")
	}
	released := false
	for _, c := range cookies {
		if !members[c.lastPID] || c.waiters == 0 {
			continue
		}
		if err := removeUdevCookie(c); err != nil {
			logger.Error(err, "failed to release the udev cookie of a cancelled LVM command", "cookie", c.key)
			continue
		}
		logger.Info("released the udev cookie a cancelled LVM command was waiting for",
			"cookie", c.key, "pid", c.lastPID)
		released = true
	}
	if released && waitForProcessGroupExit(pgid, cookieReleaseGrace) {
		return nil
	}

	killErr := killProcessGroup(pgid)
	logger.Info("killed a cancelled LVM command", "pgid", pgid)
	removeCookiesLeftBy(logger, members)
	return killErr
}

// removeCookiesLeftBy deletes the udev cookies last used by the given
// processes, which were killed: nobody else deletes them.
func removeCookiesLeftBy(logger logr.Logger, pids map[int]bool) {
	cookies, err := listUdevCookies()
	if err != nil {
		logger.Error(err, "failed to list udev cookies")
		return
	}
	for _, c := range cookies {
		if !pids[c.lastPID] {
			continue
		}
		if err := removeUdevCookie(c); err != nil {
			logger.Error(err, "failed to remove the udev cookie of a killed LVM command", "cookie", c.key)
			continue
		}
		logger.Info("removed the udev cookie of a killed LVM command", "cookie", c.key, "pid", c.lastPID)
	}
}

func killProcessGroup(pgid int) error {
	err := syscall.Kill(-pgid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

// processGroupMembers returns the live processes of a process group. It reads
// /proc, so it finds none on systems without it.
func processGroupMembers(pgid int) map[int]bool {
	members := map[int]bool{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return members
	}
	want := strconv.Itoa(pgid)
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		stat, err := os.ReadFile(filepath.Join("/proc", e.Name(), "stat"))
		if err != nil {
			continue
		}
		// The command name is in parentheses and may contain spaces, so count
		// the fields after the last ')': state, ppid, pgrp, ...
		s := string(stat)
		fields := strings.Fields(s[strings.LastIndexByte(s, ')')+1:])
		if len(fields) >= 3 && fields[2] == want && fields[0] != "Z" {
			members[pid] = true
		}
	}
	return members
}

func waitForProcessGroupExit(pgid int, timeout time.Duration) bool {
	for deadline := time.Now().Add(timeout); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if len(processGroupMembers(pgid)) == 0 {
			return true
		}
	}
	return false
}

// udevCookie is a device-mapper udev cookie as ipcs reports it.
type udevCookie struct {
	key     string // what dmsetup calls the cookie, e.g. 0x0d4df510
	semid   string
	waiters int // processes waiting for the value to reach zero
	lastPID int // process that last changed the value
}

// listUdevCookies returns the device-mapper udev cookies where LVM commands run.
func listUdevCookies() ([]udevCookie, error) {
	out, err := runHostCommand("ipcs", "-s")
	if err != nil {
		return nil, err
	}
	var cookies []udevCookie
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(strings.ToLower(fields[0]), dmCookieKeyPrefix) {
			continue
		}
		c := udevCookie{key: fields[0], semid: fields[1]}
		detail, err := runHostCommand("ipcs", "-s", "-i", c.semid)
		if err != nil {
			// The cookie may have been completed and deleted meanwhile.
			continue
		}
		if err := c.parseSemaphore(detail); err != nil {
			return nil, err
		}
		cookies = append(cookies, c)
	}
	return cookies, nil
}

// parseSemaphore reads the semaphore table of "ipcs -s -i", for example
//
//	semnum     value      ncount     zcount     pid
//	0          1          0          1          4155351
func (c *udevCookie) parseSemaphore(out []byte) error {
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		header := strings.Fields(line)
		if len(header) == 0 || header[0] != "semnum" || i+1 >= len(lines) {
			continue
		}
		values := strings.Fields(lines[i+1])
		if len(values) != len(header) {
			break
		}
		zcount := slices.Index(header, "zcount")
		pid := slices.Index(header, "pid")
		if zcount < 0 || pid < 0 {
			break
		}
		var err1, err2 error
		c.waiters, err1 = strconv.Atoi(values[zcount])
		c.lastPID, err2 = strconv.Atoi(values[pid])
		if err := errors.Join(err1, err2); err != nil {
			return fmt.Errorf("unexpected ipcs output for semaphore %s: %w", c.semid, err)
		}
		return nil
	}
	return fmt.Errorf("unexpected ipcs output for semaphore %s: %q", c.semid, out)
}

// removeUdevCookie deletes a cookie. A process waiting for it carries on as if
// udev had completed it.
func removeUdevCookie(c udevCookie) error {
	_, err := runHostCommand("ipcrm", "-s", c.semid)
	return err
}

func runHostCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostCommandTimeout)
	defer cancel()
	argv := hostCommand(name, args...)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killProcessGroup(cmd.Process.Pid) }
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
	}
	return out, nil
}
