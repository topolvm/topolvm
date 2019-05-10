// +build windows

package well

import "net"

func isMaster() bool {
	return true
}

// SystemdListeners returns (nil, nil) on Windows.
func SystemdListeners() ([]net.Listener, error) {
	return nil, nil
}

// Run simply calls g.Listen then g.Serve on Windows.
func (g *Graceful) Run() {
	env := g.Env
	if env == nil {
		env = defaultEnv
	}

	// prepare listener files
	listeners, err := g.Listen()
	if err != nil {
		env.Cancel(err)
		return
	}
	g.Serve(listeners)
}
