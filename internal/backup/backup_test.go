package backup_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentsync/internal/backup"
)

func TestCreateListAndPruneSnapshots(t *testing.T) {
	tempDir := t.TempDir()
	backupRoot := filepath.Join(tempDir, "backups")
	targetDir := filepath.Join(tempDir, "target")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	owner := backup.Owner{ID: "rule:r1:d1", Name: "Claude rules", Dir: targetDir}

	// Files that do not exist yet produce no snapshot and no folders.
	info, err := backup.CreateSnapshot(backupRoot, owner, []string{"CLAUDE.md"}, backup.ReasonSync, 20)
	if err != nil {
		t.Fatalf("CreateSnapshot on missing files failed: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil BackupInfo when files do not exist")
	}
	if _, err := os.Stat(backupRoot); !os.IsNotExist(err) {
		t.Errorf("expected no backup folder to be created")
	}

	claudePath := filepath.Join(targetDir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte("version 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := backup.CreateSnapshot(backupRoot, owner, []string{"CLAUDE.md"}, backup.ReasonSync, 20)
	if err != nil || first == nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if first.OwnerID != owner.ID || first.Name != owner.Name {
		t.Errorf("unexpected owner in snapshot info: %+v", first)
	}
	saved, err := os.ReadFile(filepath.Join(first.Path, "CLAUDE.md"))
	if err != nil || string(saved) != "version 1" {
		t.Errorf("snapshot did not keep the file content: %q, %v", saved, err)
	}

	list, err := backup.ListSnapshots(backupRoot, owner)
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(list) != 1 || list[0].Files[0] != "CLAUDE.md" {
		t.Fatalf("expected the one snapshot to list CLAUDE.md, got %+v", list)
	}

	// Retention keeps only the newest snapshots.
	for i := 2; i <= 5; i++ {
		_ = os.WriteFile(claudePath, []byte("version "+string(rune('0'+i))), 0o644)
		time.Sleep(10 * time.Millisecond)
		if _, err := backup.CreateSnapshot(backupRoot, owner, []string{"CLAUDE.md"}, backup.ReasonSync, 3); err != nil {
			t.Fatalf("CreateSnapshot iteration %d failed: %v", i, err)
		}
	}
	pruned, err := backup.ListSnapshots(backupRoot, owner)
	if err != nil {
		t.Fatalf("ListSnapshots failed: %v", err)
	}
	if len(pruned) != 3 {
		t.Fatalf("expected 3 snapshots after pruning with retention=3, got %d", len(pruned))
	}
}

func TestListSnapshotsReadsLegacyManifests(t *testing.T) {
	tempDir := t.TempDir()
	targetDir := filepath.Join(tempDir, "claude")
	snapshotDir := filepath.Join(tempDir, "backups", "claude", "010206-030405PM")
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacy, _ := json.Marshal(map[string]any{
		"connectionId": "conn-1",
		"profile":      "CLAUDE",
		"targetPath":   targetDir,
		"createdAt":    "2026-01-02T03:04:05Z",
		"reason":       "sync",
		"files":        []string{"CLAUDE.md"},
	})
	if err := os.WriteFile(filepath.Join(snapshotDir, backup.ManifestFileName), legacy, 0o644); err != nil {
		t.Fatal(err)
	}

	list, err := backup.ListSnapshots(filepath.Join(tempDir, "backups"), backup.Owner{Dir: targetDir})
	if err != nil || len(list) != 1 {
		t.Fatalf("expected the legacy snapshot to list, got %+v, %v", list, err)
	}
	if list[0].OwnerID != "conn-1" || list[0].Name != "CLAUDE" {
		t.Errorf("legacy keys were not mapped: %+v", list[0])
	}
}

func TestCreateRepoBackupAndPrune(t *testing.T) {
	tempDir := t.TempDir()
	backupRoot := filepath.Join(tempDir, "backups")
	repoRoot := filepath.Join(tempDir, "repo")

	sharedDir := filepath.Join(repoRoot, "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(sharedDir, "AGENTS.md")
	_ = os.WriteFile(filePath, []byte("repo edit v1"), 0o644)

	// Non-existent files return nil
	info, err := backup.CreateRepoBackup(backupRoot, repoRoot, []string{"NOTEXIST.md"}, backup.ReasonDiscardRepoEdit, 5)
	if err != nil || info != nil {
		t.Errorf("expected nil for non-existent file, got %v, %v", info, err)
	}

	// Create 3 snapshots
	for i := 1; i <= 3; i++ {
		_ = os.WriteFile(filePath, []byte("repo edit v"+string(rune('0'+i))), 0o644)
		if _, err := backup.CreateRepoBackup(backupRoot, repoRoot, []string{"shared/AGENTS.md"}, backup.ReasonDiscardRepoEdit, 2); err != nil {
			t.Fatalf("CreateRepoBackup iteration %d failed: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	list, err := backup.ListRepoBackups(backupRoot)
	if err != nil {
		t.Fatal(err)
	}
	// Retention is 2, so only 2 snapshots remain
	if len(list) != 2 {
		t.Errorf("expected 2 snapshots after pruning with retention=2, got %d", len(list))
	}
	if list[0].OwnerID != backup.RepoSnapshotID {
		t.Errorf("expected repo snapshots to be owned by %q, got %q", backup.RepoSnapshotID, list[0].OwnerID)
	}
	if err := backup.PruneRepoBackups(backupRoot, 1); err != nil {
		t.Fatal(err)
	}
	if list, _ = backup.ListRepoBackups(backupRoot); len(list) != 1 {
		t.Errorf("expected 1 snapshot after PruneRepoBackups(1), got %d", len(list))
	}
}
