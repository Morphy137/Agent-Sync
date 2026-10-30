package orchestrator_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/scheduler"
	"agentsync/internal/state"
)

func setupTwoClonesWithBareRemote(t *testing.T) (
	bareRemote string,
	cfgA *config.Config, stA *state.State, orchA *orchestrator.Orchestrator,
	cfgB *config.Config, stB *state.State, orchB *orchestrator.Orchestrator,
) {
	t.Helper()
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}

	tempDir := t.TempDir()
	bareRemote = filepath.Join(tempDir, "remote.git")
	if err := exec.Command("git", "init", "--bare", bareRemote).Run(); err != nil {
		t.Fatal(err)
	}

	// Setup PC-A
	pcADir := filepath.Join(tempDir, "pc-a")
	repoA := filepath.Join(pcADir, "agent-configs")
	targetA := filepath.Join(pcADir, "home", ".claude")
	backupA := filepath.Join(pcADir, "backups")
	_ = os.MkdirAll(targetA, 0o755)
	_ = os.MkdirAll(filepath.Join(repoA, "CLAUDE"), 0o755)

	if err := gitops.Init(repoA); err != nil {
		t.Fatal(err)
	}
	runnerA := gitops.NewRunner(repoA)
	_ = runnerA.SetLocalIdentity("PC-A", "pca@local")
	_ = os.WriteFile(filepath.Join(repoA, "CLAUDE", "CLAUDE.md"), []byte("# Initial v1\n"), 0o644)
	if err := runnerA.CommitPaths([]string{"."}, "chore: initial commit"); err != nil {
		t.Fatal(err)
	}
	if err := runnerA.AddRemote(bareRemote); err != nil {
		t.Fatal(err)
	}
	if err := runnerA.PushSetUpstream(); err != nil {
		t.Fatal(err)
	}

	_ = os.WriteFile(filepath.Join(targetA, "CLAUDE.md"), []byte("# Initial v1\n"), 0o644)
	initHash, _ := fsops.HashFile(filepath.Join(repoA, "CLAUDE", "CLAUDE.md"))

	connA := config.Connection{
		ID:         "conn-a",
		Profile:    "CLAUDE",
		TargetPath: targetA,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}
	cfgA = &config.Config{
		Version:           1,
		RootPath:          repoA,
		BackupPath:        backupA,
		BackupRetention:   20,
		ConfirmBeforeSync: false,
		Connections:       []config.Connection{connA},
	}
	stA = state.NewState()
	stA.SetBaseline(connA.ID, "CLAUDE.md", initHash)

	// Setup PC-B
	pcBDir := filepath.Join(tempDir, "pc-b")
	repoB := filepath.Join(pcBDir, "agent-configs")
	targetB := filepath.Join(pcBDir, "home", ".claude")
	backupB := filepath.Join(pcBDir, "backups")
	_ = os.MkdirAll(targetB, 0o755)

	if err := exec.Command("git", "clone", bareRemote, repoB).Run(); err != nil {
		t.Fatal(err)
	}
	runnerB := gitops.NewRunner(repoB)
	_ = runnerB.SetLocalIdentity("PC-B", "pcb@local")
	clonedContentB, _ := os.ReadFile(filepath.Join(repoB, "CLAUDE", "CLAUDE.md"))
	_ = os.WriteFile(filepath.Join(targetB, "CLAUDE.md"), clonedContentB, 0o644)
	initHashB, _ := fsops.HashFile(filepath.Join(repoB, "CLAUDE", "CLAUDE.md"))

	connB := config.Connection{
		ID:         "conn-b",
		Profile:    "CLAUDE",
		TargetPath: targetB,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}
	cfgB = &config.Config{
		Version:           1,
		RootPath:          repoB,
		BackupPath:        backupB,
		BackupRetention:   20,
		ConfirmBeforeSync: false,
		Connections:       []config.Connection{connB},
	}
	stB = state.NewState()
	stB.SetBaseline(connB.ID, "CLAUDE.md", initHashB)

	orchA = orchestrator.New()
	orchB = orchestrator.New()
	return
}

