package gitops_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/gitops"
)

func requireGit(t *testing.T) {
	t.Helper()
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
}

func assertAppErrCode(t *testing.T, err error, code string) {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("expected apperr %q, got %v", code, err)
	}
}

func TestValidateRemoteURL(t *testing.T) {
	absLocal := "/srv/git/repo.git"
	if runtime.GOOS == "windows" {
		absLocal = `C:\repos\repo.git`
	}

	accepted := []string{
		"https://github.com/org/repo.git",
		"http://example.com/repo",
		"https://user@github.com:443/org/repo.git",
		"ssh://git@github.com/org/repo.git",
		"ssh://git@example.com:2222/org/repo.git",
		"git@github.com:org/repo.git",
		"git@gitlab.example.com:group/sub/repo",
		"file:///C:/Users/test/remote.git",
		"file:///tmp/remote.git",
		"file://localhost/tmp/remote.git",
		absLocal,
	}
	for _, u := range accepted {
		if err := gitops.ValidateRemoteURL(u); err != nil {
			t.Errorf("expected %q to be accepted, got %v", u, err)
		}
	}

	rejected := []string{
		"",
		"   ",
		"ext::sh -c touch% /tmp/pwned",
		"ext::sh",
		"EXT::sh",
		"fd::17",
		"https::example.com/repo",
		"-uhttps://example.com/repo",
		"--upload-pack=touch /tmp/pwned",
		"https://example.com/my repo.git",
		"https://example.com/repo.git\n",
		"https://example.com/re\tpo",
		"https://example.com/re\x00po",
		"https://example.com/re\x1bpo",
		"ssh://-oProxyCommand=calc/repo",
		"ssh://-oProxyCommand=calc@host/repo",
		"ftp://example.com/repo.git",
		"git://example.com/repo.git",
		"https:///repo.git",
		"file://evil.example.com/share/repo.git",
		"file:///",
		"relative/path/repo.git",
		"repo.git",
		"git@:repo.git",
		strings.Repeat("a", gitops.MaxRemoteURLLength+1),
	}
	for _, u := range rejected {
		err := gitops.ValidateRemoteURL(u)
		if err == nil {
			t.Errorf("expected %q to be rejected", u)
			continue
		}
		var appErr *apperr.Error
		if !errors.As(err, &appErr) || appErr.Code != apperr.InvalidRemoteURL {
			t.Errorf("expected InvalidRemoteURL for %q, got %v", u, err)
		}
	}
}

func TestCloneRejectsInvalidURL(t *testing.T) {
	requireGit(t)
	target := filepath.Join(t.TempDir(), "clone")
	err := gitops.Clone("ext::sh -c echo", target)
	assertAppErrCode(t, err, apperr.InvalidRemoteURL)
	if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
		t.Fatalf("clone target must not be created for invalid URLs")
	}
}

func TestCloneWithFileURL(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, true)
	bare := newBareRemote(t)
	seedRemote(t, bare, map[string]string{"a.md": "alpha"})

	target := filepath.Join(t.TempDir(), "clone")
	if err := gitops.Clone(fileURL(bare), target); err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	if got := readFile(t, target, "a.md"); got != "alpha" {
		t.Fatalf("unexpected cloned content %q", got)
	}
}

// (a) local folder is not a repository and the remote is empty.
func TestConnectRemoteNotRepoEmptyRemote(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, true)
	bare := newBareRemote(t)
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"CLAUDE/CLAUDE.md": "claude rules",
		"notes.md":         "notes",
	})

	res, err := gitops.ConnectRemote(root, fileURL(bare))
	if err != nil {
		t.Fatalf("ConnectRemote failed: %v", err)
	}
	if res.RemoteHadContent {
		t.Errorf("expected RemoteHadContent=false")
	}
	if res.Branch != gitops.DefaultBranch {
		t.Errorf("expected branch %q, got %q", gitops.DefaultBranch, res.Branch)
	}
	if res.BackupBranch != "" {
		t.Errorf("expected no backup branch, got %q", res.BackupBranch)
	}
	if len(res.LocalChanges) != 0 {
		t.Errorf("expected clean worktree, got %+v", res.LocalChanges)
	}

	runner := gitops.NewRunner(root)
	if !runner.HasUpstream() {
		t.Fatalf("expected upstream to be configured")
	}
	msg := runGit(t, root, "log", "-1", "--format=%s")
	if !strings.HasPrefix(msg, gitops.InitialCommitMessagePrefix) {
		t.Errorf("unexpected commit message %q", msg)
	}
	remoteFiles := runGit(t, bare, "ls-tree", "-r", "--name-only", "main")
	for _, want := range []string{"CLAUDE/CLAUDE.md", "notes.md", ".gitignore"} {
		if !strings.Contains(remoteFiles, want) {
			t.Errorf("expected %s pushed to remote, got:\n%s", want, remoteFiles)
		}
	}
}

