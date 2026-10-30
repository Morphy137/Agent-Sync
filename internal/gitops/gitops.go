package gitops

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/fsops"
)

const (
	GitCommandTimeout = 60 * time.Second
	CloneTimeout      = 5 * time.Minute
)

// ChangedPath represents a file changed in git status.
type ChangedPath struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

// GitRunner wraps execution of git commands in a repository root.
type GitRunner struct {
	Root         string
	OnBeforePush func()
}

// NewRunner creates a GitRunner for the given repository root directory.
func NewRunner(root string) *GitRunner {
	return &GitRunner{Root: root}
}

// runCmd executes a git command in the repository root directory with standard environment and flags.
func (g *GitRunner) runCmd(args ...string) (string, error) {
	if !IsInstalled() {
		return "", apperr.New(apperr.GitNotInstalled)
	}

	ctx, cancel := context.WithTimeout(context.Background(), GitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Root
	cmd.SysProcAttr = getSysProcAttr()

	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	if err != nil {
		combined := errStr
		if combined == "" {
			combined = outStr
		}
		return outStr, mapGitError(err, combined)
	}

	return outStr, nil
}

// runCmdRaw executes a git command returning raw unmodified stdout without trimming whitespace (R4).
func (g *GitRunner) runCmdRaw(args ...string) (string, error) {
	if !IsInstalled() {
		return "", apperr.New(apperr.GitNotInstalled)
	}

	ctx, cancel := context.WithTimeout(context.Background(), GitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Root
	cmd.SysProcAttr = getSysProcAttr()

	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outStr := stdout.String()
	errStr := strings.TrimSpace(stderr.String())

	if err != nil {
		combined := errStr
		if combined == "" {
			combined = strings.TrimSpace(outStr)
		}
		return outStr, mapGitError(err, combined)
	}

	return outStr, nil
}

// mapGitError converts git CLI stderr messages into typed domain errors.
func mapGitError(cause error, stderr string) error {
	lower := strings.ToLower(stderr)

	if strings.Contains(lower, "not a git repository") {
		return apperr.Wrap(apperr.RootNotRepo, cause)
	}
	if strings.Contains(lower, "author identity unknown") ||
		strings.Contains(lower, "unable to auto-detect email address") ||
		strings.Contains(lower, "please tell me who you are") {
		return apperr.Wrap(apperr.GitIdentityMissing, cause)
	}
	if strings.Contains(lower, "terminal prompts disabled") ||
		strings.Contains(lower, "authentication failed") ||
		strings.Contains(lower, "permission denied (publickey)") {
		return apperr.Wrap(apperr.GitAuthFailed, cause)
	}
	if strings.Contains(lower, "could not resolve host") ||
		strings.Contains(lower, "failed to connect") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "network is unreachable") {
		return apperr.Wrap(apperr.GitNetwork, cause)
	}
	if strings.Contains(lower, "no remote repository specified") ||
		strings.Contains(lower, "no configured push destination") ||
		strings.Contains(lower, "no upstream") {
		return apperr.Wrap(apperr.GitNoRemote, cause)
	}
	if strings.Contains(lower, "not possible to fast-forward, aborting") {
		return apperr.Wrap(apperr.GitNotFastForward, cause)
	}
	if strings.Contains(lower, "local changes to the following files would be overwritten") ||
		strings.Contains(lower, "cannot pull with rebase: you have unstaged changes") ||
		strings.Contains(lower, "please commit or stash them") ||
		strings.Contains(lower, "untracked working tree files would be overwritten") {
		return apperr.Wrap(apperr.GitDirtyWorktree, cause)
	}
	if strings.Contains(lower, "non-fast-forward") ||
		strings.Contains(lower, "[rejected]") ||
		strings.Contains(lower, "fetch first") ||
		strings.Contains(lower, "failed to push some refs") {
		return apperr.Wrap(apperr.GitPushRejected, cause)
	}
	if strings.Contains(lower, "conflict (content)") ||
		strings.Contains(lower, "conflict (add/add)") ||
		strings.Contains(lower, "conflict (modify/delete)") ||
		strings.Contains(lower, "could not apply") {
		return apperr.Wrap(apperr.GitRebaseConflict, cause)
	}

	return apperr.Wrap(apperr.Unknown, fmt.Errorf("%w: %s", cause, stderr))
}

// IsInstalled returns true if git CLI is found on the system PATH.
func IsInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsRepo checks if the specified root directory is a git repository.
func IsRepo(root string) bool {
	if root == "" {
		return false
	}
	gitDir := filepath.Join(root, ".git")
	fi, err := os.Stat(gitDir)
	if err == nil && (fi.IsDir() || fi.Mode().IsRegular()) {
		return true
	}
	runner := NewRunner(root)
	_, err = runner.runCmd("rev-parse", "--is-inside-work-tree")
	return err == nil
}

// Init initializes a new git repository in the root, writes a standard .gitignore, and creates the initial commit if identity exists.
func Init(root string) error {
	if err := os.MkdirAll(root, fsops.DefaultDirPerms); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	runner := NewRunner(root)
	if _, err := runner.runCmd("init", "-b", "main"); err != nil {
		// Fallback for older git versions that don't support -b main
		if _, err := runner.runCmd("init"); err != nil {
			return err
		}
	}

	gitignorePath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		content := ".DS_Store\nThumbs.db\ndesktop.ini\n*.agentsync.tmp\n"
		_ = os.WriteFile(gitignorePath, []byte(content), fsops.DefaultFilePerms)
	}

	if _, err := runner.runCmd("add", ".gitignore"); err != nil {
		return err
	}

	// Only create the initial commit if git identity is already configured; never overwrite user identity
	if err := runner.EnsureIdentity(); err == nil {
		_, _ = runner.runCmd("commit", "-m", "chore: initialize agent configs repository")
	}

	return nil
}

