package service_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

func TestServiceSetRootPathEmitsPlan(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, filepath.Join(tempDir, "config"))
	rootDir := filepath.Join(tempDir, "root")
	_ = os.MkdirAll(rootDir, 0o755)

	var lastEmittedPlan *orchestrator.SyncPlan
	svc := service.New(func(eventName string, data any) {
		if p, ok := data.(*orchestrator.SyncPlan); ok && eventName == "status:changed" {
			lastEmittedPlan = p
		}
	})

	if _, err := svc.SetRootPath(filepath.Join(tempDir, "missing")); !apperr.HasCode(err, apperr.RootNotFound) {
		t.Fatalf("expected %s for a missing folder, got %v", apperr.RootNotFound, err)
	}
	if _, err := svc.SetRootPath(rootDir); err != nil {
		t.Fatalf("SetRootPath failed: %v", err)
	}
	if lastEmittedPlan == nil {
		t.Errorf("expected SetRootPath to emit the full SyncPlan")
	}

	if err := svc.ReloadFromDisk(); err != nil {
		t.Fatalf("ReloadFromDisk failed: %v", err)
	}
	cfg, err := svc.GetConfig()
	if err != nil || cfg.RootPath != rootDir {
		t.Fatalf("expected the saved root after reload, got %+v, %v", cfg, err)
	}
	if svc.Orchestrator() == nil {
		t.Errorf("expected non-nil Orchestrator")
	}
}

// Concurrent reads, checks and syncs must not race or deadlock.
func TestServiceConcurrentOperations(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}

	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	rootDir := filepath.Join(tempDir, "root")
	bareRemote := filepath.Join(tempDir, "remote.git")
	sharedDir := filepath.Join(rootDir, "shared")
	targetDir := filepath.Join(tempDir, "target")

	t.Setenv(config.ConfigEnvVar, configDir)

	_ = exec.Command("git", "init", "--bare", bareRemote).Run()
	_ = os.MkdirAll(sharedDir, 0o755)
	_ = os.MkdirAll(targetDir, 0o755)
	_ = os.WriteFile(filepath.Join(sharedDir, "AGENTS.md"), []byte("v1"), 0o644)

	svc := service.New(nil)
	_, _ = svc.SetRootPath(rootDir)
	_ = svc.InitRepository()
	_ = svc.SetGitIdentity("Tester", "tester@local")
	runner := gitops.NewRunner(rootDir)
	_ = runner.CommitPaths([]string{"."}, "chore: init")
	_ = runner.AddRemote(bareRemote)
	_ = runner.PushSetUpstream()

	if _, err := svc.SaveLocation("target", targetDir); err != nil {
		t.Fatalf("SaveLocation failed: %v", err)
	}
	_, err := svc.SaveRule(rules.Rule{
		Name:    "Agents",
		Enabled: true,
		Source:  rules.Source{Origin: rules.OriginRepo, Path: "shared/AGENTS.md"},
		Destinations: []rules.Destination{{
			Location: "target",
			Path:     "AGENTS.md",
			Mode:     rules.ModeTwoWay,
			Enabled:  true,
		}},
	}, rules.ScopeRepo)
	if err != nil {
		t.Fatalf("SaveRule failed: %v", err)
	}

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
			_, _ = svc.SyncRules()
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

// CreateFolder, InspectPath and SelectFolder behaviour.
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

	// 2. InspectPath validations
	// Nonexistent path
	infoNone := svc.InspectPath(filepath.Join(tempDir, "does-not-exist"))
	if infoNone.Exists || infoNone.IsDir {
		t.Errorf("expected exists=false for nonexistent path: %+v", infoNone)
	}

	// Inside root path
	insideRoot := filepath.Join(rootDir, "shared")
	_ = os.MkdirAll(insideRoot, 0o755)
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

	// 3. SelectFolder with handler
	svc.SetDialogFunc(func(title, defaultDir string) (string, error) {
		return "selected/folder/path", nil
	})
	selected, err := svc.SelectFolder("Pick Folder", tempDir)
	if err != nil || selected != "selected/folder/path" {
		t.Errorf("expected 'selected/folder/path', got '%s', err: %v", selected, err)
	}
}

