package syncer_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentsync/internal/config"
	"agentsync/internal/state"
	"agentsync/internal/syncer"
)

func TestEvaluateFileStatusTableRows(t *testing.T) {
	tests := []struct {
		name         string
		sourceHash   string
		targetHash   string
		baselineHash string
		expected     string
	}{
		{
			name:         "Row 1: S == T -> in_sync",
			sourceHash:   "hashA",
			targetHash:   "hashA",
			baselineHash: "hashA",
			expected:     syncer.FileInSync,
		},
		{
			name:         "Row 1 variant: S == T with absent baseline -> in_sync",
			sourceHash:   "hashA",
			targetHash:   "hashA",
			baselineHash: "",
			expected:     syncer.FileInSync,
		},
		{
			name:         "Row 2: B absent, T absent -> new_in_source",
			sourceHash:   "hashA",
			targetHash:   "",
			baselineHash: "",
			expected:     syncer.FileNewInSource,
		},
		{
			name:         "Row 3: B absent, T != S -> untracked_diff",
			sourceHash:   "hashA",
			targetHash:   "hashB",
			baselineHash: "",
			expected:     syncer.FileUntrackedDiff,
		},
		{
			name:         "Row 4: S != B, T == B -> source_changed",
			sourceHash:   "hashNewSource",
			targetHash:   "hashBaseline",
			baselineHash: "hashBaseline",
			expected:     syncer.FileSourceChanged,
		},
		{
			name:         "Row 5: S == B, T != B (T exists) -> target_changed",
			sourceHash:   "hashBaseline",
			targetHash:   "hashModifiedTarget",
			baselineHash: "hashBaseline",
			expected:     syncer.FileTargetChanged,
		},
		{
			name:         "Row 6: S == B, T absent -> target_missing",
			sourceHash:   "hashBaseline",
			targetHash:   "",
			baselineHash: "hashBaseline",
			expected:     syncer.FileTargetMissing,
		},
		{
			name:         "Row 6b (M6): S != B, T absent -> source_changed",
			sourceHash:   "hashNewSource",
			targetHash:   "",
			baselineHash: "hashOldBaseline",
			expected:     syncer.FileSourceChanged,
		},
		{
			name:         "Row 7: S != B, T != B, S != T -> conflict",
			sourceHash:   "hashSourceNew",
			targetHash:   "hashTargetNew",
			baselineHash: "hashOldBaseline",
			expected:     syncer.FileConflict,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := syncer.EvaluateFileStatus(tc.sourceHash, tc.targetHash, tc.baselineHash)
			if got != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, got)
			}
		})
	}
}

func TestComputeStatusAndAggregates(t *testing.T) {
	tempDir := t.TempDir()
	rootDir := filepath.Join(tempDir, "agent-configs")
	targetDir := filepath.Join(tempDir, "target-home")
	profileDir := filepath.Join(rootDir, "CLAUDE")

	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	conn := config.Connection{
		ID:         "conn-1",
		Profile:    "CLAUDE",
		TargetPath: targetDir,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}

	st := state.NewState()

	// 1. Target unavailable when path does not exist
	brokenConn := config.Connection{
		ID:         "conn-broken",
		Profile:    "CLAUDE",
		TargetPath: filepath.Join(tempDir, "nonexistent-target"),
	}
	brokenStatus, err := syncer.ComputeStatus(rootDir, brokenConn, st)
	if err != nil {
		t.Fatal(err)
	}
	if brokenStatus.Status != syncer.ConnTargetUnavailable {
		t.Errorf("expected ConnTargetUnavailable, got %s", brokenStatus.Status)
	}

	// 2. Untracked diff (file in source and file in target with different content, no baseline)
	sourceFile := filepath.Join(profileDir, "CLAUDE.md")
	targetFile := filepath.Join(targetDir, "CLAUDE.md")
	_ = os.WriteFile(sourceFile, []byte("source v1"), 0o644)
	_ = os.WriteFile(targetFile, []byte("target v1"), 0o644)

	status, err := syncer.ComputeStatus(rootDir, conn, st)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != syncer.ConnUntrackedDiff {
		t.Errorf("expected ConnUntrackedDiff, got %s", status.Status)
	}

	// 3. In sync (update target to match source, then reconcile baselines explicitly)
	_ = os.WriteFile(targetFile, []byte("source v1"), 0o644)
	status, err = syncer.ComputeStatus(rootDir, conn, st)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != syncer.ConnInSync {
		t.Errorf("expected ConnInSync, got %s", status.Status)
	}
	// M8: ComputeStatus must be pure (baseline not yet mutated until ReconcileBaselines)
	if st.GetBaseline(conn.ID, "CLAUDE.md") != "" {
		t.Errorf("expected ComputeStatus to not mutate baseline")
	}
	if !syncer.ReconcileBaselines(rootDir, conn, st) {
		t.Errorf("expected ReconcileBaselines to update baseline")
	}

	// 4. Target changed (edit target locally)
	_ = os.WriteFile(targetFile, []byte("target edited"), 0o644)
	status, err = syncer.ComputeStatus(rootDir, conn, st)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != syncer.ConnTargetChanged {
		t.Errorf("expected ConnTargetChanged, got %s", status.Status)
	}

	// 5. Diverged (add another file that is new_in_source or source_changed)
	extraSource := filepath.Join(profileDir, "commands", "review.md")
	_ = os.MkdirAll(filepath.Dir(extraSource), 0o755)
	_ = os.WriteFile(extraSource, []byte("review command"), 0o644)

	status, err = syncer.ComputeStatus(rootDir, conn, st)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != syncer.ConnDiverged {
		t.Errorf("expected ConnDiverged, got %s", status.Status)
	}
}

