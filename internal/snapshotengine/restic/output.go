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
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

const FileModeRWXAll = 0o777

type BackupOutput struct {
	// Stats shows statistics of individual hosts
	Stats []HostBackupStats `json:"stats,omitempty"`
}

type RestoreOutput struct {
	// Stats shows restore statistics of individual hosts
	Stats []HostRestoreStats `json:"stats,omitempty"`
}

type RepositoryStats struct {
	// Integrity shows result of repository integrity check after last backup
	Integrity *bool `json:"integrity,omitempty"`
	// Size show size of repository after last backup
	Size string `json:"size,omitempty"`
	// SnapshotCount shows number of snapshots stored in the repository
	SnapshotCount int64 `json:"snapshotCount,omitempty"`
	// SnapshotsRemovedOnLastCleanup shows number of old snapshots cleaned up according to retention policy on last backup session
	SnapshotsRemovedOnLastCleanup int64 `json:"snapshotsRemovedOnLastCleanup,omitempty"`
}

// ExtractBackupInfo extract information from output of "restic backup" command and
// save valuable information into backupOutput
func extractBackupInfo(output []byte, path string) (SnapshotStats, error) {
	snapshotStats := SnapshotStats{
		Path: path,
	}

	// unmarshal json output
	var jsonOutput BackupSummary
	dec := json.NewDecoder(bytes.NewReader(output))
	for {

		err := dec.Decode(&jsonOutput)
		if err == io.EOF {
			// all done
			break
		}
		if err != nil {
			return snapshotStats, err
		}
		// if message type is summary then we have found our desired message block
		if jsonOutput.MessageType == "summary" {
			break
		}
	}

	snapshotStats.FileStats.NewFiles = jsonOutput.FilesNew
	snapshotStats.FileStats.ModifiedFiles = jsonOutput.FilesChanged
	snapshotStats.FileStats.UnmodifiedFiles = jsonOutput.FilesUnmodified
	snapshotStats.FileStats.TotalFiles = jsonOutput.TotalFilesProcessed

	snapshotStats.Uploaded = formatBytes(jsonOutput.DataAdded)
	snapshotStats.TotalSize = formatBytes(jsonOutput.TotalBytesProcessed)
	snapshotStats.ProcessingTime = formatSeconds(uint64(jsonOutput.TotalDuration))
	snapshotStats.Name = jsonOutput.SnapshotID

	return snapshotStats, nil
}

// ExtractCheckInfo extract information from output of "restic check" command and
// save valuable information into backupOutput
func extractCheckInfo(out []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		line = strings.TrimSpace(line)
		if line == "no errors were found" {
			return true
		}
	}
	return false
}

// ExtractStatsInfo extract information from output of "restic stats" command and
// save valuable information into backupOutput
func extractStatsInfo(out []byte) (string, error) {
	var stat StatsContainer
	err := json.Unmarshal(out, &stat)
	if err != nil {
		return "", err
	}
	return formatBytes(stat.TotalSize), nil
}

type BackupSummary struct {
	MessageType         string  `json:"message_type"` // "summary"
	FilesNew            *int64  `json:"files_new"`
	FilesChanged        *int64  `json:"files_changed"`
	FilesUnmodified     *int64  `json:"files_unmodified"`
	DataAdded           uint64  `json:"data_added"`
	TotalFilesProcessed *int64  `json:"total_files_processed"`
	TotalBytesProcessed uint64  `json:"total_bytes_processed"`
	TotalDuration       float64 `json:"total_duration"` // in seconds
	SnapshotID          string  `json:"snapshot_id"`
}

type ForgetGroup struct {
	Keep   []json.RawMessage `json:"keep"`
	Remove []json.RawMessage `json:"remove"`
}

type StatsContainer struct {
	TotalSize uint64 `json:"total_size"`
}
