package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/rulesync"
	"agentsync/internal/service"
)

// Explicit registration keeps these tests runnable before the command table is wired.
func registerParityFileCommands(t *testing.T) {
	t.Helper()
	old := commands
	existing := map[string]bool{}
	for _, c := range commands {
		existing[c.path] = true
	}
	for _, c := range fileCommands() {
		if !existing[c.path] {
			commands = append(commands, c)
		}
	}
	t.Cleanup(func() { commands = old })
}

func TestFileParityCLIImportAndSubscriptionDiff(t *testing.T) {
	f := newFixture(t)
	registerParityFileCommands(t)
	mustJSON(t, nil, "root", "set", f.root)
	source := filepath.Join(t.TempDir(), "import.txt")
	mustWrite(t, source, "imported\n")
	var imported string
	mustJSON(t, &imported, "repo", "import", source, "--dir", "imports")
	if imported != "imports/import.txt" {
		t.Fatalf("imported path: %q", imported)
	}
	data, err := os.ReadFile(filepath.Join(f.root, filepath.FromSlash(imported)))
	if err != nil || string(data) != "imported\n" {
		t.Fatalf("import contents: %q %v", data, err)
	}
	if code, _ := runJSON(t, nil, "repo", "import", source, "--dir", "imports"); code != ExitError {
		t.Fatal("import overwrote existing target")
	}
	if code, _ := runJSON(t, nil, "repo", "import", source, "--dir", "../escape"); code != ExitError {
		t.Fatal("import traversal accepted")
	}
	remote := t.TempDir()
	gitIn(t, remote, "init", "-b", "main")
	gitIn(t, remote, "config", "user.name", "Parity")
	gitIn(t, remote, "config", "user.email", "parity@example.com")
	mustWrite(t, filepath.Join(remote, "file.txt"), "before\n")
	gitIn(t, remote, "add", ".")
	gitIn(t, remote, "commit", "-m", "initial")
	var sub config.Subscription
	mustJSON(t, &sub, "subscriptions", "add", remote)
	mustWrite(t, filepath.Join(remote, "file.txt"), "after\n")
	gitIn(t, remote, "add", ".")
	gitIn(t, remote, "commit", "-m", "update")
	mustJSON(t, nil, "subscriptions", "check", sub.ID)
	var diff rulesync.Diff
	mustJSON(t, &diff, "subscriptions", "diff", sub.ID, "file.txt")
	if diff.Left != "before\n" || diff.Right != "after\n" {
		t.Fatalf("incoming diff: %+v", diff)
	}
	if code, _ := runJSON(t, nil, "subscriptions", "diff", sub.ID, "../file.txt"); code != ExitError {
		t.Fatal("diff traversal accepted")
	}
	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--source", "file.txt", "--origin", "subscription", "--subscription", sub.ID, "--to", "home:sub.txt")
	var selected service.RuleSourceContent
	mustJSON(t, &selected, "rules", "read-file", saved.Rule.ID, ".")
	if !selected.ReadOnly {
		t.Fatal("subscription rule source should be read-only")
	}
	if code, env := runJSON(t, nil, "rules", "save-file", saved.Rule.ID, ".", "--content-file", source, "--expected-hash", selected.Hash); code != ExitError || env.Error.Code != "source_read_only" {
		t.Fatalf("subscription rule save: %d %+v", code, env)
	}
	var browser service.RuleSourceContent
	mustJSON(t, &browser, "files", "read", "--root", sub.ID, "--path", "file.txt")
	if browser.ReadOnly {
		t.Fatal("subscription browser should permit local edits")
	}
	mustJSON(t, nil, "files", "save", "--root", sub.ID, "--path", "file.txt", "--content-file", source, "--expected-hash", browser.Hash, "--expected-path", browser.AbsolutePath)
	mustJSON(t, &browser, "files", "read", "--root", sub.ID, "--path", "file.txt")
	if browser.Content != "imported\n" {
		t.Fatalf("local clone edit: %+v", browser)
	}
}

func TestReadFileContentUsesInjectedStdin(t *testing.T) {
	decoy := filepath.Join(t.TempDir(), "os-stdin.txt")
	mustWrite(t, decoy, "wrong reader")
	osInput, err := os.Open(decoy)
	if err != nil {
		t.Fatal(err)
	}
	oldOSStdin, oldStdin := os.Stdin, stdin
	os.Stdin = osInput
	stdin = strings.NewReader("injected contents\n")
	t.Cleanup(func() { os.Stdin = oldOSStdin; stdin = oldStdin; osInput.Close() })
	got, err := readFileContent("-")
	if err != nil || got != "injected contents\n" {
		t.Fatalf("stdin content = %q, %v; want injected contents", got, err)
	}
}

