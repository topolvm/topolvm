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
	"time"

	shell "gomodules.xyz/go-sh"
	"gomodules.xyz/pointer"
)

// RunBackup takes backup, cleanup old snapshots, check repository integrity etc.
// It extracts valuable information from respective restic command it runs and return them for further use.
func (w *ResticWrapper) RunBackup(backupOption BackupOptions) (*BackupOutput, error) {
	// Start clock to measure total session duration
	startTime := time.Now()

	// Run backup
	hostStats, err := w.runBackup(backupOption)

	if err != nil {
		hostStats.Phase = HostBackupFailed
		hostStats.Error = err.Error()
	} else {
		hostStats.Phase = HostBackupSucceeded
		hostStats.Duration = time.Since(startTime).String()
	}

	return &BackupOutput{
		Stats: []HostBackupStats{hostStats},
	}, err
}

func (w *ResticWrapper) runBackup(backupOption BackupOptions) (HostBackupStats, error) {
	hostStats := HostBackupStats{
		Hostname: backupOption.Host,
	}

	// fmt.Println("shell: ",w)
	// Backup from stdin
	if len(backupOption.StdinPipeCommands) != 0 {
		out, err := w.backupFromStdin(backupOption)
		if err != nil {
			return hostStats, err
		}
		// Extract information from the output of backup command
		snapStats, err := extractBackupInfo(out, backupOption.StdinFileName)
		if err != nil {
			return hostStats, err
		}
		hostStats.Snapshots = []SnapshotStats{snapStats}
		return hostStats, nil
	}

	// Backup all target paths
	for _, path := range backupOption.BackupPaths {
		params := backupParams{
			path:     path,
			host:     backupOption.Host,
			excludes: backupOption.Exclude,
			args:     backupOption.Args,
		}
		out, err := w.backup(params)
		if err != nil {
			return hostStats, err
		}
		// Extract information from the output of backup command
		stats, err := extractBackupInfo(out, path)
		if err != nil {
			return hostStats, err
		}
		hostStats = upsertSnapshotStats(hostStats, stats)
	}

	return hostStats, nil
}

func upsertSnapshotStats(hostStats HostBackupStats, snapStats SnapshotStats) HostBackupStats {
	for i, s := range hostStats.Snapshots {
		// if there is already an entry for this snapshot, then update it
		if s.Name == snapStats.Name {
			hostStats.Snapshots[i] = snapStats
			return hostStats
		}
	}
	// no entry for this snapshot. add a new entry
	hostStats.Snapshots = append(hostStats.Snapshots, snapStats)
	return hostStats
}

func (w *ResticWrapper) RepositoryAlreadyExist() bool {
	return w.repositoryExist()
}

func (w *ResticWrapper) InitializeRepository() error {
	return w.initRepository()
}

func (w *ResticWrapper) VerifyRepositoryIntegrity() (*RepositoryStats, error) {
	// Check repository integrity
	out, err := w.check()
	if err != nil {
		return nil, err
	}
	// Extract information from output of "check" command
	integrity := extractCheckInfo(out)
	// Read repository statics after cleanup
	out, err = w.stats("")
	if err != nil {
		return nil, err
	}
	// Extract information from output of "stats" command
	repoSize, err := extractStatsInfo(out)
	if err != nil {
		return nil, err
	}
	return &RepositoryStats{Integrity: pointer.BoolP(integrity), Size: repoSize}, nil
}

func (w *ResticWrapper) ValidateConnection() ([]byte, error) {
	return w.validateConnections()
}

func (w *ResticWrapper) GetShell() *shell.Session {
	return w.sh
}
