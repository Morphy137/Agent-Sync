package subscriptions_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/subscriptions"
)

const (
	testDirPerms  = 0o755
	testFilePerms = 0o644
	// unreachableURL points at a closed local port: a network failure without real network access.
	unreachableURL = "http://127.0.0.1:1/repo.git"
)

var generatedIDPattern = regexp.MustCompile(`^s_[0-9a-f]{10}$`)

// setupEnv isolates the config dir and git config (no system config, temporary global config).
func setupEnv(t *testing.T) {
	t.Helper()
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	t.Setenv(config.ConfigEnvVar, t.TempDir())
	cfgPath := filepath.Join(t.TempDir(), "gitconfig")
	content := "[init]\n\tdefaultBranch = main\n[core]\n\tautocrlf = false\n"
	if err := os.WriteFile(cfgPath, []byte(content), testFilePerms); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfgPath)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func fileURL(p string) string {
	slashed := filepath.ToSlash(p)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file://" + slashed
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), testDirPerms); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), testFilePerms); err != nil {
			t.Fatal(err)
		}
	}
}

// newPublisher creates a bare remote named "<name>.git" plus a working clone (the "second clone")
// used to push commits, seeded with files.
func newPublisher(t *testing.T, name string, files map[string]string) (bare, work string) {
	t.Helper()
	base := t.TempDir()
	bare = filepath.Join(base, name+".git")
	runGit(t, "", "init", "--bare", "--initial-branch=main", bare)
	work = filepath.Join(base, "publisher")
	runGit(t, "", "clone", fileURL(bare), work)
	runGit(t, work, "config", "user.name", "Publisher")
	runGit(t, work, "config", "user.email", "publisher@test.local")
	publish(t, work, files, "feat: initial content")
	return bare, work
}

func publish(t *testing.T, work string, files map[string]string, message string) {
	t.Helper()
	writeFiles(t, work, files)
	runGit(t, work, "add", "-A")
	runGit(t, work, "commit", "-m", message)
	runGit(t, work, "push", "origin", "HEAD:main")
}

func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected apperr %q, got %v", code, err)
	}
}

