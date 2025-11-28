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
	"fmt"

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

// getHostNameIfAnyExclusiveLock scans every lock and returns the hostname aka (Pod name) of the first exclusive lock it finds, or "" if none exist.
func (w *ResticWrapper) getHostNameIfAnyExclusiveLock() (string, error) {
	klog.Infoln("Checking for exclusive locks in the repository...")
	ids, err := w.getLockIDs()
	if err != nil {
		return "", fmt.Errorf("failed to list locks: %w", err)
	}
	for _, id := range ids {
		st, err := w.getLockStats(id)
		if err != nil {
			return "", fmt.Errorf("failed to inspect lock %s: %w", id, err)
		}
		if st.Exclusive { // There's no chances to get multiple exclusive locks, so we can return the first one we find.
			return st.Hostname, nil
		}
	}
	return "", nil
}

// EnsureNoExclusiveLock blocks until any exclusive lock is released.
// If a lock is held by a Running Pod, it waits; otherwise it unlocks.
func (w *ResticWrapper) EnsureNoExclusiveLock() error {
	repository := w.GetRepo()
	klog.Infof("Checking for exclusive lock on repository: %s", repository)
	hostName, err := w.getHostNameIfAnyExclusiveLock()
	if err != nil {
		return fmt.Errorf("failed to check exclusive lock for repository %s: %w", repository, err)
	}
	if hostName == "" {
		klog.Infof("No exclusive lock found for repository: %s, proceeding...", repository)
		return nil
	}
	_, unlockErr := w.unlock()
	return unlockErr

}
