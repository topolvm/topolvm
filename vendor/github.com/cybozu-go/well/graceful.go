package well

import (
	"net"
	"time"
)

// Graceful is a struct to implement graceful restart servers.
//
// On Windows, this is just a dummy to make porting easy.
type Graceful struct {
	// Listen is a function to create listening sockets.
	// This function is called in the master process.
	Listen func() ([]net.Listener, error)

	// Serve is a function to accept connections from listeners.
	// This function is called in child processes.
	// In case of errors, use os.Exit to exit.
	Serve func(listeners []net.Listener)

	// ExitTimeout is duration before Run gives up waiting for
	// a child to exit.  Zero disables timeout.
	ExitTimeout time.Duration

	// Env is the environment for the master process.
	// If nil, the global environment is used.
	Env *Environment
}
