package orchestrator_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentsync/internal/backup"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/profiles"
	"agentsync/internal/state"
	"agentsync/internal/syncer"
)

func hashString(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// TestE2EScenariosE1toE15 runs end-to-end verification of scenarios E1 through E15
// in an isolated sandbox created inside t.TempDir().
func TestE2EScenariosE1toE15(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed on system")
	}

	tempDir := t.TempDir()
	bareRemote := filepath.Join(tempDir, "remote.git")
	pcADir := filepath.Join(tempDir, "pc-a")
	pcBDir := filepath.Join(tempDir, "pc-b")

	// 1. Bare remote
	if err := exec.Command("git", "init", "--bare", bareRemote).Run(); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}

	// Setup PC-A
	pcARepo := filepath.Join(pcADir, "agent-configs")
	pcAHome := filepath.Join(pcADir, "home")
	pcAConfig := filepath.Join(pcADir, "config")
	pcABackup := filepath.Join(pcADir, "backups")

	_ = os.MkdirAll(pcAConfig, 0o755)
	_ = os.MkdirAll(pcAHome, 0o755)

	if err := gitops.Init(pcARepo); err != nil {
		t.Fatal(err)
	}
	runnerA := gitops.NewRunner(pcARepo)
	_ = runnerA.SetLocalIdentity("PC-A", "pca@local")
	if err := runnerA.AddRemote(bareRemote); err != nil {
		t.Fatal(err)
	}

	// Create profiles in PC-A
	claudeSrc := filepath.Join(pcARepo, "CLAUDE")
	_ = os.MkdirAll(filepath.Join(claudeSrc, "commands"), 0o755)
	_ = os.MkdirAll(filepath.Join(claudeSrc, "agents"), 0o755)
	_ = os.WriteFile(filepath.Join(claudeSrc, "CLAUDE.md"), []byte("# CLAUDE global - source v1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(claudeSrc, "commands", "review.md"), []byte("# Code review v1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(claudeSrc, "agents", "planner.md"), []byte("# Planner agent v1\n"), 0o644)

	geminiSrc := filepath.Join(pcARepo, "GEMINI")
	_ = os.MkdirAll(geminiSrc, 0o755)
	_ = os.WriteFile(filepath.Join(geminiSrc, "GEMINI.md"), []byte("# GEMINI global - source v1\n"), 0o644)

	codexSrc := filepath.Join(pcARepo, "CODEX")
	_ = os.MkdirAll(codexSrc, 0o755)
	_ = os.WriteFile(filepath.Join(codexSrc, "AGENTS.md"), []byte("# CODEX global - source v1\n"), 0o644)

	opencodeSrc := filepath.Join(pcARepo, "OPENCODE")
	_ = os.MkdirAll(opencodeSrc, 0o755)
	_ = os.WriteFile(filepath.Join(opencodeSrc, "AGENTS.md"), []byte("# OPENCODE global - source v1\n"), 0o644)

	if err := runnerA.CommitPaths([]string{"."}, "chore: initialize profiles"); err != nil {
		t.Fatal(err)
	}
	if err := runnerA.PushSetUpstream(); err != nil {
		t.Fatal(err)
	}

	// Create fake targets in PC-A home
	claudeHomeA := filepath.Join(pcAHome, ".claude")
	_ = os.MkdirAll(claudeHomeA, 0o755)
	_ = os.WriteFile(filepath.Join(claudeHomeA, "CLAUDE.md"), []byte("# CLAUDE local - pre-existing\n"), 0o644)
	settingsContent := `{"apiKey": "fake-key-123", "theme": "dark"}`
	settingsPathA := filepath.Join(claudeHomeA, "settings.json")
	_ = os.WriteFile(settingsPathA, []byte(settingsContent), 0o644)
	settingsHashBefore, _ := fsops.HashFile(settingsPathA)

	codexHomeA := filepath.Join(pcAHome, ".codex")
	_ = os.MkdirAll(codexHomeA, 0o755)

	// Setup PC-B
	pcBRepo := filepath.Join(pcBDir, "agent-configs")
	pcBHome := filepath.Join(pcBDir, "home")
	pcBConfig := filepath.Join(pcBDir, "config")
	pcBBackup := filepath.Join(pcBDir, "backups")

	_ = os.MkdirAll(pcBConfig, 0o755)
	_ = os.MkdirAll(pcBHome, 0o755)
	if err := exec.Command("git", "clone", bareRemote, pcBRepo).Run(); err != nil {
		t.Fatal(err)
	}
	_ = exec.Command("git", "-C", pcBRepo, "config", "user.name", "PC-B").Run()
	_ = exec.Command("git", "-C", pcBRepo, "config", "user.email", "pcb@local").Run()
	claudeHomeB := filepath.Join(pcBHome, ".claude")
	_ = os.MkdirAll(claudeHomeB, 0o755)
	// Initially, PC-B is in sync with v1
	v1Content := "# CLAUDE global - source v1\n"
	_ = os.WriteFile(filepath.Join(claudeHomeB, "CLAUDE.md"), []byte(v1Content), 0o644)

	// App state for PC-A and PC-B
	cfgA := &config.Config{
		Version:           1,
		RootPath:          pcARepo,
		BackupPath:        pcABackup,
		BackupRetention:   20,
		ConfirmBeforeSync: false,
		Connections:       make([]config.Connection, 0),
	}
	stA := state.NewState()

	connClaudeB := config.Connection{
		ID:         "conn-claude-b",
		Profile:    "CLAUDE",
		TargetPath: claudeHomeB,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}
	cfgB := &config.Config{
		Version:           1,
		RootPath:          pcBRepo,
		BackupPath:        pcBBackup,
		BackupRetention:   20,
		ConfirmBeforeSync: false,
		Connections:       []config.Connection{connClaudeB},
	}
	stB := state.NewState()
	stB.SetBaseline(connClaudeB.ID, "CLAUDE.md", hashString(v1Content))

	orchA := orchestrator.New()
	orchB := orchestrator.New()

	// ==========================================
	// E1: Select pc-a/agent-configs -> detects 4 profiles
	// ==========================================
	profs, err := profiles.Discover(pcARepo)
	if err != nil {
		t.Fatalf("[E1] Discover failed: %v", err)
	}
	if len(profs) != 4 {
		t.Fatalf("[E1] expected 4 profiles, got %d", len(profs))
	}
	expectedProfiles := []string{"CLAUDE", "CODEX", "GEMINI", "OPENCODE"}
	for i, name := range expectedProfiles {
		if profs[i].Name != name {
			t.Errorf("[E1] expected profile %s, got %s", name, profs[i].Name)
		}
	}

	// ==========================================
	// E2: Connect CLAUDE -> .claude (first link diff dialog -> Use repo creates backup and overwrites)
	// ==========================================
	connClaudeA := config.Connection{
		ID:         "conn-claude-a",
		Profile:    "CLAUDE",
		TargetPath: claudeHomeA,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}
	statusClaudeA, err := syncer.ComputeStatus(pcARepo, connClaudeA, stA)
	if err != nil {
		t.Fatalf("[E2] ComputeStatus failed: %v", err)
	}
	if statusClaudeA.Status != syncer.ConnUntrackedDiff {
		t.Fatalf("[E2] expected ConnUntrackedDiff on first link, got %s", statusClaudeA.Status)
	}

	// User selects "Use repo version"
	_, err = syncer.Apply(pcARepo, connClaudeA, []config.Connection{connClaudeA}, pcABackup, []string{"CLAUDE.md", "commands/review.md", "agents/planner.md"}, stA, 20, backup.ReasonInitialLink)
	if err != nil {
		t.Fatalf("[E2] Apply on initial link failed: %v", err)
	}
	cfgA.Connections = append(cfgA.Connections, connClaudeA)

	// Check backup was created with format layout
	backupsA, err := backup.ListBackups(pcABackup, connClaudeA, cfgA.Connections)
	if err != nil || len(backupsA) == 0 {
		t.Fatalf("[E2] expected backup to be created on initial overwrite")
	}
	if backupsA[0].Reason != backup.ReasonInitialLink {
		t.Errorf("[E2] expected backup reason 'initial_link', got %s", backupsA[0].Reason)
	}

	// ==========================================
	// E3: Verify .claude after E2
	// ==========================================
	claudeContentA, _ := os.ReadFile(filepath.Join(claudeHomeA, "CLAUDE.md"))
	if !strings.Contains(string(claudeContentA), "CLAUDE global - source v1") {
		t.Errorf("[E3] expected CLAUDE.md to match source, got '%s'", claudeContentA)
	}
	if _, err := os.Stat(filepath.Join(claudeHomeA, "commands", "review.md")); os.IsNotExist(err) {
		t.Errorf("[E3] expected commands/review.md to be created")
	}
	if _, err := os.Stat(filepath.Join(claudeHomeA, "agents", "planner.md")); os.IsNotExist(err) {
		t.Errorf("[E3] expected agents/planner.md to be created")
	}
	settingsHashAfter, _ := fsops.HashFile(settingsPathA)
	if settingsHashBefore != settingsHashAfter {
		t.Errorf("[E3] unmanaged file settings.json was modified")
	}

	// ==========================================
	// E4: Connect CODEX -> empty .codex -> creates AGENTS.md, without backup
	// ==========================================
	connCodexA := config.Connection{
		ID:         "conn-codex-a",
		Profile:    "CODEX",
		TargetPath: codexHomeA,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}
	cfgA.Connections = append(cfgA.Connections, connCodexA)
	_, err = syncer.Apply(pcARepo, connCodexA, cfgA.Connections, pcABackup, []string{"AGENTS.md"}, stA, 20, backup.ReasonApply)
	if err != nil {
		t.Fatalf("[E4] Apply CODEX failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexHomeA, "AGENTS.md")); os.IsNotExist(err) {
		t.Errorf("[E4] expected AGENTS.md in .codex")
	}
	backupsCodex, _ := backup.ListBackups(pcABackup, connCodexA, cfgA.Connections)
	if len(backupsCodex) != 0 {
		t.Errorf("[E4] expected no backups when creating into empty directory, got %d", len(backupsCodex))
	}

	// ==========================================
	// E5: Edit pc-a/home/.claude/CLAUDE.md -> status target_changed, icon UP
	// ==========================================
	_ = os.WriteFile(filepath.Join(claudeHomeA, "CLAUDE.md"), []byte("# CLAUDE modified by Morphy on PC-A\n"), 0o644)
	statusAfterEdit, err := syncer.ComputeStatus(pcARepo, connClaudeA, stA)
	if err != nil {
		t.Fatal(err)
	}
	if statusAfterEdit.Status != syncer.ConnTargetChanged {
		t.Errorf("[E5] expected ConnTargetChanged, got %s", statusAfterEdit.Status)
	}
	planAfterEdit, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}
	if planAfterEdit.Direction != orchestrator.DirectionUp {
		t.Errorf("[E5] expected direction Up, got %s", planAfterEdit.Direction)
	}

	// ==========================================
	// E6: Sync on PC-A -> Capture + commit + push
	// ==========================================
	resA, err := orchA.ExecuteSync(cfgA, stA, planAfterEdit.Fingerprint, nil)
	if err != nil {
		t.Fatalf("[E6] ExecuteSync failed: %v", err)
	}
	if resA.Captured != 1 {
		t.Errorf("[E6] expected 1 captured file, got %d", resA.Captured)
	}
	if resA.Pushed == 0 {
		t.Errorf("[E6] expected pushed commits to remote")
	}

	// ==========================================
	// E7: On PC-B, check -> detects behind, icon DOWN
	// ==========================================
	planB, err := orchB.PlanSync(cfgB, stB)
	if err != nil {
		t.Fatalf("[E7] PlanSync on PC-B failed: %v", err)
	}
	if planB.Behind == 0 {
		t.Errorf("[E7] expected PC-B to be behind")
	}
	if planB.Direction != orchestrator.DirectionDown {
		t.Errorf("[E7] expected direction Down on PC-B, got %s", planB.Direction)
	}

	// ==========================================
	// E8: Sync on PC-B -> pull + apply with backup; target = version of PC-A
	// ==========================================
	resB, err := orchB.ExecuteSync(cfgB, stB, planB.Fingerprint, nil)
	if err != nil {
		t.Fatalf("[E8] ExecuteSync on PC-B failed: %v", err)
	}
	if resB.Pulled == 0 {
		t.Errorf("[E8] expected pulled commits on PC-B")
	}
	claudeContentB, _ := os.ReadFile(filepath.Join(claudeHomeB, "CLAUDE.md"))
	if !strings.Contains(string(claudeContentB), "Morphy on PC-A") {
		t.Errorf("[E8] expected PC-B target to match PC-A version, got '%s'", claudeContentB)
	}
	backupsB, _ := backup.ListBackups(pcBBackup, connClaudeB, cfgB.Connections)
	if len(backupsB) == 0 {
		t.Errorf("[E8] expected backup created on PC-B before overwrite")
	}

	// ==========================================
	// E9: Edit same file in PC-A (target) and PC-B (and push from B) -> conflict resolution
	// ==========================================
	// PC-A modifies target locally
	_ = os.WriteFile(filepath.Join(claudeHomeA, "CLAUDE.md"), []byte("# Conflict line from PC-A\n"), 0o644)
	// PC-B modifies target and syncs (pushes to remote)
	_ = os.WriteFile(filepath.Join(claudeHomeB, "CLAUDE.md"), []byte("# Conflict line from PC-B\n"), 0o644)
	planBSync, _ := orchB.PlanSync(cfgB, stB)
	_, err = orchB.ExecuteSync(cfgB, stB, planBSync.Fingerprint, nil)
	if err != nil {
		t.Fatalf("[E9] PC-B push failed: %v", err)
	}

	// PC-A plans sync -> should detect conflict
	planAConflict, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}
	if planAConflict.Direction != orchestrator.DirectionConflict {
		t.Errorf("[E9] expected Conflict direction on PC-A, got %s", planAConflict.Direction)
	}

	// Resolve conflict by choosing keep_repo
	resolutions := []orchestrator.ConflictResolution{
		{
			Profile:    "CLAUDE",
			RelPath:    "CLAUDE.md",
			Resolution: "keep_repo",
		},
	}
	resAConflict, err := orchA.ExecuteSync(cfgA, stA, planAConflict.Fingerprint, resolutions)
	if err != nil {
		t.Fatalf("[E9] ExecuteSync with conflict resolution failed: %v", err)
	}
	if resAConflict.Applied == 0 {
		t.Errorf("[E9] expected applied file with keep_repo")
	}
	claudeContentAAfterConflict, _ := os.ReadFile(filepath.Join(claudeHomeA, "CLAUDE.md"))
	if !strings.Contains(string(claudeContentAAfterConflict), "PC-B") {
		t.Errorf("[E9] expected repo version from PC-B, got '%s'", claudeContentAAfterConflict)
	}

	// ==========================================
	// E10 (C4, M10): Restore a backup -> creates previous backup reason: restore,
	// restores content, leaves file as target_changed, and ExecuteSync uploads it to repo!
	// ==========================================
	time.Sleep(20 * time.Millisecond)
	backupsAList, _ := backup.ListBackups(pcABackup, connClaudeA, cfgA.Connections)
	if len(backupsAList) == 0 {
		t.Fatalf("[E10] expected backups in PC-A")
	}
	oldestBackupID := backupsAList[len(backupsAList)-1].ID
	err = backup.RestoreBackup(pcABackup, connClaudeA, cfgA.Connections, oldestBackupID, stA, 20)
	if err != nil {
		t.Fatalf("[E10] RestoreBackup failed: %v", err)
	}
	backupsAfterRestore, _ := backup.ListBackups(pcABackup, connClaudeA, cfgA.Connections)
	if backupsAfterRestore[0].Reason != backup.ReasonRestore {
		t.Errorf("[E10] expected top backup reason 'restore', got %s", backupsAfterRestore[0].Reason)
	}

	// Verify restored content equals the initial pre-existing content ("# CLAUDE local - pre-existing\n")
	restoredBytes, _ := os.ReadFile(filepath.Join(claudeHomeA, "CLAUDE.md"))
	if !strings.Contains(string(restoredBytes), "CLAUDE local - pre-existing") {
		t.Errorf("[E10] expected restored content to be 'CLAUDE local - pre-existing', got '%s'", restoredBytes)
	}

	// Verify status after restore is target_changed (C4)
	statusAfterRestore, err := syncer.ComputeStatus(pcARepo, connClaudeA, stA)
	if err != nil {
		t.Fatal(err)
	}
	if statusAfterRestore.Status != syncer.ConnTargetChanged {
		t.Errorf("[E10] expected status after restore to be target_changed (C4), got %s", statusAfterRestore.Status)
	}

	// Verify ExecuteSync uploads the restored version to the repository (C4)
	planAfterRestore, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatal(err)
	}
	resAfterRestore, err := orchA.ExecuteSync(cfgA, stA, planAfterRestore.Fingerprint, nil)
	if err != nil {
		t.Fatalf("[E10] ExecuteSync after restore failed: %v", err)
	}
	if resAfterRestore.Captured == 0 || resAfterRestore.Pushed == 0 {
		t.Errorf("[E10] expected restored file to be captured and pushed, got %+v", resAfterRestore)
	}
	repoBytesAfterSync, _ := os.ReadFile(filepath.Join(pcARepo, "CLAUDE", "CLAUDE.md"))
	if !strings.Contains(string(repoBytesAfterSync), "CLAUDE local - pre-existing") {
		t.Errorf("[E10] expected repo file to contain restored content after sync, got '%s'", repoBytesAfterSync)
	}

	// ==========================================
	// E11 (M10): Retention = 3, perform 5 updates via syncer.Apply -> exactly 3 backups remain
	// ==========================================
	for i := 1; i <= 5; i++ {
		_ = os.WriteFile(filepath.Join(pcARepo, "CLAUDE", "CLAUDE.md"), []byte("repo-v"+string(rune('0'+i))), 0o644)
		time.Sleep(10 * time.Millisecond)
		if _, err := syncer.Apply(pcARepo, connClaudeA, cfgA.Connections, pcABackup, []string{"CLAUDE.md"}, stA, 3, backup.ReasonApply); err != nil {
			t.Fatalf("[E11] Apply iteration %d failed: %v", i, err)
		}
	}
	backupsPruned, _ := backup.ListBackups(pcABackup, connClaudeA, cfgA.Connections)
	if len(backupsPruned) != 3 {
		t.Errorf("[E11] expected exactly 3 backups with retention=3 via Apply, got %d", len(backupsPruned))
	}

	// ==========================================
	// E12: Root without git -> banner + init repository works
	// ==========================================
	noGitRoot := filepath.Join(tempDir, "no-git-root")
	_ = os.MkdirAll(noGitRoot, 0o755)
	if gitops.IsRepo(noGitRoot) {
		t.Errorf("[E12] expected noGitRoot to not be repo")
	}
	if err := gitops.Init(noGitRoot); err != nil {
		t.Fatalf("[E12] Init failed: %v", err)
	}
	if !gitops.IsRepo(noGitRoot) {
		t.Errorf("[E12] expected noGitRoot to be recognized as repo after Init")
	}

	// ==========================================
	// E13: Repo without remote -> banner, syncs locally without error
	// ==========================================
	cfgLocal := &config.Config{
		RootPath:    noGitRoot,
		BackupPath:  filepath.Join(tempDir, "local-backups"),
		Connections: []config.Connection{},
	}
	stLocal := state.NewState()
	orchLocal := orchestrator.New()
	planLocal, err := orchLocal.PlanSync(cfgLocal, stLocal)
	if err != nil {
		t.Fatalf("[E13] PlanSync on local repo failed: %v", err)
	}
	if planLocal.HasRemote {
		t.Errorf("[E13] expected hasRemote false")
	}
	_, err = orchLocal.ExecuteSync(cfgLocal, stLocal, planLocal.Fingerprint, nil)
	if err != nil {
		t.Fatalf("[E13] ExecuteSync on local repo without remote failed: %v", err)
	}

	// ==========================================
	// E14: Change language and theme -> configuration validated
	// ==========================================
	t.Setenv(config.ConfigEnvVar, pcAConfig)
	cfgA.Language = "es"
	cfgA.Theme = "dark"
	if err := config.Save(cfgA); err != nil {
		t.Fatalf("[E14] Save config with es/dark failed: %v", err)
	}
	loadedCfgA, err := config.Load()
	if err != nil || loadedCfgA.Language != "es" || loadedCfgA.Theme != "dark" {
		t.Errorf("[E14] expected loaded config language=es, theme=dark")
	}

	// ==========================================
	// E15: Delete file from profile and sync -> file remains in destination; removed from baseline
	// ==========================================
	// Create an extra file in profile CLAUDE
	extraProfileFile := filepath.Join(pcARepo, "CLAUDE", "obsolete.md")
	_ = os.WriteFile(extraProfileFile, []byte("obsolete content"), 0o644)
	stA.SetBaseline(connClaudeA.ID, "obsolete.md", hashString("obsolete content"))

	// File is present in target too
	extraTargetFile := filepath.Join(claudeHomeA, "obsolete.md")
	_ = os.WriteFile(extraTargetFile, []byte("obsolete content"), 0o644)

	// Now delete from profile
	_ = os.Remove(extraProfileFile)

	// ReconcileBaselines (called during sync/status operations under lock)
	syncer.ReconcileBaselines(pcARepo, connClaudeA, stA)

	// File MUST still exist in target (rule 4: never delete target files)
	if _, err := os.Stat(extraTargetFile); os.IsNotExist(err) {
		t.Errorf("[E15] file in target was unexpectedly deleted!")
	}

	// Baseline for obsolete.md MUST be removed from state
	if stA.GetBaseline(connClaudeA.ID, "obsolete.md") != "" {
		t.Errorf("[E15] expected baseline for obsolete.md to be deleted from state")
	}
}
