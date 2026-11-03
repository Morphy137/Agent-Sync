package gitops_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/gitops"
)

func TestGitOpsWithLocalBareRemote(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed on system")
	}

	tempDir := t.TempDir()
	bareRemote := filepath.Join(tempDir, "remote.git")
	clientADir := filepath.Join(tempDir, "client-a")
	clientBDir := filepath.Join(tempDir, "client-b")

	// 1. Initialize bare remote
	cmd := exec.Command("git", "init", "--bare", "--initial-branch", gitops.DefaultBranch, bareRemote)
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}

	// 2. Client A init repo
	if err := gitops.Init(clientADir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if !gitops.IsRepo(clientADir) {
		t.Fatalf("expected client-a to be recognized as git repo")
	}

	runnerA := gitops.NewRunner(clientADir)
	_ = runnerA.SetLocalIdentity("ClientA", "clienta@test.local")
	_ = runnerA.CommitPaths([]string{".gitignore"}, "chore: init")

	// Add remote and push upstream
	if err := runnerA.AddRemote(bareRemote); err != nil {
		t.Fatalf("AddRemote failed: %v", err)
	}

	url, err := runnerA.RemoteURL()
	if err != nil || url != bareRemote {
		t.Fatalf("expected remote URL %s, got %s (err: %v)", bareRemote, url, err)
	}

	if err := runnerA.PushSetUpstream(); err != nil {
		t.Fatalf("PushSetUpstream failed: %v", err)
	}

	if !runnerA.HasUpstream() {
		t.Fatalf("expected client A to have upstream")
	}

	// 3. Client B clone from bare remote
	cloneCmd := exec.Command("git", "clone", bareRemote, clientBDir)
	if err := cloneCmd.Run(); err != nil {
		t.Fatalf("failed to clone client B: %v", err)
	}
	runnerB := gitops.NewRunner(clientBDir)
	_ = exec.Command("git", "-C", clientBDir, "config", "user.name", "ClientB").Run()
	_ = exec.Command("git", "-C", clientBDir, "config", "user.email", "clientb@test.local").Run()

	// 4. Client A creates a file, commits and pushes
	testFileA := filepath.Join(clientADir, "CLAUDE", "CLAUDE.md")
	_ = os.MkdirAll(filepath.Dir(testFileA), 0o755)
	_ = os.WriteFile(testFileA, []byte("# Claude config v1"), 0o644)

	// Check status porcelain before commit
	changes, err := runnerA.StatusPorcelain()
	if err != nil {
		t.Fatalf("StatusPorcelain failed: %v", err)
	}
	if len(changes) == 0 {
		t.Fatalf("expected uncommitted changes in client A")
	}

	if err := runnerA.CommitPaths([]string{"CLAUDE/CLAUDE.md"}, "feat: add claude config"); err != nil {
		t.Fatalf("CommitPaths failed: %v", err)
	}

	ahead, behind, err := runnerA.AheadBehind()
	if err != nil || ahead != 1 || behind != 0 {
		t.Fatalf("expected ahead=1, behind=0, got ahead=%d, behind=%d (err: %v)", ahead, behind, err)
	}

	if err := runnerA.Push(); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// 5. Client B fetches and checks incoming
	if err := runnerB.Fetch(); err != nil {
		t.Fatalf("Fetch on client B failed: %v", err)
	}

	aheadB, behindB, err := runnerB.AheadBehind()
	if err != nil || aheadB != 0 || behindB != 1 {
		t.Fatalf("expected client B ahead=0, behind=1, got ahead=%d, behind=%d (err: %v)", aheadB, behindB, err)
	}

	incoming, err := runnerB.IncomingChangedFiles()
	if err != nil {
		t.Fatalf("IncomingChangedFiles failed: %v", err)
	}
	if len(incoming) != 1 || incoming[0] != "CLAUDE/CLAUDE.md" {
		t.Fatalf("expected incoming ['CLAUDE/CLAUDE.md'], got %v", incoming)
	}

	if err := runnerB.PullFastForward(); err != nil {
		t.Fatalf("PullFastForward failed: %v", err)
	}

	// Verify file was received by client B
	testFileB := filepath.Join(clientBDir, "CLAUDE", "CLAUDE.md")
	contentB, err := os.ReadFile(testFileB)
	if err != nil || string(contentB) != "# Claude config v1" {
		t.Fatalf("expected '# Claude config v1', got '%s'", string(contentB))
	}
}

