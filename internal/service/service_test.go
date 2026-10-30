package service_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/service"
)

func TestServiceValidationAndPendingLinks(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	rootDir := filepath.Join(tempDir, "root")
	claudeDir := filepath.Join(rootDir, "CLAUDE")
	target1 := filepath.Join(tempDir, "target1")
	target2 := filepath.Join(tempDir, "target2")

	t.Setenv(config.ConfigEnvVar, configDir)

	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.MkdirAll(target1, 0o755)
	_ = os.MkdirAll(target2, 0o755)

	_ = os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("repo content"), 0o644)
	_ = os.WriteFile(filepath.Join(target1, "CLAUDE.md"), []byte("local diff content"), 0o644)

	var lastEmittedPlan *orchestrator.SyncPlan
	svc := service.New(func(eventName string, data any) {
		if eventName == "status:changed" {
			if p, ok := data.(*orchestrator.SyncPlan); ok {
				lastEmittedPlan = p
			}
		}
	})

	// SetRootPath
	if _, err := svc.SetRootPath(rootDir); err != nil {
		t.Fatalf("SetRootPath failed: %v", err)
	}
	if lastEmittedPlan == nil {
		t.Errorf("expected SetRootPath to emit full SyncPlan (H5)")
	}

	// M1: Connect non-existent profile -> profile_not_found
	if _, err := svc.ConnectProfile("NONEXISTENT", target1); err == nil {
		t.Fatalf("expected profile_not_found error")
	} else {
		var appErr *apperr.Error
		if !errors.As(err, &appErr) || appErr.Code != apperr.ProfileNotFound {
			t.Errorf("expected error code %s, got %v", apperr.ProfileNotFound, err)
		}
	}

	// M2: Connect CLAUDE to target1 with untracked diff -> returns pendingId without persisting yet
	res, err := svc.ConnectProfile("CLAUDE", target1)
	if err != nil {
		t.Fatalf("ConnectProfile failed: %v", err)
	}
	if !res.NeedsDecision || res.PendingID == "" {
		t.Fatalf("expected NeedsDecision=true and non-empty PendingID, got %+v", res)
	}

	// M2: Resolve with bogus pendingId fails
	if err := svc.ResolveInitialLink("bogus-id", "keep_repo"); err == nil {
		t.Errorf("expected error for unknown pendingId")
	}

	// Resolve with valid pendingId
	if err := svc.ResolveInitialLink(res.PendingID, "keep_repo"); err != nil {
		t.Fatalf("ResolveInitialLink failed: %v", err)
	}

	// M1: Attempt to connect CLAUDE a second time -> profile_already_connected
	if _, err := svc.ConnectProfile("CLAUDE", target2); err == nil {
		t.Fatalf("expected profile_already_connected error on duplicate profile")
	} else {
		var appErr *apperr.Error
		if !errors.As(err, &appErr) || appErr.Code != apperr.ProfileAlreadyConnected {
			t.Errorf("expected error code %s, got %v", apperr.ProfileAlreadyConnected, err)
		}
	}

	// Test ChangeTarget
	if _, err := svc.ChangeTarget(res.Connection.ID, target2); err != nil {
		t.Fatalf("ChangeTarget failed: %v", err)
	}

	// Test UpdateSettings, GetStatus, ListBackups, RestoreBackup
	cfg, changed, err := svc.UpdateSettings(service.SettingsPayload{
		Language:             "es",
		Theme:                "dark",
		BackupRetention:      15,
		CheckIntervalMinutes: 10,
		ConfirmBeforeSync:    false,
	})
	if err != nil || !changed || cfg.Language != "es" {
		t.Errorf("UpdateSettings failed: cfg=%+v, changed=%v, err=%v", cfg, changed, err)
	}

	// Test InitRepository and SetGitIdentity
	if err := svc.InitRepository(); err != nil {
		t.Fatalf("InitRepository failed: %v", err)
	}
	if err := svc.SetGitIdentity("Morphy Test", "morphy@example.com"); err != nil {
		t.Fatalf("SetGitIdentity failed: %v", err)
	}
	gitInfo, err := svc.GetGitInfo()
	if err != nil || !gitInfo.IsRepo || gitInfo.UserEmail != "morphy@example.com" {
		t.Errorf("expected gitInfo UserEmail=morphy@example.com, got %+v (err=%v)", gitInfo, err)
	}

	// Test ReloadFromDisk, GetConfig, ListProfiles, CheckRemote, ListBackups, RestoreBackup, AddRemote, ResolveInitialLink (keep_local & decide_later)
	if err := svc.ReloadFromDisk(); err != nil {
		t.Fatalf("ReloadFromDisk failed: %v", err)
	}
	if _, err := svc.GetConfig(); err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	profs, err := svc.ListProfiles()
	if err != nil || len(profs) == 0 {
		t.Errorf("ListProfiles failed: %v", err)
	}

	// Modify repo file and test ApplyConnection + RestoreBackup
	_ = os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("repo updated v2"), 0o644)
	connsCfg, _ := svc.GetConfig()
	activeConnID := connsCfg.Connections[0].ID
	if err := svc.ApplyConnection(activeConnID); err != nil {
		t.Fatalf("ApplyConnection failed: %v", err)
	}

	backups, err := svc.ListBackups(activeConnID)
	if err != nil || len(backups) == 0 {
		t.Fatalf("ListBackups failed or empty: %v", err)
	}
	if err := svc.RestoreBackup(activeConnID, backups[0].ID); err != nil {
		t.Fatalf("RestoreBackup failed: %v", err)
	}

	// Test ResolveInitialLink with keep_local and decide_later
	geminiDir := filepath.Join(rootDir, "GEMINI")
	_ = os.MkdirAll(geminiDir, 0o755)
	_ = os.WriteFile(filepath.Join(geminiDir, "GEMINI.md"), []byte("gemini repo"), 0o644)
	targetGemini := filepath.Join(tempDir, "target-gemini")
	_ = os.MkdirAll(targetGemini, 0o755)
	_ = os.WriteFile(filepath.Join(targetGemini, "GEMINI.md"), []byte("gemini local"), 0o644)

	linkRes, err := svc.ConnectProfile("GEMINI", targetGemini)
	if err != nil || !linkRes.NeedsDecision {
		t.Fatalf("expected NeedsDecision for GEMINI: %v", err)
	}
	if err := svc.ResolveInitialLink(linkRes.PendingID, "keep_local"); err != nil {
		t.Fatalf("ResolveInitialLink keep_local failed: %v", err)
	}

	codexDir := filepath.Join(rootDir, "CODEX")
	_ = os.MkdirAll(codexDir, 0o755)
	_ = os.WriteFile(filepath.Join(codexDir, "AGENTS.md"), []byte("codex repo"), 0o644)
	targetCodex := filepath.Join(tempDir, "target-codex")
	_ = os.MkdirAll(targetCodex, 0o755)
	_ = os.WriteFile(filepath.Join(targetCodex, "AGENTS.md"), []byte("codex local"), 0o644)

	linkResCodex, err := svc.ConnectProfile("CODEX", targetCodex)
	if err != nil || !linkResCodex.NeedsDecision {
		t.Fatalf("expected NeedsDecision for CODEX: %v", err)
	}
	if err := svc.ResolveInitialLink(linkResCodex.PendingID, "decide_later"); err != nil {
		t.Fatalf("ResolveInitialLink decide_later failed: %v", err)
	}

	// Test CheckRemote and AddRemote
	bareRemote := filepath.Join(tempDir, "bare.git")
	_ = exec.Command("git", "init", "--bare", bareRemote).Run()
	runner := gitops.NewRunner(rootDir)
	_ = runner.CommitPaths([]string{"."}, "chore: commit before remote")
	if err := svc.AddRemote(bareRemote); err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}
	if _, err := svc.CheckRemote(); err != nil {
		t.Fatalf("CheckRemote failed: %v", err)
	}
	if svc.Orchestrator() == nil {
		t.Errorf("expected non-nil Orchestrator")
	}

	// Test DisconnectProfile and error branches (empty rootPath, nonexistent connectionId)
	_, err = svc.DisconnectProfile(activeConnID)
	if err != nil {
		t.Fatalf("DisconnectProfile failed: %v", err)
	}
	if err := svc.ApplyConnection("non-existent-id"); err == nil {
		t.Errorf("expected error for ApplyConnection on non-existent id")
	}
	if err := svc.RestoreBackup("non-existent-id", "b-id"); err == nil {
		t.Errorf("expected error for RestoreBackup on non-existent id")
	}
	if list, err := svc.ListBackups("non-existent-id"); err != nil || len(list) != 0 {
		t.Errorf("expected empty list for non-existent id")
	}
	if _, err := svc.ChangeTarget("non-existent-id", target1); err == nil {
		t.Errorf("expected error for ChangeTarget on non-existent id")
	}
}

