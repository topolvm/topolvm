package lvmd

import (
	internalLvmdCommand "github.com/topolvm/topolvm/internal/lvmd/command"
)

// SetLVMPath sets the path to the lvm command.
var SetLVMPath = internalLvmdCommand.SetLVMPath

// SetLVMCommandPrefix sets a list of strings necessary to run a LVM command.
// For example, if it's X, `/sbin/lvm lvcreate ...` will be run as `X /sbin/lvm lvcreate ...`.
var SetLVMCommandPrefix = internalLvmdCommand.SetLVMCommandPrefix
