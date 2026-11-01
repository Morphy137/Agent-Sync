package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
)

func TestRuleSourceEditor(t *testing.T) {
	f := newRulesFixture(t)
	writeFile(t, filepath.Join(f.root, "shared", "other.md"), "other")
	writeFile(t, filepath.Join(f.root, "shared", "secret.key"), "secret")
	rule := instructionsRule()
	rule.Source.Path = "shared"
	rule.Source.Include = []string{"AGENTS.md"}
	saved, err := f.svc.SaveRule(rule, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	id := saved.Rule.ID
	files, err := f.svc.ListRuleSourceFiles(id)
	if err != nil || len(files) != 1 || files[0].Path != "AGENTS.md" {
		t.Fatalf("files: %+v, %v", files, err)
	}
	if _, err := f.svc.ReadRuleSourceFile(id, "other.md"); err == nil {
		t.Fatal("read unselected file")
	}
	if _, err := f.svc.ReadRuleSourceFile(id, "../agentsync.rules.json"); err == nil {
		t.Fatal("read escaped path")
	}
	file, err := f.svc.ReadRuleSourceFile(id, "AGENTS.md")
	if err != nil || file.ReadOnly {
		t.Fatalf("read: %+v, %v", file, err)
	}
	if _, err := f.svc.SaveRuleSourceFile(id, "AGENTS.md", "updated", file.Hash); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.root, "shared", "AGENTS.md")
	if got := readFile(t, path); got != "updated" {
		t.Fatalf("saved %q", got)
	}
	if _, err := f.svc.SaveRuleSourceFile(id, "AGENTS.md", "stale", file.Hash); err == nil {
		t.Fatal("accepted stale hash")
	}
	if got := readFile(t, path); got != "updated" {
		t.Fatalf("stale write changed file: %q", got)
	}
	if _, err := f.svc.SaveRuleSourceFile(id, "AGENTS.md", strings.Repeat("x", rulesync.MaxDiffBytes+1), file.Hash); err == nil {
		t.Fatal("accepted large file")
	}
	if err := os.WriteFile(path, []byte{'x', 0}, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ReadRuleSourceFile(id, "AGENTS.md"); err == nil {
		t.Fatal("accepted binary file")
	}
}

func TestRuleSourceEditorRejectsEscapingSymlink(t *testing.T) {
	f := newRulesFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeFile(t, outside, "outside")
	link := filepath.Join(f.root, "shared", "link.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	rule := instructionsRule()
	rule.Source.Path = "shared/link.md"
	saved, err := f.svc.SaveRule(rule, rules.ScopeRepo)
	if err != nil {
		t.Skipf("rule validation rejected escaping symlink: %v", err)
	}
	if _, err := f.svc.ReadRuleSourceFile(saved.Rule.ID, "."); err == nil {
		t.Fatal("read outside repository through symlink")
	}
}
