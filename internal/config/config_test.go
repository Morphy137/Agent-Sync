package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"agentsync/internal/apperr"
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

	cfg.RootPath = "C:\\test\\root"
	cfg.Connections = append(cfg.Connections, config.Connection{
		ID:         "conn-1",
		Profile:    "CLAUDE",
		TargetPath: "C:\\test\\target",
		CreatedAt:  config.FormatRFC3339(time.Now()),
	})

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
	if len(loaded.Connections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(loaded.Connections))
	}
	if loaded.Connections[0].Profile != "CLAUDE" {
		t.Errorf("expected profile CLAUDE, got %s", loaded.Connections[0].Profile)
	}
}

func TestValidateConnection(t *testing.T) {
	tempDir := t.TempDir()
	root := filepath.Join(tempDir, "root")
	targetInside := filepath.Join(root, "sub-target")
	targetOutside := filepath.Join(tempDir, "outside")

	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetInside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetOutside, 0o755); err != nil {
		t.Fatal(err)
	}

	// Inside root should fail
	err := config.ValidateConnection(root, targetInside, nil, "")
	if err == nil {
		t.Errorf("expected error for target inside root")
	}

	// Outside root should pass
	err = config.ValidateConnection(root, targetOutside, nil, "")
	if err != nil {
		t.Errorf("expected target outside root to be valid, got %v", err)
	}

	// Duplicate target should fail
	existing := []config.Connection{
		{ID: "c1", TargetPath: targetOutside},
	}
	err = config.ValidateConnection(root, targetOutside, existing, "c2")
	if err == nil {
		t.Errorf("expected error for duplicate target")
	}
	appErr, ok := err.(*apperr.Error)
	if !ok || appErr.Code != apperr.TargetAlreadyConnected {
		t.Errorf("expected TargetAlreadyConnected, got %v", err)
	}
}