// (b) local folder is not a repository and the remote already has content.
func TestConnectRemoteNotRepoRemoteWithContent(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, true)
	bare := newBareRemote(t)
	seedRemote(t, bare, map[string]string{
		"shared.md":       "remote version",
		"same.md":         "identical",
		"remote-only.md":  "only on remote",
		"dir/nested.md":   "nested remote",
		"blocked/file.md": "parent occupied locally",
	})

	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"shared.md":     "local version",
		"same.md":       "identical",
		"local-only.md": "only local",
		"blocked":       "a local file where the remote has a directory",
	})

	res, err := gitops.ConnectRemote(root, fileURL(bare))
	if err != nil {
		t.Fatalf("ConnectRemote failed: %v", err)
	}
	if !res.RemoteHadContent || res.Branch != "main" || res.BackupBranch != "" {
		t.Fatalf("unexpected result %+v", res)
	}

	// No local file lost or overwritten.
	if got := readFile(t, root, "shared.md"); got != "local version" {
		t.Errorf("local shared.md overwritten: %q", got)
	}
	if got := readFile(t, root, "local-only.md"); got != "only local" {
		t.Errorf("local-only.md lost: %q", got)
	}
	if got := readFile(t, root, "blocked"); got != "a local file where the remote has a directory" {
		t.Errorf("local file blocking a remote directory was replaced: %q", got)
	}
	// Remote-only files restored.
	if got := readFile(t, root, "remote-only.md"); got != "only on remote" {
		t.Errorf("remote-only.md not restored: %q", got)
	}
	if got := readFile(t, root, "dir/nested.md"); got != "nested remote" {
		t.Errorf("dir/nested.md not restored: %q", got)
	}

	changes := map[string]string{}
	for _, c := range res.LocalChanges {
		changes[c.Path] = c.Status
	}
	if changes["shared.md"] != " M" {
		t.Errorf("expected shared.md modified, got %q (all: %+v)", changes["shared.md"], res.LocalChanges)
	}
	if changes["local-only.md"] != "??" {
		t.Errorf("expected local-only.md untracked, got %q", changes["local-only.md"])
	}
	for _, unchanged := range []string{"same.md", "remote-only.md", "dir/nested.md"} {
		if status, ok := changes[unchanged]; ok {
			t.Errorf("expected %s unchanged, got status %q", unchanged, status)
		}
	}

	runner := gitops.NewRunner(root)
	head, err := runner.HeadCommit()
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := runner.UpstreamCommit()
	if err != nil {
		t.Fatal(err)
	}
	if head != upstream {
		t.Errorf("expected HEAD to equal upstream after connect")
	}
}

