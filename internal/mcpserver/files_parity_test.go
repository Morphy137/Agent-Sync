package mcpserver

import (
	"agentsync/internal/rulesync"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/ops"
	"agentsync/internal/rules"
	"agentsync/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Register directly so file parity can be tested before shared table wiring.
func newFileParityFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{root: filepath.Join(base, "repo"), home: filepath.Join(base, "home")}
	t.Setenv(config.ConfigEnvVar, filepath.Join(base, "config"))
	t.Setenv(rules.HomeOverrideEnvVar, f.home)
	for _, p := range []string{f.root, f.home} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(f.root, "AGENTS.md"), "before\n")
	srv := New(ops.New(service.New(nil)), testVersion)
	srv.registerFileTools()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := srv.MCP().Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "files-test", Version: testVersion}, nil)
	f.session, err = client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.session.Close(); _ = ss.Wait() })
	f.mustCall(t, "set_root", map[string]any{"path": f.root}, nil)
	return f
}

func TestFileParityMCPToolSchemas(t *testing.T) {
	f := newFixture(t)
	res, err := f.session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		byName[tool.Name] = tool
	}
	expected := map[string][]string{
		"list_file_roots": {}, "list_file_directory": {"rootId"}, "read_file": {"rootId", "path"},
		"save_file":              {"rootId", "path", "content", "expectedHash", "expectedPath"},
		"list_rule_source_files": {"ruleId"}, "read_rule_source_file": {"ruleId", "path"},
		"save_rule_source_file": {"ruleId", "path", "content", "expectedHash"},
		"import_into_repo":      {"sourcePath"}, "subscription_file_diff": {"id", "path"},
	}
	for name, fields := range expected {
		tool := byName[name]
		if tool == nil {
			t.Errorf("missing tool %s", name)
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		required := map[string]bool{}
		for _, field := range schema.Required {
			required[field] = true
		}
		for _, field := range fields {
			if !required[field] {
				t.Errorf("%s does not require %s: %s", name, field, raw)
			}
		}
		if name == "save_file" || name == "save_rule_source_file" {
			if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
				t.Errorf("%s must be annotated destructive", name)
			}
		}
	}
}

func TestFileParityMCPImport(t *testing.T) {
	f := newFileParityFixture(t)
	source := filepath.Join(t.TempDir(), "import.txt")
	writeFile(t, source, "imported\n")
	var result struct {
		Path string `json:"path"`
	}
	args := map[string]any{"sourcePath": source, "repoRelDir": "imports"}
	f.mustCall(t, "import_into_repo", args, &result)
	if result.Path != "imports/import.txt" {
		t.Fatalf("path: %+v", result)
	}
	data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(result.Path)))
	if err != nil || string(data) != "imported\n" {
		t.Fatalf("import content: %q %v", data, err)
	}
	if res := f.call(t, "import_into_repo", args, nil); !res.IsError {
		t.Fatal("overwrite accepted")
	}
	args["repoRelDir"] = "../escape"
	if res := f.call(t, "import_into_repo", args, nil); !res.IsError {
		t.Fatal("import traversal accepted")
	}
}

func fileParityGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestFileParityMCPSubscriptionDiffAndEditing(t *testing.T) {
	f := newFileParityFixture(t)
	remote := t.TempDir()
	fileParityGit(t, remote, "init", "-b", "main")
	fileParityGit(t, remote, "config", "user.name", "Parity")
	fileParityGit(t, remote, "config", "user.email", "parity@example.com")
	writeFile(t, filepath.Join(remote, "AGENTS.md"), "before\n")
	fileParityGit(t, remote, "add", ".")
	fileParityGit(t, remote, "commit", "-m", "initial")
	var sub config.Subscription
	f.mustCall(t, "add_subscription", map[string]any{"url": remote}, &sub)
	writeFile(t, filepath.Join(remote, "AGENTS.md"), "after\n")
	fileParityGit(t, remote, "add", ".")
	fileParityGit(t, remote, "commit", "-m", "updated")
	f.mustCall(t, "check_subscription", map[string]any{"id": sub.ID}, nil)
	var diff rulesync.Diff
	f.mustCall(t, "subscription_file_diff", map[string]any{"id": sub.ID, "path": "AGENTS.md"}, &diff)
	if diff.Left != "before\n" || diff.Right != "after\n" {
		t.Fatalf("diff: %+v", diff)
	}
	if res := f.call(t, "subscription_file_diff", map[string]any{"id": sub.ID, "path": "../outside"}, nil); !res.IsError {
		t.Fatal("diff traversal accepted")
	}
	var rule service.SaveRuleResult
	f.mustCall(t, "add_rule", map[string]any{"origin": "subscription", "subscription": sub.ID, "sourcePath": "AGENTS.md", "destinations": []map[string]any{{"location": "home", "path": "sub.md"}}}, &rule)
	var source service.RuleSourceContent
	f.mustCall(t, "read_rule_source_file", map[string]any{"ruleId": rule.Rule.ID, "path": "."}, &source)
	if !source.ReadOnly {
		t.Fatal("subscription rule source not read-only")
	}
	if res := f.call(t, "save_rule_source_file", map[string]any{"ruleId": rule.Rule.ID, "path": ".", "content": "refused", "expectedHash": source.Hash}, nil); !res.IsError || !strings.Contains(errorText(res), "source_read_only") {
		t.Fatalf("subscription rule save: %s", errorText(res))
	}
	var browser service.RuleSourceContent
	f.mustCall(t, "read_file", map[string]any{"rootId": sub.ID, "path": "AGENTS.md"}, &browser)
	if browser.ReadOnly {
		t.Fatal("subscription browser incorrectly read-only")
	}
	f.mustCall(t, "save_file", map[string]any{"rootId": sub.ID, "path": "AGENTS.md", "content": "local edit", "expectedHash": browser.Hash, "expectedPath": browser.AbsolutePath}, nil)
	f.mustCall(t, "read_file", map[string]any{"rootId": sub.ID, "path": "AGENTS.md"}, &browser)
	if browser.Content != "local edit" {
		t.Fatalf("local edit: %+v", browser)
	}
}

