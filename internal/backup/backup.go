package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/state"
)

const (
	BackupTimestampLayout = "010206-030405PM"
	ManifestFileName      = "manifest.json"
	ReasonApply           = "apply"
	ReasonSync            = "sync"
	ReasonRestore         = "restore"
	ReasonInitialLink     = "initial_link"
)

// Manifest defines the metadata stored with each backup snapshot.
type Manifest struct {
	ConnectionID string   `json:"connectionId"`
	Profile      string   `json:"profile"`
	TargetPath   string   `json:"targetPath"`
	CreatedAt    string   `json:"createdAt"`
	Reason       string   `json:"reason"`
	Files        []string `json:"files"`
}

// BackupInfo represents a backup item listed for display.
type BackupInfo struct {
	ID           string   `json:"id"`
	ConnectionID string   `json:"connectionId"`
	Profile      string   `json:"profile"`
	TargetPath   string   `json:"targetPath"`
	CreatedAt    string   `json:"createdAt"`
	Reason       string   `json:"reason"`
	Files        []string `json:"files"`
	Path         string   `json:"path"`
}

// TargetFolderID calculates the directory name under backupPath for a connection.
// If another connection shares the same folder name, it appends '__' and the first 6 chars of connectionID.
func TargetFolderID(targetPath string, connectionID string, allConnections []config.Connection) string {
	baseName := filepath.Base(filepath.Clean(targetPath))
	if baseName == "." || baseName == "/" || baseName == "\\" {
		baseName = "root"
	}

	collision := false
	for _, c := range allConnections {
		if c.ID != connectionID && filepath.Base(filepath.Clean(c.TargetPath)) == baseName {
			collision = true
			break
		}
	}

	if collision {
		prefix := connectionID
		if len(prefix) > 6 {
			prefix = prefix[:6]
		}
		return fmt.Sprintf("%s__%s", baseName, prefix)
	}

	return baseName
}

// FormatTimestamp creates the timestamp folder name according to the spec, adding suffix if duplicate.
func FormatTimestamp(t time.Time, parentDir string) string {
	base := t.Format(BackupTimestampLayout)
	target := filepath.Join(parentDir, base)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return base
	}

	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s-%d", base, suffix)
		if _, err := os.Stat(filepath.Join(parentDir, candidate)); os.IsNotExist(err) {
			return candidate
		}
	}
}

// CreateBackup creates a backup of existing target files before they are overwritten.
// If none of the files exist in the target, no backup directory is created.
func CreateBackup(backupRoot string, conn config.Connection, allConnections []config.Connection, files []string, reason string, retention int) (*BackupInfo, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// Filter files that actually exist in the target directory
	var existingFiles []string
	for _, relPath := range files {
		targetFile := filepath.Join(conn.TargetPath, filepath.FromSlash(relPath))
		if fi, err := os.Stat(targetFile); err == nil && !fi.IsDir() {
			existingFiles = append(existingFiles, relPath)
		}
	}

	if len(existingFiles) == 0 {
		return nil, nil
	}

	targetDirName := TargetFolderID(conn.TargetPath, conn.ID, allConnections)
	connBackupDir := filepath.Join(backupRoot, targetDirName)
	if err := os.MkdirAll(connBackupDir, fsops.DefaultDirPerms); err != nil {
		return nil, apperr.Wrap(apperr.BackupFailed, err)
	}

	now := time.Now()
	backupFolderID := FormatTimestamp(now, connBackupDir)
	snapshotDir := filepath.Join(connBackupDir, backupFolderID)

	if err := os.MkdirAll(snapshotDir, fsops.DefaultDirPerms); err != nil {
		return nil, apperr.Wrap(apperr.BackupFailed, err)
	}

	for _, relPath := range existingFiles {
		src := filepath.Join(conn.TargetPath, filepath.FromSlash(relPath))
		dst := filepath.Join(snapshotDir, filepath.FromSlash(relPath))
		if err := fsops.AtomicCopy(src, dst); err != nil {
			_ = os.RemoveAll(snapshotDir)
			return nil, apperr.Wrap(apperr.BackupFailed, err)
		}
	}

	manifest := Manifest{
		ConnectionID: conn.ID,
		Profile:      conn.Profile,
		TargetPath:   conn.TargetPath,
		CreatedAt:    config.FormatRFC3339(now),
		Reason:       reason,
		Files:        existingFiles,
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = os.RemoveAll(snapshotDir)
		return nil, apperr.Wrap(apperr.BackupFailed, err)
	}

	manifestPath := filepath.Join(snapshotDir, ManifestFileName)
	if err := fsops.AtomicWrite(manifestPath, manifestData, fsops.DefaultFilePerms); err != nil {
		_ = os.RemoveAll(snapshotDir)
		return nil, apperr.Wrap(apperr.BackupFailed, err)
	}

	// Prune older backups according to retention
	if retention > 0 {
		_ = PruneBackups(backupRoot, conn, allConnections, retention)
	}

	return &BackupInfo{
		ID:           backupFolderID,
		ConnectionID: conn.ID,
		Profile:      conn.Profile,
		TargetPath:   conn.TargetPath,
		CreatedAt:    manifest.CreatedAt,
		Reason:       reason,
		Files:        existingFiles,
		Path:         snapshotDir,
	}, nil
}

