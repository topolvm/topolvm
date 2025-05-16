package lvmd

import (
	internalLvmdCommand "github.com/topolvm/topolvm/internal/lvmd/command"
)

// SetLVMPath sets the path to the lvm command.
var SetLVMPath = internalLvmdCommand.SetLVMPath

// SetRunLVMCommandsInContainer sets whether to run LVM commands like lvcreate
// in the container or on the host (i.e., in the root namespace). If set to
// true, the commands will be run directly in the container. If set to false,
// the commands will be run in the root namespace through nsenter.
var SetRunLVMCommandsInContainer = internalLvmdCommand.SetRunLVMCommandsInContainer
