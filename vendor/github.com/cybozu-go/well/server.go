package well

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/netutil"
)

// Server is a generic network server that accepts connections
// and invokes Handler in a goroutine for each connection.
//
// In addition, Serve method gracefully waits all its goroutines to
// complete before returning.
type Server struct {

	// Handler handles a connection.  This must not be nil.
	//
	// ctx is a derived context from the base context that will be
	// canceled when Handler returns.
	//
	// conn will be closed when Handler returns.
	Handler func(ctx context.Context, conn net.Conn)

	// ShutdownTimeout is the maximum duration the server waits for
	// all connections to be closed before shutdown.
	//
	// Zero duration disables timeout.
	ShutdownTimeout time.Duration

	// Env is the environment where this server runs.
	//
	// The global environment is used if Env is nil.
	Env *Environment

	wg       sync.WaitGroup
	timedout int32
}

// Serve starts a managed goroutine to accept connections.
//
// Serve itself returns immediately.  The goroutine continues
// to accept and handle connections until the base context is
// canceled.
//
// If the listener is *net.TCPListener, TCP keep-alive is automatically
// enabled.
//
// The listener l will be closed automatically when the environment's
// Cancel is called.
func (s *Server) Serve(l net.Listener) {
	env := s.Env
	if env == nil {
		env = defaultEnv
	}

	l = netutil.KeepAliveListener(l)

	go func() {
		<-env.ctx.Done()
		l.Close()
	}()

	env.Go(func(ctx context.Context) error {
		generator := NewIDGenerator()
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Debug("well: Listener.Accept error", map[string]interface{}{
					"addr":  l.Addr().String(),
					"error": err.Error(),
				})
				goto OUT
			}

			s.wg.Add(1)
			go func() {
				ctx, cancel := context.WithCancel(ctx)
				defer func() {
					cancel()
					conn.Close()
				}()
				ctx = WithRequestID(ctx, generator.Generate())
				s.Handler(ctx, conn)
				s.wg.Done()
			}()
		}
	OUT:
		s.wait()
		return nil
	})
}

func (s *Server) wait() {
	if s.ShutdownTimeout == 0 {
		s.wg.Wait()
		return
	}

	ch := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(ch)
	}()

	select {
	case <-ch:
	case <-time.After(s.ShutdownTimeout):
		log.Warn("well: timeout waiting for shutdown", nil)
		atomic.StoreInt32(&s.timedout, 1)
	}
}

// TimedOut returns true if the server shut down before all connections
// got closed.
func (s *Server) TimedOut() bool {
	return atomic.LoadInt32(&s.timedout) != 0
}