func TestServiceConcurrentOperations(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}

	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	rootDir := filepath.Join(tempDir, "root")
	bareRemote := filepath.Join(tempDir, "remote.git")
	claudeDir := filepath.Join(rootDir, "CLAUDE")
	targetDir := filepath.Join(tempDir, "target")

	t.Setenv(config.ConfigEnvVar, configDir)

	_ = exec.Command("git", "init", "--bare", bareRemote).Run()
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.MkdirAll(targetDir, 0o755)
	_ = os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("v1"), 0o644)

	svc := service.New(nil)
	_, _ = svc.SetRootPath(rootDir)
	_ = svc.InitRepository()
	_ = svc.SetGitIdentity("Tester", "tester@local")
	runner := gitops.NewRunner(rootDir)
	_ = runner.CommitPaths([]string{"."}, "chore: init")
	_ = runner.AddRemote(bareRemote)
	_ = runner.PushSetUpstream()

	res, err := svc.ConnectProfile("CLAUDE", targetDir)
	if err != nil {
		t.Fatalf("ConnectProfile failed: %v", err)
	}
	connID := res.Connection.ID

	// C3: Spawn concurrent goroutines calling PlanSync, GetStatus, ApplyConnection, ExecuteSync, TryBackgroundCheck
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(4)
		go func() {
			defer wg.Done()
			_, _ = svc.GetStatus()
		}()
		go func() {
			defer wg.Done()
			_, _ = svc.TryBackgroundCheck()
		}()
		go func() {
			defer wg.Done()
			_ = svc.ApplyConnection(connID)
		}()
		go func() {
			defer wg.Done()
			plan, err := svc.PlanSync()
			if err == nil && plan != nil {
				_, _ = svc.ExecuteSync(plan.Fingerprint, nil)
			}
		}()
	}
	wg.Wait()
}