// (c) local repository with its own commits and a remote with content: history is backed up.
func TestConnectRemoteLocalCommitsCreatesBackup(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, true)
	bare := newBareRemote(t)
	seedRemote(t, bare, map[string]string{"remote.md": "remote"})

	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runGit(t, root, "config", "user.name", "Local")
	runGit(t, root, "config", "user.email", "local@test.local")
	writeFiles(t, root, map[string]string{"mine.md": "mine"})
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "feat: local work")
	localHead := runGit(t, root, "rev-parse", "HEAD")

	res, err := gitops.ConnectRemote(root, fileURL(bare))
	if err != nil {
		t.Fatalf("ConnectRemote failed: %v", err)
	}
	if !strings.HasPrefix(res.BackupBranch, gitops.BackupBranchPrefix) {
		t.Fatalf("expected backup branch, got %q", res.BackupBranch)
	}
	if got := runGit(t, root, "rev-parse", "refs/heads/"+res.BackupBranch); got != localHead {
		t.Errorf("backup branch points to %s, want %s", got, localHead)
	}
	if got := readFile(t, root, "mine.md"); got != "mine" {
		t.Errorf("local file lost: %q", got)
	}
	if got := readFile(t, root, "remote.md"); got != "remote" {
		t.Errorf("remote file not restored: %q", got)
	}
	found := false
	for _, c := range res.LocalChanges {
		if c.Path == "mine.md" && c.Status == "??" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected mine.md reported as untracked, got %+v", res.LocalChanges)
	}

	// Reconnecting the same remote must not create another backup (history already contained).
	again, err := gitops.ConnectRemote(root, fileURL(bare))
	if err != nil {
		t.Fatalf("second ConnectRemote failed: %v", err)
	}
	if again.BackupBranch != "" {
		t.Errorf("expected no backup on reconnect, got %q", again.BackupBranch)
	}
}

// (c') repository initialized but without commits (unborn HEAD) and a remote with content.
func TestConnectRemoteUnbornHead(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, true)
	bare := newBareRemote(t)
	seedRemote(t, bare, map[string]string{"remote.md": "remote"})

	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	writeFiles(t, root, map[string]string{"draft.md": "draft"})
	runGit(t, root, "add", "draft.md") // staged but never committed

	res, err := gitops.ConnectRemote(root, fileURL(bare))
	if err != nil {
		t.Fatalf("ConnectRemote failed: %v", err)
	}
	if res.BackupBranch != "" {
		t.Errorf("expected no backup for unborn HEAD, got %q", res.BackupBranch)
	}
	if readFile(t, root, "draft.md") != "draft" || readFile(t, root, "remote.md") != "remote" {
		t.Fatalf("expected both local and remote files present")
	}
	if !gitops.NewRunner(root).HasUpstream() {
		t.Errorf("expected upstream configured")
	}
}

// (d) missing identity with an empty remote: origin is set and GitIdentityMissing is returned;
// after configuring an identity a retry commits and pushes.
func TestConnectRemoteMissingIdentityEmptyRemote(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, false)
	bare := newBareRemote(t)
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"a.md": "alpha"})
	remoteURL := fileURL(bare)

	_, err := gitops.ConnectRemote(root, remoteURL)
	assertAppErrCode(t, err, apperr.GitIdentityMissing)

	runner := gitops.NewRunner(root)
	if got, _ := runner.RemoteURL(); got != remoteURL {
		t.Fatalf("expected origin %q to be set before failing, got %q", remoteURL, got)
	}
	if readFile(t, root, "a.md") != "alpha" {
		t.Fatalf("local file lost")
	}

	if err := runner.SetLocalIdentity("Retry User", "retry@test.local"); err != nil {
		t.Fatal(err)
	}
	res, err := gitops.ConnectRemote(root, remoteURL)
	if err != nil {
		t.Fatalf("retry ConnectRemote failed: %v", err)
	}
	if res.RemoteHadContent || !runner.HasUpstream() {
		t.Fatalf("unexpected retry result %+v", res)
	}
	if files := runGit(t, bare, "ls-tree", "-r", "--name-only", "main"); !strings.Contains(files, "a.md") {
		t.Errorf("expected a.md pushed, got %s", files)
	}
}

func TestConnectRemoteRejectsInvalidInput(t *testing.T) {
	requireGit(t)
	_, err := gitops.ConnectRemote(t.TempDir(), "ext::sh -c calc")
	assertAppErrCode(t, err, apperr.InvalidRemoteURL)

	_, err = gitops.ConnectRemote("relative/root", "https://example.com/repo.git")
	assertAppErrCode(t, err, apperr.PathNotAbsolute)

	_, err = gitops.ConnectRemote(filepath.Join(t.TempDir(), "missing"), "https://example.com/repo.git")
	assertAppErrCode(t, err, apperr.RootNotFound)
}

