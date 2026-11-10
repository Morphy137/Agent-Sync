package mcpserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/ops"
	"agentsync/internal/rules"
	"agentsync/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func categoryParityMCP(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{root: filepath.Join(base, "repo"), home: filepath.Join(base, "home")}
	t.Setenv(config.ConfigEnvVar, filepath.Join(base, "config"))
	t.Setenv(rules.HomeOverrideEnvVar, f.home)
	for _, path := range []string{f.root, f.home} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(f.root, "AGENTS.md"), "# Instructions")
	srv := New(ops.New(service.New(nil)), testVersion)
	srv.registerCategoryTools()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := srv.MCP().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "category-parity", Version: testVersion}, nil)
	f.session, err = client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.session.Close(); _ = serverSession.Wait() })
	return f
}

func TestCategoryParityMCPSave(t *testing.T) {
	f := categoryParityMCP(t)
	var saved rules.Category
	f.mustCall(t, "save_category", map[string]any{"category": map[string]any{"name": "Work", "color": "purple", "icon": "star"}, "scope": "local"}, &saved)
	if saved.ID == "" || saved.Scope != "local" || saved.Icon != "star" {
		t.Fatalf("saved: %+v", saved)
	}
	f.mustCall(t, "save_category", map[string]any{"category": map[string]any{"id": saved.ID, "name": "Updated", "color": "teal"}, "scope": "local"}, &saved)
	var out struct {
		Categories []rules.Category `json:"categories"`
	}
	f.mustCall(t, "list_categories", nil, &out)
	if len(out.Categories) != 4 || out.Categories[3].Name != "Updated" || out.Categories[3].Icon != "star" {
		t.Fatalf("list: %+v", out)
	}
	for _, args := range []map[string]any{
		{"category": map[string]any{"name": "X", "color": "blue", "typo": true}, "scope": "local"},
		{"category": map[string]any{"name": "X", "color": "blue"}, "scope": "local", "typo": true},
		{"category": map[string]any{"name": "X", "color": "blue"}},
		{"category": map[string]any{"name": "X", "color": "blue"}, "scope": ""},
		{"category": map[string]any{"name": "X", "color": "invalid"}, "scope": "local"},
	} {
		result, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "save_category", Arguments: args})
		if err == nil && !result.IsError {
			t.Fatalf("accepted invalid args: %+v", args)
		}
	}
}

func TestCategoryParityMCPDelete(t *testing.T) {
	f := categoryParityMCP(t)
	var saved rules.Category
	f.mustCall(t, "save_category", map[string]any{"category": map[string]any{"name": "Work", "color": "blue"}, "scope": "local"}, &saved)
	file, err := rules.LoadLocalRules()
	if err != nil {
		t.Fatal(err)
	}
	file.Rules = []rules.Rule{{ID: "assigned", CategoryID: saved.ID}}
	if err := rules.SaveLocalRules(file); err != nil {
		t.Fatal(err)
	}
	args := map[string]any{"id": saved.ID, "scope": "local"}
	result := f.call(t, "delete_category", args, nil)
	if !result.IsError || !strings.Contains(errorText(result), "confirmation_required") {
		t.Fatalf("confirmation: %s", errorText(result))
	}
	var out struct {
		Categories []rules.Category `json:"categories"`
	}
	f.mustCall(t, "list_categories", nil, &out)
	if len(out.Categories) != 4 {
		t.Fatalf("unconfirmed deletion mutated categories: %+v", out)
	}
	args["confirm"] = true
	f.mustCall(t, "delete_category", args, nil)
	file, err = rules.LoadLocalRules()
	if err != nil || file.Rules[0].CategoryID != "uncategorized" || len(file.Categories) != 0 {
		t.Fatalf("assignment not cleared: %+v %v", file, err)
	}
	tools, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "delete_category" {
			raw, _ := json.Marshal(tool.InputSchema)
			if !strings.Contains(string(raw), `"confirm"`) || !strings.Contains(string(raw), `"scope"`) || tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Fatalf("delete metadata: %+v %s", tool, raw)
			}
		}
	}
}