func TestTwoClientSyncIntegration(t *testing.T) {
	_, cfgA, stA, orchA, cfgB, stB, orchB := setupTwoClonesWithBareRemote(t)

	// PC-A modifies file in target
	fileInTargetA := filepath.Join(cfgA.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(fileInTargetA, []byte("# Config from PC-A"), 0o644)

	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatalf("PlanSync on PC-A failed: %v", err)
	}
	if planA.Direction != orchestrator.DirectionUp {
		t.Errorf("expected Up direction on PC-A, got %s", planA.Direction)
	}

	resA, err := orchA.ExecuteSync(cfgA, stA, planA.Fingerprint, nil)
	if err != nil {
		t.Fatalf("ExecuteSync on PC-A failed: %v", err)
	}
	if resA.Captured != 1 || resA.Pushed == 0 {
		t.Errorf("expected 1 captured file and push on PC-A, got %+v", resA)
	}

	// PC-B Plans and Executes Sync
	planB, err := orchB.PlanSync(cfgB, stB)
	if err != nil {
		t.Fatalf("PlanSync on PC-B failed: %v", err)
	}
	if planB.Behind == 0 {
		t.Errorf("expected PC-B to be behind after PC-A push")
	}

	resB, err := orchB.ExecuteSync(cfgB, stB, planB.Fingerprint, nil)
	if err != nil {
		t.Fatalf("ExecuteSync on PC-B failed: %v", err)
	}
	if resB.Pulled == 0 || resB.Applied != 1 {
		t.Errorf("expected pulled commits and 1 applied file on PC-B, got %+v", resB)
	}
	// M5: BackupSessions must contain actual BackupInfo.ID, not connection ID
	if len(resB.BackupSessions) != 1 || resB.BackupSessions[0] == cfgB.Connections[0].ID {
		t.Errorf("expected BackupSessions to contain BackupInfo.ID (not conn.ID), got %v", resB.BackupSessions)
	}

	fileInTargetB := filepath.Join(cfgB.Connections[0].TargetPath, "CLAUDE.md")
	contentB, err := os.ReadFile(fileInTargetB)
	if err != nil || string(contentB) != "# Config from PC-A" {
		t.Fatalf("expected PC-B target to have '# Config from PC-A', got '%s'", string(contentB))
	}

	// Test Scheduler callback
	var checkExecuted bool
	sched := scheduler.NewScheduler(func() bool {
		checkExecuted = true
		return true
	})
	sched.RunCheck(true)
	if !checkExecuted {
		t.Errorf("expected scheduler check to run")
	}
}

