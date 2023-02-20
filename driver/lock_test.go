package driver

import (
	"testing"
	"time"
)

func TestPreventConcurrentExecutionBySameID(t *testing.T) {
	l := NewLockWithID()

	l.LockByID("a")

	c := make(chan struct{})

	go func() {
		l.LockByID("a")
		defer l.UnlockByID("a")

		close(c)
	}()

	select {
	case <-c:
		t.Error("Failed to prevent concurrent execution")
	case <-time.After(time.Second):
		// success
	}

	// check goroutine can progress if the lock is released.
	l.UnlockByID("a")

	<-c
}

func TestAllowConcurrentExecutionByDifferentID(t *testing.T) {
	l := NewLockWithID()

	l.LockByID("a")
	defer l.UnlockByID("a")

	c := make(chan struct{})

	go func() {
		l.LockByID("b")
		defer l.UnlockByID("b")

		close(c)
	}()

	select {
	case <-c:
		// success
	case <-time.After(time.Second):
		t.Error("Failed to allow concurrent execution")
	}
}
