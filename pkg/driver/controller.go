package driver

import (
	internalDriver "github.com/topolvm/topolvm/internal/driver"
)

// NewControllerServer is an externally consumable wrapper.
// It allows starting a new controller server even without access to the package internals.
var NewControllerServer = internalDriver.NewControllerServer

// ControllerServerSettings is an externally consumable wrapper.
// It is used to configure the controller server.
type ControllerServerSettings = internalDriver.ControllerServerSettings

// MinimumAllocationSettings is an externally consumable wrapper.
// It contains the minimum allocation settings for the controller inside controller server settings.
type MinimumAllocationSettings = internalDriver.MinimumAllocationSettings

// Quantity is an externally consumable wrapper.
// It is used to represent a quantity of a resource.
type Quantity = internalDriver.Quantity

// NewQuantityFlagVar is an externally consumable wrapper.
// It is used to create a new quantity flag variable.
var NewQuantityFlagVar = internalDriver.NewQuantityFlagVar

// QuantityVar is an externally consumable wrapper.
// It is used to create a new quantity variable.
var QuantityVar = internalDriver.QuantityVar
