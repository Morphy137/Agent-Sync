package state_test

import (
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/state"
)

func TestStateLoadSaveAndBaselines(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, tempDir)

	st, err := state.Load()
	if err != nil {
		t.Fatalf("Load on empty dir failed: %v", err)
	}

	st.SetBaseline("conn-1", "CLAUDE.md", "hash123")
	st.SetBaseline("conn-1", "commands/review.md", "hash456")
	st.SetTimestamps("2026-10-29T10:00:00Z", "2026-10-29T10:05:00Z")

	if err := st.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := state.Load()
	if err != nil {
		t.Fatalf("Load after save failed: %v", err)
	}

	if loaded.GetBaseline("conn-1", "CLAUDE.md") != "hash123" {
		t.Errorf("expected hash123, got %s", loaded.GetBaseline("conn-1", "CLAUDE.md"))
	}
	if loaded.GetBaseline("conn-1", "commands/review.md") != "hash456" {
		t.Errorf("expected hash456, got %s", loaded.GetBaseline("conn-1", "commands/review.md"))
	}
	if loaded.LastSyncAt != "2026-10-29T10:00:00Z" {
		t.Errorf("expected LastSyncAt preserved, got %s", loaded.LastSyncAt)
	}

	loaded.RemoveBaseline("conn-1", "CLAUDE.md")
	if loaded.GetBaseline("conn-1", "CLAUDE.md") != "" {
		t.Errorf("expected empty baseline after remove")
	}

	loaded.RemoveConnection("conn-1")
	if len(loaded.GetConnectionBaselines("conn-1")) != 0 {
		t.Errorf("expected 0 baselines after connection removed")
	}
}
