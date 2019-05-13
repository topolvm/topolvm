package netutil

import (
	"net"
	"time"
)

const (
	keepAlivePeriod = 3 * time.Minute
)

// TCPKeepAliveListener wraps *net.TCPListener.
type TCPKeepAliveListener struct {
	*net.TCPListener
}

// Accept returns a TCP keep-alive enabled connection.
func (l TCPKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := l.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(keepAlivePeriod)
	return tc, nil
}

// KeepAliveListener returns TCPKeepAliveListener if l is a
// *net.TCPListener.  Otherwise, l is returned without change.
func KeepAliveListener(l net.Listener) net.Listener {
	if tl, ok := l.(*net.TCPListener); ok {
		return TCPKeepAliveListener{tl}
	}
	return l
}

// SetKeepAlive enables TCP keep-alive if c is a *net.TCPConn.
func SetKeepAlive(c net.Conn) {
	tc, ok := c.(*net.TCPConn)
	if !ok {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(keepAlivePeriod)
}
