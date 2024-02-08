package driver

import (
	"fmt"
	"sync"
)

// LockByID is a mutex which takes lock with a given ID.
// If same id is given, only one process can take lock.
// If different id is given, each process can take lock separately.
type LockByID struct {
	cond   sync.Cond
	locked map[string]struct{}
}

func NewLockWithID() *LockByID {
	return &LockByID{
		cond:   *sync.NewCond(&sync.Mutex{}),
		locked: map[string]struct{}{},
	}
}

func (l *LockByID) LockByID(id string) {
	l.cond.L.Lock()
	defer l.cond.L.Unlock()

	for {
		if _, locked := l.locked[id]; !locked {
			// we can take lock
			l.locked[id] = struct{}{}
			return
		}

		l.cond.Wait()
	}
}

func (l *LockByID) UnlockByID(id string) {
	l.cond.L.Lock()
	defer l.cond.L.Unlock()

	if _, locked := l.locked[id]; !locked {
		panic(fmt.Sprintf("BUG: try to unlock not taken lock, id=%s", id))
	}

	delete(l.locked, id)

	l.cond.Broadcast()
}