func TestCategoryParityMCPMigrate(t *testing.T) {
	f := categoryParityMCP(t)
	if err := rules.SaveLocalRules(&rules.File{Version: rules.CurrentVersion, Rules: []rules.Rule{{ID: "legacy", Source: rules.Source{Origin: "local", Path: filepath.Join(f.root, "AGENTS.md")}}}}); err != nil {
		t.Fatal(err)
	}
	f.mustCall(t, "list_categories", nil, nil)
	result := f.call(t, "migrate_categories", nil, nil)
	if !result.IsError || !strings.Contains(errorText(result), "confirmation_required") {
		t.Fatalf("confirmation: %s", errorText(result))
	}
	before, err := rules.LoadLocalRules()
	if err != nil || before.Rules[0].CategoryID != "" {
		t.Fatalf("implicit migration: %+v %v", before, err)
	}
	f.mustCall(t, "migrate_categories", map[string]any{"confirm": true}, nil)
	after, err := rules.LoadLocalRules()
	if err != nil || after.Rules[0].CategoryID != rules.KindInstructions {
		t.Fatalf("migration: %+v %v", after, err)
	}
	f.mustCall(t, "migrate_categories", map[string]any{"confirm": true}, nil)
	tools, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "migrate_categories" && (tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint) {
			t.Fatalf("migration must advertise writes: %+v", tool)
		}
	}
}

func TestCategoryParityMCPSuggest(t *testing.T) {
	f := categoryParityMCP(t)
	f.mustCall(t, "set_root", map[string]any{"path": f.root}, nil)
	var out struct {
		CategoryID string `json:"categoryId"`
	}
	f.mustCall(t, "suggest_category", map[string]any{"source": map[string]any{"path": "AGENTS.md"}}, &out)
	if out.CategoryID != rules.KindInstructions {
		t.Fatalf("repo suggestion: %+v", out)
	}
	f.mustCall(t, "suggest_category", map[string]any{"source": map[string]any{"path": filepath.Join(f.root, "AGENTS.md"), "origin": "local"}, "kind": "file"}, &out)
	if out.CategoryID != "uncategorized" {
		t.Fatalf("explicit kind: %+v", out)
	}
	f.mustCall(t, "suggest_category", map[string]any{"source": map[string]any{"path": "AGENTS.md", "origin": "subscription", "subscription": "missing"}}, &out)
	if out.CategoryID != "uncategorized" {
		t.Fatalf("unavailable subscription: %+v", out)
	}
	f.mustCall(t, "delete_category", map[string]any{"id": "instructions", "scope": "builtin", "confirm": true}, nil)
	f.mustCall(t, "suggest_category", map[string]any{"source": map[string]any{"path": "AGENTS.md"}}, &out)
	if out.CategoryID != "uncategorized" {
		t.Fatalf("deleted builtin: %+v", out)
	}
}

func TestCategoryParityMCPSchemas(t *testing.T) {
	f := categoryParityMCP(t)
	result, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	tools := map[string]*mcp.Tool{}
	for _, tool := range result.Tools {
		tools[tool.Name] = tool
	}
	for _, name := range []string{"list_categories", "save_category", "delete_category", "migrate_categories", "suggest_category"} {
		tool := tools[name]
		if tool == nil || tool.Description == "" || tool.OutputSchema == nil {
			t.Fatalf("missing schema or metadata: %s %+v", name, tool)
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		if schema["type"] != "object" || schema["additionalProperties"] != false {
			t.Fatalf("open input schema: %s %s", name, raw)
		}
		if name == "save_category" {
			if tool.Annotations == nil || tool.Annotations.IdempotentHint {
				t.Fatalf("creating generated IDs is not idempotent: %+v", tool.Annotations)
			}
			props := schema["properties"].(map[string]any)
			category := props["category"].(map[string]any)
			if category["additionalProperties"] != false {
				t.Fatalf("open category schema: %s", raw)
			}
		}
	}
	for name, args := range map[string]map[string]any{
		"list_categories":    {"unknown": true},
		"delete_category":    {"id": "skill", "scope": "builtin", "confirm": true, "unknown": true},
		"migrate_categories": {"confirm": true, "unknown": true},
		"suggest_category":   {"source": map[string]any{"path": "AGENTS.md", "unknown": true}},
	} {
		res, err := f.session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err == nil && !res.IsError {
			t.Fatalf("%s accepted unknown field", name)
		}
	}
}

func TestCategoryParityMCPList(t *testing.T) {
	f := categoryParityMCP(t)
	var out struct {
		Categories []rules.Category `json:"categories"`
	}
	f.mustCall(t, "list_categories", nil, &out)
	if len(out.Categories) != 3 {
		t.Fatalf("builtins: %+v", out)
	}
	tools, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		if tool.Name == "list_categories" {
			schema, err := json.Marshal(tool.OutputSchema)
			if err != nil || len(schema) == 0 || tool.Description == "" || tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
				t.Fatalf("list metadata: %+v %s %v", tool, schema, err)
			}
		}
	}
}
