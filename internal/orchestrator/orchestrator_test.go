package orchestrator_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/scheduler"
	"agentsync/internal/state"
)

const (
	managedDir      = "shared"
	managedFile     = "shared/AGENTS.md"
	managedFileName = "AGENTS.md"
)

// pc is one computer: a clone of the shared repository with its own config, state and orchestrator.
type pc struct {
	cfg  *config.Config
	st   *state.State
	orch *orchestrator.Orchestrator
}

func (p *pc) file(rel string) string {
	return filepath.Join(p.cfg.RootPath, filepath.FromSlash(rel))
}

func (p *pc) write(t *testing.T, rel, content string) {
	t.Helper()
	path := p.file(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (p *pc) read(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(p.file(rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func (p *pc) runner() *gitops.GitRunner {
	return gitops.NewRunner(p.cfg.RootPath)
}

// sync plans and executes a full sync without resolutions.
func (p *pc) sync(t *testing.T) *orchestrator.SyncResult {
	t.Helper()
	plan, err := p.orch.PlanSync(p.cfg, p.st)
	if err != nil {
		t.Fatalf("PlanSync failed: %v", err)
	}
	res, err := p.orch.ExecuteSync(p.cfg, p.st, plan.Fingerprint, nil)
	if err != nil {
		t.Fatalf("ExecuteSync failed: %v", err)
	}
	return res
}

func newPC(repo, backupDir string) *pc {
	orch := orchestrator.New()
	orch.ManagedRepoPaths = []string{managedDir}
	return &pc{
		cfg: &config.Config{
			Version:         1,
			RootPath:        repo,
			BackupPath:      backupDir,
			BackupRetention: 20,
		},
		st:   state.NewState(),
		orch: orch,
	}
}

// setupTwoClonesWithBareRemote creates two computers sharing a bare remote that already holds
// shared/AGENTS.md. Only the "shared" folder is managed by sync rules.
func setupTwoClonesWithBareRemote(t *testing.T) (a, b *pc) {
	t.Helper()
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}

	tempDir := t.TempDir()
	bareRemote := filepath.Join(tempDir, "remote.git")
	if err := exec.Command("git", "init", "--bare", "--initial-branch", gitops.DefaultBranch, bareRemote).Run(); err != nil {
		t.Fatal(err)
	}

	repoA := filepath.Join(tempDir, "pc-a", "agent-configs")
	if err := os.MkdirAll(filepath.Join(repoA, managedDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := gitops.Init(repoA); err != nil {
		t.Fatal(err)
	}
	a = newPC(repoA, filepath.Join(tempDir, "pc-a", "backups"))
	_ = a.runner().SetLocalIdentity("PC-A", "pca@local")
	a.write(t, managedFile, "# Initial v1\n")
	if err := a.runner().CommitPaths([]string{"."}, "chore: initial commit"); err != nil {
		t.Fatal(err)
	}
	if err := a.runner().AddRemote(bareRemote); err != nil {
		t.Fatal(err)
	}
	if err := a.runner().PushSetUpstream(); err != nil {
		t.Fatal(err)
	}

	repoB := filepath.Join(tempDir, "pc-b", "agent-configs")
	if err := exec.Command("git", "clone", bareRemote, repoB).Run(); err != nil {
		t.Fatal(err)
	}
	b = newPC(repoB, filepath.Join(tempDir, "pc-b", "backups"))
	_ = b.runner().SetLocalIdentity("PC-B", "pcb@local")
	return a, b
}

func TestTwoClientSyncIntegration(t *testing.T) {
	a, b := setupTwoClonesWithBareRemote(t)

	// PC-A edits a managed file inside the repository
	a.write(t, managedFile, "# Config from PC-A")

	planA, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatalf("PlanSync on PC-A failed: %v", err)
	}
	if planA.Direction != orchestrator.DirectionUp {
		t.Errorf("expected Up direction on PC-A, got %s", planA.Direction)
	}
	if len(planA.CommitPreview) != 1 || planA.CommitPreview[0] != managedFile {
		t.Errorf("expected the managed edit in CommitPreview, got %v", planA.CommitPreview)
	}

	resA, err := a.orch.ExecuteSync(a.cfg, a.st, planA.Fingerprint, nil)
	if err != nil {
		t.Fatalf("ExecuteSync on PC-A failed: %v", err)
	}
	if resA.Pushed == 0 {
		t.Errorf("expected a push on PC-A, got %+v", resA)
	}

	// PC-B plans and executes its sync
	planB, err := b.orch.PlanSync(b.cfg, b.st)
	if err != nil {
		t.Fatalf("PlanSync on PC-B failed: %v", err)
	}
	if planB.Direction != orchestrator.DirectionDown || planB.Behind == 0 {
		t.Errorf("expected PC-B to be behind with a Down direction, got %s behind=%d", planB.Direction, planB.Behind)
	}

	resB, err := b.orch.ExecuteSync(b.cfg, b.st, planB.Fingerprint, nil)
	if err != nil {
		t.Fatalf("ExecuteSync on PC-B failed: %v", err)
	}
	if resB.Pulled == 0 {
		t.Errorf("expected pulled commits on PC-B, got %+v", resB)
	}
	if got := b.read(t, managedFile); got != "# Config from PC-A" {
		t.Fatalf("expected PC-B repo to have the PC-A content, got %q", got)
	}

	idle, err := b.orch.PlanSync(b.cfg, b.st)
	if err != nil {
		t.Fatal(err)
	}
	if idle.Direction != orchestrator.DirectionIdle {
		t.Errorf("expected Idle after syncing, got %s", idle.Direction)
	}

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

// pushRemoteEdit makes PC-B change the managed file and push it.
func pushRemoteEdit(t *testing.T, b *pc, content string) {
	t.Helper()
	b.write(t, managedFile, content)
	b.sync(t)
}

func TestOverlapIsAConflictThatNeedsADecision(t *testing.T) {
	a, b := setupTwoClonesWithBareRemote(t)
	pushRemoteEdit(t, b, "# Version from PC-B\n")
	a.write(t, managedFile, "# Version on PC-A\n")

	plan, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Direction != orchestrator.DirectionConflict {
		t.Fatalf("expected conflict direction, got %s", plan.Direction)
	}
	if len(plan.Conflicts) != 1 || plan.Conflicts[0].Path != managedFile || plan.Conflicts[0].Reason != orchestrator.ConflictRemoteAndLocalOverlap {
		t.Fatalf("unexpected conflicts: %+v", plan.Conflicts)
	}

	_, err = a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, nil)
	if !apperr.HasCode(err, apperr.UnresolvedConflicts) {
		t.Fatalf("expected %s without a resolution, got %v", apperr.UnresolvedConflicts, err)
	}
	_, err = a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, []orchestrator.ConflictResolution{{Path: managedFile, Resolution: "nonsense"}})
	if !apperr.HasCode(err, apperr.UnresolvedConflicts) {
		t.Fatalf("expected %s for an unknown resolution, got %v", apperr.UnresolvedConflicts, err)
	}
	if got := a.read(t, managedFile); got != "# Version on PC-A\n" {
		t.Errorf("a refused sync must not touch the local edit, got %q", got)
	}
}

// keep_local pushes the local version without a rebase conflict and keeps the local file as it was.
func TestKeepLocalWithRemoteOverlap(t *testing.T) {
	a, b := setupTwoClonesWithBareRemote(t)
	pushRemoteEdit(t, b, "# Version from PC-B\n")
	a.write(t, managedFile, "# Version kept on PC-A\n")

	plan, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatal(err)
	}
	resolutions := []orchestrator.ConflictResolution{{Path: managedFile, Resolution: orchestrator.ResolutionKeepLocal}}
	res, err := a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, resolutions)
	if err != nil {
		t.Fatalf("ExecuteSync with keep_local failed: %v", err)
	}
	if res.Pulled == 0 || res.Pushed == 0 {
		t.Errorf("expected a pull and a push with keep_local, got %+v", res)
	}
	if len(res.BackupSessions) != 1 {
		t.Errorf("expected the kept local version to be backed up once, got %v", res.BackupSessions)
	}
	if got := a.read(t, managedFile); got != "# Version kept on PC-A\n" {
		t.Errorf("expected PC-A to keep its version, got %q", got)
	}

	b.sync(t)
	if got := strings.TrimSpace(b.read(t, managedFile)); got != "# Version kept on PC-A" {
		t.Errorf("expected PC-B to receive the PC-A version, got %q", got)
	}
}

// keep_repo takes the remote version and backs up the discarded local edit.
func TestKeepRepoWithRemoteOverlap(t *testing.T) {
	a, b := setupTwoClonesWithBareRemote(t)
	pushRemoteEdit(t, b, "# Remote version from PC-B\n")
	a.write(t, managedFile, "# Local version on PC-A to be backed up\n")

	plan, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatal(err)
	}
	resolutions := []orchestrator.ConflictResolution{{Path: managedFile, Resolution: orchestrator.ResolutionKeepRepo}}
	res, err := a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, resolutions)
	if err != nil {
		t.Fatalf("ExecuteSync with keep_repo failed: %v", err)
	}
	if got := strings.TrimSpace(a.read(t, managedFile)); got != "# Remote version from PC-B" {
		t.Errorf("expected PC-A to receive the remote version, got %q", got)
	}

	backups, err := backup.ListRepoBackups(a.cfg.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 || backups[0].Reason != backup.ReasonDiscardRepoEdit {
		t.Fatalf("expected one discard backup, got %+v", backups)
	}
	if len(res.BackupSessions) != 1 || res.BackupSessions[0] != backups[0].ID {
		t.Errorf("expected backup session %s in the result, got %v", backups[0].ID, res.BackupSessions)
	}
	saved, err := os.ReadFile(filepath.Join(backups[0].Path, filepath.FromSlash(managedFile)))
	if err != nil || string(saved) != "# Local version on PC-A to be backed up\n" {
		t.Errorf("expected the discarded edit in the backup, got %q, %v", saved, err)
	}
}

// A local edit plus a remote change in another file syncs without conflicts.
func TestManualRepoEditPlusRemoteChangeInAnotherFile(t *testing.T) {
	a, b := setupTwoClonesWithBareRemote(t)

	b.write(t, "shared/review.md", "# Review from PC-B\n")
	b.sync(t)

	a.write(t, managedFile, "# Manual edit inside repoA\n")
	plan, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Direction != orchestrator.DirectionBoth || len(plan.Conflicts) != 0 {
		t.Fatalf("expected Both direction without conflicts, got %s %+v", plan.Direction, plan.Conflicts)
	}

	res, err := a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, nil)
	if err != nil {
		t.Fatalf("ExecuteSync failed: %v", err)
	}
	if res.Pulled == 0 || res.Pushed == 0 {
		t.Errorf("expected both pull and push, got %+v", res)
	}
	if got := strings.TrimSpace(a.read(t, "shared/review.md")); got != "# Review from PC-B" {
		t.Errorf("expected PC-A to receive review.md, got %q", got)
	}
}

