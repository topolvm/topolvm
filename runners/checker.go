package runners

import (
	"context"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type readinessCheck struct {
	check    func() error
	interval time.Duration

	mu    sync.RWMutex
	ready bool
	err   error
}

// Checker is the interface to check plugin readiness.
type Checker interface {
	manager.Runnable
	Ready() (bool, error)
}

var _ manager.LeaderElectionRunnable = &readinessCheck{}

// NewChecker creates controller-runtime's manager.Runnable to run
// health check function periodically at given interval.
func NewChecker(check func() error, interval time.Duration) Checker {
	return &readinessCheck{check: check, interval: interval}
}

func (c *readinessCheck) setError(e error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// plugin becomes ready at the first non-nil check.
	if e == nil {
		c.ready = true
	}
	c.err = e
}

// Start implements controller-runtime's manager.Runnable.
func (c *readinessCheck) Start(ctx context.Context) error {
	c.setError(c.check())

	tick := time.NewTicker(c.interval)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			c.setError(c.check())
		case <-ctx.Done():
			return nil
		}
	}
}

// NeedLeaderElection implements controller-runtime's manager.LeaderElectionRunnable.
func (c *readinessCheck) NeedLeaderElection() bool {
	return false
}

// Ready can be passed to driver.NewIdentityService.
func (c *readinessCheck) Ready() (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready, c.err
}
