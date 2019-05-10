package netutil

// HalfCloser is an interface for connections that can be half-closed.
// TCPConn and UNIXConn implement this.
type HalfCloser interface {
	CloseRead() error
	CloseWrite() error
}