// A.1 Test 1 (C5): keep_local with remote overlap -> succeeds without rebase conflict, remote receives local version, PC-A target unchanged.
func TestKeepLocalWithRemoteOverlap(t *testing.T) {
	_, cfgA, stA, orchA, cfgB, stB, orchB := setupTwoClonesWithBareRemote(t)

	// PC-B edits CLAUDE.md and pushes to remote
	targetFileB := filepath.Join(cfgB.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(targetFileB, []byte("# Version from PC-B\n"), 0o644)
	planB, err := orchB.PlanSync(cfgB, stB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchB.ExecuteSync(cfgB, stB, planB.Fingerprint, nil); err != nil {
		t.Fatalf("PC-B sync failed: %v", err)
	}

	// PC-A also edits CLAUDE.md locally in target
	targetFileA := filepath.Join(cfgA.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(targetFileA, []byte("# Version kept on PC-A\n"), 0o644)

	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}
	if planA.Direction != orchestrator.DirectionConflict {
		t.Fatalf("expected conflict direction on PC-A, got %s", planA.Direction)
	}

	resolutions := []orchestrator.ConflictResolution{
		{
			Profile:    "CLAUDE",
			RelPath:    "CLAUDE.md",
			Resolution: "keep_local",
		},
	}

	resA, err := orchA.ExecuteSync(cfgA, stA, planA.Fingerprint, resolutions)
	if err != nil {
		t.Fatalf("ExecuteSync with keep_local on remote overlap failed (C5): %v", err)
	}
	if resA.Captured != 1 || resA.Pushed == 0 {
		t.Errorf("expected 1 captured and pushed commit with keep_local, got %+v", resA)
	}

	// Verify PC-A target was NOT overwritten
	contentA, _ := os.ReadFile(targetFileA)
	if string(contentA) != "# Version kept on PC-A\n" {
		t.Errorf("expected PC-A target to remain '# Version kept on PC-A\\n', got '%s'", contentA)
	}

	// Verify PC-B pulls and receives PC-A's version from remote
	planB2, err := orchB.PlanSync(cfgB, stB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchB.ExecuteSync(cfgB, stB, planB2.Fingerprint, nil); err != nil {
		t.Fatalf("PC-B second sync failed: %v", err)
	}
	contentB, _ := os.ReadFile(targetFileB)
	if strings.TrimSpace(string(contentB)) != "# Version kept on PC-A" {
		t.Errorf("expected remote / PC-B to have '# Version kept on PC-A', got '%s'", contentB)
	}
}

// A.1 Test 2: keep_repo with remote overlap -> target receives remote version and a backup of local version is created.
func TestKeepRepoWithRemoteOverlap(t *testing.T) {
	_, cfgA, stA, orchA, cfgB, stB, orchB := setupTwoClonesWithBareRemote(t)

	targetFileB := filepath.Join(cfgB.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(targetFileB, []byte("# Remote version from PC-B\n"), 0o644)
	planB, _ := orchB.PlanSync(cfgB, stB)
	if _, err := orchB.ExecuteSync(cfgB, stB, planB.Fingerprint, nil); err != nil {
		t.Fatal(err)
	}

	targetFileA := filepath.Join(cfgA.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(targetFileA, []byte("# Local version on PC-A to be backed up\n"), 0o644)

	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}

	resolutions := []orchestrator.ConflictResolution{
		{
			Profile:    "CLAUDE",
			RelPath:    "CLAUDE.md",
			Resolution: "keep_repo",
		},
	}

	resA, err := orchA.ExecuteSync(cfgA, stA, planA.Fingerprint, resolutions)
	if err != nil {
		t.Fatalf("ExecuteSync with keep_repo failed: %v", err)
	}
	if resA.Applied != 1 || len(resA.BackupSessions) != 1 {
		t.Errorf("expected 1 applied file and 1 backup session, got %+v", resA)
	}

	contentA, _ := os.ReadFile(targetFileA)
	if strings.TrimSpace(string(contentA)) != "# Remote version from PC-B" {
		t.Errorf("expected PC-A target to receive remote version, got '%s'", contentA)
	}

	backups, _ := backup.ListBackups(cfgA.BackupPath, cfgA.Connections[0], cfgA.Connections)
	if len(backups) != 1 || backups[0].ID != resA.BackupSessions[0] {
		t.Errorf("expected backup ID %v to match BackupSessions, got %+v", resA.BackupSessions, backups)
	}
}

// A.1 Test 3: Manual uncommitted edit in PC-A repo + remote change on another file -> sync OK.
func TestManualRepoEditPlusRemoteChangeInAnotherFile(t *testing.T) {
	_, cfgA, stA, orchA, cfgB, stB, orchB := setupTwoClonesWithBareRemote(t)

	// PC-B adds a new file commands/review.md and pushes
	reviewRepoB := filepath.Join(cfgB.RootPath, "CLAUDE", "commands", "review.md")
	_ = os.MkdirAll(filepath.Dir(reviewRepoB), 0o755)
	_ = os.WriteFile(reviewRepoB, []byte("# Review from PC-B\n"), 0o644)
	planB, _ := orchB.PlanSync(cfgB, stB)
	if _, err := orchB.ExecuteSync(cfgB, stB, planB.Fingerprint, nil); err != nil {
		t.Fatalf("PC-B sync failed: %v", err)
	}

	// PC-A makes a manual uncommitted edit directly inside repoA/CLAUDE/CLAUDE.md
	repoFileA := filepath.Join(cfgA.RootPath, "CLAUDE", "CLAUDE.md")
	_ = os.WriteFile(repoFileA, []byte("# Manual edit inside repoA\n"), 0o644)

	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}
	if len(planA.CommitPreview) == 0 {
		t.Errorf("expected CommitPreview to contain manual repo edit")
	}

	resA, err := orchA.ExecuteSync(cfgA, stA, planA.Fingerprint, nil)
	if err != nil {
		t.Fatalf("ExecuteSync with manual repo edit + remote change failed: %v", err)
	}
	if resA.Pulled == 0 || resA.Pushed == 0 {
		t.Errorf("expected both pull and push to succeed, got %+v", resA)
	}

	// Verify PC-A target received both the manual repo edit and PC-B's review.md
	targetReviewA := filepath.Join(cfgA.Connections[0].TargetPath, "commands", "review.md")
	if _, err := os.Stat(targetReviewA); os.IsNotExist(err) {
		t.Errorf("expected PC-A target to receive commands/review.md")
	}
}

// A.1 Test 4 (H3): Push rejected (PC-B pushes between PC-A's fetch/pull and push) -> retry with pull --rebase + push succeeds.
func TestPushRejectedRetrySuccess(t *testing.T) {
	_, cfgA, stA, orchA, cfgB, _, _ := setupTwoClonesWithBareRemote(t)

	// PC-A modifies CLAUDE.md in target
	targetFileA := filepath.Join(cfgA.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(targetFileA, []byte("# PC-A update\n"), 0o644)

	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}

	// Inject hook: right before PC-A's first push, PC-B commits and pushes a different file to bareRemote
	fired := false
	orchA.OnBeforePush = func() {
		if fired {
			return
		}
		fired = true
		extraB := filepath.Join(cfgB.RootPath, "CLAUDE", "extra.md")
		_ = os.WriteFile(extraB, []byte("# Concurrent push from PC-B\n"), 0o644)
		runnerB := gitops.NewRunner(cfgB.RootPath)
		_ = runnerB.CommitPaths([]string{"CLAUDE/extra.md"}, "feat: concurrent commit from B")
		_ = runnerB.Push()
	}

	resA, err := orchA.ExecuteSync(cfgA, stA, planA.Fingerprint, nil)
	if err != nil {
		t.Fatalf("expected push rejected retry (H3) to succeed, got error: %v", err)
	}
	if !fired || resA.Pushed == 0 {
		t.Errorf("expected OnBeforePush hook to fire and push to succeed after rebase retry")
	}
}

// A.1 Test 5 (H1): Changing state between PlanSync and ExecuteSync returns plan_changed.
func TestExecuteSyncFingerprintMismatchReturnsPlanChanged(t *testing.T) {
	_, cfgA, stA, orchA, _, _, _ := setupTwoClonesWithBareRemote(t)

	targetFileA := filepath.Join(cfgA.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(targetFileA, []byte("# Edit 1\n"), 0o644)

	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}

	// Add a new file in repo before calling ExecuteSync so the re-planned fingerprint differs
	_ = os.WriteFile(filepath.Join(cfgA.RootPath, "CLAUDE", "unexpected.md"), []byte("new"), 0o644)

	_, err = orchA.ExecuteSync(cfgA, stA, planA.Fingerprint, nil)
	if err == nil {
		t.Fatalf("expected plan_changed error when files change between PlanSync and ExecuteSync (H1)")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.PlanChanged {
		t.Errorf("expected error code %s, got %v", apperr.PlanChanged, err)
	}
}

// C2 & C6 Test: Repo without user.email returns git_identity_missing on ExecuteSync.
func TestMissingGitIdentityReturnsError(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}

	tempDir := t.TempDir()
	// Override HOME and USERPROFILE so global .gitconfig is not found
	emptyHome := filepath.Join(tempDir, "empty-home")
	_ = os.MkdirAll(emptyHome, 0o755)
	t.Setenv("HOME", emptyHome)
	t.Setenv("USERPROFILE", emptyHome)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(emptyHome, ".gitconfig"))

	repoDir := filepath.Join(tempDir, "repo")
	targetDir := filepath.Join(tempDir, "target")
	_ = os.MkdirAll(filepath.Join(repoDir, "CLAUDE"), 0o755)
	_ = os.MkdirAll(targetDir, 0o755)

	if err := gitops.Init(repoDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_ = os.WriteFile(filepath.Join(repoDir, "CLAUDE", "CLAUDE.md"), []byte("v1"), 0o644)
	_ = os.WriteFile(filepath.Join(targetDir, "CLAUDE.md"), []byte("v2-modified"), 0o644)

	conn := config.Connection{
		ID:         "conn-no-id",
		Profile:    "CLAUDE",
		TargetPath: targetDir,
	}
	cfg := &config.Config{
		RootPath:    repoDir,
		BackupPath:  filepath.Join(tempDir, "backups"),
		Connections: []config.Connection{conn},
	}
	st := state.NewState()
	h1, _ := fsops.HashFile(filepath.Join(repoDir, "CLAUDE", "CLAUDE.md"))
	st.SetBaseline(conn.ID, "CLAUDE.md", h1)

	orch := orchestrator.New()
	plan, err := orch.PlanSync(cfg, st)
	if err != nil {
		t.Fatal(err)
	}

	_, err = orch.ExecuteSync(cfg, st, plan.Fingerprint, nil)
	if err == nil {
		t.Fatalf("expected git_identity_missing when user.email is not configured (C2, C6)")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.GitIdentityMissing {
		t.Errorf("expected error code %s, got %v", apperr.GitIdentityMissing, err)
	}
}

// H2 Test: DirtyOutside overlapping with incoming remote changes returns git_dirty_worktree.
func TestDirtyOutsideOverlapBlocksSync(t *testing.T) {
	_, cfgA, stA, orchA, cfgB, _, _ := setupTwoClonesWithBareRemote(t)

	// PC-B commits a root-level file README.md outside profiles and pushes
	readmeB := filepath.Join(cfgB.RootPath, "README.md")
	_ = os.WriteFile(readmeB, []byte("# Remote README\n"), 0o644)
	runnerB := gitops.NewRunner(cfgB.RootPath)
	_ = runnerB.CommitPaths([]string{"README.md"}, "docs: add readme")
	_ = runnerB.Push()

	// PC-A also has an uncommitted local README.md outside profiles
	readmeA := filepath.Join(cfgA.RootPath, "README.md")
	_ = os.WriteFile(readmeA, []byte("# Dirty local README\n"), 0o644)

	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}
	if !planA.DirtyOverlapExists {
		t.Errorf("expected DirtyOverlapExists to be true (H2)")
	}

	_, err = orchA.ExecuteSync(cfgA, stA, planA.Fingerprint, nil)
	if err == nil {
		t.Fatalf("expected git_dirty_worktree error when DirtyOutside overlaps incoming changes (H2)")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.GitDirtyWorktree {
		if !strings.Contains(err.Error(), apperr.GitDirtyWorktree) {
			t.Errorf("expected %s, got %v", apperr.GitDirtyWorktree, err)
		}
	}
}

// R1 Test: keep_repo discards manual uncommitted repo edits after backing them up.
func TestR1_KeepRepoBacksUpDiscardedRepoEdits(t *testing.T) {
	_, cfgA, stA, orchA, cfgB, stB, orchB := setupTwoClonesWithBareRemote(t)

	// PC-B creates and commits a remote change to CLAUDE/CLAUDE.md and pushes
	targetFileB := filepath.Join(cfgB.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(targetFileB, []byte("# Remote version B\n"), 0o644)
	planB, err := orchB.PlanSync(cfgB, stB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := orchB.ExecuteSync(cfgB, stB, planB.Fingerprint, nil); err != nil {
		t.Fatalf("PC-B sync failed: %v", err)
	}

	// PC-A makes a manual uncommitted edit directly in the repo worktree
	repoFileA := filepath.Join(cfgA.RootPath, "CLAUDE", "CLAUDE.md")
	_ = os.WriteFile(repoFileA, []byte("# Manual uncommitted repo edit A\n"), 0o644)

	// And PC-A target also differs or matches baseline
	targetFileA := filepath.Join(cfgA.Connections[0].TargetPath, "CLAUDE.md")
	_ = os.WriteFile(targetFileA, []byte("# Local target A\n"), 0o644)

	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}

	resolutions := []orchestrator.ConflictResolution{
		{
			Profile:    "CLAUDE",
			RelPath:    "CLAUDE.md",
			Resolution: "keep_repo",
		},
	}

	resA, err := orchA.ExecuteSync(cfgA, stA, planA.Fingerprint, resolutions)
	if err != nil {
		t.Fatalf("ExecuteSync with keep_repo failed: %v", err)
	}

	// Verify that backup exists in cfgA.BackupPath/_repo/<session>/CLAUDE/CLAUDE.md
	repoBackups, err := backup.ListRepoBackups(cfgA.BackupPath)
	if err != nil {
		t.Fatalf("ListRepoBackups failed: %v", err)
	}
	if len(repoBackups) == 0 {
		t.Fatalf("expected at least 1 repo backup for discarded edit, got 0")
	}
	if repoBackups[0].Reason != backup.ReasonDiscardRepoEdit {
		t.Errorf("expected reason %s, got %s", backup.ReasonDiscardRepoEdit, repoBackups[0].Reason)
	}

	backedUpFile := filepath.Join(repoBackups[0].Path, "CLAUDE", "CLAUDE.md")
	backedUpContent, err := os.ReadFile(backedUpFile)
	if err != nil {
		t.Fatalf("could not read backed up repo file: %v", err)
	}
	if string(backedUpContent) != "# Manual uncommitted repo edit A\n" {
		t.Errorf("expected backed up content '# Manual uncommitted repo edit A\\n', got '%s'", backedUpContent)
	}

	// Target receives remote version
	contentA, err := os.ReadFile(targetFileA)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(contentA)) != "# Remote version B" {
		t.Errorf("expected target to receive remote version, got '%s'", contentA)
	}

	// Also verify backup session ID was recorded in SyncResult
	foundSession := false
	for _, s := range resA.BackupSessions {
		if s == repoBackups[0].ID {
			foundSession = true
			break
		}
	}
	if !foundSession {
		t.Errorf("expected backup session %s to be recorded in SyncResult.BackupSessions: %+v", repoBackups[0].ID, resA.BackupSessions)
	}
}
