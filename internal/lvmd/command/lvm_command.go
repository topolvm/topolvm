package command

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"syscall"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	verbosityLVMStateUpdate   = 1
	verbosityLVMStateNoUpdate = 4
)

var (
	lvmCommandPrefix = []string{"/usr/bin/nsenter", "-m", "-u", "-i", "-n", "-p", "-t", "1", "/sbin/lvm"}
)

// SetLVMPath sets the path to the lvm command. Note that this function must not
// be called together with SetLVMCommandPrefix.
func SetLVMPath(path string) {
	if path != "" {
		lvmCommandPrefix[len(lvmCommandPrefix)-1] = path
	}
}

// SetLVMCommandPrefix sets a list of strings necessary to run a LVM command.
// For example, if it's X, `/sbin/lvm lvcreate ...` will be run as `X /sbin/lvm
// lvcreate ...`.  This function must not be called together with SetLVMPath.
func SetLVMCommandPrefix(prefix []string) {
	lvmCommandPrefix = prefix
}

// callLVM calls lvm sub-commands and prints the output to the log.
func callLVM(ctx context.Context, args ...string) error {
	return callLVMInto(ctx, nil, verbosityLVMStateUpdate, args...)
}

// callLVMInto calls lvm sub-commands and decodes the output via JSON into the provided struct pointer.
// if the struct pointer is nil, the output will be printed to the log instead.
func callLVMInto(ctx context.Context, into any, logVerbosity int, args ...string) error {
	output, err := callLVMStreamed(ctx, logVerbosity, args...)
	if err != nil {
		return fmt.Errorf("failed to execute command: %v", err)
	}

	// if we don't decode the output into a struct, we can still log the command results from stdout.
	if into == nil {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			log.FromContext(ctx).Info(strings.TrimSpace(scanner.Text()))
		}
		err = scanner.Err()
	} else {
		err = json.NewDecoder(output).Decode(&into)
	}
	closeErr := output.Close()

	return errors.Join(closeErr, err)
}

// callLVMStreamed calls lvm sub-commands and returns the output as a ReadCloser.
// The caller is responsible for closing the ReadCloser, which will cause the command to complete.
// Not calling close on this method will result in a resource leak.
func callLVMStreamed(ctx context.Context, logVerbosity int, args ...string) (io.ReadCloser, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithCallDepth(1))
	wholeCommand := slices.Concat(lvmCommandPrefix, args)
	// Use CommandContext so kubelet timing out on an RPC (default 2 min
	// csiTimeout) cancels the underlying vgs/lvs/... subprocess instead
	// of leaving it running on the node. exec.Command ignores ctx and
	// accumulates orphans under repeated timeout scenarios (#1163).
	cmd := exec.CommandContext(ctx, wholeCommand[0], wholeCommand[1:]...)
	killProcessGroupOnCancel(cmd)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	return runCommand(ctx, logVerbosity, cmd)
}

// killProcessGroupOnCancel makes ctx cancellation kill the command's whole
// process group instead of only the direct child.
//
// The default prefix runs lvm through "nsenter -p", and nsenter forks before
// it execs lvm when it enters a PID namespace. exec.CommandContext alone kills
// only nsenter, so lvm keeps running: it still holds the VG lock and the write
// end of our stdout pipe, and commandReadCloser.Close blocks draining stdout.
// That is what happens when lvcreate waits forever for a udev cookie that a
// killed udev worker never completed.
//
// The signal must be SIGKILL: lvm blocks all other signals while it holds a
// VG lock, and libdm retries the cookie wait on EINTR. The VG lock is a
// flock, so it is released when lvm dies.
func killProcessGroupOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
}

// runCommand runs the command and returns the stdout as a ReadCloser that also Waits for the command to finish.
// After the Close command is called the cmd is closed and the resources are released.
// Not calling close on this method will result in a resource leak.
func runCommand(ctx context.Context, logVerbosity int, cmd *exec.Cmd) (io.ReadCloser, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, err
	}

	log.FromContext(ctx).V(logVerbosity).Info("invoking command", "args", cmd.Args)
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, err
	}
	// Return a read closer that will wait for the command to finish when closed to release all resources.
	return commandReadCloser{cmd: cmd, ReadCloser: stdout, stderr: stderr}, nil
}

// commandReadCloser is a ReadCloser that calls the Wait function of the command when Close is called.
// This is used to wait for the command the pipe before waiting for the command to finish.
type commandReadCloser struct {
	cmd *exec.Cmd
	io.ReadCloser
	stderr io.ReadCloser
}

// Close drains any unread stdout, reads stderr, and waits for the command to exit.
func (p commandReadCloser) Close() error {
	// Drain stdout before Wait() to prevent blocking when the caller stopped
	// reading early and the child has more output than the pipe buffer.
	_, _ = io.Copy(io.Discard, p.ReadCloser)

	// Read the stderr output after the read has finished since we are sure by then the command must have run.
	stderr, err := io.ReadAll(p.stderr)
	if err != nil {
		return err
	}

	if err := p.cmd.Wait(); err != nil {
		// wait can result in an exit code error
		return &lvmErr{
			err:    err,
			stderr: stderr,
		}
	}
	return nil
}
