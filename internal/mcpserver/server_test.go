package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agentsync/internal/config"
	"agentsync/internal/ops"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

const testVersion = "test"

type fixture struct {
	root    string
	home    string
	session *mcp.ClientSession
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{root: filepath.Join(base, "repo"), home: filepath.Join(base, "home")}
	t.Setenv(config.ConfigEnvVar, filepath.Join(base, "config"))
	t.Setenv(rules.HomeOverrideEnvVar, f.home)
	for _, dir := range []string{f.root, f.home, filepath.Join(f.root, "skills", "demo")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(f.root, "AGENTS.md"), "# shared instructions\n")
	writeFile(t, filepath.Join(f.root, "skills", "demo", "SKILL.md"), "---\nname: demo\ndescription: demo skill\n---\nbody\n")

	srv := New(ops.New(service.New(nil)), testVersion)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := srv.MCP().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: testVersion}, nil)
	f.session, err = client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() {
		_ = f.session.Close()
		_ = serverSession.Wait()
	})
	return f
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// call invokes a tool and decodes its structured output into out (when non-nil).
func (f *fixture) call(t *testing.T, name string, args map[string]any, out any) *mcp.CallToolResult {
	t.Helper()
	res, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: protocol error: %v", name, err)
	}
	if out != nil && !res.IsError {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("%s: decode %s: %v", name, raw, err)
		}
	}
	return res
}

func (f *fixture) mustCall(t *testing.T, name string, args map[string]any, out any) {
	t.Helper()
	if res := f.call(t, name, args, out); res.IsError {
		t.Fatalf("%s failed: %s", name, errorText(res))
	}
}

func errorText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, " ")
}

func TestListToolsExposesSchemasAndDestructiveConfirm(t *testing.T) {
	f := newFixture(t)
	res, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		byName[tool.Name] = tool
		if tool.Description == "" {
			t.Errorf("tool %s has no description", tool.Name)
		}
		if tool.InputSchema == nil {
			t.Errorf("tool %s has no input schema", tool.Name)
		}
	}
	for _, name := range []string{"status", "sync", "plan_sync", "sync_rules", "list_rules", "add_rule", "delete_rule",
		"set_rule_enabled", "add_destination", "remove_destination", "list_locations", "set_location", "remove_location",
		"detect_source", "suggest_placement", "list_subscriptions", "add_subscription", "update_subscription",
		"remove_subscription", "connect_remote", "list_repo_changes", "repo_file_diff", "discard_repo_file"} {
		if byName[name] == nil {
			t.Errorf("missing tool %s", name)
		}
	}
	for _, name := range []string{"delete_rule", "remove_destination", "remove_subscription", "discard_repo_file"} {
		schema, _ := json.Marshal(byName[name].InputSchema)
		if !strings.Contains(string(schema), `"confirm"`) {
			t.Errorf("%s schema lacks confirm: %s", name, schema)
		}
		if ann := byName[name].Annotations; ann == nil || ann.DestructiveHint == nil || !*ann.DestructiveHint {
			t.Errorf("%s must be annotated destructive", name)
		}
	}
}

