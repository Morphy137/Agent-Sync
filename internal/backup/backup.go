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
)

const (
	BackupTimestampLayout = "010206-030405PM"
	ManifestFileName      = "manifest.json"
	ReasonSync            = "sync"
	ReasonDiscardRepoEdit = "discard_repo_edit"

	// RepoSnapshotID is the owner id and folder name of snapshots of discarded repository edits.
	RepoSnapshotID = "_repo"
)

// Manifest defines the metadata stored with each backup snapshot. The legacy fields are read from
// snapshots written by earlier versions and are never written back.
type Manifest struct {
	OwnerID    string   `json:"ownerId"`
	Name       string   `json:"name"`
	TargetPath string   `json:"targetPath"`
	CreatedAt  string   `json:"createdAt"`
	Reason     string   `json:"reason"`
	Files      []string `json:"files"`

	LegacyConnectionID string `json:"connectionId,omitempty"`
	LegacyProfile      string `json:"profile,omitempty"`
}

// BackupInfo represents a backup item listed for display.
type BackupInfo struct {
	ID         string   `json:"id"`
	OwnerID    string   `json:"ownerId"`
	Name       string   `json:"name"`
	TargetPath string   `json:"targetPath"`
	CreatedAt  string   `json:"createdAt"`
	Reason     string   `json:"reason"`
	Files      []string `json:"files"`
	Path       string   `json:"path"`
}

// Owner identifies what a snapshot belongs to: a rule destination, a rule source or the repository.
type Owner struct {
	ID   string
	Name string
	// Dir is the folder whose files are snapshotted; files are stored relative to it.
	Dir string
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

// snapshotFolderName is the folder under the backup root that groups the snapshots of an owner.
func snapshotFolderName(dir string) string {
	baseName := filepath.Base(filepath.Clean(dir))
	if baseName == "." || baseName == "/" || baseName == "\\" {
		return "root"
	}
	return baseName
}

// CreateSnapshot backs up the files of owner.Dir (relative paths, forward slashes) that exist before
// they are overwritten or deleted. When none of them exist no snapshot is created.
func CreateSnapshot(backupRoot string, owner Owner, files []string, reason string, retention int) (*BackupInfo, error) {
	return writeSnapshot(filepath.Join(backupRoot, snapshotFolderName(owner.Dir)), owner, files, reason, retention)
}

// writeSnapshot stores a snapshot under groupDir and prunes the group down to retention snapshots.
func writeSnapshot(groupDir string, owner Owner, files []string, reason string, retention int) (*BackupInfo, error) {
	var existingFiles []string
	for _, relPath := range files {
		sourceFile := filepath.Join(owner.Dir, filepath.FromSlash(relPath))
		if fi, err := os.Stat(sourceFile); err == nil && !fi.IsDir() {
			existingFiles = append(existingFiles, relPath)
		}
	}
	if len(existingFiles) == 0 {
		return nil, nil
	}

	if err := os.MkdirAll(groupDir, fsops.DefaultDirPerms); err != nil {
		return nil, apperr.Wrap(apperr.BackupFailed, err)
	}

	now := time.Now()
	snapshotID := FormatTimestamp(now, groupDir)
	snapshotDir := filepath.Join(groupDir, snapshotID)
	if err := os.MkdirAll(snapshotDir, fsops.DefaultDirPerms); err != nil {
		return nil, apperr.Wrap(apperr.BackupFailed, err)
	}

	for _, relPath := range existingFiles {
		src := filepath.Join(owner.Dir, filepath.FromSlash(relPath))
		dst := filepath.Join(snapshotDir, filepath.FromSlash(relPath))
		if err := fsops.AtomicCopy(src, dst); err != nil {
			_ = os.RemoveAll(snapshotDir)
			return nil, apperr.Wrap(apperr.BackupFailed, err)
		}
	}

	manifest := Manifest{
		OwnerID:    owner.ID,
		Name:       owner.Name,
		TargetPath: owner.Dir,
		CreatedAt:  config.FormatRFC3339(now),
		Reason:     reason,
		Files:      existingFiles,
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

	if retention > 0 {
		_ = pruneGroup(groupDir, retention)
	}

	return &BackupInfo{
		ID:         snapshotID,
		OwnerID:    owner.ID,
		Name:       owner.Name,
		TargetPath: owner.Dir,
		CreatedAt:  manifest.CreatedAt,
		Reason:     reason,
		Files:      existingFiles,
		Path:       snapshotDir,
	}, nil
}

// readManifest loads the manifest of a snapshot folder, accepting snapshots from earlier versions.
func readManifest(snapshotDir string) (Manifest, error) {
	var manifest Manifest
	data, err := os.ReadFile(filepath.Join(snapshotDir, ManifestFileName))
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.OwnerID == "" {
		manifest.OwnerID = manifest.LegacyConnectionID
	}
	if manifest.Name == "" {
		manifest.Name = manifest.LegacyProfile
	}
	return manifest, nil
}

// listGroup returns the snapshots stored directly under groupDir, newest first.
func listGroup(groupDir string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(groupDir)
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
		snapshotDir := filepath.Join(groupDir, entry.Name())
		manifest, err := readManifest(snapshotDir)
		if err != nil {
			continue
		}
		results = append(results, BackupInfo{
			ID:         entry.Name(),
			OwnerID:    manifest.OwnerID,
			Name:       manifest.Name,
			TargetPath: manifest.TargetPath,
			CreatedAt:  manifest.CreatedAt,
			Reason:     manifest.Reason,
			Files:      manifest.Files,
			Path:       snapshotDir,
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

// pruneGroup keeps only the newest retentionCount snapshots under groupDir.
func pruneGroup(groupDir string, retentionCount int) error {
	if retentionCount <= 0 {
		return nil
	}
	list, err := listGroup(groupDir)
	if err != nil {
		return err
	}
	if len(list) <= retentionCount {
		return nil
	}
	for _, b := range list[retentionCount:] {
		_ = os.RemoveAll(b.Path)
	}
	return nil
}

// ListSnapshots returns the snapshots of owner stored under backupRoot, newest first.
func ListSnapshots(backupRoot string, owner Owner) ([]BackupInfo, error) {
	return listGroup(filepath.Join(backupRoot, snapshotFolderName(owner.Dir)))
}

// repoGroupDir is the folder holding snapshots of discarded repository edits.
func repoGroupDir(backupRoot string) string {
	return filepath.Join(backupRoot, RepoSnapshotID)
}

// CreateRepoBackup backs up uncommitted manual edits in the repository before they are discarded.
func CreateRepoBackup(backupRoot string, repoRoot string, files []string, reason string, retention int) (*BackupInfo, error) {
	owner := Owner{ID: RepoSnapshotID, Name: RepoSnapshotID, Dir: repoRoot}
	return writeSnapshot(repoGroupDir(backupRoot), owner, files, reason, retention)
}

// ListRepoBackups returns all snapshots for discarded repo edits, ordered newest first.
func ListRepoBackups(backupRoot string) ([]BackupInfo, error) {
	return listGroup(repoGroupDir(backupRoot))
}

// PruneRepoBackups keeps only the newest retentionCount snapshots of repo discards.
func PruneRepoBackups(backupRoot string, retentionCount int) error {
	return pruneGroup(repoGroupDir(backupRoot), retentionCount)
}
