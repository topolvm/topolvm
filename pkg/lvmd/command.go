package lvmd

import (
	internalLvmdCommand "github.com/topolvm/topolvm/internal/lvmd/command"
)

// SetLVMPath sets the path to the lvm command.
var SetLVMPath = internalLvmdCommand.SetLVMPath

// SetUseNsenter sets whether to use nsenter to call lvm commands.
var SetUseNsenter = internalLvmdCommand.SetUseNsenter
