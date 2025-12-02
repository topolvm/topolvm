/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package restic

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

func (w *ResticWrapper) UnlockRepository() error {
	_, err := w.unlock()
	return err
}

// getLockIDs lists every lock ID currently held in the repository.
func (w *ResticWrapper) getLockIDs() ([]string, error) {
	w.sh.ShowCMD = true
	out, err := w.listLocks()
	if err != nil {
		return nil, err
	}
	return extractLockIDs(bytes.NewReader(out))
}

// getLockStats returns the decoded JSON for a single lock.
func (w *ResticWrapper) getLockStats(lockID string) (*LockStats, error) {
	w.sh.ShowCMD = true
	out, err := w.lockStats(lockID)
	if err != nil {
		return nil, err
	}
	return extractLockStats(out)
}

// EnsureNoExclusiveLock blocks until any exclusive lock is released.
// If a lock is held by a Running Pod, it waits; otherwise it unlocks.
func (w *ResticWrapper) EnsureNoExclusiveLock() error {
	repository := w.GetRepo()
	klog.Infof("Checking for locks on repository: %s", repository)
	klog.Infof("Processing repository: %s", repository)

	// Remove stale locks
	klog.Infof("Removing stale locks from repository: %s", repository)
	_, err := w.unlockStale()
	if err != nil {
		klog.Warningf("Failed to remove stale locks (non-fatal): %v", err)
	}

	// Check if any exclusive locks remain
	// If they do, restic determined they're active (it would have removed them if stale)
	klog.Infof("Checking for exclusive locks in repository: %s", repository)
	hasLock, podName, err := w.hasExclusiveLock()
	if err != nil {
		return fmt.Errorf("failed to check for exclusive locks in repository %s: %w", repository, err)
	}

	if !hasLock {
		klog.Infof("No exclusive lock found. Repository %s is ready.", repository)
		return nil
	}

	// : Wait for the exclusive lock to be released
	// Periodically retry unlockStale() in case the process crashes during wait
	const lockWaitTimeout = 1 * time.Hour

	klog.Infof("Exclusive lock found (held by %s). Waiting up to %v for it to be released...", podName, lockWaitTimeout)
	err = wait.PollUntilContextTimeout(
		context.Background(),
		10*time.Second,
		lockWaitTimeout,
		true,
		func(ctx context.Context) (bool, error) {
			klog.Infof("Polling: checking if exclusive lock is released...")

			// Try to cleanup stale locks (in case process crashed)
			_, unlockErr := w.unlockStale()
			if unlockErr != nil {
				klog.Warningf("Failed to remove stale locks during polling: %v", unlockErr)
			}

			// Check if exclusive lock still exists
			hasLock, currentPodName, err := w.hasExclusiveLock()
			if err != nil {
				klog.Warningf("Error checking locks during polling: %v", err)
				return false, nil // Don't fail, retry
			}

			if !hasLock {
				klog.Infof("Exclusive lock released. Repository is ready.")
				return true, nil
			}

			// Lock still exists
			klog.Infof("Exclusive lock still held by %s. Waiting...", currentPodName)
			return false, nil
		},
	)

	if err != nil {
		return fmt.Errorf("timeout waiting for exclusive lock to be released in repository %s: %w", repository, err)
	}
	klog.Infof("Repository %s is ready.", repository)
	return nil
}

// hasExclusiveLock checks if any exclusive lock exists in the repository.
// This should be called AFTER unlockStale() - any remaining exclusive locks are active.
func (w *ResticWrapper) hasExclusiveLock() (bool, string, error) {
	ids, err := w.getLockIDs()
	if err != nil {
		return false, "", fmt.Errorf("failed to list locks: %w", err)
	}

	if len(ids) == 0 {
		return false, "", nil
	}

	// Check each lock to find exclusive locks
	for _, id := range ids {
		st, err := w.getLockStats(id)
		if err != nil {
			klog.Warningf("Failed to inspect lock %s: %v", id, err)
			continue
		}

		if st.Exclusive {
			klog.Infof("Found exclusive lock: %s (hostname: %s)", id, st.Hostname)
			return true, st.Hostname, nil
		}
	}

	return false, "", nil
}
