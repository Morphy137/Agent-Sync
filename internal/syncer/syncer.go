package syncer

import (
	"os"
	"path/filepath"
	"sort"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/state"
)

// File-level status constants.
const (
	FileInSync        = "in_sync"
	FileNewInSource   = "new_in_source"
	FileUntrackedDiff = "untracked_diff"
	FileSourceChanged = "source_changed"
	FileTargetChanged = "target_changed"
	FileTargetMissing = "target_missing"
	FileConflict      = "conflict"
)

// Connection-level aggregated status constants.
const (
	ConnInSync            = "in_sync"
	ConnSourceChanged     = "source_changed"
	ConnTargetChanged     = "target_changed"
	ConnDiverged          = "diverged"
	ConnUntrackedDiff     = "untracked_diff"
	ConnConflict          = "conflict"
	ConnTargetUnavailable = "target_unavailable"
)

// Resolution choices.
const (
	ResolutionKeepRepo  = "keep_repo"
	ResolutionKeepLocal = "keep_local"
)

// FileStatus represents sync state for an individual file.
type FileStatus struct {
	RelPath      string `json:"relPath"`
	Status       string `json:"status"`
	SourceHash   string `json:"sourceHash"`
	TargetHash   string `json:"targetHash"`
	BaselineHash string `json:"baselineHash"`
}

// ConnectionStatus represents aggregated sync state for a connection.
type ConnectionStatus struct {
	ConnectionID string       `json:"connectionId"`
	Profile      string       `json:"profile"`
	TargetPath   string       `json:"targetPath"`
	Status       string       `json:"status"`
	Files        []FileStatus `json:"files"`
}

// EvaluateFileStatus computes the status for a single managed file according to the spec table.
func EvaluateFileStatus(sourceHash, targetHash, baselineHash string) string {
	// Rule 1: S == T -> in_sync
	if sourceHash == targetHash && sourceHash != "" {
		return FileInSync
	}

	// Rule 2: B is absent and T is absent -> new_in_source
	if baselineHash == "" && targetHash == "" {
		return FileNewInSource
	}

	// Rule 3: B is absent and T != S (with T present) -> untracked_diff
	if baselineHash == "" && targetHash != "" && targetHash != sourceHash {
		return FileUntrackedDiff
	}

	// Rule 4: S != B and T == B -> source_changed
	if baselineHash != "" && sourceHash != baselineHash && targetHash == baselineHash {
		return FileSourceChanged
	}

	// Rule 5: S == B and T != B (T present) -> target_changed
	if baselineHash != "" && sourceHash == baselineHash && targetHash != "" && targetHash != baselineHash {
		return FileTargetChanged
	}

	// Rule 6: S == B and T absent -> target_missing
	if baselineHash != "" && sourceHash == baselineHash && targetHash == "" {
		return FileTargetMissing
	}

	// Rule 7: S != B and T != B and S != T -> conflict
	if baselineHash != "" && sourceHash != baselineHash && targetHash != baselineHash && sourceHash != targetHash {
		return FileConflict
	}

	// Default fallback
	return FileConflict
}

