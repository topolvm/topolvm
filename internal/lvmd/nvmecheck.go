package lvmd

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/topolvm/topolvm/internal/lvmd/command"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Result values printed by the NVMe VUC check tool on stdout (format
// "<device>: <N>").
const (
	// nvmeCheckUnsupported means the device does not support the feature.
	nvmeCheckUnsupported = 0
	// nvmeCheckSupported means the device supports the feature and the
	// operation may proceed.
	nvmeCheckSupported = 1

	// nvmeCheckMaxAttempts is how many times a device may be checked while it
	// keeps yielding a non-supported result before the checker gives up and
	// stops invoking the tool (serving the cached failure instead). Restart
	// lvmd to reset and re-check.
	nvmeCheckMaxAttempts = 3
)

// nvmeCheckEntry is the cached per-device state.
type nvmeCheckEntry struct {
	// supported is set once the device passed the check; further operations
	// skip the tool entirely.
	supported bool
	// attempts counts consecutive non-supported checks so far.
	attempts int
	// final is set after attempts reached nvmeCheckMaxAttempts; the tool is no
	// longer invoked and finalErr is returned directly.
	final    bool
	finalErr error
}

// nvmeChecker runs the external NVMe support-check tool per device and caches
// the outcome:
//   - a supported device is cached and never re-checked (until lvmd restart);
//   - a device that keeps failing is re-checked up to maxAttempts times (once
//     per volume operation, i.e. spaced by the external-provisioner's retry
//     backoff); after that it is marked final and the tool is no longer run.
type nvmeChecker struct {
	cmd         []string
	maxAttempts int
	// run performs a single check for one device; nil error means supported.
	// It is a field so tests can substitute a fake.
	run func(ctx context.Context, cmd []string, device string) error

	mu    sync.Mutex
	cache map[string]*nvmeCheckEntry
}

// newNVMeChecker returns a checker for the given command, or nil if the check
// is disabled (empty command).
func newNVMeChecker(cmd []string) *nvmeChecker {
	if len(cmd) == 0 {
		return nil
	}
	return &nvmeChecker{
		cmd:         cmd,
		maxAttempts: nvmeCheckMaxAttempts,
		run:         runNVMeCheckDevice,
		cache:       map[string]*nvmeCheckEntry{},
	}
}

// check verifies a single device, applying the cache and retry policy.
func (c *nvmeChecker) check(ctx context.Context, device string) error {
	logger := log.FromContext(ctx)

	c.mu.Lock()
	entry := c.cache[device]
	if entry == nil {
		entry = &nvmeCheckEntry{}
		c.cache[device] = entry
	}
	switch {
	case entry.supported:
		c.mu.Unlock()
		return nil
	case entry.final:
		err := entry.finalErr
		c.mu.Unlock()
		return err
	}
	c.mu.Unlock()

	err := c.run(ctx, c.cmd, device)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err == nil {
		entry.supported = true
		entry.attempts = 0
		logger.Info("NVMe check passed; caching device as supported", "device", device)
		return nil
	}

	entry.attempts++
	if c.maxAttempts > 0 && entry.attempts >= c.maxAttempts {
		entry.final = true
		entry.finalErr = status.Errorf(status.Code(err),
			"%s (giving up after %d failed NVMe checks; restart lvmd to re-check)",
			status.Convert(err).Message(), entry.attempts)
		logger.Error(err, "NVMe check failed; giving up and blocking this device until lvmd restarts",
			"device", device, "attempts", entry.attempts, "max_attempts", c.maxAttempts)
		return entry.finalErr
	}

	logger.Error(err, "NVMe check failed; will retry on the next volume operation",
		"device", device, "attempts", entry.attempts, "max_attempts", c.maxAttempts)
	return err
}

// runNVMeCheckDevice runs `cmd[0] cmd[1:]... <name>` once and parses the
// trailing integer N from the tool's stdout (format "<name>: <N>").
//
// The device name is passed WITHOUT the leading "/dev/" (the Sails tool rejects
// a "/dev/"-prefixed argument), e.g. "/dev/nvme0n1" is passed as "nvme0n1".
//
//	N == 1 (supported)                               -> nil
//	N == 0 (unsupported)                             -> codes.FailedPrecondition
//	any other N / unparseable stdout / spawn failure -> codes.Internal
func runNVMeCheckDevice(ctx context.Context, cmd []string, device string) error {
	logger := log.FromContext(ctx)

	name := strings.TrimPrefix(device, "/dev/")
	args := append(append([]string{}, cmd[1:]...), name)
	out, err := exec.CommandContext(ctx, cmd[0], args...).Output()
	if err != nil {
		// The tool reports its verdict on stdout, so a non-zero exit does not
		// necessarily mean failure. Only bail out here if we could not run the
		// tool at all or it produced no stdout to parse.
		stderr := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr = string(exitErr.Stderr)
		}
		if len(out) == 0 {
			logger.Error(err, "failed to run NVMe check command", "command", cmd, "device", device, "stderr", stderr)
			return status.Errorf(codes.Internal, "failed to run NVMe check command %v for device %s: %v", cmd, device, err)
		}
		logger.Info("NVMe check command exited non-zero but produced output; parsing stdout",
			"command", cmd, "device", device, "stderr", stderr, "error", err.Error())
	}

	result, perr := parseNVMeCheckOutput(out)
	if perr != nil {
		logger.Error(perr, "failed to parse NVMe check output", "command", cmd, "device", device, "output", string(out))
		return status.Errorf(codes.Internal, "failed to parse NVMe check output for device %s: %v (output: %q)", device, perr, string(out))
	}

	switch result {
	case nvmeCheckSupported:
		return nil
	case nvmeCheckUnsupported:
		return status.Errorf(codes.FailedPrecondition, "NVMe device %s does not support this feature", device)
	default:
		return status.Errorf(codes.Internal, "NVMe check command returned unexpected result %d for device %s", result, device)
	}
}

// parseNVMeCheckOutput extracts the trailing integer from the tool's stdout.
// The output is expected to look like "<device>: <N>"; the integer after the
// last colon on the last non-empty line is returned.
func parseNVMeCheckOutput(out []byte) (int, error) {
	var lastLine string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" {
			lastLine = line
		}
	}
	if lastLine == "" {
		return 0, status.Error(codes.Internal, "empty output")
	}

	field := lastLine
	if idx := strings.LastIndex(lastLine, ":"); idx >= 0 {
		field = lastLine[idx+1:]
	}
	field = strings.TrimSpace(field)

	n, err := strconv.Atoi(field)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// checkNVMe resolves the physical volume device paths backing the given volume
// group and runs the NVMe VUC check against them. It is a no-op when the check
// is not configured.
func (s *lvService) checkNVMe(ctx context.Context, vgName string) error {
	if s.nvmeChecker == nil {
		return nil
	}
	vg, err := command.FindVolumeGroup(ctx, vgName)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to find volume group %s for NVMe check: %v", vgName, err)
	}
	devicePaths, err := vg.ListPhysicalVolumes(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to list physical volumes for volume group %s: %v", vgName, err)
	}
	if len(devicePaths) == 0 {
		return status.Errorf(codes.Internal, "NVMe check: no physical volume device found for volume group %s", vgName)
	}
	for _, dev := range devicePaths {
		if err := s.nvmeChecker.check(ctx, dev); err != nil {
			return err
		}
	}
	return nil
}
