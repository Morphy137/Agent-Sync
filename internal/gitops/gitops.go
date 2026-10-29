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
)

// ChangedPath represents a file changed in git status.
type ChangedPath struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

// GitRunner wraps execution of git commands in a repository root.
type GitRunner struct {
	Root string
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
		return outStr, mapGitError(err, errStr)
	}

	return outStr, nil
}

// mapGitError converts git CLI stderr messages into typed domain errors.
func mapGitError(cause error, stderr string) error {
	lower := strings.ToLower(stderr)

	if strings.Contains(lower, "not a git repository") {
		return apperr.Wrap(apperr.RootNotRepo, cause)
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
	if strings.Contains(lower, "conflict") || strings.Contains(lower, "could not apply") {
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

// Init initializes a new git repository in the root, writes a standard .gitignore, and creates the initial commit.
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

	// Ensure git identity exists in this repo so commits won't fail
	_, _ = runner.runCmd("config", "user.name", "AgentSync")
	_, _ = runner.runCmd("config", "user.email", "agentsync@local")

	gitignorePath := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		content := ".DS_Store\nThumbs.db\ndesktop.ini\n*.agentsync.tmp\n"
		_ = os.WriteFile(gitignorePath, []byte(content), fsops.DefaultFilePerms)
	}

	if _, err := runner.runCmd("add", ".gitignore"); err != nil {
		return err
	}

	_, err := runner.runCmd("commit", "-m", "chore: initialize agent configs repository")
	return err
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

// StatusPorcelain lists changed paths in the worktree.
func (g *GitRunner) StatusPorcelain() ([]ChangedPath, error) {
	out, err := g.runCmd("status", "--porcelain=v1", "-z")
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
		pathPart := strings.TrimSpace(entry[3:])

		// If rename (starts with R), the next entry in -z output is the new path
		if strings.HasPrefix(statusCode, "R") && i+1 < len(entries) {
			i++
			pathPart = entries[i]
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

	out, err := g.runCmd("diff", "--name-only", "HEAD...@{u}")
	if err != nil {
		return nil, err
	}

	if out == "" {
		return []string{}, nil
	}

	lines := strings.Split(out, "\n")
	var results []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			results = append(results, filepath.ToSlash(trimmed))
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
		return apperr.Wrap(apperr.GitRebaseConflict, err)
	}
	return nil
}

// CommitPaths stages the specified paths and creates a commit.
func (g *GitRunner) CommitPaths(paths []string, message string) error {
	if len(paths) == 0 {
		return nil
	}

	args := append([]string{"add", "--"}, paths...)
	if _, err := g.runCmd(args...); err != nil {
		return err
	}

	_, err := g.runCmd("commit", "-m", message)
	return err
}

// Push pushes commits to upstream.
func (g *GitRunner) Push() error {
	_, err := g.runCmd("push")
	return err
}
