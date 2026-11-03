package service

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/rules"
)

const browserRepoRoot = "repo"
const browserRulePrefix = "rule:"

type FileRoot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FileDirectoryEntry struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
}

// ListFileRoots returns only the configured repository and subscription clones.
func (s *Service) ListFileRoots() ([]FileRoot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	roots := []FileRoot{}
	if s.cfg.RootPath != "" {
		roots = append(roots, FileRoot{ID: browserRepoRoot, Name: filepath.Base(filepath.Clean(s.cfg.RootPath))})
	}
	for _, sub := range s.cfg.Subscriptions {
		roots = append(roots, FileRoot{ID: sub.ID, Name: sub.Name})
	}
	return roots, nil
}

// browserRootLocked resolves only configured roots or a local source of an existing rule.
func (s *Service) browserRootLocked(id string) (string, bool, error) {
	if id == browserRepoRoot {
		if s.cfg.RootPath == "" {
			return "", false, apperr.New(apperr.RootNotFound)
		}
		return s.cfg.RootPath, false, nil
	}
	if strings.HasPrefix(id, browserRulePrefix) {
		all, err := s.allRulesLocked()
		if err != nil {
			return "", false, err
		}
		for _, rule := range all {
			if rule.ID == strings.TrimPrefix(id, browserRulePrefix) && rule.Source.Origin == rules.OriginLocal {
				root, err := rules.SourcePath(s.rulesEnvLocked(), rule)
				return root, false, err
			}
		}
		return "", false, apperr.New(apperr.RuleNotFound)
	}
	if !s.hasSubscriptionLocked(id) {
		return "", false, apperr.New(apperr.SubscriptionNotFound).WithParam("id", id)
	}
	return subscriptionsRoot(id), true, nil
}

// browserPathLocked rejects traversal, Git metadata and symlinks escaping the configured root.
func (s *Service) browserPathLocked(id, rel string) (string, bool, error) {
	root, subscription, err := s.browserRootLocked(id)
	if err != nil {
		return "", false, err
	}
	resolved, err := resolveBrowserPath(root, rel)
	return resolved, subscription, err
}

// isBrowserRoot reports whether rel designates the root of a browsed folder.
func isBrowserRoot(rel string) bool {
	trimmed := strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	return trimmed == "" || trimmed == "."
}

func containsGitPath(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if strings.EqualFold(part, ".git") {
			return true
		}
	}
	return false
}

func resolveBrowserPath(root, rel string) (string, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", apperr.Wrap(apperr.SourceNotFound, err)
	}
	if containsGitPath(root) {
		return "", apperr.New(apperr.SourceNotFound)
	}
	path := strings.ReplaceAll(rel, "\\", "/")
	if path == "" {
		path = "."
	}
	if strings.ContainsRune(path, 0) || strings.HasPrefix(path, "/") || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", apperr.New(apperr.InvalidArgument).WithParam("path", rel)
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." || strings.EqualFold(part, ".git") {
			return "", apperr.New(apperr.InvalidArgument).WithParam("path", rel)
		}
	}
	joined := filepath.Join(root, filepath.FromSlash(path))
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", apperr.Wrap(apperr.SourceNotFound, err).WithParam("path", rel)
	}
	if !config.IsSubdirectory(resolved, root) || containsGitPath(resolved) {
		return "", apperr.New(apperr.SourceNotFound).WithParam("path", rel)
	}
	return resolved, nil
}

func (s *Service) ListFileDirectory(id, rel string) ([]FileDirectoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, isSubscription, err := s.browserPathLocked(id, rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, apperr.Wrap(apperr.SourceNotFound, err)
	}
	if !info.IsDir() {
		if strings.HasPrefix(id, browserRulePrefix) && info.Mode().IsRegular() && (rel == "." || rel == "") {
			return []FileDirectoryEntry{{Path: ".", Name: info.Name()}}, nil
		}
		return nil, apperr.New(apperr.SourceNotFound)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, apperr.Wrap(apperr.SourceNotFound, err)
	}
	root, _, err := s.browserRootLocked(id)
	if err != nil {
		return nil, err
	}
	result := make([]FileDirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), ".git") {
			continue
		}
		if isSubscription && isBrowserRoot(rel) && entry.Name() == rules.RulesFileName {
			continue
		}
		child := filepath.ToSlash(filepath.Join(rel, entry.Name()))
		resolved, err := resolveBrowserPath(root, child)
		if err != nil {
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || (!info.IsDir() && !info.Mode().IsRegular()) {
			continue
		}
		result = append(result, FileDirectoryEntry{Path: child, Name: entry.Name(), IsDir: info.IsDir()})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func (s *Service) ReadBrowserFile(id, rel string) (*RuleSourceContent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, _, err := s.browserPathLocked(id, rel)
	if err != nil {
		return nil, err
	}
	data, _, err := readEditorFile(path)
	if err != nil {
		return nil, err
	}
	return &RuleSourceContent{Content: string(data), Hash: editorHash(data), AbsolutePath: path}, nil
}

func (s *Service) SaveBrowserFile(id, rel, content, expectedHash, expectedPath string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, _, err := s.browserPathLocked(id, rel)
	if err != nil {
		return "", err
	}
	if expectedPath == "" || filepath.Clean(path) != filepath.Clean(expectedPath) {
		return "", apperr.New(apperr.PlanChanged)
	}
	return s.saveEditorFileLocked(path, content, expectedHash)
}