// CreateRoot and CloneRepository behaviour.
func TestCreateRootAndCloneRepository(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("AGENTSYNC_CONFIG_DIR", tempDir)

	svc := service.New(func(eventName string, data any) {})

	// 1. CreateRoot validations
	_, err := svc.CreateRoot("relative/path", "my-root")
	if appErr, ok := err.(*apperr.Error); !ok || appErr.Code != apperr.PathNotAbsolute {
		t.Errorf("expected PathNotAbsolute, got %v", err)
	}

	_, err = svc.CreateRoot(tempDir, "invalid*name")
	if appErr, ok := err.(*apperr.Error); !ok || appErr.Code != apperr.InvalidFolderName {
		t.Errorf("expected InvalidFolderName, got %v", err)
	}

	_, err = svc.CreateRoot(filepath.Join(tempDir, "nonexistent"), "my-root")
	if appErr, ok := err.(*apperr.Error); !ok || appErr.Code != apperr.TargetNotFound {
		t.Errorf("expected TargetNotFound, got %v", err)
	}

	// Successful CreateRoot
	cfg, err := svc.CreateRoot(tempDir, "new-root")
	if err != nil {
		t.Fatalf("CreateRoot failed: %v", err)
	}
	expectedRoot := filepath.Join(tempDir, "new-root")
	if cfg.RootPath != expectedRoot {
		t.Errorf("expected root path %s, got %s", expectedRoot, cfg.RootPath)
	}
	if fi, err := os.Stat(expectedRoot); err != nil || !fi.IsDir() {
		t.Errorf("expected root folder to exist on disk as directory")
	}

	// Folder already exists
	_, err = svc.CreateRoot(tempDir, "new-root")
	if appErr, ok := err.(*apperr.Error); !ok || appErr.Code != apperr.FolderExists {
		t.Errorf("expected FolderExists, got %v", err)
	}

	// 2. CloneRepository validations
	_, err = svc.CloneRepository("", tempDir, "cloned-root")
	if err == nil {
		t.Errorf("expected error for empty clone URL")
	}

	_, err = svc.CloneRepository("https://github.com/example/repo.git", "relative/path", "cloned-root")
	if appErr, ok := err.(*apperr.Error); !ok || appErr.Code != apperr.PathNotAbsolute {
		t.Errorf("expected PathNotAbsolute, got %v", err)
	}

	_, err = svc.CloneRepository("https://github.com/example/repo.git", tempDir, "NUL")
	if appErr, ok := err.(*apperr.Error); !ok || appErr.Code != apperr.InvalidFolderName {
		t.Errorf("expected InvalidFolderName, got %v", err)
	}

	// Setup a local bare git repo to clone from
	bareDir := filepath.Join(tempDir, "remote.git")
	_ = os.MkdirAll(bareDir, 0o755)
	initCmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v: %s", err, string(out))
	}

	// Successful clone
	cfg, err = svc.CloneRepository(bareDir, tempDir, "cloned-root")
	if err != nil {
		t.Fatalf("CloneRepository failed: %v", err)
	}
	expectedCloned := filepath.Join(tempDir, "cloned-root")
	if cfg.RootPath != expectedCloned {
		t.Errorf("expected root path %s, got %s", expectedCloned, cfg.RootPath)
	}
	if !gitops.IsRepo(expectedCloned) {
		t.Errorf("expected cloned folder to be a git repository")
	}

	// Existing folder returns FolderExists
	_, err = svc.CloneRepository(bareDir, tempDir, "cloned-root")
	if appErr, ok := err.(*apperr.Error); !ok || appErr.Code != apperr.FolderExists {
		t.Errorf("expected FolderExists, got %v", err)
	}
}
