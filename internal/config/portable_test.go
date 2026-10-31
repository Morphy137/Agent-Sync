package config

import (
	"os"
	"path/filepath"
	"testing"
)

func withExecutable(t *testing.T, path string) {
	t.Helper()
	previous := executablePath
	executablePath = func() (string, error) { return path, nil }
	t.Cleanup(func() { executablePath = previous })
}

func TestPortableModeUsesDataFolderNextToExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "agentsync.exe")
	withExecutable(t, exe)
	t.Setenv(ConfigEnvVar, "")

	if PortableDataDir() != "" {
		t.Fatal("without the marker the app must not be portable")
	}
	if err := os.WriteFile(filepath.Join(dir, PortableMarkerName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, PortableDataDirName)
	if got := GetConfigDir(); got != want {
		t.Fatalf("GetConfigDir() = %q, want %q", got, want)
	}
	if !IsPortable() {
		t.Fatal("expected portable mode")
	}
	if got := DefaultConfig().BackupPath; got != filepath.Join(want, "backups") {
		t.Fatalf("backups should live in the portable data folder, got %q", got)
	}
}

func TestEnvironmentOverridesPortableMode(t *testing.T) {
	dir := t.TempDir()
	withExecutable(t, filepath.Join(dir, "agentsync.exe"))
	if err := os.WriteFile(filepath.Join(dir, PortableMarkerName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(dir, "override")
	t.Setenv(ConfigEnvVar, override)
	if got := GetConfigDir(); got != override {
		t.Fatalf("GetConfigDir() = %q, want %q", got, override)
	}
	if IsPortable() {
		t.Fatal("the environment override wins over portable mode")
	}
}