// Clone clones a git repository from url into targetPath (B.2).
func Clone(url, targetPath string) error {
	if !IsInstalled() {
		return apperr.New(apperr.GitNotInstalled)
	}

	ctx, cancel := context.WithTimeout(context.Background(), CloneTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", url, targetPath)
	cmd.SysProcAttr = getSysProcAttr()
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		combined := strings.TrimSpace(stderr.String())
		if combined == "" {
			combined = strings.TrimSpace(stdout.String())
		}
		mapped := mapGitError(err, combined)
		if appErr, ok := mapped.(*apperr.Error); ok && appErr.Code != apperr.Unknown {
			return appErr
		}
		return apperr.Wrap(apperr.CloneFailed, fmt.Errorf("%w: %s", err, combined))
	}
	return nil
}

// EnsureIdentity checks if git user.email (effective, including global) is configured.
// If missing or empty, returns GitIdentityMissing. Never overwrites existing identity.
func (g *GitRunner) EnsureIdentity() error {
	email, err := g.runCmd("config", "--get", "user.email")
	if err != nil || strings.TrimSpace(email) == "" {
		return apperr.New(apperr.GitIdentityMissing)
	}
	return nil
}

// GetIdentity returns the effective git user.name and user.email for this repository.
func (g *GitRunner) GetIdentity() (name, email string) {
	name, _ = g.runCmd("config", "--get", "user.name")
	email, _ = g.runCmd("config", "--get", "user.email")
	return strings.TrimSpace(name), strings.TrimSpace(email)
}

// SetLocalIdentity sets user.name and user.email locally in this repository.
func (g *GitRunner) SetLocalIdentity(name, email string) error {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if email == "" {
		return apperr.New(apperr.GitIdentityMissing)
	}
	if name == "" {
		name = email
	}
	if _, err := g.runCmd("config", "--local", "user.name", name); err != nil {
		return err
	}
	if _, err := g.runCmd("config", "--local", "user.email", email); err != nil {
		return err
	}
	return nil
}

// CurrentBranch returns the name of the active branch.
func (g *GitRunner) CurrentBranch() (string, error) {
	out, err := g.runCmd("branch", "--show-current")
	if err != nil {
		return "", err
	}
	if out == "" {
		// In detached HEAD or initial state
		return g.runCmd("rev-parse", "--abbrev-ref", "HEAD")
	}
	return out, nil
}

// HasUpstream returns true if the current branch has an upstream tracking branch.
func (g *GitRunner) HasUpstream() bool {
	_, err := g.runCmd("rev-parse", "--abbrev-ref", "@{u}")
	return err == nil
}

// RemoteURL returns the URL of the remote (defaults to origin).
func (g *GitRunner) RemoteURL() (string, error) {
	out, err := g.runCmd("remote", "get-url", "origin")
	if err != nil {
		return "", nil // No remote configured
	}
	return out, nil
}

// AddRemote adds or sets the origin remote URL.
func (g *GitRunner) AddRemote(url string) error {
	existing, _ := g.RemoteURL()
	if existing != "" {
		_, err := g.runCmd("remote", "set-url", "origin", url)
		return err
	}
	_, err := g.runCmd("remote", "add", "origin", url)
	return err
}

// PushSetUpstream pushes the current branch and sets upstream to origin.
func (g *GitRunner) PushSetUpstream() error {
	branch, err := g.CurrentBranch()
	if err != nil {
		return err
	}
	_, err = g.runCmd("push", "-u", "origin", branch)
	return err
}

