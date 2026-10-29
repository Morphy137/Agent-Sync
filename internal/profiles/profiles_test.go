package profiles_test

import (
	"os"
	"path/filepath"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/profiles"
)

func TestDiscoverProfiles(t *testing.T) {
	tempDir := t.TempDir()

	// Profiles: CLAUDE, GEMINI, CODEX, .git (ignored), .hidden (ignored)
	dirs := []string{"CLAUDE", "GEMINI", "CODEX", ".git", ".hidden"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tempDir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Add files inside profiles
	if err := os.WriteFile(filepath.Join(tempDir, "CLAUDE", "CLAUDE.md"), []byte("claude"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "GEMINI", "GEMINI.md"), []byte("gemini"), 0o644); err != nil {
		t.Fatal(err)
	}

	profs, err := profiles.Discover(tempDir)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	if len(profs) != 3 {
		t.Fatalf("expected 3 profiles (CLAUDE, CODEX, GEMINI), got %d", len(profs))
	}

	expectedNames := []string{"CLAUDE", "CODEX", "GEMINI"}
	for i, expected := range expectedNames {
		if profs[i].Name != expected {
			t.Errorf("at index %d expected %s, got %s", i, expected, profs[i].Name)
		}
	}

	if profs[0].FileCount != 1 {
		t.Errorf("expected CLAUDE file count 1, got %d", profs[0].FileCount)
	}
	if profs[1].FileCount != 0 {
		t.Errorf("expected CODEX file count 0, got %d", profs[1].FileCount)
	}

	// Test non-existent root
	_, err = profiles.Discover(filepath.Join(tempDir, "nonexistent"))
	if err == nil {
		t.Errorf("expected error for nonexistent root")
	}
	appErr, ok := err.(*apperr.Error)
	if !ok || appErr.Code != apperr.RootNotFound {
		t.Errorf("expected RootNotFound error, got %v", err)
	}
}
