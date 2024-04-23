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
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var Containerized = false

// callLVM calls lvm sub-commands and prints the output to the log.
func callLVM(ctx context.Context, args ...string) error {
	return callLVMInto(ctx, nil, args...)
}

// callLVMInto calls lvm sub-commands and decodes the output via JSON into the provided struct pointer.
// if the struct pointer is nil, the output will be printed to the log instead.
func callLVMInto(ctx context.Context, into any, args ...string) error {
	output, err := callLVMStreamed(ctx, args...)
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
func callLVMStreamed(ctx context.Context, args ...string) (io.ReadCloser, error) {
	ctx = log.IntoContext(ctx, log.FromContext(ctx).WithCallDepth(1))
	cmd := wrapExecCommand(lvm, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	return runCommand(ctx, cmd)
}

// wrapExecCommand calls cmd with args but wrapped to run on the host with nsenter if Containerized is true.
func wrapExecCommand(cmd string, args ...string) *exec.Cmd {
	if Containerized {
		args = append([]string{"-m", "-u", "-i", "-n", "-p", "-t", "1", cmd}, args...)
		cmd = nsenter
	}
	c := exec.Command(cmd, args...)
	return c
}

// runCommand runs the command and returns the stdout as a ReadCloser that also Waits for the command to finish.
// After the Close command is called the cmd is closed and the resources are released.
// Not calling close on this method will result in a resource leak.
func runCommand(ctx context.Context, cmd *exec.Cmd) (io.ReadCloser, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	log.FromContext(ctx).Info("invoking command", "args", cmd.Args)
	if err := cmd.Start(); err != nil {
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

// Close closes stdout and stderr and waits for the command to exit. Close
// should not be called before all reads from stdout have completed.
func (p commandReadCloser) Close() error {
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

// AsLVMError returns the LVMError from the error if it exists and a bool indicating if is an LVMError or not.
func AsLVMError(err error) (LVMError, bool) {
	var lvmErr LVMError
	ok := errors.As(err, &lvmErr)
	return lvmErr, ok
}

// LVMError is an error that wraps the original error and the stderr output of the lvm command if found.
// It also provides an exit code if present that can be used to determine the type of error from LVM.
// Regular inaccessible errors will have an exit code of 5.
type LVMError interface {
	error
	ExitCode() int
	Unwrap() error
}

type lvmErr struct {
	err    error
	stderr []byte
}

func (e *lvmErr) Error() string {
	if e.stderr != nil {
		return fmt.Sprintf("%v: %v", e.err, string(bytes.TrimSpace(e.stderr)))
	}
	return e.err.Error()
}

func (e *lvmErr) Unwrap() error {
	return e.err
}

func (e *lvmErr) ExitCode() int {
	type exitError interface {
		ExitCode() int
		error
	}
	var err exitError
	if errors.As(e.err, &err) {
		return err.ExitCode()
	}
	return -1
}