// Push rejected (PC-B pushes between PC-A's pull and push) -> retry with pull --rebase succeeds.
func TestPushRejectedRetrySuccess(t *testing.T) {
	a, b := setupTwoClonesWithBareRemote(t)
	a.write(t, managedFile, "# PC-A update\n")

	plan, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatal(err)
	}

	fired := false
	a.orch.OnBeforePush = func() {
		if fired {
			return
		}
		fired = true
		b.write(t, "shared/extra.md", "# Concurrent push from PC-B\n")
		_ = b.runner().CommitPaths([]string{"shared/extra.md"}, "feat: concurrent commit from B")
		_ = b.runner().Push()
	}

	res, err := a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, nil)
	if err != nil {
		t.Fatalf("expected the push rejected retry to succeed, got error: %v", err)
	}
	if !fired || res.Pushed == 0 {
		t.Errorf("expected OnBeforePush hook to fire and the push to succeed after the rebase retry")
	}
}

// Changing the repository between PlanSync and ExecuteSync returns plan_changed.
func TestExecuteSyncFingerprintMismatchReturnsPlanChanged(t *testing.T) {
	a, _ := setupTwoClonesWithBareRemote(t)
	a.write(t, managedFile, "# Edit 1\n")

	plan, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatal(err)
	}
	a.write(t, "shared/unexpected.md", "new")

	_, err = a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, nil)
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.PlanChanged {
		t.Errorf("expected error code %s, got %v", apperr.PlanChanged, err)
	}
}