func TestApplyAndCapture(t *testing.T) {
	tempDir := t.TempDir()
	rootDir := filepath.Join(tempDir, "root")
	targetDir := filepath.Join(tempDir, "target")
	backupDir := filepath.Join(tempDir, "backups")
	profileDir := filepath.Join(rootDir, "CLAUDE")

	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.MkdirAll(targetDir, 0o755)

	conn := config.Connection{
		ID:         "conn-apply-test",
		Profile:    "CLAUDE",
		TargetPath: targetDir,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}
	allConns := []config.Connection{conn}
	st := state.NewState()

	// Create source file
	sourceFile := filepath.Join(profileDir, "CLAUDE.md")
	_ = os.WriteFile(sourceFile, []byte("source content"), 0o644)

	// Existing target file to be backed up
	targetFile := filepath.Join(targetDir, "CLAUDE.md")
	_ = os.WriteFile(targetFile, []byte("pre-existing target"), 0o644)

	// C1: Apply with invalid backupRoot (a file instead of directory) must abort BEFORE overwriting target
	invalidBackupRoot := filepath.Join(tempDir, "file-as-backup-root")
	_ = os.WriteFile(invalidBackupRoot, []byte("bloqueo"), 0o644)
	if _, err := syncer.Apply(rootDir, conn, allConns, invalidBackupRoot, []string{"CLAUDE.md"}, st, 20, "apply"); err == nil {
		t.Fatalf("expected Apply to fail when backup fails (C1)")
	}
	contentAfterFailedBackup, _ := os.ReadFile(targetFile)
	if string(contentAfterFailedBackup) != "pre-existing target" {
		t.Fatalf("expected target to remain intact ('pre-existing target') after backup failure, got '%s'", contentAfterFailedBackup)
	}

	// Apply with valid backupDir
	bInfo, err := syncer.Apply(rootDir, conn, allConns, backupDir, []string{"CLAUDE.md"}, st, 20, "apply")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	if bInfo == nil || bInfo.ID == "" {
		t.Errorf("expected non-empty BackupInfo.ID on Apply (M5)")
	}

	// Verify target now has source content
	content, _ := os.ReadFile(targetFile)
	if string(content) != "source content" {
		t.Errorf("expected 'source content', got '%s'", content)
	}

	// Verify baseline was recorded
	baseline := st.GetBaseline(conn.ID, "CLAUDE.md")
	if baseline == "" {
		t.Errorf("expected baseline to be recorded")
	}

	// Modify target and test Capture
	_ = os.WriteFile(targetFile, []byte("locally updated content"), 0o644)
	err = syncer.Capture(rootDir, conn, []string{"CLAUDE.md"}, st)
	if err != nil {
		t.Fatalf("Capture failed: %v", err)
	}

	// Verify profile source now has the captured content
	srcContent, _ := os.ReadFile(sourceFile)
	if string(srcContent) != "locally updated content" {
		t.Errorf("expected 'locally updated content', got '%s'", srcContent)
	}
}

func TestDeleteFileFromProfileBaselineCleanup(t *testing.T) {
	tempDir := t.TempDir()
	rootDir := filepath.Join(tempDir, "root")
	targetDir := filepath.Join(tempDir, "target")
	profileDir := filepath.Join(rootDir, "CLAUDE")

	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.MkdirAll(targetDir, 0o755)

	conn := config.Connection{
		ID:         "conn-del-test",
		Profile:    "CLAUDE",
		TargetPath: targetDir,
	}

	// Two files in profile
	file1 := filepath.Join(profileDir, "CLAUDE.md")
	file2 := filepath.Join(profileDir, "OLD.md")
	_ = os.WriteFile(file1, []byte("file1"), 0o644)
	_ = os.WriteFile(file2, []byte("file2"), 0o644)

	st := state.NewState()
	st.SetBaseline(conn.ID, "CLAUDE.md", "hash1")
	st.SetBaseline(conn.ID, "OLD.md", "hash2")

	// Now delete OLD.md from profile (simulating file removed from profile)
	_ = os.Remove(file2)

	// Target still has OLD.md
	targetOld := filepath.Join(targetDir, "OLD.md")
	_ = os.WriteFile(targetOld, []byte("file2"), 0o644)

	// Pure ComputeStatus should NOT mutate state
	_, err := syncer.ComputeStatus(rootDir, conn, st)
	if err != nil {
		t.Fatal(err)
	}
	if st.GetBaseline(conn.ID, "OLD.md") == "" {
		t.Errorf("ComputeStatus should be pure and not delete baseline directly (M8)")
	}

	// ReconcileBaselines cleans up deleted file baseline
	syncer.ReconcileBaselines(rootDir, conn, st)

	// Target OLD.md MUST still exist (rule 4: never delete)
	if _, err := os.Stat(targetOld); os.IsNotExist(err) {
		t.Errorf("target file should NOT be deleted")
	}

	// OLD.md baseline MUST be removed from state
	if st.GetBaseline(conn.ID, "OLD.md") != "" {
		t.Errorf("expected OLD.md baseline to be deleted from state")
	}
}
