package orchestrator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/scheduler"
	"agentsync/internal/state"
)

func TestTwoClientSyncIntegration(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}

	tempDir := t.TempDir()
	bareRemote := filepath.Join(tempDir, "remote.git")

	// 1. Bare remote
	if err := exec.Command("git", "init", "--bare", bareRemote).Run(); err != nil {
		t.Fatal(err)
	}

	// 2. Setup PC-A
	pcADir := filepath.Join(tempDir, "pc-a")
	repoA := filepath.Join(pcADir, "agent-configs")
	targetA := filepath.Join(pcADir, "home", ".claude")
	backupA := filepath.Join(pcADir, "backups")
	_ = os.MkdirAll(targetA, 0o755)

	if err := gitops.Init(repoA); err != nil {
		t.Fatal(err)
	}
	runnerA := gitops.NewRunner(repoA)
	if err := runnerA.AddRemote(bareRemote); err != nil {
		t.Fatal(err)
	}
	if err := runnerA.PushSetUpstream(); err != nil {
		t.Fatal(err)
	}

	connA := config.Connection{
		ID:         "conn-a",
		Profile:    "CLAUDE",
		TargetPath: targetA,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}
	cfgA := &config.Config{
		Version:           1,
		RootPath:          repoA,
		BackupPath:        backupA,
		BackupRetention:   20,
		ConfirmBeforeSync: false,
		Connections:       []config.Connection{connA},
	}
	stA := state.NewState()

	// 3. Setup PC-B (clone bare remote)
	pcBDir := filepath.Join(tempDir, "pc-b")
	repoB := filepath.Join(pcBDir, "agent-configs")
	targetB := filepath.Join(pcBDir, "home", ".claude")
	backupB := filepath.Join(pcBDir, "backups")
	_ = os.MkdirAll(targetB, 0o755)

	if err := exec.Command("git", "clone", bareRemote, repoB).Run(); err != nil {
		t.Fatal(err)
	}
	_ = exec.Command("git", "-C", repoB, "config", "user.name", "PC-B").Run()
	_ = exec.Command("git", "-C", repoB, "config", "user.email", "pcb@local").Run()

	connB := config.Connection{
		ID:         "conn-b",
		Profile:    "CLAUDE",
		TargetPath: targetB,
		CreatedAt:  config.FormatRFC3339(time.Now()),
	}
	cfgB := &config.Config{
		Version:           1,
		RootPath:          repoB,
		BackupPath:        backupB,
		BackupRetention:   20,
		ConfirmBeforeSync: false,
		Connections:       []config.Connection{connB},
	}
	stB := state.NewState()

	orchA := orchestrator.New()
	orchB := orchestrator.New()

	// 4. PC-A modifies file in target
	fileInTargetA := filepath.Join(targetA, "CLAUDE.md")
	_ = os.WriteFile(fileInTargetA, []byte("# Config from PC-A"), 0o644)

	// Create profile folder in repoA so WalkManaged finds the profile
	_ = os.MkdirAll(filepath.Join(repoA, "CLAUDE"), 0o755)
	fileInRepoA := filepath.Join(repoA, "CLAUDE", "CLAUDE.md")
	_ = os.WriteFile(fileInRepoA, []byte("# Initial in repo"), 0o644)
	// Baseline matches repo (source), so target being different is a clean target_changed (upload)
	initHash, _ := fsops.HashFile(fileInRepoA)
	stA.SetBaseline(connA.ID, "CLAUDE.md", initHash)

	// Plan on PC-A
	planA, err := orchA.PlanSync(cfgA, stA)
	if err != nil {
		t.Fatalf("PlanSync on PC-A failed: %v", err)
	}
	if planA.Direction != orchestrator.DirectionUp && planA.Direction != orchestrator.DirectionBoth {
		t.Errorf("expected Up or Both direction on PC-A, got %s", planA.Direction)
	}

	// Execute on PC-A
	resA, err := orchA.ExecuteSync(cfgA, stA, planA, nil)
	if err != nil {
		t.Fatalf("ExecuteSync on PC-A failed: %v", err)
	}
	if resA.Captured == 0 && len(planA.Uploads) > 0 {
		t.Errorf("expected captured files on PC-A")
	}

	// 5. PC-B Plans and Executes Sync
	// Also ensure CLAUDE profile exists in repoB after pull
	planB, err := orchB.PlanSync(cfgB, stB)
	if err != nil {
		t.Fatalf("PlanSync on PC-B failed: %v", err)
	}
	if planB.Behind == 0 {
		t.Errorf("expected PC-B to be behind after PC-A push")
	}

	resB, err := orchB.ExecuteSync(cfgB, stB, planB, nil)
	if err != nil {
		t.Fatalf("ExecuteSync on PC-B failed: %v", err)
	}
	if resB.Pulled == 0 {
		t.Errorf("expected pulled commits on PC-B")
	}

	// Verify target on PC-B received the file
	fileInTargetB := filepath.Join(targetB, "CLAUDE.md")
	contentB, err := os.ReadFile(fileInTargetB)
	if err != nil || string(contentB) != "# Config from PC-A" {
		t.Fatalf("expected PC-B target to have '# Config from PC-A', got '%s'", string(contentB))
	}

	// 6. Test Scheduler event emission
	var eventFired bool
	var payloadReceived scheduler.StatusEventPayload
	sched := scheduler.NewScheduler(orchB, func(eventName string, data any) {
		if eventName == "status:changed" {
			eventFired = true
			if payload, ok := data.(scheduler.StatusEventPayload); ok {
				payloadReceived = payload
			}
		}
	})

	t.Setenv(config.ConfigEnvVar, filepath.Join(pcBDir, "config"))
	_ = config.Save(cfgB)
	_ = stB.Save()

	sched.RunCheck(true)
	if !eventFired {
		t.Errorf("expected scheduler to emit status:changed")
	}
	if payloadReceived.Direction == "" {
		t.Errorf("expected non-empty direction in payload")
	}
}

