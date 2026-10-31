package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"agentsync/internal/apperr"
	"agentsync/internal/fsops"
)

const (
	GitCommandTimeout = 60 * time.Second
	CloneTimeout      = 5 * time.Minute
	// NetworkTimeout bounds network-heavy operations (fetch, ls-remote, push) issued by ConnectRemote.
	NetworkTimeout = 5 * time.Minute

	// MaxRemoteURLLength is the longest remote URL accepted by ValidateRemoteURL.
	MaxRemoteURLLength = 2048

	// OriginRemote is the remote name used for the synchronized repository and subscriptions.
	OriginRemote = "origin"
	// DefaultBranch is the branch created when a new repository is initialized.
	DefaultBranch = "main"
	// LegacyDefaultBranch is the fallback branch name preferred after DefaultBranch on remotes.
	LegacyDefaultBranch = "master"
	// BackupBranchPrefix prefixes the branch that preserves local history replaced by ConnectRemote.
	BackupBranchPrefix = "agentsync/local-backup-"
	// BackupBranchTimeLayout formats the timestamp suffix of backup branches (yyyymmdd-hhmmss).
	BackupBranchTimeLayout = "20060102-150405"
	// InitialCommitMessagePrefix is the message of the commit created when pushing to an empty remote.
	InitialCommitMessagePrefix = "chore(sync): initial commit from "
	// UnknownHostname is used in commit messages when the hostname cannot be determined.
	UnknownHostname = "unknown-host"
	// GitignoreFileName is the name of the git ignore file.
	GitignoreFileName = ".gitignore"
	// pathspecBatchSize limits how many paths are passed to a single git invocation
	// to stay well below command line length limits (notably on Windows).
	pathspecBatchSize = 100
	// maxBackupBranchAttempts bounds the search for a free backup branch name.
	maxBackupBranchAttempts = 100
)

// hardeningArgs are global git options prepended to every invocation.
// They disable the "ext::" transport, which allows arbitrary command execution from a URL.
var hardeningArgs = []string{"-c", "protocol.ext.allow=never"}

// hardeningEnv are environment variables added to every git invocation.
var hardeningEnv = []string{
	"GIT_TERMINAL_PROMPT=0",
	"LC_ALL=C",
}

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

// newGitCommand builds a hardened git command (protocol restrictions, no prompts).
// dir may be empty to run in the current process directory.
func newGitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	fullArgs := make([]string, 0, len(hardeningArgs)+len(args))
	fullArgs = append(fullArgs, hardeningArgs...)
	fullArgs = append(fullArgs, args...)

	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	cmd.Dir = dir
	cmd.SysProcAttr = getSysProcAttr()
	cmd.Env = append(os.Environ(), hardeningEnv...)
	return cmd
}

