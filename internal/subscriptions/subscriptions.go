// Package subscriptions manages read-only clones of remote repositories that can be used
// as sources for sync rules. Clones live under <configDir>/subscriptions/<id>/repo.
package subscriptions

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/gitops"
)

const (
	DirName  = "subscriptions" // under config.GetConfigDir()
	CloneDir = "repo"

	// IDPrefix prefixes every generated subscription identifier.
	IDPrefix = "s_"
	// idRandomBytes is the number of random bytes hex-encoded into an identifier (10 hex chars).
	idRandomBytes = 5
	// MaxTreeDepth caps Tree recursion; maxDepth <= 0 or above this value uses it.
	MaxTreeDepth = 32
	// gitSuffix is the conventional suffix of repository URLs, stripped from derived names.
	gitSuffix = ".git"
	// writableFilePerms is applied to read-only files (git objects) before retrying removal.
	writableFilePerms = 0o666
)

// validIDPattern restricts identifiers to a single safe path segment.
var validIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// UpdateInfo reports whether a subscription has new commits upstream.
type UpdateInfo struct {
	SubscriptionID string   `json:"subscriptionId"`
	Behind         int      `json:"behind"`
	IncomingFiles  []string `json:"incomingFiles"`
	HeadCommit     string   `json:"headCommit"`
	RemoteCommit   string   `json:"remoteCommit"`
	Offline        bool     `json:"offline"`
}

// TreeEntry is one file or directory inside a subscription clone.
type TreeEntry struct {
	Path  string `json:"path"` // forward slashes, relative to clone root
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// Root returns the clone directory of a subscription: <configDir>/subscriptions/<id>/repo.
func Root(id string) string {
	return filepath.Join(subscriptionDir(id), CloneDir)
}

// subscriptionDir returns <configDir>/subscriptions/<id>.
func subscriptionDir(id string) string {
	return filepath.Join(config.GetConfigDir(), DirName, id)
}

// validateID guards every filesystem operation against identifiers that could escape DirName.
func validateID(id string) error {
	if !validIDPattern.MatchString(id) {
		return apperr.New(apperr.SubscriptionNotFound).WithParam("id", id)
	}
	return nil
}

// existingRunner returns a runner for an existing, valid subscription clone.
func existingRunner(id string) (*gitops.GitRunner, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	root := Root(id)
	fi, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil || !fi.IsDir() {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("id", id)
	}
	return gitops.NewRunner(root), nil
}

// newID generates a random identifier such as "s_0a1b2c3d4e".
func newID() (string, error) {
	buf := make([]byte, idRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", apperr.Wrap(apperr.Unknown, err)
	}
	return IDPrefix + hex.EncodeToString(buf), nil
}

// DeriveName returns the repository name of a remote URL ("https://host/org/repo.git" -> "repo").
func DeriveName(remoteURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(remoteURL), "/\\")
	trimmed = strings.TrimSuffix(trimmed, gitSuffix)
	trimmed = strings.TrimRight(trimmed, "/\\")
	cut := strings.LastIndexAny(trimmed, "/\\:")
	if cut >= 0 {
		trimmed = trimmed[cut+1:]
	}
	return strings.TrimSpace(trimmed)
}

// Add validates the URL, clones it into a fresh directory (removing any partial clone on
// failure) and detects the checked out branch.
func Add(remoteURL, name string) (config.Subscription, error) {
	if err := gitops.ValidateRemoteURL(remoteURL); err != nil {
		return config.Subscription{}, err
	}

	id, err := newID()
	if err != nil {
		return config.Subscription{}, err
	}
	dir := subscriptionDir(id)
	if _, err := os.Stat(dir); err == nil {
		return config.Subscription{}, apperr.New(apperr.FolderExists).WithParam("path", dir)
	}
	if err := os.MkdirAll(dir, fsops.DefaultDirPerms); err != nil {
		return config.Subscription{}, apperr.Wrap(apperr.WriteFailed, err)
	}

	sub, err := cloneSubscription(id, remoteURL, name)
	if err != nil {
		_ = removeAll(dir)
		return config.Subscription{}, err
	}
	return sub, nil
}

// cloneSubscription clones the remote into Root(id) and builds the subscription record.
func cloneSubscription(id, remoteURL, name string) (config.Subscription, error) {
	root := Root(id)
	if err := gitops.Clone(remoteURL, root); err != nil {
		return config.Subscription{}, err
	}

	runner := gitops.NewRunner(root)
	if _, err := runner.HeadCommit(); err != nil {
		// An empty remote has nothing to subscribe to.
		return config.Subscription{}, apperr.New(apperr.RemoteBranchNotFound).WithParam("url", remoteURL)
	}
	branch, err := runner.CurrentBranch()
	if err != nil {
		return config.Subscription{}, err
	}

	displayName := strings.TrimSpace(name)
	if displayName == "" {
		displayName = DeriveName(remoteURL)
	}
	if displayName == "" {
		displayName = id
	}

	return config.Subscription{
		ID:      id,
		Name:    displayName,
		URL:     remoteURL,
		Branch:  branch,
		AddedAt: config.FormatRFC3339(time.Now()),
	}, nil
}

// Remove deletes the subscription clone directory. Removing a missing subscription is a no-op.
func Remove(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	dir := subscriptionDir(id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	if err := removeAll(dir); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}
	return nil
}

