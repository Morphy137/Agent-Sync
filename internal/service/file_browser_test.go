package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/rules"
)

func TestFileBrowserRepositoryAndLocalRule(t *testing.T) {
	f := newRulesFixture(t)
	writeFile(t, filepath.Join(f.root, ".gitignore"), "*.tmp\n")
	writeFile(t, filepath.Join(f.root, ".git", "config"), "private")
	roots, err := f.svc.ListFileRoots()
	if err != nil || len(roots) != 1 || roots[0].ID != "repo" {
		t.Fatalf("roots: %+v %v", roots, err)
	}
	entries, err := f.svc.ListFileDirectory("repo", ".")
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names[".gitignore"] || !names["shared"] || names[".git"] {
		t.Fatalf("entries: %+v", entries)
	}
	for _, path := range []string{"../outside", ".git/config", "shared/../../outside", `C:\outside`} {
		if _, err := f.svc.ReadBrowserFile("repo", path); err == nil {
			t.Errorf("allowed path %q", path)
		}
	}
	data, err := f.svc.ReadBrowserFile("repo", "shared/AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SaveBrowserFile("repo", "shared/AGENTS.md", "changed", data.Hash, filepath.Join(t.TempDir(), "other")); !apperr.HasCode(err, apperr.PlanChanged) {
		t.Fatalf("wrong root must reject save: %v", err)
	}
	if _, err := f.svc.SaveBrowserFile("repo", "shared/AGENTS.md", "changed", data.Hash, data.AbsolutePath); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SaveBrowserFile("repo", "shared/AGENTS.md", "stale", data.Hash, data.AbsolutePath); !apperr.HasCode(err, apperr.PlanChanged) {
		t.Fatalf("stale save: %v", err)
	}
	if got := readFile(t, data.AbsolutePath); got != "changed" {
		t.Fatalf("content = %q", got)
	}
	writeFile(t, filepath.Join(f.root, "large.txt"), strings.Repeat("a", 2<<20))
	if _, err := f.svc.ReadBrowserFile("repo", "large.txt"); err != nil {
		t.Fatalf("2 MiB text: %v", err)
	}

	local := filepath.Join(t.TempDir(), "local.md")
	writeFile(t, local, "local")
	rule := instructionsRule()
	rule.Source = rules.Source{Origin: rules.OriginLocal, Path: local}
	saved, err := f.svc.SaveRule(rule, rules.ScopeLocal)
	if err != nil {
		t.Fatal(err)
	}
	id := "rule:" + saved.Rule.ID
	entries, err = f.svc.ListFileDirectory(id, ".")
	if err != nil || len(entries) != 1 || entries[0].Path != "." {
		t.Fatalf("local file: %+v %v", entries, err)
	}
	if _, err := f.svc.ReadBrowserFile(id, "."); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ReadBrowserFile("rule:missing", "."); err == nil {
		t.Fatal("unknown rule allowed")
	}
	if _, err := f.svc.ReadBrowserFile("rule:"+saved.Rule.ID, "../outside"); err == nil {
		t.Fatal("local escape allowed")
	}
}

func TestFileBrowserRejectsEscapingLinks(t *testing.T) {
	f := newRulesFixture(t)
	outside := filepath.Join(t.TempDir(), "outside.md")
	writeFile(t, outside, "outside")
	if err := os.Symlink(outside, filepath.Join(f.root, "outside.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := f.svc.ReadBrowserFile("repo", "outside.md"); err == nil {
		t.Fatal("read escaping link")
	}
}
