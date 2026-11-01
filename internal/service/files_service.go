package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"unicode/utf8"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
)

type RuleSourceFile struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	AbsolutePath string `json:"absolutePath"`
}

type RuleSourceContent struct {
	Content      string `json:"content"`
	Hash         string `json:"hash"`
	ReadOnly     bool   `json:"readOnly"`
	AbsolutePath string `json:"absolutePath"`
}

// sourceFilesLocked restricts the editor to files selected by a configured rule.
func (s *Service) sourceFilesLocked(ruleID string) (rules.Rule, []RuleSourceFile, error) {
	all, err := s.allRulesLocked()
	if err != nil {
		return rules.Rule{}, nil, err
	}
	var rule rules.Rule
	found := false
	for _, r := range all {
		if r.ID == ruleID {
			rule, found = r, true
			break
		}
	}
	if !found {
		return rule, nil, apperr.New(apperr.RuleNotFound).WithParam("id", ruleID)
	}
	root, err := rules.SourcePath(s.rulesEnvLocked(), rule)
	if err != nil {
		return rule, nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return rule, nil, apperr.Wrap(apperr.SourceNotFound, err).WithParam("path", root)
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return rule, nil, apperr.New(apperr.SourceNotFound).WithParam("path", root)
		}
		return rule, []RuleSourceFile{{Path: ".", Name: filepath.Base(root), AbsolutePath: root}}, nil
	}
	managed, err := rules.WalkSource(root)
	if err != nil {
		return rule, nil, err
	}
	selected := make(map[string]bool, len(rule.Source.Include))
	for _, path := range rule.Source.Include {
		selected[filepath.ToSlash(filepath.Clean(path))] = true
	}
	files := make([]RuleSourceFile, 0, len(managed))
	for path, abs := range managed {
		if (len(selected) > 0 && !selected[path]) || rules.IsExcluded(path, rule.Exclude) {
			continue
		}
		files = append(files, RuleSourceFile{Path: path, Name: filepath.Base(abs), AbsolutePath: abs})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return rule, files, nil
}

func (s *Service) ListRuleSourceFiles(ruleID string) ([]RuleSourceFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, files, err := s.sourceFilesLocked(ruleID)
	return files, err
}

func (s *Service) sourceFileLocked(ruleID, path string) (rules.Rule, RuleSourceFile, error) {
	rule, files, err := s.sourceFilesLocked(ruleID)
	if err != nil {
		return rule, RuleSourceFile{}, err
	}
	for _, file := range files {
		if file.Path != path {
			continue
		}
		resolved, err := filepath.EvalSymlinks(file.AbsolutePath)
		if err != nil {
			return rule, RuleSourceFile{}, apperr.Wrap(apperr.SourceNotFound, err)
		}
		env := s.rulesEnvLocked()
		root, err := rules.SourcePath(env, rule)
		if err != nil {
			return rule, RuleSourceFile{}, err
		}
		root, err = filepath.EvalSymlinks(root)
		if err != nil || !config.IsSubdirectory(resolved, root) {
			return rule, RuleSourceFile{}, apperr.New(apperr.SourceNotFound).WithParam("path", path)
		}
		var trusted string
		switch rule.Source.Origin {
		case rules.OriginRepo:
			trusted = env.RepoRoot
		case rules.OriginSubscription:
			trusted = env.SubscriptionRoots[rule.Source.Subscription]
		}
		if trusted != "" {
			trusted, err = filepath.EvalSymlinks(trusted)
			if err != nil || !config.IsSubdirectory(root, trusted) {
				return rule, RuleSourceFile{}, apperr.New(apperr.SourceNotFound).WithParam("path", path)
			}
		}
		file.AbsolutePath = resolved
		return rule, file, nil
	}
	return rule, RuleSourceFile{}, apperr.New(apperr.SourceNotFound).WithParam("path", path)
}

func readEditorFile(path string) ([]byte, os.FileMode, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, apperr.Wrap(apperr.SourceNotFound, err).WithParam("path", path)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, 0, apperr.New(apperr.SourceNotFound).WithParam("path", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, rulesync.MaxDiffBytes+1))
	if err != nil {
		return nil, 0, apperr.Wrap(apperr.SourceNotFound, err).WithParam("path", path)
	}
	if len(data) > rulesync.MaxDiffBytes {
		return nil, 0, apperr.New(apperr.InvalidArgument).WithParam("reason", "file_too_large")
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return nil, 0, apperr.New(apperr.InvalidArgument).WithParam("reason", "file_not_text")
	}
	return data, info.Mode().Perm(), nil
}

func editorHash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func (s *Service) ReadRuleSourceFile(ruleID, path string) (*RuleSourceContent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, file, err := s.sourceFileLocked(ruleID, path)
	if err != nil {
		return nil, err
	}
	data, _, err := readEditorFile(file.AbsolutePath)
	if err != nil {
		return nil, err
	}
	return &RuleSourceContent{Content: string(data), Hash: editorHash(data), ReadOnly: rule.Source.Origin == rules.OriginSubscription, AbsolutePath: file.AbsolutePath}, nil
}

func (s *Service) SaveRuleSourceFile(ruleID, path, content, expectedHash string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rule, file, err := s.sourceFileLocked(ruleID, path)
	if err != nil {
		return "", err
	}
	if rule.Source.Origin == rules.OriginSubscription {
		return "", apperr.New(apperr.SourceReadOnly)
	}
	data := []byte(content)
	if len(data) > rulesync.MaxDiffBytes {
		return "", apperr.New(apperr.InvalidArgument).WithParam("reason", "file_too_large")
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", apperr.New(apperr.InvalidArgument).WithParam("reason", "file_not_text")
	}
	resolved, err := filepath.EvalSymlinks(file.AbsolutePath)
	if err != nil {
		return "", apperr.Wrap(apperr.SourceNotFound, err)
	}
	previous, perm, err := readEditorFile(resolved)
	if err != nil {
		return "", err
	}
	if expectedHash == "" || editorHash(previous) != expectedHash {
		return "", apperr.New(apperr.PlanChanged)
	}
	tmp, err := os.CreateTemp(filepath.Dir(resolved), ".agentsync-editor-*")
	if err != nil {
		return "", apperr.Wrap(apperr.WriteFailed, err)
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(perm); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		current, _, readErr := readEditorFile(resolved)
		if readErr != nil || editorHash(current) != expectedHash {
			return "", apperr.New(apperr.PlanChanged)
		}
	}
	if err == nil {
		err = os.Rename(tmp.Name(), resolved)
	}
	if err != nil {
		return "", apperr.Wrap(apperr.WriteFailed, err)
	}
	s.emitRulesChangedLocked()
	return editorHash(data), nil
}