// R4 Test: StatusPorcelain handles paths with spaces and renames cleanly via runCmdRaw.
func TestStatusPorcelainRawWithSpacesAndRename(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}

	tempDir := t.TempDir()
	if err := gitops.Init(tempDir); err != nil {
		t.Fatal(err)
	}
	runner := gitops.NewRunner(tempDir)
	_ = runner.SetLocalIdentity("Tester", "tester@test.local")
	_ = runner.CommitPaths([]string{".gitignore"}, "chore: init")

	// 1. File with space in path
	spaceDir := filepath.Join(tempDir, "Agent Profile")
	_ = os.MkdirAll(spaceDir, 0o755)
	spaceFile := filepath.Join(spaceDir, "custom configuration file.md")
	_ = os.WriteFile(spaceFile, []byte("rules with spaces"), 0o644)

	changes, err := runner.StatusPorcelain()
	if err != nil {
		t.Fatalf("StatusPorcelain failed: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 untracked change, got %d: %+v", len(changes), changes)
	}
	expectedPath := "Agent Profile/custom configuration file.md"
	if changes[0].Path != expectedPath {
		t.Errorf("expected path '%s', got '%s'", expectedPath, changes[0].Path)
	}
	if changes[0].Status != "??" {
		t.Errorf("expected status '??', got '%s'", changes[0].Status)
	}

	// Commit the file with spaces
	if err := runner.CommitPaths([]string{expectedPath}, "feat: add file with spaces"); err != nil {
		t.Fatalf("CommitPaths failed: %v", err)
	}

	// 2. Uncommitted modification in worktree: status must be " M" with leading space preserved!
	_ = os.WriteFile(spaceFile, []byte("rules with spaces - modified"), 0o644)
	changes, err = runner.StatusPorcelain()
	if err != nil {
		t.Fatalf("StatusPorcelain failed: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Status != " M" {
		t.Errorf("expected status ' M' (leading space preserved), got '%s'", changes[0].Status)
	}
	if changes[0].Path != expectedPath {
		t.Errorf("expected path '%s', got '%s'", expectedPath, changes[0].Path)
	}

	// Commit modification
	_ = runner.CommitPaths([]string{expectedPath}, "feat: update file")

	// 3. Rename with git mv
	renamedPath := "Agent Profile/renamed file.md"
	cmd := exec.Command("git", "-C", tempDir, "mv", expectedPath, renamedPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git mv failed: %v, out: %s", err, string(out))
	}

	changes, err = runner.StatusPorcelain()
	if err != nil {
		t.Fatalf("StatusPorcelain failed: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 rename change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Path != renamedPath {
		t.Errorf("expected renamed path '%s', got '%s'", renamedPath, changes[0].Path)
	}
	if !strings.HasPrefix(changes[0].Status, "R") {
		t.Errorf("expected status starting with 'R', got '%s'", changes[0].Status)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestCommitPathsSkipsIgnoredPaths(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed on system")
	}

	repo := t.TempDir()
	if err := gitops.Init(repo); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	runner := gitops.NewRunner(repo)
	if err := runner.SetLocalIdentity("Tester", "tester@test.local"); err != nil {
		t.Fatal(err)
	}
	write := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("kept.md", "v1")
	write(".gitignore", "agentsync.rules.json\n")
	if err := runner.CommitPaths([]string{".gitignore", "kept.md"}, "chore: init"); err != nil {
		t.Fatalf("initial commit: %v", err)
	}

	// An ignored path next to a real change must not make git add fail, and must not be committed.
	write("agentsync.rules.json", "{}")
	write("shared.md", "shared")
	if err := runner.CommitPaths([]string{"agentsync.rules.json", "shared.md"}, "chore: shared"); err != nil {
		t.Fatalf("commit with ignored path: %v", err)
	}
	tracked := gitOutput(t, repo, "ls-files")
	if strings.Contains(tracked, "agentsync.rules.json") || !strings.Contains(tracked, "shared.md") {
		t.Fatalf("unexpected tracked files:\n%s", tracked)
	}

	// Only ignored paths: nothing to do, no error.
	if err := runner.CommitPaths([]string{"agentsync.rules.json"}, "chore: nothing"); err != nil {
		t.Fatalf("commit of only ignored paths: %v", err)
	}

	// A tracked file that matches an ignore pattern keeps being committed.
	write(".gitignore", "agentsync.rules.json\nkept.md\n")
	write("kept.md", "v2")
	if err := runner.CommitPaths([]string{"kept.md"}, "chore: update kept"); err != nil {
		t.Fatalf("commit of tracked ignored file: %v", err)
	}
	if got := gitOutput(t, repo, "show", "HEAD:kept.md"); got != "v2" {
		t.Fatalf("tracked file not committed, HEAD has %q", got)
	}
}
