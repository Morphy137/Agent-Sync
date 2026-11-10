package mcpserver

import (
	"context"
	"testing"

	"agentsync/internal/service"
)

func TestOperationalSettingsThroughMCP(t *testing.T) {
	f := newFixture(t)
	var got service.OperationalSettings
	f.mustCall(t, "update_settings", map[string]any{"autoApplyUpdates": true, "checkIntervalMinutes": 0}, &got)
	if !got.AutoApplyUpdates || got.CheckIntervalMinutes != 0 {
		t.Fatalf("not applied: %+v", got)
	}
	f.mustCall(t, "get_settings", nil, &got)
	if !got.AutoApplyUpdates {
		t.Fatal("not persisted")
	}
	res := f.call(t, "add_remote", map[string]any{"url": "https://example.invalid/repo.git"}, nil)
	if !res.IsError {
		t.Fatal("push without confirmation accepted")
	}
}

func TestAddRemoteToolNetworkAnnotations(t *testing.T) {
	f := newFixture(t)
	res, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "add_remote" {
			continue
		}
		if tool.Annotations == nil {
			t.Fatal("missing add_remote annotations")
		}
		annotations := tool.Annotations
		if annotations.DestructiveHint == nil || !*annotations.DestructiveHint {
			t.Error("add_remote must advertise destructive writes")
		}
		if annotations.OpenWorldHint != nil && !*annotations.OpenWorldHint {
			t.Error("add_remote pushes to a network remote but advertises closed-world behavior")
		}
		if annotations.ReadOnlyHint {
			t.Error("add_remote must not advertise read-only behavior")
		}
		return
	}
	t.Fatal("add_remote missing from tools/list")
}

func TestSettingsAndSetupToolSchemas(t *testing.T) {
	f := newFixture(t)
	res, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tool := range res.Tools {
		names[tool.Name] = true
	}
	for _, name := range []string{"get_settings", "update_settings", "create_root", "clone_repository", "init_repository", "get_git_info", "set_git_identity", "add_remote", "get_recognizers_path"} {
		if !names[name] {
			t.Errorf("missing %s", name)
		}
	}
}
