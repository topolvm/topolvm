package well

import (
	"errors"
	"os"
	"os/signal"

	"github.com/cybozu-go/log"
)

var (
	errSignaled = errors.New("signaled")
)

// IsSignaled returns true if err returned by Wait indicates that
// the program has received SIGINT or SIGTERM.
func IsSignaled(err error) bool {
	return err == errSignaled
}

// handleSignal runs independent goroutine to cancel an environment.
func handleSignal(env *Environment) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, stopSignals...)

	go func() {
		s := <-ch
		log.Warn("well: got signal", map[string]interface{}{
			"signal": s.String(),
		})
		env.Cancel(errSignaled)
	}()
}
