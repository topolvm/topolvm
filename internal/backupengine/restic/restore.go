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
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/errors"
)

// RunRestore run restore process for a single host.
func (w *ResticWrapper) RunRestore(restoreOptions RestoreOptions) (*RestoreOutput, error) {
	// Start clock to measure total restore duration
	startTime := time.Now()

	restoreStats := HostRestoreStats{
		Hostname: restoreOptions.Host,
	}

	err := w.runRestore(restoreOptions)
	if err != nil {
		restoreStats.Phase = HostRestoreFailed
		restoreStats.Error = err.Error()
		return nil, err
	} else {
		restoreStats.Phase = HostRestoreSucceeded
		restoreStats.Duration = time.Since(startTime).String()
	}

	return &RestoreOutput{
		Stats: []HostRestoreStats{restoreStats},
	}, err
}

func (w *ResticWrapper) runRestore(restoreOptions RestoreOptions) error {
	if len(restoreOptions.Snapshots) != 0 {
		for _, snapshot := range restoreOptions.Snapshots {
			// if snapshot is specified then host and path does not matter.
			params := restoreParams{
				destination: restoreOptions.Destination,
				snapshotId:  snapshot,
				excludes:    restoreOptions.Exclude,
				includes:    restoreOptions.Include,
				args:        restoreOptions.Args,
			}
			if _, err := w.restore(params); err != nil {
				return err
			}
		}
	} else if len(restoreOptions.RestorePaths) != 0 {
		for _, path := range restoreOptions.RestorePaths {
			params := restoreParams{
				path:        path,
				host:        restoreOptions.SourceHost,
				destination: restoreOptions.Destination,
				excludes:    restoreOptions.Exclude,
				includes:    restoreOptions.Include,
				args:        restoreOptions.Args,
			}
			if _, err := w.restore(params); err != nil {
				return err
			}
		}
	}
	return nil
}

// Dump run restore process for a single host and output the restored files in stdout.
func (w *ResticWrapper) Dump(dumpOptions DumpOptions) (*RestoreOutput, error) {
	// Start clock to measure total restore duration
	startTime := time.Now()

	restoreStats := HostRestoreStats{
		Hostname: dumpOptions.Host,
	}

	// if source host is not specified then use current host as source host
	if dumpOptions.SourceHost == "" {
		dumpOptions.SourceHost = dumpOptions.Host
	}

	_, err := w.DumpOnce(dumpOptions)
	if err != nil {
		restoreStats.Phase = HostRestoreFailed
		restoreStats.Error = err.Error()
	} else {
		restoreStats.Phase = HostRestoreSucceeded
		restoreStats.Duration = time.Since(startTime).String()
	}

	return &RestoreOutput{
		Stats: []HostRestoreStats{restoreStats},
	}, err
}

// ParallelDump run DumpOnce for multiple hosts concurrently using go routine.
// You can control maximum number of parallel restore process using maxConcurrency parameter.
func (w *ResticWrapper) ParallelDump(dumpOptions []DumpOptions, maxConcurrency int) (*RestoreOutput, error) {
	// WaitGroup to wait until all go routine finish
	wg := sync.WaitGroup{}
	// concurrencyLimiter channel is used to limit maximum number simultaneous go routine
	concurrencyLimiter := make(chan bool, maxConcurrency)
	defer close(concurrencyLimiter)

	var (
		restoreErrs []error
		mu          sync.Mutex
	)

	restoreOutput := &RestoreOutput{}

	for i := range dumpOptions {
		// try to send message in concurrencyLimiter channel.
		// if maximum allowed concurrent restore is already running, program control will stuck here.
		concurrencyLimiter <- true

		// starting new go routine. add it to WaitGroup
		wg.Add(1)

		go func(opt DumpOptions, startTime time.Time) {
			// when this go routine completes its task, release a slot from the concurrencyLimiter channel
			// so that another go routine can start. Also, tell the WaitGroup that it is done with its task.
			defer func() {
				<-concurrencyLimiter
				wg.Done()
			}()

			// sh field in ResticWrapper is a pointer. we must not use same w in multiple go routine.
			// otherwise they might enter in a racing condition.
			nw := w.Copy()

			// if source host is not specified then use current host as source host
			if opt.SourceHost == "" {
				opt.SourceHost = opt.Host
			}

			hostStats := HostRestoreStats{
				Hostname: opt.Host,
			}
			// run restore
			_, err := nw.DumpOnce(opt)
			if err != nil {
				hostStats.Phase = HostRestoreFailed
				hostStats.Error = err.Error()
				mu.Lock()
				restoreErrs = append(restoreErrs, err)
				mu.Unlock()
			} else {
				hostStats.Phase = HostRestoreSucceeded
				hostStats.Duration = time.Since(startTime).String()
			}
			// add hostStats to restoreOutput. use lock to avoid racing condition.
			mu.Lock()
			restoreOutput.upsertHostRestoreStats(hostStats)
			mu.Unlock()
		}(dumpOptions[i], time.Now())
	}
	// wait for all the go routines to complete
	wg.Wait()

	return restoreOutput, errors.NewAggregate(restoreErrs)
}

func (restoreOutput *RestoreOutput) upsertHostRestoreStats(hostStats HostRestoreStats) {
	// check if a entry already exist for this host in restoreOutput. If exist then update it.
	for i, v := range restoreOutput.Stats {
		if v.Hostname == hostStats.Hostname {
			restoreOutput.Stats[i] = hostStats
			return
		}
	}
	// no entry for this host. add a new entry
	restoreOutput.Stats = append(restoreOutput.Stats, hostStats)
}
