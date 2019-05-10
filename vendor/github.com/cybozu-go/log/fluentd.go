package log

import (
	"net"
	"os"
	"time"
)

const (
	fluentTimeout    = 5 * time.Second
	fluentSocketPath = "/run/fluentd/fluentd.sock"
	fluentEnvSocket  = "FLUENTD_SOCK"
)

type fluentConn struct {
	net.Conn
}

func newFluentConn() (*fluentConn, error) {
	path := os.Getenv(fluentEnvSocket)
	if len(path) == 0 {
		path = fluentSocketPath
	}
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	conn, err := net.DialTimeout("unix", path, fluentTimeout)
	if err != nil {
		return nil, err
	}
	return &fluentConn{conn}, nil
}

func (f *fluentConn) SendMessage(b []byte) error {
	f.SetWriteDeadline(time.Now().Add(fluentTimeout))
	_, err := f.Write(b)
	return err
}
