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
