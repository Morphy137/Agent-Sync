package gitops_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testUserName  = "Test User"
	testUserEmail = "test.user@test.local"
	testDirPerms  = 0o755
	testFilePerms = 0o644
)

// isolateGitConfig points git at a temporary global config (never the user's real one) and
// disables the system config. When withIdentity is true the temporary config holds a test identity.
func isolateGitConfig(t *testing.T, withIdentity bool) {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "gitconfig")
	content := "[init]\n\tdefaultBranch = main\n[core]\n\tautocrlf = false\n"
	if withIdentity {
		content += "[user]\n\tname = " + testUserName + "\n\temail = " + testUserEmail + "\n"
	}
	if err := os.WriteFile(cfgPath, []byte(content), testFilePerms); err != nil {
		t.Fatalf("write isolated git config: %v", err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfgPath)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// runGit runs git in dir and fails the test on error, returning trimmed stdout.
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

// fileURL converts an absolute path into a file:// URL (file:///C:/... on Windows).
func fileURL(p string) string {
	slashed := filepath.ToSlash(p)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return "file://" + slashed
}

// newBareRemote creates an empty bare repository whose HEAD points to main.
func newBareRemote(t *testing.T) string {
	t.Helper()
	bare := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, "", "init", "--bare", "--initial-branch=main", bare)
	return bare
}

// writeFiles writes relPath -> content (forward-slash paths) under root.
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

// readFile returns the content of a forward-slash relative path under root.
func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// seedRemote clones bare, commits files with a local identity and pushes them to main.
// It returns the working clone so tests can push further commits.
func seedRemote(t *testing.T, bare string, files map[string]string) string {
	t.Helper()
	work := filepath.Join(t.TempDir(), "seed")
	runGit(t, "", "clone", fileURL(bare), work)
	runGit(t, work, "config", "user.name", "Seeder")
	runGit(t, work, "config", "user.email", "seeder@test.local")
	commitAndPush(t, work, files, "feat: seed remote")
	return work
}

// commitAndPush writes files in a working clone, commits them and pushes to origin main.
func commitAndPush(t *testing.T, work string, files map[string]string, message string) {
	t.Helper()
	writeFiles(t, work, files)
	runGit(t, work, "add", "-A")
	runGit(t, work, "commit", "-m", message)
	runGit(t, work, "push", "origin", "HEAD:main")
}