// removeAll deletes dir, clearing read-only attributes (git object files on Windows) when needed.
func removeAll(dir string) error {
	if err := os.RemoveAll(dir); err == nil {
		return nil
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Type()&fs.ModeSymlink == 0 {
			_ = os.Chmod(p, writableFilePerms)
		}
		return nil
	})
	return os.RemoveAll(dir)
}

// Check fetches the remote and compares HEAD with upstream without touching the working tree.
// Network failures are reported as Offline=true (with the last known state) instead of an error.
func Check(sub config.Subscription) (UpdateInfo, error) {
	runner, err := existingRunner(sub.ID)
	if err != nil {
		return UpdateInfo{}, err
	}

	info := UpdateInfo{SubscriptionID: sub.ID, IncomingFiles: []string{}}
	if err := runner.Fetch(); err != nil {
		if !isNetworkError(err) {
			return UpdateInfo{}, err
		}
		info.Offline = true
	}

	head, err := runner.HeadCommit()
	if err != nil {
		return UpdateInfo{}, err
	}
	info.HeadCommit = head

	remote, err := runner.UpstreamCommit()
	if err != nil {
		return UpdateInfo{}, err
	}
	info.RemoteCommit = remote

	if head == remote {
		return info, nil
	}

	_, behind, err := runner.AheadBehind()
	if err != nil {
		return UpdateInfo{}, err
	}
	info.Behind = behind

	files, err := runner.ChangedFilesBetween(head, remote)
	if err != nil {
		return UpdateInfo{}, err
	}
	info.IncomingFiles = files
	return info, nil
}

// isNetworkError reports whether err means the remote could not be reached.
func isNetworkError(err error) bool {
	var appErr *apperr.Error
	return errors.As(err, &appErr) && appErr.Code == apperr.GitNetwork
}

// Update fetches and hard-resets the clone to its upstream, returning the files that changed.
func Update(sub config.Subscription) ([]string, error) {
	runner, err := existingRunner(sub.ID)
	if err != nil {
		return nil, err
	}
	if err := runner.Fetch(); err != nil {
		return nil, err
	}

	head, err := runner.HeadCommit()
	if err != nil {
		return nil, err
	}
	remote, err := runner.UpstreamCommit()
	if err != nil {
		return nil, err
	}

	changed := []string{}
	if head != remote {
		changed, err = runner.ChangedFilesBetween(head, remote)
		if err != nil {
			return nil, err
		}
	}

	if err := runner.ResetHardToUpstream(); err != nil {
		return nil, err
	}
	return changed, nil
}

// Tree lists entries of the clone (skipping .git, fsops.ShouldIgnore names and symlinks)
// up to maxDepth levels, sorted by path. maxDepth <= 0 means MaxTreeDepth.
func Tree(id string, maxDepth int) ([]TreeEntry, error) {
	if _, err := existingRunner(id); err != nil {
		return nil, err
	}
	if maxDepth <= 0 || maxDepth > MaxTreeDepth {
		maxDepth = MaxTreeDepth
	}

	root := Root(id)
	entries := []TreeEntry{}
	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == root {
			return nil
		}
		if fsops.ShouldIgnore(d.Name()) || d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		depth := strings.Count(relSlash, "/") + 1

		entry := TreeEntry{Path: relSlash, IsDir: d.IsDir()}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			entry.Size = info.Size()
		}
		entries = append(entries, entry)

		if d.IsDir() && depth >= maxDepth {
			return filepath.SkipDir
		}
		return nil
	})
	if walkErr != nil {
		return nil, apperr.Wrap(apperr.SourceNotFound, walkErr).WithParam("id", id)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries, nil
}

// cleanRelPath normalizes relPath to a forward-slash path inside the clone, rejecting
// empty, absolute and escaping ("..") paths with apperr InvalidRule.
func cleanRelPath(relPath string) (string, error) {
	invalid := apperr.New(apperr.InvalidRule).WithParam("path", relPath)
	slashed := strings.ReplaceAll(strings.TrimSpace(relPath), "\\", "/")
	if slashed == "" || strings.HasPrefix(slashed, "/") || filepath.IsAbs(relPath) || filepath.VolumeName(relPath) != "" {
		return "", invalid
	}
	if strings.Contains(slashed, ":") {
		return "", invalid // drive letters and git "rev:path" syntax
	}
	for _, segment := range strings.Split(slashed, "/") {
		if segment == ".." {
			return "", invalid
		}
	}
	cleaned := path.Clean(slashed)
	if cleaned == "." || cleaned == ".git" || strings.HasPrefix(cleaned, ".git/") {
		return "", invalid
	}
	return cleaned, nil
}

// IncomingDiff returns the content of relPath at HEAD (before) and at upstream (after).
// A side where the file does not exist is nil; when neither side has it, SourceNotFound is returned.
func IncomingDiff(id, relPath string) (before, after []byte, err error) {
	runner, err := existingRunner(id)
	if err != nil {
		return nil, nil, err
	}
	cleaned, err := cleanRelPath(relPath)
	if err != nil {
		return nil, nil, err
	}

	before, beforeExists, err := runner.ShowFile("HEAD", cleaned)
	if err != nil {
		return nil, nil, err
	}
	after, afterExists, err := runner.ShowFile("@{u}", cleaned)
	if err != nil {
		return nil, nil, err
	}
	if !beforeExists && !afterExists {
		return nil, nil, apperr.New(apperr.SourceNotFound).WithParam("path", relPath)
	}
	if !beforeExists {
		before = nil
	}
	if !afterExists {
		after = nil
	}
	return before, after, nil
}
