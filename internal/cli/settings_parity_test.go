package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/service"
)

func TestSettingsPartialUpdatePreservesDesktopPreferences(t *testing.T) {
	f := newFixture(t)
	cfg := config.DefaultConfig()
	cfg.CloseToTray, cfg.StartWithWindows, cfg.ConfirmBeforeSync = true, true, true
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(f.root, "settings.json")
	mustWrite(t, input, `{"autoApplyUpdates":true,"checkIntervalMinutes":0}`)
	mustJSON(t, nil, "settings", "update", "--input", input)
	var got service.SettingsPayload
	mustJSON(t, &got, "settings", "show")
	if !got.AutoApplyUpdates || got.CheckIntervalMinutes != 0 {
		t.Fatalf("settings not applied: %+v", got)
	}
	saved, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !saved.CloseToTray || !saved.StartWithWindows || !saved.ConfirmBeforeSync {
		t.Fatalf("unrelated settings lost: %+v", saved)
	}
	mustWrite(t, input, `{"autoApplyUpdates":false}`)
	mustJSON(t, nil, "settings", "update", "--input", input)
	saved, err = config.Load()
	if err != nil || saved.AutoApplyUpdates {
		t.Fatalf("false not applied: %+v %v", saved, err)
	}
}

func TestSettingsInvalidUpdateDoesNotMutate(t *testing.T) {
	f := newFixture(t)
	input := filepath.Join(f.root, "settings.json")
	for _, body := range []string{`{"checkIntervalMinutes":-1}`, `{"backupRetention":0}`, `{"language":"xx"}`, `{"closeToTray":true}`} {
		mustWrite(t, input, body)
		code, result := runJSON(t, nil, "settings", "update", "--input", input)
		if code == ExitOK || result.OK {
			t.Fatalf("accepted invalid settings: %s", body)
		}
	}
}

func TestRepositorySetupCommands(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "create", f.root, "created")
	mustJSON(t, nil, "repo", "init")
	mustJSON(t, nil, "repo", "identity", "--name", "AgentSync test", "--email", "test@example.invalid")
	var info service.GitInfoPayload
	mustJSON(t, &info, "repo", "info")
	if !info.IsRepo || info.UserName != "AgentSync test" || info.UserEmail != "test@example.invalid" {
		t.Fatalf("setup failed: %+v", info)
	}
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	root := filepath.Join(f.root, "created")
	git("-C", root, "commit", "--allow-empty", "-m", "fixture")
	remote := filepath.Join(f.root, "remote.git")
	git("init", "--bare", remote)
	if code, _ := runJSON(t, nil, "repo", "add-remote", remote); code == ExitOK {
		t.Fatal("upload accepted without --yes")
	}
	mustJSON(t, nil, "repo", "add-remote", remote, "--yes")
	mustJSON(t, nil, "root", "clone", remote, f.root, "cloned")
	mustJSON(t, &info, "repo", "info")
	if !info.IsRepo || info.RemoteURL == "" {
		t.Fatalf("clone failed: %+v", info)
	}
	var path map[string]string
	mustJSON(t, &path, "recognizers", "path")
	if _, err := os.Stat(path["path"]); err != nil {
		t.Fatalf("recognizers not initialized: %v", err)
	}
}
