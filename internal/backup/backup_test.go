package backup_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/state"
)

func TestTargetFolderID(t *testing.T) {
	conn1 := config.Connection{ID: "111111-aaa", TargetPath: "C:\\path\\.claude"}
	conn2 := config.Connection{ID: "222222-bbb", TargetPath: "D:\\other\\.claude"}
	conn3 := config.Connection{ID: "333333-ccc", TargetPath: "E:\\standalone\\.gemini"}

	all := []config.Connection{conn1, conn2, conn3}

	// No collision
	id3 := backup.TargetFolderID(conn3.TargetPath, conn3.ID, all)
	if id3 != ".gemini" {
		t.Errorf("expected .gemini, got %s", id3)
	}

	// Collision between conn1 and conn2
	id1 := backup.TargetFolderID(conn1.TargetPath, conn1.ID, all)
	if id1 != ".claude__111111" {
		t.Errorf("expected .claude__111111, got %s", id1)
	}
}

func TestCreateListPruneAndRestoreBackup(t *testing.T) {
	tempDir := t.TempDir()
	backupRoot := filepath.Join(tempDir, "backups")
	targetDir := filepath.Join(tempDir, "target")

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	conn := config.Connection{
		ID:         "conn-claude",
		Profile:    "CLAUDE",
		TargetPath: targetDir,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}

	// 1. Target files do not exist yet -> CreateBackup should return nil without creating folders
	info, err := backup.CreateBackup(backupRoot, conn, []config.Connection{conn}, []string{"CLAUDE.md"}, backup.ReasonApply, 20)
	if err != nil {
		t.Fatalf("CreateBackup on non-existent files failed: %v", err)
	}
	if info != nil {
		t.Errorf("expected nil BackupInfo when files do not exist")
	}

	// 2. Create target file and take backup
	claudePath := filepath.Join(targetDir, "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte("version 1"), 0o644); err != nil {
		t.Fatal(err)
	}

	b1, err := backup.CreateBackup(backupRoot, conn, []config.Connection{conn}, []string{"CLAUDE.md"}, backup.ReasonApply, 20)
	if err != nil || b1 == nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	list, err := backup.ListBackups(backupRoot, conn, []config.Connection{conn})
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(list))
	}
	if list[0].Files[0] != "CLAUDE.md" {
		t.Errorf("expected CLAUDE.md in backup list")
	}

	// 3. Test Pruning (retention = 3, create 4 more backups)
	for i := 2; i <= 5; i++ {
		// Modify target content
		_ = os.WriteFile(claudePath, []byte("version "+string(rune('0'+i))), 0o644)
		// Small sleep to ensure timestamp differences if needed
		time.Sleep(10 * time.Millisecond)
		_, err := backup.CreateBackup(backupRoot, conn, []config.Connection{conn}, []string{"CLAUDE.md"}, backup.ReasonSync, 3)
		if err != nil {
			t.Fatalf("CreateBackup iteration %d failed: %v", i, err)
		}
	}

	listPruned, err := backup.ListBackups(backupRoot, conn, []config.Connection{conn})
	if err != nil {
		t.Fatalf("ListBackups failed: %v", err)
	}
	if len(listPruned) != 3 {
		t.Fatalf("expected 3 backups after pruning with retention=3, got %d", len(listPruned))
	}

	// 4. Test RestoreBackup (C4: must NOT touch baseline, leaving restored file as target_changed)
	st := state.NewState()
	st.SetBaseline(conn.ID, "CLAUDE.md", "baseline-hash-before-restore")
	// Change target to version modified
	if err := os.WriteFile(claudePath, []byte("modified version"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Restore the oldest of the 3 preserved backups
	oldestBackupID := listPruned[len(listPruned)-1].ID
	if err := backup.RestoreBackup(backupRoot, conn, []config.Connection{conn}, oldestBackupID, st, 10); err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	restoredContent, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(restoredContent) == "modified version" {
		t.Errorf("file was not restored to older content")
	}

	// Baseline MUST remain untouched after restore (C4)
	baseline := st.GetBaseline(conn.ID, "CLAUDE.md")
	if baseline != "baseline-hash-before-restore" {
		t.Errorf("expected baseline to remain untouched ('baseline-hash-before-restore'), got '%s'", baseline)
	}

	// 5. Test RestoreBackup aborts if pre-restore CreateBackup fails (C1)
	badBackupFile := filepath.Join(tempDir, "not-a-dir-file")
	_ = os.WriteFile(badBackupFile, []byte("file"), 0o644)
	if err := backup.RestoreBackup(badBackupFile, conn, []config.Connection{conn}, oldestBackupID, st, 10); err == nil {
		t.Errorf("expected RestoreBackup to fail when backupRoot is a file")
	}
}

func TestCreateRepoBackupAndPrune(t *testing.T) {
	tempDir := t.TempDir()
	backupRoot := filepath.Join(tempDir, "backups")
	repoRoot := filepath.Join(tempDir, "repo")

	claudeDir := filepath.Join(repoRoot, "CLAUDE")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(claudeDir, "CLAUDE.md")
	_ = os.WriteFile(filePath, []byte("repo edit v1"), 0o644)

	// Non-existent files return nil
	info, err := backup.CreateRepoBackup(backupRoot, repoRoot, []string{"NOTEXIST.md"}, backup.ReasonDiscardRepoEdit, 5)
	if err != nil || info != nil {
		t.Errorf("expected nil for non-existent file, got %v, %v", info, err)
	}

	// Create 3 snapshots
	for i := 1; i <= 3; i++ {
		_ = os.WriteFile(filePath, []byte("repo edit v"+string(rune('0'+i))), 0o644)
		_, err := backup.CreateRepoBackup(backupRoot, repoRoot, []string{"CLAUDE/CLAUDE.md"}, backup.ReasonDiscardRepoEdit, 2)
		if err != nil {
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
}
