package well

import (
	"context"
	"sync"

	"github.com/cybozu-go/log"
)

// Environment implements context-based goroutine management.
type Environment struct {
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	generator *IDGenerator

	mu       sync.RWMutex
	stopped  bool
	stopCh   chan struct{}
	canceled bool
	err      error
}

// NewEnvironment creates a new Environment.
//
// This does *not* install signal handlers for SIGINT/SIGTERM
// for new environments.  Only the global environment will be
// canceled on these signals.
func NewEnvironment(ctx context.Context) *Environment {
	ctx, cancel := context.WithCancel(ctx)
	e := &Environment{
		ctx:       ctx,
		cancel:    cancel,
		generator: NewIDGenerator(),
		stopCh:    make(chan struct{}),
	}
	return e
}

// Stop just declares no further Go will be called.
//
// Calling Stop is optional if and only if Cancel is guaranteed
// to be called at some point.  For instance, if the program runs
// until SIGINT or SIGTERM, Stop is optional.
func (e *Environment) Stop() {
	e.mu.Lock()

	if !e.stopped {
		e.stopped = true
		close(e.stopCh)
	}

	e.mu.Unlock()
}

// Cancel cancels the base context.
//
// Passed err will be returned by Wait().
// Once canceled, Go() will not start new goroutines.
//
// Note that calling Cancel(nil) is perfectly valid.
// Unlike Stop(), Cancel(nil) cancels the base context and can
// gracefully stop goroutines started by Server.Serve or
// HTTPServer.ListenAndServe.
//
// This returns true if the caller is the first that calls Cancel.
// For second and later calls, Cancel does nothing and returns false.
func (e *Environment) Cancel(err error) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.canceled {
		return false
	}
	e.canceled = true
	e.err = err
	e.cancel()

	if e.stopped {
		return true
	}

	e.stopped = true
	close(e.stopCh)
	return true
}

// Wait waits for Stop or Cancel, and for all goroutines started by
// Go to finish.
//
// The returned err is the one passed to Cancel, or nil.
// err can be tested by IsSignaled to determine whether the
// program got SIGINT or SIGTERM.
func (e *Environment) Wait() error {
	<-e.stopCh
	if log.Enabled(log.LvDebug) {
		log.Debug("well: waiting for all goroutines to complete", nil)
	}
	e.wg.Wait()
	e.cancel() // in case no one calls Cancel

	e.mu.Lock()
	defer e.mu.Unlock()

	return e.err
}

// Go starts a goroutine that executes f.
//
// f takes a drived context from the base context.  The context
// will be canceled when f returns.
//
// Goroutines started by this function will be waited for by
// Wait until all such goroutines return.
//
// If f returns non-nil error, Cancel is called immediately
// with that error.
//
// f should watch ctx.Done() channel and return quickly when the
// channel is closed.
func (e *Environment) Go(f func(ctx context.Context) error) {
	e.mu.RLock()
	if e.stopped {
		e.mu.RUnlock()
		return
	}
	e.wg.Add(1)
	e.mu.RUnlock()

	go func() {
		ctx, cancel := context.WithCancel(e.ctx)
		defer cancel()
		err := f(ctx)
		if err != nil {
			e.Cancel(err)
		}
		e.wg.Done()
	}()
}

// GoWithID calls Go with a context having a new request tracking ID.
func (e *Environment) GoWithID(f func(ctx context.Context) error) {
	e.Go(func(ctx context.Context) error {
		return f(WithRequestID(ctx, e.generator.Generate()))
	})
}