// ComputeStatus inspects the files in a connection's profile and target directory.
func ComputeStatus(rootPath string, conn config.Connection, st *state.State) (ConnectionStatus, error) {
	status := ConnectionStatus{
		ConnectionID: conn.ID,
		Profile:      conn.Profile,
		TargetPath:   conn.TargetPath,
		Status:       ConnInSync,
		Files:        make([]FileStatus, 0),
	}

	targetInfo, err := os.Stat(conn.TargetPath)
	if err != nil || !targetInfo.IsDir() {
		status.Status = ConnTargetUnavailable
		return status, nil
	}

	profileDir := filepath.Join(rootPath, conn.Profile)
	managed, err := fsops.WalkManaged(profileDir)
	if err != nil {
		return status, err
	}

	var fileStatuses []FileStatus
	hasConflict := false
	hasUntrackedDiff := false
	hasUpload := false
	hasDownload := false

	// Cleanup baselines for files that no longer exist in profile
	if st != nil {
		currentBaselines := st.GetConnectionBaselines(conn.ID)
		for bRel := range currentBaselines {
			if _, ok := managed[bRel]; !ok {
				st.RemoveBaseline(conn.ID, bRel)
			}
		}
	}

	for relPath, absSource := range managed {
		sourceHash, err := fsops.HashFile(absSource)
		if err != nil {
			continue
		}

		targetFile := filepath.Join(conn.TargetPath, filepath.FromSlash(relPath))
		var targetHash string
		if fi, err := os.Stat(targetFile); err == nil && !fi.IsDir() {
			targetHash, _ = fsops.HashFile(targetFile)
		}

		var baselineHash string
		if st != nil {
			baselineHash = st.GetBaseline(conn.ID, relPath)
		}

		fileState := EvaluateFileStatus(sourceHash, targetHash, baselineHash)

		// Auto-update baseline if in_sync and baseline was missing or outdated
		if fileState == FileInSync && st != nil && baselineHash != sourceHash {
			st.SetBaseline(conn.ID, relPath, sourceHash)
			baselineHash = sourceHash
		}

		fileStatuses = append(fileStatuses, FileStatus{
			RelPath:      relPath,
			Status:       fileState,
			SourceHash:   sourceHash,
			TargetHash:   targetHash,
			BaselineHash: baselineHash,
		})

		switch fileState {
		case FileConflict:
			hasConflict = true
		case FileUntrackedDiff:
			hasUntrackedDiff = true
		case FileTargetChanged:
			hasUpload = true
		case FileSourceChanged, FileNewInSource, FileTargetMissing:
			hasDownload = true
		}
	}

	// Sort files alphabetically by relPath
	sort.Slice(fileStatuses, func(i, j int) bool {
		return fileStatuses[i].RelPath < fileStatuses[j].RelPath
	})
	status.Files = fileStatuses

	// Determine aggregated connection status
	if hasConflict {
		status.Status = ConnConflict
	} else if hasUntrackedDiff {
		status.Status = ConnUntrackedDiff
	} else if hasUpload && hasDownload {
		status.Status = ConnDiverged
	} else if hasUpload {
		status.Status = ConnTargetChanged
	} else if hasDownload {
		status.Status = ConnSourceChanged
	} else {
		status.Status = ConnInSync
	}

	return status, nil
}

// Apply applies source files to targetPath with backup and atomic copy.
func Apply(rootPath string, conn config.Connection, allConnections []config.Connection, backupRoot string, files []string, st *state.State, retention int, reason string) error {
	profileDir := filepath.Join(rootPath, conn.Profile)
	if err := os.MkdirAll(conn.TargetPath, fsops.DefaultDirPerms); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	// 1. Take backup of existing target files to be overwritten
	if backupRoot != "" {
		_, _ = backup.CreateBackup(backupRoot, conn, allConnections, files, reason, retention)
	}

	// 2. Atomically copy files from source to destination
	for _, relPath := range files {
		src := filepath.Join(profileDir, filepath.FromSlash(relPath))
		dst := filepath.Join(conn.TargetPath, filepath.FromSlash(relPath))

		if err := fsops.AtomicCopy(src, dst); err != nil {
			return apperr.Wrap(apperr.WriteFailed, err)
		}

		if st != nil {
			hash, err := fsops.HashFile(dst)
			if err == nil {
				st.SetBaseline(conn.ID, relPath, hash)
			}
		}
	}

	if st != nil {
		_ = st.Save()
	}

	return nil
}

// Capture uploads files from targetPath into the profile directory.
func Capture(rootPath string, conn config.Connection, files []string, st *state.State) error {
	profileDir := filepath.Join(rootPath, conn.Profile)
	if err := os.MkdirAll(profileDir, fsops.DefaultDirPerms); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	for _, relPath := range files {
		src := filepath.Join(conn.TargetPath, filepath.FromSlash(relPath))
		dst := filepath.Join(profileDir, filepath.FromSlash(relPath))

		if err := fsops.AtomicCopy(src, dst); err != nil {
			return apperr.Wrap(apperr.WriteFailed, err)
		}

		if st != nil {
			hash, err := fsops.HashFile(dst)
			if err == nil {
				st.SetBaseline(conn.ID, relPath, hash)
			}
		}
	}

	if st != nil {
		_ = st.Save()
	}

	return nil
}