// Fetch fetches updates from all remotes.
func (g *GitRunner) Fetch() error {
	_, err := g.runCmd("fetch", "--all", "--prune")
	return err
}

// AheadBehind returns how many commits HEAD is ahead of and behind its upstream.
func (g *GitRunner) AheadBehind() (ahead, behind int, err error) {
	if !g.HasUpstream() {
		return 0, 0, nil
	}

	out, err := g.runCmd("rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(out)
	if len(parts) >= 2 {
		ahead, _ = strconv.Atoi(parts[0])
		behind, _ = strconv.Atoi(parts[1])
	}

	return ahead, behind, nil
}

// StatusPorcelain lists changed paths in the worktree using raw porcelain-v1 -z output (R4).
func (g *GitRunner) StatusPorcelain() ([]ChangedPath, error) {
	out, err := g.runCmdRaw("status", "--porcelain=v1", "-z", "-uall")
	if err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return []ChangedPath{}, nil
	}

	entries := strings.Split(out, "\x00")
	var results []ChangedPath

	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 3 {
			continue
		}

		statusCode := entry[:2]
		pathPart := entry[3:]

		// In -z mode, for renames (R) and copies (C), entry[3:] is the destination (new path)
		// and the following entry is the source (old path), which we consume and skip.
		if (statusCode[0] == 'R' || statusCode[0] == 'C' || strings.HasPrefix(statusCode, "R")) && i+1 < len(entries) {
			i++
		}

		results = append(results, ChangedPath{
			Status: statusCode,
			Path:   filepath.ToSlash(pathPart),
		})
	}

	return results, nil
}

// IncomingChangedFiles returns the list of files modified between HEAD and upstream.
func (g *GitRunner) IncomingChangedFiles() ([]string, error) {
	if !g.HasUpstream() {
		return []string{}, nil
	}

	out, err := g.runCmdRaw("diff", "--name-only", "-z", "HEAD...@{u}")
	if err != nil {
		return nil, err
	}

	if out == "" {
		return []string{}, nil
	}

	entries := strings.Split(out, "\x00")
	var results []string
	for _, entry := range entries {
		if entry != "" {
			results = append(results, filepath.ToSlash(entry))
		}
	}

	return results, nil
}

// PullFastForward performs a fast-forward only pull from upstream.
func (g *GitRunner) PullFastForward() error {
	_, err := g.runCmd("pull", "--ff-only")
	return err
}

// PullRebase performs a pull with rebase, aborting if conflicts occur.
func (g *GitRunner) PullRebase() error {
	_, err := g.runCmd("pull", "--rebase")
	if err != nil {
		// If rebase fails, abort cleanly
		_, _ = g.runCmd("rebase", "--abort")
		if appErr, ok := err.(*apperr.Error); ok && appErr.Code == apperr.GitDirtyWorktree {
			return err
		}
		return apperr.Wrap(apperr.GitRebaseConflict, err)
	}
	return nil
}

// CommitPaths stages the specified paths and creates a commit.
// If there is nothing to commit, it returns nil without error.
func (g *GitRunner) CommitPaths(paths []string, message string) error {
	if len(paths) == 0 {
		return nil
	}

	args := append([]string{"add", "--"}, paths...)
	if _, err := g.runCmd(args...); err != nil {
		return err
	}

	// Check if anything is actually staged
	if _, err := g.runCmd("diff", "--cached", "--quiet"); err == nil {
		// Exit code 0 means no staged changes
		return nil
	}

	out, err := g.runCmd("commit", "-m", message)
	if err != nil {
		lower := strings.ToLower(out + " " + err.Error())
		if strings.Contains(lower, "nothing to commit") || strings.Contains(lower, "nothing added to commit") {
			return nil
		}
		return err
	}
	return nil
}

// DiscardPath restores a tracked file to HEAD or removes an untracked file.
func (g *GitRunner) DiscardPath(relPath string) (string, error) {
	out, err := g.runCmd("checkout", "--", relPath)
	if err != nil {
		fullPath := filepath.Join(g.Root, filepath.FromSlash(relPath))
		if fi, statErr := os.Stat(fullPath); statErr == nil && !fi.IsDir() {
			if rmErr := os.Remove(fullPath); rmErr == nil {
				return "", nil
			}
		}
		return out, err
	}
	return out, nil
}

// Push pushes commits to upstream.
func (g *GitRunner) Push() error {
	if g.OnBeforePush != nil {
		g.OnBeforePush()
	}
	_, err := g.runCmd("push")
	return err
}