// ListBackups returns all snapshots for a connection, ordered newest first.
func ListBackups(backupRoot string, conn config.Connection, allConnections []config.Connection) ([]BackupInfo, error) {
	targetDirName := TargetFolderID(conn.TargetPath, conn.ID, allConnections)
	connBackupDir := filepath.Join(backupRoot, targetDirName)

	entries, err := os.ReadDir(connBackupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, apperr.Wrap(apperr.BackupFailed, err)
	}

	var results []BackupInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		snapshotDir := filepath.Join(connBackupDir, entry.Name())
		manifestPath := filepath.Join(snapshotDir, ManifestFileName)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}

		results = append(results, BackupInfo{
			ID:           entry.Name(),
			ConnectionID: manifest.ConnectionID,
			Profile:      manifest.Profile,
			TargetPath:   manifest.TargetPath,
			CreatedAt:    manifest.CreatedAt,
			Reason:       manifest.Reason,
			Files:        manifest.Files,
			Path:         snapshotDir,
		})
	}

	// Sort descending by CreatedAt, using ID as tie-breaker
	sort.Slice(results, func(i, j int) bool {
		if results[i].CreatedAt != results[j].CreatedAt {
			return results[i].CreatedAt > results[j].CreatedAt
		}
		return results[i].ID > results[j].ID
	})

	return results, nil
}

// PruneBackups keeps only the newest retentionCount snapshots, removing older ones.
func PruneBackups(backupRoot string, conn config.Connection, allConnections []config.Connection, retentionCount int) error {
	if retentionCount <= 0 {
		return nil
	}

	list, err := ListBackups(backupRoot, conn, allConnections)
	if err != nil {
		return err
	}

	if len(list) <= retentionCount {
		return nil
	}

	toRemove := list[retentionCount:]
	for _, b := range toRemove {
		_ = os.RemoveAll(b.Path)
	}

	return nil
}

// RestoreBackup restores files from a snapshot back to targetPath.
// Before restoring, it takes a backup snapshot of current target with ReasonRestore.
// It does NOT touch state baselines so that restored files remain target_changed and will be uploaded on the next sync.
func RestoreBackup(backupRoot string, conn config.Connection, allConnections []config.Connection, backupID string, st *state.State, retention int) error {
	targetDirName := TargetFolderID(conn.TargetPath, conn.ID, allConnections)
	snapshotDir := filepath.Join(backupRoot, targetDirName, backupID)

	manifestPath := filepath.Join(snapshotDir, ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return apperr.Wrap(apperr.BackupFailed, fmt.Errorf("backup snapshot not found: %w", err))
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return apperr.Wrap(apperr.BackupFailed, err)
	}

	// 1. Create mandatory backup of current target state before restoring
	if _, err := CreateBackup(backupRoot, conn, allConnections, manifest.Files, ReasonRestore, retention); err != nil {
		return err
	}

	// 2. Restore each file from snapshot without modifying baseline
	for _, relPath := range manifest.Files {
		src := filepath.Join(snapshotDir, filepath.FromSlash(relPath))
		dst := filepath.Join(conn.TargetPath, filepath.FromSlash(relPath))

		if err := fsops.AtomicCopy(src, dst); err != nil {
			return apperr.Wrap(apperr.WriteFailed, err)
		}
	}

	return nil
}