// execGit runs a hardened git command and returns raw stdout and trimmed stderr.
func execGit(timeout time.Duration, dir string, args ...string) (stdout string, stderr string, err error) {
	if !IsInstalled() {
		return "", "", apperr.New(apperr.GitNotInstalled)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := newGitCommand(ctx, dir, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()
	return outBuf.String(), strings.TrimSpace(errBuf.String()), runErr
}

// runCmdTimeout executes a git command in the repository root with the given timeout,
// returning trimmed stdout.
func (g *GitRunner) runCmdTimeout(timeout time.Duration, args ...string) (string, error) {
	out, errStr, err := execGit(timeout, g.Root, args...)
	outStr := strings.TrimSpace(out)
	if err == nil {
		return outStr, nil
	}
	if appErr, ok := err.(*apperr.Error); ok {
		return "", appErr
	}
	combined := errStr
	if combined == "" {
		combined = outStr
	}
	return outStr, mapGitError(err, combined)
}

// runCmd executes a git command in the repository root directory with standard environment and flags.
func (g *GitRunner) runCmd(args ...string) (string, error) {
	return g.runCmdTimeout(GitCommandTimeout, args...)
}

// runCmdRaw executes a git command returning raw unmodified stdout without trimming whitespace (R4).
func (g *GitRunner) runCmdRaw(args ...string) (string, error) {
	out, errStr, err := execGit(GitCommandTimeout, g.Root, args...)
	if err == nil {
		return out, nil
	}
	if appErr, ok := err.(*apperr.Error); ok {
		return "", appErr
	}
	combined := errStr
	if combined == "" {
		combined = strings.TrimSpace(out)
	}
	return out, mapGitError(err, combined)
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

// hasOwnGitDir reports whether root itself holds a git directory (not merely nested in a parent repo).
func hasOwnGitDir(root string) bool {
	fi, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil && (fi.IsDir() || fi.Mode().IsRegular())
}

// initBare runs "git init" on root, preferring DefaultBranch as the initial branch.
func initBare(runner *GitRunner) error {
	if _, err := runner.runCmd("init", "-b", DefaultBranch); err != nil {
		// Fallback for older git versions that don't support -b
		if _, err := runner.runCmd("init"); err != nil {
			return err
		}
	}
	return nil
}

// DefaultGitignoreContent is written to new repositories when no .gitignore exists.
// Secrets are excluded by default because agent folders often sit next to tokens and keys.
const DefaultGitignoreContent = `# Operating system files
.DS_Store
Thumbs.db
desktop.ini
$RECYCLE.BIN/

# Secrets and credentials (never commit these)
.env
.env.*
!.env.example
*.pem
*.key
*.p12
*.pfx
id_rsa*
id_ed25519*
auth.json
credentials*.json
*.secret

# Logs, caches and dependencies
*.log
logs/
node_modules/
__pycache__/
.cache/
tmp/

# Editor swap files
*.swp
*~

# AgentSync temporary files
*.agentsync.tmp
`

// GitignorePatterns returns the non-comment, non-blank lines of a .gitignore body.
func GitignorePatterns(content string) []string {
	patterns := make([]string, 0)
	for _, line := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, trimmed)
	}
	return patterns
}

// AppendGitignorePatterns appends the patterns missing from content, under an optional comment
// header, and returns the new content plus the patterns that were actually added.
func AppendGitignorePatterns(content, header string, patterns []string) (string, []string) {
	existing := make(map[string]bool)
	for _, p := range GitignorePatterns(content) {
		existing[p] = true
	}
	added := make([]string, 0)
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" || existing[p] {
			continue
		}
		existing[p] = true
		added = append(added, p)
	}
	if len(added) == 0 {
		return content, added
	}
	var b strings.Builder
	b.WriteString(content)
	if content != "" && !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	if content != "" {
		b.WriteString("\n")
	}
	if header != "" {
		b.WriteString("# " + header + "\n")
	}
	for _, p := range added {
		b.WriteString(p + "\n")
	}
	return b.String(), added
}

// writeDefaultGitignore writes the default .gitignore into root when none exists.
func writeDefaultGitignore(root string) {
	gitignorePath := filepath.Join(root, GitignoreFileName)
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		_ = os.WriteFile(gitignorePath, []byte(DefaultGitignoreContent), fsops.DefaultFilePerms)
	}
}

// Init initializes a new git repository in the root, writes a standard .gitignore, and creates the initial commit if identity exists.
func Init(root string) error {
	if err := os.MkdirAll(root, fsops.DefaultDirPerms); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	runner := NewRunner(root)
	if err := initBare(runner); err != nil {
		return err
	}

	writeDefaultGitignore(root)

	if _, err := runner.runCmd("add", GitignoreFileName); err != nil {
		return err
	}

	// Only create the initial commit if git identity is already configured; never overwrite user identity
	if err := runner.EnsureIdentity(); err == nil {
		_, _ = runner.runCmd("commit", "-m", "chore: initialize agent configs repository")
	}

	return nil
}

