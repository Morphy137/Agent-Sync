package service

import (
	"os"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/fsops"
	"agentsync/internal/gitops"
)

// recommendedHeader labels the block appended by AddRecommendedGitignore.
const recommendedHeader = "Recommended by AgentSync"

// excludedHeader labels entries added from the UI ("Exclude from repository").
const excludedHeader = "Excluded from sync"

// GitignorePayload describes the repository's .gitignore for the exclusions editor.
type GitignorePayload struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Content string `json:"content"`
	// MissingRecommended lists default patterns not yet present in the file.
	MissingRecommended []string `json:"missingRecommended"`
}

// GetGitignore returns the repository .gitignore content and the recommended patterns it lacks.
func (s *Service) GetGitignore() (*GitignorePayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gitignoreLocked()
}

// SaveGitignore overwrites the repository .gitignore with content.
func (s *Service) SaveGitignore(content string) (*GitignorePayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.gitignorePathLocked()
	if err != nil {
		return nil, err
	}
	if err := fsops.AtomicWrite(path, []byte(normalizeGitignore(content)), fsops.DefaultFilePerms); err != nil {
		return nil, apperr.Wrap(apperr.WriteFailed, err)
	}
	return s.gitignoreLocked()
}

// AddRecommendedGitignore appends every recommended pattern that is still missing.
func (s *Service) AddRecommendedGitignore() (*GitignorePayload, error) {
	return s.appendGitignore(recommendedHeader, gitops.GitignorePatterns(gitops.DefaultGitignoreContent))
}

// ExcludeRepoPath adds a file or folder inside the repository (absolute or repository-relative)
// to .gitignore. Files that were already committed stay tracked until they are removed from git.
func (s *Service) ExcludeRepoPath(path string) (*GitignorePayload, error) {
	pattern, err := s.exclusionPattern(path)
	if err != nil {
		return nil, err
	}
	return s.appendGitignore(excludedHeader, []string{pattern})
}

func (s *Service) exclusionPattern(path string) (string, error) {
	s.mu.Lock()
	root := s.cfg.RootPath
	s.mu.Unlock()

	if root == "" {
		return "", apperr.New(apperr.RootNotFound)
	}
	relPath := strings.TrimSpace(path)
	if filepath.IsAbs(relPath) {
		rel, err := filepath.Rel(root, relPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", apperr.New(apperr.InvalidRule).WithParam("reason", "path_escapes_root")
		}
		relPath = rel
	}
	clean := strings.Trim(filepath.ToSlash(filepath.Clean(relPath)), "/")
	if clean == "" || clean == "." || clean == ".git" || strings.HasPrefix(clean, ".git/") {
		return "", apperr.New(apperr.InvalidRule).WithParam("reason", "invalid_exclusion")
	}
	abs, err := safeJoin(root, clean)
	if err != nil {
		return "", err
	}
	info, isPresent := statExists(abs)
	isDir := isPresent && info.IsDir()
	// Anchor to the repository root so "skills" does not also hide "docs/skills".
	pattern := "/" + clean
	if isDir {
		pattern += "/"
	}
	return pattern, nil
}

func (s *Service) appendGitignore(header string, patterns []string) (*GitignorePayload, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.gitignorePathLocked()
	if err != nil {
		return nil, err
	}
	current, err := readOptional(path)
	if err != nil {
		return nil, err
	}
	updated, added := gitops.AppendGitignorePatterns(current, header, patterns)
	if len(added) > 0 {
		if err := fsops.AtomicWrite(path, []byte(updated), fsops.DefaultFilePerms); err != nil {
			return nil, apperr.Wrap(apperr.WriteFailed, err)
		}
	}
	return s.gitignoreLocked()
}

func (s *Service) gitignorePathLocked() (string, error) {
	if s.cfg.RootPath == "" {
		return "", apperr.New(apperr.RootNotFound)
	}
	return filepath.Join(s.cfg.RootPath, gitops.GitignoreFileName), nil
}

func (s *Service) gitignoreLocked() (*GitignorePayload, error) {
	path, err := s.gitignorePathLocked()
	if err != nil {
		return nil, err
	}
	content, err := readOptional(path)
	if err != nil {
		return nil, err
	}
	_, exists := statExists(path)
	_, missing := gitops.AppendGitignorePatterns(content, "", gitops.GitignorePatterns(gitops.DefaultGitignoreContent))
	return &GitignorePayload{Path: path, Exists: exists, Content: content, MissingRecommended: missing}, nil
}

func readOptional(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", apperr.Wrap(apperr.WriteFailed, err)
	}
	return string(data), nil
}

func statExists(path string) (os.FileInfo, bool) {
	info, err := os.Stat(path)
	return info, err == nil
}

func normalizeGitignore(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if normalized != "" && !strings.HasSuffix(normalized, "\n") {
		normalized += "\n"
	}
	return normalized
}