func TestOrchestratorConflictResolution(t *testing.T) {
	tempDir := t.TempDir()
	rootDir := filepath.Join(tempDir, "root")
	targetDir := filepath.Join(tempDir, "target")
	profileDir := filepath.Join(rootDir, "CLAUDE")

	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.MkdirAll(targetDir, 0o755)

	sourceFile := filepath.Join(profileDir, "CLAUDE.md")
	targetFile := filepath.Join(targetDir, "CLAUDE.md")
	_ = os.WriteFile(sourceFile, []byte("source v2"), 0o644)
	_ = os.WriteFile(targetFile, []byte("target v2"), 0o644)

	conn := config.Connection{
		ID:         "conn-conflict",
		Profile:    "CLAUDE",
		TargetPath: targetDir,
	}
	cfg := &config.Config{
		RootPath:    rootDir,
		BackupPath:  filepath.Join(tempDir, "backups"),
		Connections: []config.Connection{conn},
	}

	st := state.NewState()
	st.SetBaseline(conn.ID, "CLAUDE.md", "oldhash")

	orch := orchestrator.New()
	plan, err := orch.PlanSync(cfg, st)
	if err != nil {
		t.Fatal(err)
	}

	if plan.Direction != orchestrator.DirectionConflict {
		t.Errorf("expected conflict direction, got %s", plan.Direction)
	}

	// 1. Execute without resolution should fail
	_, err = orch.ExecuteSync(cfg, st, plan, nil)
	if err == nil {
		t.Errorf("expected error when unresolved conflicts exist")
	}

	// 2. Execute with keep_repo resolution
	res := []orchestrator.ConflictResolution{
		{
			Profile:    "CLAUDE",
			RelPath:    "CLAUDE.md",
			Resolution: "keep_repo",
		},
	}

	result, err := orch.ExecuteSync(cfg, st, plan, res)
	if err != nil {
		t.Fatalf("ExecuteSync with keep_repo failed: %v", err)
	}
	if result.Applied != 1 {
		t.Errorf("expected 1 applied file with keep_repo, got %d", result.Applied)
	}

	targetContent, _ := os.ReadFile(targetFile)
	if string(targetContent) != "source v2" {
		t.Errorf("expected target to have 'source v2', got '%s'", targetContent)
	}
}