func TestFileParityMCPRuleSourceLifecycle(t *testing.T) {
	f := newFileParityFixture(t)
	var rule service.SaveRuleResult
	f.mustCall(t, "add_rule", map[string]any{"sourcePath": "AGENTS.md", "destinations": []map[string]any{{"location": "home", "path": "AGENTS.md"}}}, &rule)
	var files struct {
		Files []service.RuleSourceFile `json:"files"`
	}
	f.mustCall(t, "list_rule_source_files", map[string]any{"ruleId": rule.Rule.ID}, &files)
	if len(files.Files) != 1 || files.Files[0].Path != "." {
		t.Fatalf("files: %+v", files)
	}
	var before service.RuleSourceContent
	f.mustCall(t, "read_rule_source_file", map[string]any{"ruleId": rule.Rule.ID, "path": "."}, &before)
	args := map[string]any{"ruleId": rule.Rule.ID, "path": ".", "content": "updated", "expectedHash": before.Hash}
	var after FileHashOutput
	f.mustCall(t, "save_rule_source_file", args, &after)
	if after.Hash == "" || after.Hash == before.Hash {
		t.Fatalf("hash: %+v", after)
	}
	if res := f.call(t, "save_rule_source_file", args, nil); !res.IsError || !strings.Contains(errorText(res), "plan_changed") {
		t.Fatalf("stale rule hash: %s", errorText(res))
	}
	args["expectedHash"] = ""
	if res := f.call(t, "save_rule_source_file", args, nil); !res.IsError {
		t.Fatal("unguarded rule save")
	}
	for _, tool := range []string{"read_rule_source_file", "save_rule_source_file"} {
		args := map[string]any{"ruleId": rule.Rule.ID, "path": "../outside"}
		if tool == "save_rule_source_file" {
			args["content"] = "escape"
			args["expectedHash"] = after.Hash
		}
		if res := f.call(t, tool, args, nil); !res.IsError {
			t.Fatalf("traversal accepted: %s", tool)
		}
	}
}

func TestFileParityMCPBrowserLifecycle(t *testing.T) {
	f := newFileParityFixture(t)
	var roots FileRootsOutput
	f.mustCall(t, "list_file_roots", nil, &roots)
	if len(roots.Roots) != 1 || roots.Roots[0].ID != "repo" {
		t.Fatalf("roots: %+v", roots)
	}
	var entries FileDirectoryOutput
	f.mustCall(t, "list_file_directory", map[string]any{"rootId": "repo"}, &entries)
	if len(entries.Entries) == 0 {
		t.Fatal("empty directory")
	}
	var before service.RuleSourceContent
	f.mustCall(t, "read_file", map[string]any{"rootId": "repo", "path": "AGENTS.md"}, &before)
	if before.Content != "before\n" || before.Hash == "" || before.AbsolutePath == "" {
		t.Fatalf("read: %+v", before)
	}
	save := map[string]any{"rootId": "repo", "path": "AGENTS.md", "content": "", "expectedHash": before.Hash, "expectedPath": before.AbsolutePath}
	var saved FileHashOutput
	f.mustCall(t, "save_file", save, &saved)
	if saved.Hash == "" || saved.Hash == before.Hash {
		t.Fatalf("hash: %+v", saved)
	}
	if res := f.call(t, "save_file", save, nil); !res.IsError || !strings.Contains(errorText(res), "plan_changed") {
		t.Fatalf("stale hash accepted: %s", errorText(res))
	}
	save["expectedHash"] = saved.Hash
	save["expectedPath"] = filepath.Join(t.TempDir(), "other")
	if res := f.call(t, "save_file", save, nil); !res.IsError || !strings.Contains(errorText(res), "plan_changed") {
		t.Fatalf("wrong path accepted: %s", errorText(res))
	}
	for _, tool := range []string{"list_file_directory", "read_file", "save_file"} {
		args := map[string]any{"rootId": "repo", "path": "../outside"}
		if tool == "save_file" {
			args["content"] = "escape"
			args["expectedHash"] = saved.Hash
			args["expectedPath"] = before.AbsolutePath
		}
		if res := f.call(t, tool, args, nil); !res.IsError {
			t.Fatalf("traversal accepted by %s", tool)
		}
	}
	for _, guard := range []string{"expectedHash", "expectedPath"} {
		args := map[string]any{"rootId": "repo", "path": "AGENTS.md", "content": "unguarded", "expectedHash": saved.Hash, "expectedPath": before.AbsolutePath}
		args[guard] = ""
		if res := f.call(t, "save_file", args, nil); !res.IsError {
			t.Fatalf("empty %s accepted", guard)
		}
	}
	data, err := os.ReadFile(filepath.Join(f.root, "AGENTS.md"))
	if err != nil || string(data) != "" {
		t.Fatalf("guarded content: %q %v", data, err)
	}
}