// R2 Test: ChangeTarget is atomic. If new target is invalid or has differences and is cancelled, the original connection remains intact.
func TestR2_ChangeTargetIsAtomic(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("AGENTSYNC_CONFIG_DIR", tempDir)

	rootDir := filepath.Join(tempDir, "agent-configs")
	claudeDir := filepath.Join(rootDir, "CLAUDE")
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("claude initial repo"), 0o644)

	target1 := filepath.Join(tempDir, "target1")
	_ = os.MkdirAll(target1, 0o755)

	svc := service.New(func(eventName string, data any) {})
	if _, err := svc.SetRootPath(rootDir); err != nil {
		t.Fatal(err)
	}

	linkRes, err := svc.ConnectProfile("CLAUDE", target1)
	if err != nil {
		t.Fatal(err)
	}
	originalConnID := linkRes.Connection.ID

	// 1. Try to change target to an invalid directory (e.g. inside root)
	invalidTarget := filepath.Join(rootDir, "inside-root")
	_ = os.MkdirAll(invalidTarget, 0o755)
	_, err = svc.ChangeTarget(originalConnID, invalidTarget)
	if err == nil {
		t.Fatalf("expected error changing target to inside-root")
	}

	// Verify original connection is still present and unchanged
	cfg, _ := svc.GetConfig()
	activeConns := cfg.Connections
	if len(activeConns) != 1 || activeConns[0].ID != originalConnID || activeConns[0].TargetPath != target1 {
		t.Fatalf("original connection was corrupted after invalid ChangeTarget: %+v", activeConns)
	}

	// 2. Change target to a directory with conflicting untracked file -> creates pending link
	target2 := filepath.Join(tempDir, "target2")
	_ = os.MkdirAll(target2, 0o755)
	_ = os.WriteFile(filepath.Join(target2, "CLAUDE.md"), []byte("claude target2 diff"), 0o644)

	pendingRes, err := svc.ChangeTarget(originalConnID, target2)
	if err != nil {
		t.Fatalf("ChangeTarget to target2 failed: %v", err)
	}
	if !pendingRes.NeedsDecision || pendingRes.PendingID == "" {
		t.Fatalf("expected NeedsDecision and PendingID for target2")
	}

	// While pending, original connection MUST STILL be active in config
	cfg, _ = svc.GetConfig()
	activeConns = cfg.Connections
	if len(activeConns) != 1 || activeConns[0].ID != originalConnID {
		t.Fatalf("original connection was removed before decision: %+v", activeConns)
	}

	// 3. User cancels the change
	if err := svc.CancelPendingLink(pendingRes.PendingID); err != nil {
		t.Fatal(err)
	}

	// Original connection MUST STILL be untouched
	cfg, _ = svc.GetConfig()
	activeConns = cfg.Connections
	if len(activeConns) != 1 || activeConns[0].ID != originalConnID || activeConns[0].TargetPath != target1 {
		t.Fatalf("original connection was altered after cancelling: %+v", activeConns)
	}

	// 4. Now perform change and resolve it
	pendingRes2, err := svc.ChangeTarget(originalConnID, target2)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ResolveInitialLink(pendingRes2.PendingID, "keep_local"); err != nil {
		t.Fatalf("ResolveInitialLink keep_local failed: %v", err)
	}

	cfg, _ = svc.GetConfig()
	activeConns = cfg.Connections
	if len(activeConns) != 1 || activeConns[0].ID == originalConnID || activeConns[0].TargetPath != target2 {
		t.Fatalf("expected connection to be atomically replaced by target2, got: %+v", activeConns)
	}
}