func TestRuleLifecycleThroughTools(t *testing.T) {
	f := newFixture(t)

	var status ops.StatusReport
	f.mustCall(t, "set_root", map[string]any{"path": f.root}, &status)
	if status.RootPath == "" {
		t.Fatal("root not set")
	}

	claudeDir := filepath.Join(f.home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var locs LocationsOutput
	f.mustCall(t, "set_location", map[string]any{"alias": "claude", "path": claudeDir}, &locs)
	if len(locs.Locations) < 2 {
		t.Fatalf("expected home + claude, got %+v", locs.Locations)
	}

	var saved service.SaveRuleResult
	f.mustCall(t, "add_rule", map[string]any{
		"sourcePath":   "AGENTS.md",
		"destinations": []map[string]any{{"location": "claude", "path": "CLAUDE.md"}},
	}, &saved)
	ruleID := saved.Rule.ID
	if ruleID == "" || saved.Rule.Name != "AGENTS.md" || len(saved.Rule.Destinations) != 1 {
		t.Fatalf("unexpected rule: %+v", saved.Rule)
	}

	f.mustCall(t, "sync_rules", nil, nil)
	got, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil || string(got) != "# shared instructions\n" {
		t.Fatalf("destination not written: %q %v", got, err)
	}

	f.mustCall(t, "set_rule_enabled", map[string]any{"ruleId": ruleID, "enabled": false}, &saved)
	if saved.Rule.Enabled {
		t.Fatal("rule should be disabled")
	}

	res := f.call(t, "delete_rule", map[string]any{"ruleId": ruleID}, nil)
	if !res.IsError || !strings.Contains(errorText(res), "confirmation_required") {
		t.Fatalf("delete without confirm must fail, got %v", errorText(res))
	}
	f.mustCall(t, "delete_rule", map[string]any{"ruleId": ruleID, "confirm": true}, nil)

	var rulesOut RulesOutput
	f.mustCall(t, "list_rules", nil, &rulesOut)
	if len(rulesOut.Report.Rules) != 0 {
		t.Fatalf("rule not deleted: %+v", rulesOut.Report.Rules)
	}
	if _, err := os.Stat(filepath.Join(claudeDir, "CLAUDE.md")); err != nil {
		t.Fatal("deleting a rule must keep written files")
	}
}

func TestDestinationsAndDetection(t *testing.T) {
	f := newFixture(t)
	f.mustCall(t, "set_root", map[string]any{"path": f.root}, nil)

	var info struct {
		Kind string `json:"kind"`
	}
	f.mustCall(t, "detect_source", map[string]any{"path": "skills/demo"}, &info)
	if info.Kind != rules.KindSkill {
		t.Fatalf("expected skill, got %q", info.Kind)
	}

	var saved service.SaveRuleResult
	f.mustCall(t, "add_rule", map[string]any{
		"sourcePath":   "skills/demo",
		"destinations": []map[string]any{{"location": "home", "path": "skills-a/demo"}},
	}, &saved)
	ruleID := saved.Rule.ID

	f.mustCall(t, "add_destination", map[string]any{
		"ruleId":      ruleID,
		"destination": map[string]any{"location": "home", "path": "skills-b/demo", "mode": rules.ModeOneWay},
	}, &saved)
	if len(saved.Rule.Destinations) != 2 {
		t.Fatalf("expected 2 destinations, got %+v", saved.Rule.Destinations)
	}
	destID := saved.Rule.Destinations[1].ID

	res := f.call(t, "remove_destination", map[string]any{"ruleId": ruleID, "destinationId": destID}, nil)
	if !res.IsError {
		t.Fatal("remove_destination without confirm must fail")
	}
	f.mustCall(t, "remove_destination", map[string]any{"ruleId": ruleID, "destinationId": destID, "confirm": true}, &saved)
	if len(saved.Rule.Destinations) != 1 {
		t.Fatalf("destination not removed: %+v", saved.Rule.Destinations)
	}

	res = f.call(t, "remove_destination", map[string]any{"ruleId": ruleID, "destinationId": "d_missing", "confirm": true}, nil)
	if !res.IsError || !strings.Contains(errorText(res), "destination_not_found") {
		t.Fatalf("expected destination_not_found, got %s", errorText(res))
	}
}

func TestErrorsAndConfirmGuards(t *testing.T) {
	f := newFixture(t)

	var status ops.StatusReport
	f.mustCall(t, "status", nil, &status)
	if status.RootPath != "" {
		t.Fatalf("fresh config must have no root, got %q", status.RootPath)
	}

	res := f.call(t, "get_rule", map[string]any{"ruleId": "r_nope"}, nil)
	if !res.IsError || !strings.Contains(errorText(res), "rule_not_found") {
		t.Fatalf("expected rule_not_found, got %s", errorText(res))
	}
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"remove_subscription", map[string]any{"id": "s_x"}},
		{"discard_repo_file", map[string]any{"path": "AGENTS.md"}},
		{"save_gitignore", map[string]any{"content": "x\n"}},
	} {
		res := f.call(t, tc.tool, tc.args, nil)
		if !res.IsError || !strings.Contains(errorText(res), "confirmation_required") {
			t.Errorf("%s without confirm: %s", tc.tool, errorText(res))
		}
	}
	res = f.call(t, "remove_subscription", map[string]any{"id": "s_x", "confirm": true}, nil)
	if !res.IsError || !strings.Contains(errorText(res), "subscription_not_found") {
		t.Fatalf("expected subscription_not_found, got %s", errorText(res))
	}
}

func TestGitignoreTools(t *testing.T) {
	f := newFixture(t)
	f.mustCall(t, "set_root", map[string]any{"path": f.root}, nil)

	var gi service.GitignorePayload
	f.mustCall(t, "exclude_repo_path", map[string]any{"path": "secrets/token.txt"}, &gi)
	if !strings.Contains(gi.Content, "secrets/token.txt") {
		t.Fatalf("exclusion not written: %q", gi.Content)
	}
	f.mustCall(t, "add_recommended_gitignore", nil, &gi)
	if len(gi.MissingRecommended) != 0 {
		t.Fatalf("recommended patterns still missing: %v", gi.MissingRecommended)
	}
	f.mustCall(t, "save_gitignore", map[string]any{"content": "only-this\n", "confirm": true}, &gi)
	f.mustCall(t, "get_gitignore", nil, &gi)
	if strings.TrimSpace(gi.Content) != "only-this" {
		t.Fatalf("save_gitignore: %q", gi.Content)
	}
}
