package service

import (
	"os"
	"path/filepath"
	"testing"

	"agentsync/internal/config"
)

func TestOperationalSettingsRejectsInvalidPatchAtomically(t *testing.T) {
	t.Setenv(config.ConfigEnvVar, t.TempDir())
	svc := New(nil)
	language, retention := "es", 0
	_, err := svc.UpdateOperationalSettings(OperationalSettingsPatch{Language: &language, BackupRetention: &retention})
	if err == nil {
		t.Fatal("invalid retention accepted")
	}
	got, err := svc.GetOperationalSettings()
	if err != nil || got.Language != config.DefaultLanguage {
		t.Fatalf("partial invalid update: %+v %v", got, err)
	}
}

func TestOperationalSettingsSaveFailurePreservesMemory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, dir)
	svc := New(nil)
	if err := os.Mkdir(filepath.Join(dir, config.ConfigFileName), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, config.ConfigFileName, "keep"), []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	enabled := true
	_, err := svc.UpdateOperationalSettings(OperationalSettingsPatch{AutoApplyUpdates: &enabled})
	if err == nil {
		t.Fatal("expected save failure")
	}
	got, err := svc.GetOperationalSettings()
	if err != nil || got.AutoApplyUpdates {
		t.Fatalf("failed save mutated memory: %+v %v", got, err)
	}
}

func TestOperationalSettingsSeparateServicesMergeFreshConfig(t *testing.T) {
	t.Setenv(config.ConfigEnvVar, t.TempDir())
	first, second := New(nil), New(nil)
	language, enabled := "es", true
	if _, err := first.UpdateOperationalSettings(OperationalSettingsPatch{Language: &language}); err != nil {
		t.Fatal(err)
	}
	if _, err := second.UpdateOperationalSettings(OperationalSettingsPatch{AutoApplyUpdates: &enabled}); err != nil {
		t.Fatal(err)
	}
	got, err := first.GetOperationalSettings()
	if err != nil || got.Language != language || !got.AutoApplyUpdates {
		t.Fatalf("stale write: %+v %v", got, err)
	}
}

func TestOperationalSettingsRequiresAbsoluteBackupPath(t *testing.T) {
	t.Setenv(config.ConfigEnvVar, t.TempDir())
	svc := New(nil)
	for _, path := range []string{"", "backups"} {
		if _, err := svc.UpdateOperationalSettings(OperationalSettingsPatch{BackupPath: &path}); err == nil {
			t.Fatalf("accepted unsafe backup path %q", path)
		}
	}
}