// A repository without user.email returns git_identity_missing when there is something to commit.
func TestMissingGitIdentityReturnsError(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}

	tempDir := t.TempDir()
	emptyHome := filepath.Join(tempDir, "empty-home")
	_ = os.MkdirAll(emptyHome, 0o755)
	t.Setenv("HOME", emptyHome)
	t.Setenv("USERPROFILE", emptyHome)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(emptyHome, ".gitconfig"))

	repoDir := filepath.Join(tempDir, "repo")
	_ = os.MkdirAll(filepath.Join(repoDir, managedDir), 0o755)
	if err := gitops.Init(repoDir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	p := newPC(repoDir, filepath.Join(tempDir, "backups"))
	p.write(t, managedFile, "v1")

	plan, err := p.orch.PlanSync(p.cfg, p.st)
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.orch.ExecuteSync(p.cfg, p.st, plan.Fingerprint, nil)
	if !apperr.HasCode(err, apperr.GitIdentityMissing) {
		t.Errorf("expected error code %s, got %v", apperr.GitIdentityMissing, err)
	}
}

// DirtyOutside overlapping with incoming remote changes returns git_dirty_worktree.
func TestDirtyOutsideOverlapBlocksSync(t *testing.T) {
	a, b := setupTwoClonesWithBareRemote(t)

	b.write(t, "README.md", "# Remote README\n")
	_ = b.runner().CommitPaths([]string{"README.md"}, "docs: add readme")
	_ = b.runner().Push()

	a.write(t, "README.md", "# Dirty local README\n")

	plan, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.DirtyOverlapExists {
		t.Errorf("expected DirtyOverlapExists to be true")
	}
	if len(plan.DirtyOutside) != 1 || plan.DirtyOutside[0] != "README.md" {
		t.Errorf("expected README.md as dirty outside managed paths, got %v", plan.DirtyOutside)
	}

	_, err = a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, nil)
	if !apperr.HasCode(err, apperr.GitDirtyWorktree) {
		t.Fatalf("expected %s when DirtyOutside overlaps incoming changes, got %v", apperr.GitDirtyWorktree, err)
	}
	if got := a.read(t, "README.md"); got != "# Dirty local README\n" {
		t.Errorf("a blocked sync must keep the local file, got %q", got)
	}
}

