package command

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
)

var (
	// NotFoundPattern is a regular expression that matches the error message when a volume group or logical volume is not found.
	// The volume group might not be present or the logical volume might not be present in the volume group.
	NotFoundPattern = regexp.MustCompile(`Volume group "(.*?)" not found|Failed to find logical volume "(.*?)"`)
)

// IsLVMNotFound returns true if the error is a LVM recognized error and it determined that either
// the underlying volume group or logical volume is not found.
func IsLVMNotFound(err error) bool {
	lvmErr, ok := AsLVMError(err)

	// If the exit code is not 5, it is guaranteed that the error is not a not found error.
	if !ok || lvmErr.ExitCode() != 5 {
		return false
	}

	return NotFoundPattern.Match([]byte(lvmErr.Error()))
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