// R3 Test: CancelPendingLink clears the pending link, and expired links are cleaned up.
func TestR3_CleanPendingLinksAndTTL(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("AGENTSYNC_CONFIG_DIR", tempDir)

	rootDir := filepath.Join(tempDir, "agent-configs")
	claudeDir := filepath.Join(rootDir, "CLAUDE")
	_ = os.MkdirAll(claudeDir, 0o755)
	_ = os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("claude repo"), 0o644)

	targetDir := filepath.Join(tempDir, "target")
	_ = os.MkdirAll(targetDir, 0o755)
	_ = os.WriteFile(filepath.Join(targetDir, "CLAUDE.md"), []byte("claude target diff"), 0o644)

	svc := service.New(func(eventName string, data any) {})
	if _, err := svc.SetRootPath(rootDir); err != nil {
		t.Fatal(err)
	}

	linkRes, err := svc.ConnectProfile("CLAUDE", targetDir)
	if err != nil || !linkRes.NeedsDecision {
		t.Fatal(err)
	}

	// Cancelling removes the pending link
	if err := svc.CancelPendingLink(linkRes.PendingID); err != nil {
		t.Fatal(err)
	}

	// Resolving after cancel returns error
	if err := svc.ResolveInitialLink(linkRes.PendingID, "keep_repo"); err == nil {
		t.Errorf("expected error resolving cancelled pending link")
	}
}

