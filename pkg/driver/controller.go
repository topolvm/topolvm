package driver

import (
	internalDriver "github.com/topolvm/topolvm/internal/driver"
)

// NewControllerServer is a externally consumable wrapper.
// It allows starting a new controller server even without access to the package internals.
var NewControllerServer = internalDriver.NewControllerServer

// ControllerServerSettings is a externally consumable wrapper.
// It is used to configure the controller server.
type ControllerServerSettings = internalDriver.ControllerServerSettings