// Clone clones a git repository from url into targetPath (B.2).
// The URL is validated first and passed after "--" so it can never be parsed as an option.
func Clone(url, targetPath string) error {
	if err := ValidateRemoteURL(url); err != nil {
		return err
	}

	out, errStr, err := execGit(CloneTimeout, "", "clone", "--", url, targetPath)
	if err == nil {
		return nil
	}
	if appErr, ok := err.(*apperr.Error); ok {
		return appErr
	}

	combined := errStr
	if combined == "" {
		combined = strings.TrimSpace(out)
	}
	mapped := mapGitError(err, combined)
	if appErr, ok := mapped.(*apperr.Error); ok && appErr.Code != apperr.Unknown {
		return appErr
	}
	return apperr.Wrap(apperr.CloneFailed, fmt.Errorf("%w: %s", err, combined))
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
// The URL is validated and passed after "--" so it can never be parsed as an option.
func (g *GitRunner) AddRemote(url string) error {
	if err := ValidateRemoteURL(url); err != nil {
		return err
	}
	existing, _ := g.RemoteURL()
	if existing != "" {
		_, err := g.runCmd("remote", "set-url", "--", OriginRemote, url)
		return err
	}
	_, err := g.runCmd("remote", "add", "--", OriginRemote, url)
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

// ---------------------------------------------------------------------------
// Remote URL validation
// ---------------------------------------------------------------------------

// Supported URL schemes for remotes.
const (
	schemeHTTPS = "https"
	schemeHTTP  = "http"
	schemeSSH   = "ssh"
	schemeFile  = "file"
	// fileLocalHost is the only host accepted in file:// URLs besides an empty host.
	fileLocalHost = "localhost"
)

// Reasons attached to InvalidRemoteURL errors (param "reason").
const (
	InvalidURLReasonEmpty       = "empty"
	InvalidURLReasonTooLong     = "too_long"
	InvalidURLReasonOption      = "leading_dash"
	InvalidURLReasonCharacters  = "invalid_characters"
	InvalidURLReasonTransport   = "transport_helper"
	InvalidURLReasonScheme      = "unsupported_scheme"
	InvalidURLReasonMissingHost = "missing_host"
	InvalidURLReasonMissingPath = "missing_path"
)

// transportHelperPattern matches git's "<transport>::<address>" remote-helper syntax (e.g. "ext::").
var transportHelperPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+.-]*::`)

// scpLikePattern matches scp-like SSH remotes such as "git@github.com:org/repo.git".
var scpLikePattern = regexp.MustCompile(`^[A-Za-z0-9._~-]+@[A-Za-z0-9][A-Za-z0-9.-]*:[^:].*$`)

func invalidRemoteURL(rawURL, reason string) error {
	return apperr.New(apperr.InvalidRemoteURL).WithParam("url", rawURL).WithParam("reason", reason)
}

// ValidateRemoteURL accepts https://, http://, ssh://, git@host:path and file:// URLs.
// Absolute local filesystem paths are also accepted (git treats them like file:// remotes).
// It rejects "ext::" and any other "<transport>::" helper syntax, a leading "-" (option injection),
// whitespace and control characters. Errors are apperr InvalidRemoteURL.
func ValidateRemoteURL(rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return invalidRemoteURL(rawURL, InvalidURLReasonEmpty)
	}
	if len(rawURL) > MaxRemoteURLLength {
		return invalidRemoteURL(rawURL, InvalidURLReasonTooLong)
	}
	if strings.HasPrefix(rawURL, "-") {
		return invalidRemoteURL(rawURL, InvalidURLReasonOption)
	}
	if transportHelperPattern.MatchString(rawURL) || strings.Contains(strings.ToLower(rawURL), "ext::") {
		return invalidRemoteURL(rawURL, InvalidURLReasonTransport)
	}

	if isLocalAbsolutePath(rawURL) {
		if containsControlChars(rawURL) {
			return invalidRemoteURL(rawURL, InvalidURLReasonCharacters)
		}
		return nil
	}

	if containsSpaceOrControl(rawURL) {
		return invalidRemoteURL(rawURL, InvalidURLReasonCharacters)
	}

	if scpLikePattern.MatchString(rawURL) && !strings.Contains(rawURL, "://") {
		return nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return invalidRemoteURL(rawURL, InvalidURLReasonScheme)
	}

	switch strings.ToLower(parsed.Scheme) {
	case schemeHTTPS, schemeHTTP, schemeSSH:
		host := parsed.Hostname()
		if host == "" {
			return invalidRemoteURL(rawURL, InvalidURLReasonMissingHost)
		}
		if strings.HasPrefix(host, "-") || strings.HasPrefix(parsed.User.Username(), "-") {
			return invalidRemoteURL(rawURL, InvalidURLReasonOption)
		}
		return nil
	case schemeFile:
		if parsed.Host != "" && !strings.EqualFold(parsed.Host, fileLocalHost) {
			return invalidRemoteURL(rawURL, InvalidURLReasonMissingHost)
		}
		if parsed.Path == "" || parsed.Path == "/" {
			return invalidRemoteURL(rawURL, InvalidURLReasonMissingPath)
		}
		return nil
	default:
		return invalidRemoteURL(rawURL, InvalidURLReasonScheme)
	}
}

// isLocalAbsolutePath reports whether s is an absolute filesystem path (and not a URL).
func isLocalAbsolutePath(s string) bool {
	if strings.Contains(s, "://") {
		return false
	}
	return filepath.IsAbs(s)
}

func containsControlChars(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func containsSpaceOrControl(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Revision helpers
// ---------------------------------------------------------------------------

// validateRev rejects empty revisions and revisions that could be parsed as options.
func validateRev(rev string) error {
	if strings.TrimSpace(rev) == "" || strings.HasPrefix(rev, "-") || containsSpaceOrControl(rev) {
		return apperr.Wrap(apperr.Unknown, fmt.Errorf("invalid git revision %q", rev))
	}
	return nil
}

// revExists reports whether rev resolves to an object.
func (g *GitRunner) revExists(rev string) bool {
	_, err := g.runCmd("rev-parse", "--verify", "--quiet", rev)
	return err == nil
}

// hasCommits reports whether HEAD points to an existing commit (false for an unborn branch).
func (g *GitRunner) hasCommits() bool {
	return g.revExists("HEAD^{commit}")
}

// splitNul splits NUL-separated git output, dropping empty entries and normalizing to forward slashes.
func splitNul(out string) []string {
	results := []string{}
	for _, entry := range strings.Split(out, "\x00") {
		if entry != "" {
			results = append(results, filepath.ToSlash(entry))
		}
	}
	return results
}

// ShowFile returns the content of relPath (forward slashes, relative to the repository root)
// at revision rev, as with "git show rev:path". When the revision or the path does not exist
// (or the path is not a regular blob) it returns exists=false and a nil error.
func (g *GitRunner) ShowFile(rev, relPath string) ([]byte, bool, error) {
	if err := validateRev(rev); err != nil {
		return nil, false, err
	}
	cleanPath := strings.TrimPrefix(filepath.ToSlash(relPath), "./")
	if cleanPath == "" {
		return nil, false, nil
	}

	object := rev + ":" + cleanPath
	objectType, err := g.runCmd("cat-file", "-t", object)
	if err != nil {
		if appErr, ok := err.(*apperr.Error); ok && appErr.Code == apperr.GitNotInstalled {
			return nil, false, err
		}
		return nil, false, nil
	}
	if objectType != "blob" {
		return nil, false, nil
	}

	content, err := g.runCmdRaw("cat-file", "blob", object)
	if err != nil {
		return nil, false, err
	}
	return []byte(content), true, nil
}

// ResetHardToUpstream discards local commits and working tree changes, moving the current
// branch to its upstream tracking branch.
func (g *GitRunner) ResetHardToUpstream() error {
	if !g.HasUpstream() {
		return apperr.New(apperr.GitNoRemote)
	}
	_, err := g.runCmd("reset", "--hard", "@{u}")
	return err
}

// HeadCommit returns the full hash of HEAD.
func (g *GitRunner) HeadCommit() (string, error) {
	return g.runCmd("rev-parse", "--verify", "HEAD^{commit}")
}

// UpstreamCommit returns the full hash of the upstream tracking branch.
func (g *GitRunner) UpstreamCommit() (string, error) {
	if !g.HasUpstream() {
		return "", apperr.New(apperr.GitNoRemote)
	}
	return g.runCmd("rev-parse", "--verify", "@{u}^{commit}")
}

// ChangedFilesBetween lists files that differ between two revisions (forward slashes).
// Renames are reported as both the old and the new path.
func (g *GitRunner) ChangedFilesBetween(fromRev, toRev string) ([]string, error) {
	if err := validateRev(fromRev); err != nil {
		return nil, err
	}
	if err := validateRev(toRev); err != nil {
		return nil, err
	}
	out, err := g.runCmdRaw("diff", "--name-only", "--no-renames", "-z", fromRev, toRev, "--")
	if err != nil {
		return nil, err
	}
	return splitNul(out), nil
}

// RemoteDefaultBranch returns the default branch of the origin remote. It prefers the branch
// referenced by the remote HEAD, then DefaultBranch, LegacyDefaultBranch and finally the first
// branch in name order. It returns ("", false, nil) when the remote has no branches.
func (g *GitRunner) RemoteDefaultBranch() (string, bool, error) {
	headsOut, err := g.runCmdTimeout(NetworkTimeout, "ls-remote", "--heads", OriginRemote)
	if err != nil {
		return "", false, err
	}
	branches := map[string]bool{}
	var names []string
	for _, line := range strings.Split(headsOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[1], "refs/heads/") {
			name := strings.TrimPrefix(fields[1], "refs/heads/")
			branches[name] = true
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", false, nil
	}

	symOut, err := g.runCmdTimeout(NetworkTimeout, "ls-remote", "--symref", OriginRemote, "HEAD")
	if err != nil {
		return "", false, err
	}
	for _, line := range strings.Split(symOut, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "ref:" && strings.HasPrefix(fields[1], "refs/heads/") {
			if name := strings.TrimPrefix(fields[1], "refs/heads/"); branches[name] {
				return name, true, nil
			}
		}
	}

	for _, preferred := range []string{DefaultBranch, LegacyDefaultBranch} {
		if branches[preferred] {
			return preferred, true, nil
		}
	}
	sort.Strings(names)
	return names[0], true, nil
}

// CreateBranchAt creates a new local branch named name pointing at rev (no tracking, no checkout).
func (g *GitRunner) CreateBranchAt(name, rev string) error {
	if err := validateRev(rev); err != nil {
		return err
	}
	if strings.HasPrefix(name, "-") {
		return apperr.Wrap(apperr.Unknown, fmt.Errorf("invalid branch name %q", name))
	}
	if _, err := g.runCmd("check-ref-format", "--branch", name); err != nil {
		return apperr.Wrap(apperr.Unknown, fmt.Errorf("invalid branch name %q: %w", name, err))
	}
	_, err := g.runCmd("branch", "--no-track", name, rev)
	return err
}

// ---------------------------------------------------------------------------
// ConnectRemote
// ---------------------------------------------------------------------------

// ConnectRemoteResult describes the outcome of ConnectRemote.
type ConnectRemoteResult struct {
	Branch           string        `json:"branch"`
	RemoteHadContent bool          `json:"remoteHadContent"`
	BackupBranch     string        `json:"backupBranch,omitempty"` // local history preserved here when replaced
	LocalChanges     []ChangedPath `json:"localChanges"`           // uncommitted differences vs remote for the user to review
}

// ConnectRemote attaches root (initializing git if needed) to url without losing local files:
//  1. init if not a repo; add/set origin; fetch.
//  2. remote empty  -> commit local files (if identity present; otherwise leave staged and
//     return GitIdentityMissing so the caller can ask for an identity and retry) and push -u.
//  3. remote has branch B:
//     - if local has commits not contained in origin/B: create backup branch
//     "agentsync/local-backup-<yyyymmdd-hhmmss>" at HEAD.
//     - point HEAD to B, `reset --mixed origin/B` (working tree untouched), set upstream.
//     - restore files that exist in origin/B but are missing locally (`checkout -- <path>`).
//     - report remaining uncommitted changes as LocalChanges (user reviews with diffs).
func ConnectRemote(root, url string) (*ConnectRemoteResult, error) {
	if err := ValidateRemoteURL(url); err != nil {
		return nil, err
	}
	if root == "" || !filepath.IsAbs(root) {
		return nil, apperr.New(apperr.PathNotAbsolute).WithParam("path", root)
	}
	fi, err := os.Stat(root)
	if err != nil || !fi.IsDir() {
		return nil, apperr.New(apperr.RootNotFound).WithParam("path", root)
	}
	if !IsInstalled() {
		return nil, apperr.New(apperr.GitNotInstalled)
	}

	runner := NewRunner(root)
	if !hasOwnGitDir(root) {
		if err := initBare(runner); err != nil {
			return nil, err
		}
	}

	if err := runner.AddRemote(url); err != nil {
		return nil, err
	}
	if _, err := runner.runCmdTimeout(NetworkTimeout, "fetch", "--prune", OriginRemote); err != nil {
		return nil, err
	}

	branch, hasBranch, err := runner.RemoteDefaultBranch()
	if err != nil {
		return nil, err
	}
	if !hasBranch {
		return runner.connectToEmptyRemote()
	}
	return runner.connectToRemoteBranch(branch)
}

// connectToEmptyRemote commits all local files and publishes the current branch to origin.
func (g *GitRunner) connectToEmptyRemote() (*ConnectRemoteResult, error) {
	branch, err := g.ensureNamedBranch()
	if err != nil {
		return nil, err
	}

	writeDefaultGitignore(g.Root)
	if _, err := g.runCmd("add", "-A"); err != nil {
		return nil, err
	}

	// Without an identity the files stay staged; origin is already configured so the
	// caller can set an identity and call ConnectRemote again.
	if err := g.EnsureIdentity(); err != nil {
		return nil, err
	}

	if err := g.commitStagedForInitialPush(); err != nil {
		return nil, err
	}

	if g.OnBeforePush != nil {
		g.OnBeforePush()
	}
	refspec := "HEAD:refs/heads/" + branch
	if _, err := g.runCmdTimeout(NetworkTimeout, "push", "-u", OriginRemote, refspec); err != nil {
		return nil, err
	}

	changes, err := g.StatusPorcelain()
	if err != nil {
		return nil, err
	}
	return &ConnectRemoteResult{
		Branch:           branch,
		RemoteHadContent: false,
		LocalChanges:     nonNilChanges(changes),
	}, nil
}

// commitStagedForInitialPush commits staged changes, or creates an empty commit when the
// branch is unborn and nothing is staged (a branch needs at least one commit to be pushed).
func (g *GitRunner) commitStagedForInitialPush() error {
	message := InitialCommitMessagePrefix + localHostname()
	_, diffErr := g.runCmd("diff", "--cached", "--quiet")
	hasStaged := diffErr != nil

	if hasStaged {
		_, err := g.runCmd("commit", "-m", message)
		return err
	}
	if g.hasCommits() {
		return nil
	}
	_, err := g.runCmd("commit", "--allow-empty", "-m", message)
	return err
}

// ensureNamedBranch returns the current branch name, creating DefaultBranch when HEAD is detached.
func (g *GitRunner) ensureNamedBranch() (string, error) {
	branch, err := g.runCmd("branch", "--show-current")
	if err != nil {
		return "", err
	}
	if branch != "" {
		return branch, nil
	}
	if _, err := g.runCmd("switch", "-c", DefaultBranch); err != nil {
		return "", err
	}
	return DefaultBranch, nil
}

// connectToRemoteBranch adopts origin/<branch> as the history of the local repository while
// keeping every local file in the working tree.
func (g *GitRunner) connectToRemoteBranch(branch string) (*ConnectRemoteResult, error) {
	remoteRef := "refs/remotes/" + OriginRemote + "/" + branch
	if !g.revExists(remoteRef + "^{commit}") {
		return nil, apperr.New(apperr.RemoteBranchNotFound).WithParam("branch", branch)
	}

	backupBranch, err := g.backupDivergentHistory(branch, remoteRef)
	if err != nil {
		return nil, err
	}

	if _, err := g.runCmd("symbolic-ref", "HEAD", "refs/heads/"+branch); err != nil {
		return nil, err
	}
	if _, err := g.runCmd("reset", "--mixed", "--quiet", remoteRef); err != nil {
		return nil, err
	}
	if _, err := g.runCmd("branch", "--set-upstream-to="+OriginRemote+"/"+branch, branch); err != nil {
		return nil, err
	}

	if err := g.restoreMissingFiles(remoteRef); err != nil {
		return nil, err
	}

	changes, err := g.StatusPorcelain()
	if err != nil {
		return nil, err
	}
	return &ConnectRemoteResult{
		Branch:           branch,
		RemoteHadContent: true,
		BackupBranch:     backupBranch,
		LocalChanges:     nonNilChanges(changes),
	}, nil
}

// isAncestor reports whether ancestor is reachable from descendant.
func (g *GitRunner) isAncestor(ancestor, descendant string) bool {
	_, err := g.runCmd("merge-base", "--is-ancestor", ancestor, descendant)
	return err == nil
}

// backupDivergentHistory preserves local history that adopting remoteRef would make unreachable:
// HEAD (when it has commits not contained in remoteRef) and the local branch with the remote
// branch name (which the mixed reset moves) when it holds other commits. It returns the backup
// branch name reported to the user ("" when nothing needed preserving).
func (g *GitRunner) backupDivergentHistory(branch, remoteRef string) (string, error) {
	reported := ""
	if g.hasCommits() && !g.isAncestor("HEAD", remoteRef) {
		name, err := g.createBackupBranch("HEAD")
		if err != nil {
			return "", err
		}
		reported = name
	}

	localRef := "refs/heads/" + branch
	if !g.revExists(localRef + "^{commit}") {
		return reported, nil
	}
	if g.isAncestor(localRef, remoteRef) || (g.hasCommits() && g.isAncestor(localRef, "HEAD")) {
		return reported, nil
	}
	name, err := g.createBackupBranch(localRef)
	if err != nil {
		return "", err
	}
	if reported == "" {
		reported = name
	}
	return reported, nil
}

// createBackupBranch preserves rev in a uniquely named backup branch.
func (g *GitRunner) createBackupBranch(rev string) (string, error) {
	base := BackupBranchPrefix + time.Now().Format(BackupBranchTimeLayout)
	for attempt := 0; attempt < maxBackupBranchAttempts; attempt++ {
		name := base
		if attempt > 0 {
			name = base + "-" + strconv.Itoa(attempt+1)
		}
		if g.revExists("refs/heads/" + name) {
			continue
		}
		if err := g.CreateBranchAt(name, rev); err != nil {
			return "", err
		}
		return name, nil
	}
	return "", apperr.Wrap(apperr.Unknown, errors.New("could not find a free backup branch name"))
}

// restoreMissingFiles checks out (from the index, which equals remoteRef after a mixed reset)
// every file tracked in remoteRef that does not exist in the working tree. Files whose parent
// path is occupied by a local non-directory are skipped so no local file is ever replaced.
func (g *GitRunner) restoreMissingFiles(remoteRef string) error {
	out, err := g.runCmdRaw("ls-tree", "-r", "-z", "--name-only", remoteRef)
	if err != nil {
		return err
	}

	var missing []string
	for _, relPath := range splitNul(out) {
		if g.canRestore(relPath) {
			missing = append(missing, relPath)
		}
	}

	for start := 0; start < len(missing); start += pathspecBatchSize {
		end := start + pathspecBatchSize
		if end > len(missing) {
			end = len(missing)
		}
		args := append([]string{"--literal-pathspecs", "checkout", "--"}, missing[start:end]...)
		if _, err := g.runCmd(args...); err != nil {
			return err
		}
	}
	return nil
}

// canRestore reports whether relPath is absent locally and every existing ancestor is a directory.
func (g *GitRunner) canRestore(relPath string) bool {
	segments := strings.Split(relPath, "/")
	current := g.Root
	for i, segment := range segments {
		current = filepath.Join(current, segment)
		fi, err := os.Lstat(current)
		if err != nil {
			return os.IsNotExist(err)
		}
		isLast := i == len(segments)-1
		if isLast {
			return false // file already exists locally: keep the local version
		}
		if !fi.IsDir() {
			return false // a local file occupies a parent directory path
		}
	}
	return false
}

func nonNilChanges(changes []ChangedPath) []ChangedPath {
	if changes == nil {
		return []ChangedPath{}
	}
	return changes
}

// localHostname returns the machine hostname for commit messages.
func localHostname() string {
	name, err := os.Hostname()
	if err != nil || strings.TrimSpace(name) == "" {
		return UnknownHostname
	}
	return name
}
