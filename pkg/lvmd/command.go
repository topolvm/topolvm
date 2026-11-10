package lvmd

import (
	internalLvmdCommand "github.com/topolvm/topolvm/internal/lvmd/command"
)

// SetLVMPath sets the path to the lvm command. Note that this function must not
// be called together with SetLVMCommandPrefix.
var SetLVMPath = internalLvmdCommand.SetLVMPath

// SetLVMCommandPrefix sets a list of strings necessary to run a LVM command.
// For example, if it's X, `/sbin/lvm lvcreate ...` will be run as `X /sbin/lvm
// lvcreate ...`.  This function must not be called together with SetLVMPath.
var SetLVMCommandPrefix = internalLvmdCommand.SetLVMCommandPrefix

// DefaultLVMCommandWarningThreshold is how long an LVM command that changes
// LVM state may run before a warning is logged, unless
// SetLVMCommandWarningThreshold is called.
const DefaultLVMCommandWarningThreshold = internalLvmdCommand.DefaultLVMCommandWarningThreshold

// SetLVMCommandWarningThreshold sets how long an LVM command that changes LVM
// state may run before a warning is logged. The warning is repeated at the
// same interval until the command finishes. Zero or less disables it.
var SetLVMCommandWarningThreshold = internalLvmdCommand.SetLVMCommandWarningThreshold
