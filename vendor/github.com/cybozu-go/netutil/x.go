package netutil

import (
	"net"

	_netutil "golang.org/x/net/netutil"
)

// LimitListener is the same as
// https://godoc.org/golang.org/x/net/netutil#LimitListener
func LimitListener(l net.Listener, n int) net.Listener {
	return _netutil.LimitListener(l, n)
}
