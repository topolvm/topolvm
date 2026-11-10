package command

import (
	"bufio"
	"bytes"
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
	"time"

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

// readCommands are the LVM commands that only read LVM state. Stopping one has
// no side effects, so it is stopped when its context ends; see stopOnCancel.
// The other commands change LVM state and are not stopped.
var readCommands = map[string]bool{
	"fullreport": true,
	"lvs":        true,
	"vgs":        true,
}

// readCommandWaitDelay bounds how long Wait waits, after the context of a read
// command ended, for the killed command to exit and close its output.
var readCommandWaitDelay = 10 * time.Second

// callLVMStreamed calls lvm sub-commands and returns the output as a ReadCloser.
// The caller is responsible for closing the ReadCloser, which will cause the command to complete.
// Not calling close on this method will result in a resource leak.
func callLVMStreamed(ctx context.Context, logVerbosity int, args ...string) (io.ReadCloser, error) {
	logger := log.FromContext(ctx)
	ctx = log.IntoContext(ctx, logger.WithCallDepth(1))
	wholeCommand := slices.Concat(lvmCommandPrefix, args)
	read := len(args) > 0 && readCommands[args[0]]

	var cmd *exec.Cmd
	if read {
		// Use CommandContext so kubelet timing out on an RPC (default 2 min
		// csiTimeout) cancels the underlying vgs/lvs/... subprocess instead
		// of leaving it running on the node. exec.Command ignores ctx and
		// accumulates orphans under repeated timeout scenarios (#1163).
		cmd = exec.CommandContext(ctx, wholeCommand[0], wholeCommand[1:]...)
		stopOnCancel(cmd)
	} else {
		// A command that changes LVM state is not stopped when ctx ends: why
		// it hangs, and whether stopping it is safe, depends on the node. It
		// runs to the end, and warnWhileRunning reports it if it takes long.
		// As before, it is not started once ctx has ended.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cmd = exec.Command(wholeCommand[0], wholeCommand[1:]...)
	}
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	rc, err := runCommand(ctx, logVerbosity, cmd)
	if err != nil {
		return nil, err
	}
	if !read {
		go warnWhileRunning(logger, cmd, lvmCommandWarningThreshold, rc.done)
	}
	return rc, nil
}

// stopOnCancel makes the end of cmd's context kill cmd's whole process group,
// and makes Wait return even if a killed process keeps its output open.
//
// The default prefix runs lvm through "nsenter -p", and nsenter forks before
// it execs lvm. Killing only the direct child, as exec.CommandContext does,
// kills nsenter and leaves lvm running. So the command runs in its own process
// group, and the whole group is killed.
//
// This depends on hostPID: true, which the lvmd and topolvm-node DaemonSets
// set: "nsenter -t 1" then enters the host's namespaces, and lvm runs in the
// PID namespace that lvmd sees, in nsenter's process group.
//
// A process stuck in the kernel (D state) does not exit even when killed, and
// keeps the output pipes open. WaitDelay makes Wait close them after
// readCommandWaitDelay, so the RPC returns anyway. The killed direct child,
// nsenter with the default prefix, must still exit for Wait to return.
func stopOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = readCommandWaitDelay
}

// runCommand runs the command and returns the stdout as a ReadCloser that also Waits for the command to finish.
// After the Close command is called the cmd is closed and the resources are released.
// Not calling close on this method will result in a resource leak.
func runCommand(ctx context.Context, logVerbosity int, cmd *exec.Cmd) (commandReadCloser, error) {
	// exec copies the output through pipes of its own, which Wait closes when
	// cmd.WaitDelay expires. It does not close pipes made by StdoutPipe and
	// StderrPipe, so Wait runs in the background while the caller reads.
	stdout, stdoutWriter := io.Pipe()
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdoutWriter
	cmd.Stderr = stderr

	log.FromContext(ctx).V(logVerbosity).Info("invoking command", "args", cmd.Args)
	if err := cmd.Start(); err != nil {
		return commandReadCloser{}, err
	}

	rc := commandReadCloser{
		ReadCloser: stdout,
		stderr:     stderr,
		done:       make(chan struct{}),
		waitErr:    new(error),
	}
	go func() {
		*rc.waitErr = cmd.Wait()
		// Wait returns after all output has been copied, so the caller reads
		// all of it before EOF.
		_ = stdoutWriter.Close()
		close(rc.done)
	}()
	return rc, nil
}

// commandReadCloser is the stdout of a command started by runCommand.
// Close waits for the command to finish.
type commandReadCloser struct {
	io.ReadCloser
	stderr *bytes.Buffer
	// done is closed when Wait has returned, and *waitErr holds its result.
	done    chan struct{}
	waitErr *error
}

// Close drains any unread stdout and waits for the command to exit.
func (p commandReadCloser) Close() error {
	// Drain stdout to prevent blocking when the caller stopped reading early
	// and the child has more output than the pipe buffer.
	_, _ = io.Copy(io.Discard, p.ReadCloser)

	<-p.done
	if err := *p.waitErr; err != nil {
		// wait can result in an exit code error
		return &lvmErr{
			err: err,
			// Not nil even when stderr was empty, as before.
			stderr: append([]byte{}, p.stderr.Bytes()...),
		}
	}
	return nil
}