// Edits outside rule-managed paths are never committed by Sync, and do not block it when the
// remote does not touch them.
func TestDirtyOutsideWithoutOverlapStaysUncommitted(t *testing.T) {
	a, b := setupTwoClonesWithBareRemote(t)

	b.write(t, managedFile, "# Remote change\n")
	b.sync(t)
	a.write(t, "notes.md", "# Local notes\n")

	plan, err := a.orch.PlanSync(a.cfg, a.st)
	if err != nil {
		t.Fatal(err)
	}
	if plan.DirtyOverlapExists || len(plan.CommitPreview) != 0 {
		t.Fatalf("expected no overlap and nothing to commit, got %+v", plan)
	}
	if len(plan.DirtyOutside) != 1 || plan.DirtyOutside[0] != "notes.md" {
		t.Fatalf("expected notes.md outside managed paths, got %v", plan.DirtyOutside)
	}

	if _, err := a.orch.ExecuteSync(a.cfg, a.st, plan.Fingerprint, nil); err != nil {
		t.Fatalf("sync with unrelated dirty files failed: %v", err)
	}
	if got := strings.TrimSpace(a.read(t, managedFile)); got != "# Remote change" {
		t.Errorf("expected the remote change to be pulled, got %q", got)
	}
	changes, err := a.runner().StatusPorcelain()
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].Path != "notes.md" {
		t.Errorf("expected notes.md to stay uncommitted, got %+v", changes)
	}
}

func TestPlanSyncWithoutRootFails(t *testing.T) {
	orch := orchestrator.New()
	_, err := orch.PlanSync(&config.Config{}, state.NewState())
	if !apperr.HasCode(err, apperr.RootNotFound) {
		t.Errorf("expected %s, got %v", apperr.RootNotFound, err)
	}
}

func TestPlanSyncOnPlainFolderIsIdle(t *testing.T) {
	root := t.TempDir()
	orch := orchestrator.New()
	plan, err := orch.PlanSync(&config.Config{RootPath: root}, state.NewState())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Direction != orchestrator.DirectionIdle || plan.HasRemote {
		t.Errorf("expected an idle plan without remote, got %+v", plan)
	}
}