func TestDeriveName(t *testing.T) {
	cases := map[string]string{
		"https://github.com/org/agent-rules.git":   "agent-rules",
		"https://github.com/org/agent-rules/":      "agent-rules",
		"git@github.com:org/skills.git":            "skills",
		"git@github.com:skills.git":                "skills",
		"ssh://git@host:2222/team/configs":         "configs",
		"file:///C:/Users/test/shared-prompts.git": "shared-prompts",
		`C:\repos\local.git`:                       "local",
	}
	for in, want := range cases {
		if got := subscriptions.DeriveName(in); got != want {
			t.Errorf("DeriveName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRootLayout(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, cfgDir)
	want := filepath.Join(cfgDir, subscriptions.DirName, "s_abc", subscriptions.CloneDir)
	if got := subscriptions.Root("s_abc"); got != want {
		t.Fatalf("Root = %q, want %q", got, want)
	}
}

func TestSubscriptionLifecycle(t *testing.T) {
	setupEnv(t)
	bare, work := newPublisher(t, "team-rules", map[string]string{
		"README.md":                "readme v1",
		"skills/review/SKILL.md":   "review skill v1",
		"skills/old/SKILL.md":      "old skill",
		"deep/a/b/c/d.md":          "deep file",
		".github/workflows/ci.yml": "ignored dotfolder",
	})

	// Add with default name.
	sub, err := subscriptions.Add(fileURL(bare), "")
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if !generatedIDPattern.MatchString(sub.ID) {
		t.Errorf("unexpected id %q", sub.ID)
	}
	if sub.Name != "team-rules" || sub.Branch != "main" || sub.URL != fileURL(bare) {
		t.Errorf("unexpected subscription %+v", sub)
	}
	if _, err := time.Parse(time.RFC3339, sub.AddedAt); err != nil {
		t.Errorf("AddedAt %q is not RFC3339: %v", sub.AddedAt, err)
	}
	root := subscriptions.Root(sub.ID)
	if data, err := os.ReadFile(filepath.Join(root, "README.md")); err != nil || string(data) != "readme v1" {
		t.Fatalf("clone content missing: %q, %v", data, err)
	}

	// Explicit name is kept.
	named, err := subscriptions.Add(fileURL(bare), "  My Rules  ")
	if err != nil {
		t.Fatalf("Add with name failed: %v", err)
	}
	if named.Name != "My Rules" || named.ID == sub.ID {
		t.Errorf("unexpected named subscription %+v", named)
	}

	// Tree: skips .git and dot-folders, respects depth, sorted.
	tree, err := subscriptions.Tree(sub.ID, 2)
	if err != nil {
		t.Fatalf("Tree failed: %v", err)
	}
	paths := map[string]subscriptions.TreeEntry{}
	for _, e := range tree {
		paths[e.Path] = e
		if strings.HasPrefix(e.Path, ".git") {
			t.Errorf("Tree must skip %s", e.Path)
		}
	}
	if e, ok := paths["README.md"]; !ok || e.IsDir || e.Size != int64(len("readme v1")) {
		t.Errorf("README.md entry wrong: %+v", e)
	}
	if e, ok := paths["skills/review"]; !ok || !e.IsDir {
		t.Errorf("expected skills/review dir at depth 2, got %+v", e)
	}
	if _, ok := paths["skills/review/SKILL.md"]; ok {
		t.Errorf("depth 3 entry must be excluded with maxDepth=2")
	}
	fullTree, err := subscriptions.Tree(sub.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundDeep := false
	for i, e := range fullTree {
		if e.Path == "deep/a/b/c/d.md" {
			foundDeep = true
		}
		if i > 0 && fullTree[i-1].Path > e.Path {
			t.Errorf("tree not sorted at %s", e.Path)
		}
	}
	if !foundDeep {
		t.Errorf("expected deep file with maxDepth<=0, got %+v", fullTree)
	}

	// Up to date before the publisher pushes.
	info, err := subscriptions.Check(sub)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.Behind != 0 || len(info.IncomingFiles) != 0 || info.Offline || info.HeadCommit != info.RemoteCommit {
		t.Errorf("expected up-to-date info, got %+v", info)
	}

	// Second clone pushes new commits: modify, add, delete.
	runGit(t, work, "rm", "-q", "skills/old/SKILL.md")
	publish(t, work, map[string]string{
		"README.md":              "readme v2",
		"skills/new/SKILL.md":    "new skill",
		"skills/review/SKILL.md": "review skill v2",
	}, "feat: update skills")
	publish(t, work, map[string]string{"CHANGELOG.md": "changes"}, "docs: changelog")

	info, err = subscriptions.Check(sub)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.SubscriptionID != sub.ID || info.Behind != 2 || info.Offline || info.HeadCommit == info.RemoteCommit {
		t.Errorf("unexpected check info %+v", info)
	}
	wantIncoming := "CHANGELOG.md,README.md,skills/new/SKILL.md,skills/old/SKILL.md,skills/review/SKILL.md"
	if got := strings.Join(info.IncomingFiles, ","); got != wantIncoming {
		t.Errorf("incoming = %s, want %s", got, wantIncoming)
	}
	// Check never touches the working tree.
	if data, _ := os.ReadFile(filepath.Join(root, "README.md")); string(data) != "readme v1" {
		t.Errorf("Check modified the working tree: %q", data)
	}

	// IncomingDiff: modified, added, deleted, missing.
	before, after, err := subscriptions.IncomingDiff(sub.ID, "README.md")
	if err != nil || string(before) != "readme v1" || string(after) != "readme v2" {
		t.Errorf("IncomingDiff README = %q, %q, %v", before, after, err)
	}
	before, after, err = subscriptions.IncomingDiff(sub.ID, "skills/new/SKILL.md")
	if err != nil || before != nil || string(after) != "new skill" {
		t.Errorf("IncomingDiff added = %q, %q, %v", before, after, err)
	}
	before, after, err = subscriptions.IncomingDiff(sub.ID, `skills\old\SKILL.md`)
	if err != nil || string(before) != "old skill" || after != nil {
		t.Errorf("IncomingDiff deleted = %q, %q, %v", before, after, err)
	}
	_, _, err = subscriptions.IncomingDiff(sub.ID, "does/not/exist.md")
	assertCode(t, err, apperr.SourceNotFound)

	// Update applies upstream and reports changed files.
	changed, err := subscriptions.Update(sub)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(changed, ","); got != wantIncoming {
		t.Errorf("changed = %s, want %s", got, wantIncoming)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "README.md")); string(data) != "readme v2" {
		t.Errorf("Update did not apply README v2: %q", data)
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "old", "SKILL.md")); !os.IsNotExist(err) {
		t.Errorf("deleted file still present after Update")
	}
	info, err = subscriptions.Check(sub)
	if err != nil || info.Behind != 0 || len(info.IncomingFiles) != 0 {
		t.Errorf("expected up to date after Update, got %+v, %v", info, err)
	}
	changed, err = subscriptions.Update(sub)
	if err != nil || len(changed) != 0 || changed == nil {
		t.Errorf("expected empty non-nil change list on no-op update, got %v, %v", changed, err)
	}

	// Remove deletes the clone; operations then report SubscriptionNotFound.
	if err := subscriptions.Remove(sub.ID); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(root)); !os.IsNotExist(err) {
		t.Errorf("subscription dir still exists after Remove")
	}
	if err := subscriptions.Remove(sub.ID); err != nil {
		t.Errorf("Remove must be idempotent, got %v", err)
	}
	_, err = subscriptions.Check(sub)
	assertCode(t, err, apperr.SubscriptionNotFound)
	_, err = subscriptions.Tree(sub.ID, 1)
	assertCode(t, err, apperr.SubscriptionNotFound)
	if _, err := os.Stat(subscriptions.Root(named.ID)); err != nil {
		t.Errorf("removing one subscription must not affect another: %v", err)
	}
}

func TestIncomingDiffRejectsEscapingPaths(t *testing.T) {
	setupEnv(t)
	bare, _ := newPublisher(t, "rules", map[string]string{"a.md": "a"})
	sub, err := subscriptions.Add(fileURL(bare), "")
	if err != nil {
		t.Fatal(err)
	}
	// A file outside the clone that a traversal would reach.
	writeFiles(t, filepath.Dir(subscriptions.Root(sub.ID)), map[string]string{"secret.txt": "secret"})

	bad := []string{
		"",
		"../secret.txt",
		"..\\secret.txt",
		"a/../../secret.txt",
		"/etc/passwd",
		`C:\Windows\win.ini`,
		"C:/Windows/win.ini",
		"HEAD:a.md",
		".git/config",
		".",
	}
	for _, p := range bad {
		_, _, err := subscriptions.IncomingDiff(sub.ID, p)
		assertCode(t, err, apperr.InvalidRule)
	}

	// Benign relative forms are normalized.
	before, _, err := subscriptions.IncomingDiff(sub.ID, "./a.md")
	if err != nil || string(before) != "a" {
		t.Errorf("expected ./a.md to resolve, got %q, %v", before, err)
	}
}

func TestInvalidIDsAreRejected(t *testing.T) {
	setupEnv(t)
	outside := filepath.Join(config.GetConfigDir(), "keep.txt")
	writeFiles(t, config.GetConfigDir(), map[string]string{"keep.txt": "keep"})

	for _, id := range []string{"", "..", "../..", "a/b", `a\b`, "s_1/../../x"} {
		assertCode(t, subscriptions.Remove(id), apperr.SubscriptionNotFound)
		_, err := subscriptions.Tree(id, 1)
		assertCode(t, err, apperr.SubscriptionNotFound)
		_, _, err = subscriptions.IncomingDiff(id, "a.md")
		assertCode(t, err, apperr.SubscriptionNotFound)
		_, err = subscriptions.Check(config.Subscription{ID: id})
		assertCode(t, err, apperr.SubscriptionNotFound)
		_, err = subscriptions.Update(config.Subscription{ID: id})
		assertCode(t, err, apperr.SubscriptionNotFound)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("config dir content was touched: %v", err)
	}
}

func TestAddFailuresLeaveNoPartialDirs(t *testing.T) {
	setupEnv(t)
	subsDir := filepath.Join(config.GetConfigDir(), subscriptions.DirName)

	_, err := subscriptions.Add("ext::sh -c calc", "")
	assertCode(t, err, apperr.InvalidRemoteURL)

	missing := filepath.Join(t.TempDir(), "missing.git")
	if _, err := subscriptions.Add(fileURL(missing), ""); err == nil {
		t.Fatalf("expected clone of missing repo to fail")
	}

	emptyBare := filepath.Join(t.TempDir(), "empty.git")
	runGit(t, "", "init", "--bare", "--initial-branch=main", emptyBare)
	_, err = subscriptions.Add(fileURL(emptyBare), "")
	assertCode(t, err, apperr.RemoteBranchNotFound)

	entries, _ := os.ReadDir(subsDir)
	if len(entries) != 0 {
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected no leftover subscription dirs, got %v", names)
	}
}

func TestCheckOfflineReportsInsteadOfFailing(t *testing.T) {
	setupEnv(t)
	bare, _ := newPublisher(t, "rules", map[string]string{"a.md": "a"})
	sub, err := subscriptions.Add(fileURL(bare), "")
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, subscriptions.Root(sub.ID), "remote", "set-url", "origin", unreachableURL)

	info, err := subscriptions.Check(sub)
	if err != nil {
		t.Fatalf("Check must not fail when offline: %v", err)
	}
	if !info.Offline || info.HeadCommit == "" || info.HeadCommit != info.RemoteCommit {
		t.Errorf("unexpected offline info %+v", info)
	}
}