func TestFileParityCLIRuleSourceLifecycle(t *testing.T) {
	f := newFixture(t)
	registerParityFileCommands(t)
	mustJSON(t, nil, "root", "set", f.root)
	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--source", "AGENTS.md", "--to", "home:AGENTS.md")
	id := saved.Rule.ID
	var files []service.RuleSourceFile
	mustJSON(t, &files, "rules", "files", id)
	if len(files) != 1 || files[0].Path != "." {
		t.Fatalf("source files: %+v", files)
	}
	var before service.RuleSourceContent
	mustJSON(t, &before, "rules", "read-file", id, ".")
	input := filepath.Join(t.TempDir(), "stdin.txt")
	mustWrite(t, input, "")
	inputReader, err := os.Open(input)
	if err != nil {
		t.Fatal(err)
	}
	oldStdin := stdin
	stdin = inputReader
	t.Cleanup(func() { stdin = oldStdin; inputReader.Close() })
	save := []string{"rules", "save-file", id, ".", "--content-file", "-", "--expected-hash", before.Hash}
	mustJSON(t, nil, save...)
	var after service.RuleSourceContent
	mustJSON(t, &after, "rules", "read-file", id, ".")
	if after.Content != "" || after.Hash == before.Hash {
		t.Fatalf("empty save: %+v", after)
	}
	if code, env := runJSON(t, nil, save...); code != ExitError || env.Error.Code != "plan_changed" {
		t.Fatalf("stale rule hash: %d %+v", code, env)
	}
	for _, action := range []string{"read-file", "save-file"} {
		args := []string{"rules", action, id, "../AGENTS.md"}
		if action == "save-file" {
			args = append(args, "--content-file", input, "--expected-hash", after.Hash)
		}
		if code, _ := runJSON(t, nil, args...); code != ExitError {
			t.Fatalf("rule traversal %s: %d", action, code)
		}
	}
	if code, _ := runJSON(t, nil, "rules", "save-file", id, ".", "--content-file", input); code != ExitUsage {
		t.Fatalf("missing hash: %d", code)
	}
}

func TestFileParityCLIBrowserLifecycle(t *testing.T) {
	f := newFixture(t)
	registerParityFileCommands(t)
	mustJSON(t, nil, "root", "set", f.root)
	var roots []service.FileRoot
	mustJSON(t, &roots, "files", "roots")
	if len(roots) != 1 || roots[0].ID != "repo" {
		t.Fatalf("roots: %+v", roots)
	}
	var entries []service.FileDirectoryEntry
	mustJSON(t, &entries, "files", "list", "--root", "repo")
	if len(entries) == 0 {
		t.Fatal("empty directory")
	}
	var before service.RuleSourceContent
	mustJSON(t, &before, "files", "read", "--root", "repo", "--path", "AGENTS.md")
	if before.Content != "# shared\n" || before.Hash == "" || before.AbsolutePath == "" {
		t.Fatalf("read: %+v", before)
	}
	content := filepath.Join(t.TempDir(), "content.txt")
	mustWrite(t, content, "updated\n")
	save := []string{"files", "save", "--root", "repo", "--path", "AGENTS.md", "--content-file", content, "--expected-hash", before.Hash, "--expected-path", before.AbsolutePath}
	mustJSON(t, nil, save...)
	var after service.RuleSourceContent
	mustJSON(t, &after, "files", "read", "--root", "repo", "--path", "AGENTS.md")
	if after.Content != "updated\n" || before.Hash == after.Hash {
		t.Fatalf("saved: %+v", after)
	}
	if code, env := runJSON(t, nil, save...); code != ExitError || env.Error.Code != "plan_changed" {
		t.Fatalf("stale hash: %d %+v", code, env)
	}
	wrongPath := []string{"files", "save", "--root", "repo", "--path", "AGENTS.md", "--content-file", content, "--expected-hash", after.Hash, "--expected-path", filepath.Join(t.TempDir(), "other")}
	if code, env := runJSON(t, nil, wrongPath...); code != ExitError || env.Error.Code != "plan_changed" {
		t.Fatalf("wrong resolved path: %d %+v", code, env)
	}
	for _, action := range []string{"list", "read", "save"} {
		args := []string{"files", action, "--root", "repo", "--path", "../outside"}
		if action == "save" {
			args = append(args, "--content-file", content, "--expected-hash", after.Hash, "--expected-path", after.AbsolutePath)
		}
		if code, _ := runJSON(t, nil, args...); code != ExitError {
			t.Fatalf("%s traversal accepted: %d", action, code)
		}
	}
	for _, args := range [][]string{
		{"files", "read", "--path", "AGENTS.md"},
		{"files", "save", "--root", "repo", "--path", "AGENTS.md", "--content-file", content, "--expected-hash", after.Hash},
		{"files", "save", "--root", "repo", "--path", "AGENTS.md", "--content-file", content, "--expected-path", after.AbsolutePath},
	} {
		if code, _ := runJSON(t, nil, args...); code != ExitUsage {
			t.Fatalf("missing required flag: %v: %d", args, code)
		}
	}
	data, err := os.ReadFile(filepath.Join(f.root, "AGENTS.md"))
	if err != nil || string(data) != "updated\n" {
		t.Fatalf("guarded file: %q %v", data, err)
	}
}
