package well

import "context"

var (
	defaultEnv *Environment
)

func init() {
	defaultEnv = NewEnvironment(context.Background())
	handleSignal(defaultEnv)
	handleSigPipe()
}

// Stop just declares no further Go will be called.
//
// Calling Stop is optional if and only if Cancel is guaranteed
// to be called at some point.  For instance, if the program runs
// until SIGINT or SIGTERM, Stop is optional.
func Stop() {
	defaultEnv.Stop()
}

// Cancel cancels the base context of the global environment.
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
func Cancel(err error) bool {
	return defaultEnv.Cancel(err)
}

// Wait waits for Stop or Cancel, and for all goroutines started by
// Go to finish.
//
// The returned err is the one passed to Cancel, or nil.
// err can be tested by IsSignaled to determine whether the
// program got SIGINT or SIGTERM.
func Wait() error {
	return defaultEnv.Wait()
}

// Go starts a goroutine that executes f in the global environment.
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
func Go(f func(ctx context.Context) error) {
	defaultEnv.Go(f)
}

// GoWithID calls Go with a context having a new request tracking ID.
func GoWithID(f func(ctx context.Context) error) {
	defaultEnv.GoWithID(f)
}
