package well

import (
	"errors"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/cybozu-go/log"
)

var (
	errSignaled = errors.New("signaled")

	cancellationDelaySecondsEnv = "CANCELLATION_DELAY_SECONDS"

	defaultCancellationDelaySeconds = 5
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
		delay := getDelaySecondsFromEnv()
		log.Warn("well: got signal", map[string]interface{}{
			"signal": s.String(),
			"delay":  delay,
		})
		time.Sleep(time.Duration(delay) * time.Second)
		env.Cancel(errSignaled)
	}()
}

func getDelaySecondsFromEnv() int {
	delayStr := os.Getenv(cancellationDelaySecondsEnv)
	if len(delayStr) == 0 {
		return defaultCancellationDelaySeconds
	}

	delay, err := strconv.Atoi(delayStr)
	if err != nil {
		log.Warn("well: set default cancellation delay seconds", map[string]interface{}{
			"env":       delayStr,
			"delay":     defaultCancellationDelaySeconds,
			log.FnError: err,
		})
		return defaultCancellationDelaySeconds
	}
	if delay < 0 {
		log.Warn("well: round up negative cancellation delay seconds to 0s", map[string]interface{}{
			"env":   delayStr,
			"delay": 0,
		})
		return 0
	}
	return delay
}