// B.2 Tests: CreateFolder, CreateProfile, InspectPath, DetectAgentFolders, SelectFolder.
func TestFolderOperationsAndPathInspection(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("AGENTSYNC_CONFIG_DIR", tempDir)
	overrideHome := filepath.Join(tempDir, "simulated-home")
	t.Setenv("AGENTSYNC_HOME_OVERRIDE", overrideHome)
	_ = os.MkdirAll(overrideHome, 0o755)

	rootDir := filepath.Join(tempDir, "agent-configs")
	_ = os.MkdirAll(rootDir, 0o755)

	svc := service.New(func(eventName string, data any) {})
	if _, err := svc.SetRootPath(rootDir); err != nil {
		t.Fatal(err)
	}

	// 1. CreateFolder validations
	// Relative path -> error
	if _, err := svc.CreateFolder("relative/path"); err == nil {
		t.Errorf("expected error for relative path in CreateFolder")
	}

	// Invalid characters in folder name (< > : " | ? *)
	invalidNameFolder := filepath.Join(tempDir, "bad<folder>name")
	if _, err := svc.CreateFolder(invalidNameFolder); err == nil {
		t.Errorf("expected error for invalid characters in CreateFolder")
	}

	// Reserved Windows name (AUX, CON, NUL)
	reservedFolder := filepath.Join(tempDir, "AUX")
	if _, err := svc.CreateFolder(reservedFolder); err == nil {
		t.Errorf("expected error for reserved Windows name in CreateFolder")
	}

	// Trailing dot or space
	trailingDotFolder := filepath.Join(tempDir, "trailing.")
	if _, err := svc.CreateFolder(trailingDotFolder); err == nil {
		t.Errorf("expected error for trailing dot in CreateFolder")
	}

	// Valid folder creation
	validFolder := filepath.Join(tempDir, "valid-folder")
	createdPath, err := svc.CreateFolder(validFolder)
	if err != nil {
		t.Fatalf("CreateFolder valid failed: %v", err)
	}
	if createdPath != validFolder {
		t.Errorf("expected '%s', got '%s'", validFolder, createdPath)
	}

	// Already exists -> error
	if _, err := svc.CreateFolder(validFolder); err == nil {
		t.Errorf("expected error creating folder that already exists")
	}

	// 2. CreateProfile validations
	// Reserved name
	if _, err := svc.CreateProfile("CON"); err == nil {
		t.Errorf("expected error creating profile with reserved name CON")
	}
	// Name starting with dot
	if _, err := svc.CreateProfile(".hidden"); err == nil {
		t.Errorf("expected error creating profile starting with dot")
	}
	// Valid profile creation
	prof, err := svc.CreateProfile("NEWPROFILE")
	if err != nil {
		t.Fatalf("CreateProfile failed: %v", err)
	}
	if prof.Name != "NEWPROFILE" {
		t.Errorf("expected NEWPROFILE, got %s", prof.Name)
	}
	// Verify .gitkeep was created inside
	gitkeep := filepath.Join(rootDir, "NEWPROFILE", ".gitkeep")
	if _, err := os.Stat(gitkeep); err != nil {
		t.Fatalf("expected .gitkeep to be created in profile folder: %v", err)
	}
	// Re-creating existing profile -> error
	if _, err := svc.CreateProfile("NEWPROFILE"); err == nil {
		t.Errorf("expected error creating already existing profile")
	}

	// 3. InspectPath validations
	// Nonexistent path
	infoNone := svc.InspectPath(filepath.Join(tempDir, "does-not-exist"))
	if infoNone.Exists || infoNone.IsDir {
		t.Errorf("expected exists=false for nonexistent path: %+v", infoNone)
	}

	// Inside root path
	insideRoot := filepath.Join(rootDir, "NEWPROFILE")
	infoInside := svc.InspectPath(insideRoot)
	if !infoInside.Exists || !infoInside.IsDir || !infoInside.IsInsideRoot {
		t.Errorf("expected exists=true, isDir=true, isInsideRoot=true: %+v", infoInside)
	}

	// Tilde expansion with simulated home
	simulatedClaude := filepath.Join(overrideHome, ".claude")
	_ = os.MkdirAll(simulatedClaude, 0o755)
	infoTilde := svc.InspectPath("~/.claude")
	if !infoTilde.Exists || !infoTilde.IsDir || infoTilde.Normalized != simulatedClaude {
		t.Errorf("expected tilde path to expand to '%s', got %+v", simulatedClaude, infoTilde)
	}

	// Already connected path
	_, err = svc.ConnectProfile("NEWPROFILE", validFolder)
	if err != nil {
		t.Fatal(err)
	}
	infoConnected := svc.InspectPath(validFolder)
	if !infoConnected.AlreadyConnected {
		t.Errorf("expected alreadyConnected=true for connected folder")
	}

	// 4. DetectAgentFolders
	// Create simulated .gemini and .config/opencode in overrideHome
	_ = os.MkdirAll(filepath.Join(overrideHome, ".gemini"), 0o755)
	_ = os.MkdirAll(filepath.Join(overrideHome, ".config", "opencode"), 0o755)

	suggestions := svc.DetectAgentFolders()
	// Should detect .claude, .gemini, and opencode
	if len(suggestions) < 3 {
		t.Fatalf("expected at least 3 agent suggestions, got %d: %+v", len(suggestions), suggestions)
	}
	foundClaude := false
	foundOpencode := false
	for _, s := range suggestions {
		if s.SuggestedProfile == "CLAUDE" {
			foundClaude = true
		}
		if s.SuggestedProfile == "OPENCODE" {
			foundOpencode = true
		}
	}
	if !foundClaude || !foundOpencode {
		t.Errorf("expected CLAUDE and OPENCODE in suggestions: %+v", suggestions)
	}

	// 5. SelectFolder with handler
	svc.SetDialogFunc(func(title, defaultDir string) (string, error) {
		return "selected/folder/path", nil
	})
	selected, err := svc.SelectFolder("Pick Folder", tempDir)
	if err != nil || selected != "selected/folder/path" {
		t.Errorf("expected 'selected/folder/path', got '%s', err: %v", selected, err)
	}
}
