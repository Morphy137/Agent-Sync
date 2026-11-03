package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"agentsync/internal/config"
)

func TestConfigLoadSave(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, tempDir)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load on empty config dir failed: %v", err)
	}

	if cfg.BackupRetention != config.DefaultBackupRetention {
		t.Errorf("expected default retention %d, got %d", config.DefaultBackupRetention, cfg.BackupRetention)
	}

	cfg.RootPath = "root-folder"
	cfg.Locations = append(cfg.Locations, config.Location{Alias: "claude", Path: "claude-folder"})

	if err := config.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}

	if loaded.RootPath != cfg.RootPath {
		t.Errorf("expected rootPath %s, got %s", cfg.RootPath, loaded.RootPath)
	}
	if len(loaded.Locations) != 1 || loaded.Locations[0].Alias != "claude" {
		t.Fatalf("expected the claude location to persist, got %+v", loaded.Locations)
	}
}

func TestLoadIgnoresUnknownLegacyFields(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, tempDir)

	raw := `{"version":1,"rootPath":"repo","connections":[{"id":"c1","profile":"CLAUDE","targetPath":"t"}],"futureField":true}`
	if err := os.WriteFile(filepath.Join(tempDir, config.ConfigFileName), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load with unknown fields failed: %v", err)
	}
	if cfg.RootPath != "repo" {
		t.Errorf("expected rootPath preserved, got %s", cfg.RootPath)
	}
}