func TestRevisionHelpers(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, true)
	bare := newBareRemote(t)
	seed := seedRemote(t, bare, map[string]string{"keep.md": "v1", "old.md": "old"})

	clone := filepath.Join(t.TempDir(), "clone")
	if err := gitops.Clone(fileURL(bare), clone); err != nil {
		t.Fatal(err)
	}
	runner := gitops.NewRunner(clone)

	branch, ok, err := runner.RemoteDefaultBranch()
	if err != nil || !ok || branch != "main" {
		t.Fatalf("RemoteDefaultBranch = %q, %v, %v", branch, ok, err)
	}

	// Remote moves ahead: keep.md modified, old.md removed, new.md added.
	writeFiles(t, seed, map[string]string{"keep.md": "v2", "new.md": "new"})
	runGit(t, seed, "rm", "-q", "old.md")
	commitAndPush(t, seed, nil, "feat: update")
	if err := runner.Fetch(); err != nil {
		t.Fatal(err)
	}

	head, err := runner.HeadCommit()
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := runner.UpstreamCommit()
	if err != nil || upstream == head {
		t.Fatalf("expected upstream ahead of HEAD (%v)", err)
	}

	changed, err := runner.ChangedFilesBetween(head, upstream)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(changed, ",") != "keep.md,new.md,old.md" {
		t.Errorf("unexpected changed files %v", changed)
	}

	content, exists, err := runner.ShowFile("HEAD", "keep.md")
	if err != nil || !exists || string(content) != "v1" {
		t.Errorf("ShowFile HEAD keep.md = %q, %v, %v", content, exists, err)
	}
	content, exists, err = runner.ShowFile("@{u}", "keep.md")
	if err != nil || !exists || string(content) != "v2" {
		t.Errorf("ShowFile @{u} keep.md = %q, %v, %v", content, exists, err)
	}
	if _, exists, err = runner.ShowFile("HEAD", "new.md"); err != nil || exists {
		t.Errorf("expected new.md missing at HEAD (exists=%v, err=%v)", exists, err)
	}
	if _, _, err = runner.ShowFile("--output=x", "keep.md"); err == nil {
		t.Errorf("expected option-like revision to be rejected")
	}

	if err := runner.CreateBranchAt("agentsync/test-branch", head); err != nil {
		t.Fatalf("CreateBranchAt failed: %v", err)
	}
	if got := runGit(t, clone, "rev-parse", "agentsync/test-branch"); got != head {
		t.Errorf("branch points to %s, want %s", got, head)
	}
	if err := runner.CreateBranchAt("-bad", head); err == nil {
		t.Errorf("expected invalid branch name to be rejected")
	}

	if err := runner.ResetHardToUpstream(); err != nil {
		t.Fatalf("ResetHardToUpstream failed: %v", err)
	}
	if got := readFile(t, clone, "keep.md"); got != "v2" {
		t.Errorf("expected keep.md v2 after reset, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(clone, "old.md")); !os.IsNotExist(err) {
		t.Errorf("expected old.md removed after reset")
	}
}

func TestRemoteDefaultBranchEmptyRemote(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, true)
	bare := newBareRemote(t)
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	runner := gitops.NewRunner(root)
	if err := runner.AddRemote(fileURL(bare)); err != nil {
		t.Fatal(err)
	}
	branch, ok, err := runner.RemoteDefaultBranch()
	if err != nil || ok || branch != "" {
		t.Fatalf("expected (\"\", false, nil), got (%q, %v, %v)", branch, ok, err)
	}
}

// A remote configured with the ext:: transport (e.g. by a malicious config) must never execute.
func TestExtTransportIsBlocked(t *testing.T) {
	requireGit(t)
	isolateGitConfig(t, true)
	root := t.TempDir()
	runGit(t, root, "init", "-b", "main")
	marker := filepath.Join(root, "pwned")
	runGit(t, root, "config", "remote.origin.url", "ext::sh -c touch% "+filepath.ToSlash(marker))
	runGit(t, root, "config", "protocol.ext.allow", "always") // local config tries to re-enable it

	if err := gitops.NewRunner(root).Fetch(); err == nil {
		t.Fatalf("expected fetch through ext:: to fail")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("ext:: transport command was executed")
	}
}
