package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"agentsync/internal/recognize"
	"agentsync/internal/rules"
)

const uncategorized = "uncategorized"

var builtinCategories = []rules.Category{
	{ID: rules.KindSkill, Color: "amber", Icon: "puzzle", Scope: "builtin"},
	{ID: rules.KindMCP, Color: "blue", Icon: "terminal", Scope: "builtin"},
	{ID: rules.KindInstructions, Color: "green", Icon: "book", Scope: "builtin"},
}

func categoryExists(categories []rules.Category, id string) bool {
	for _, category := range categories {
		if category.ID == id {
			return true
		}
	}
	return false
}

func defaultCategoryIcon(id string) string {
	for _, category := range builtinCategories {
		if category.ID == id {
			return category.Icon
		}
	}
	return "folder"
}

func categoryDeleted(id string, repo, local *rules.File) bool {
	if !categoryExists(builtinCategories, id) {
		return false
	}
	for _, file := range []*rules.File{repo, local} {
		for _, category := range file.Categories {
			if category.ID == id && category.Deleted {
				return true
			}
		}
	}
	return false
}

func availableCategory(id string, repo, local *rules.File) string {
	if categoryDeleted(id, repo, local) {
		return uncategorized
	}
	return id
}

func inferRuleCategory(rule rules.Rule, env rules.Env, set *recognize.Set) string {
	path, err := rules.SourcePath(env, rule)
	if err != nil {
		return ""
	}
	info, err := set.DetectSource(path)
	if err != nil {
		return ""
	}
	kind := rule.Kind
	if kind == "" || kind == rules.KindAuto {
		kind = info.Kind
		if kind == rules.KindFile && strings.HasSuffix(strings.ToLower(filepath.Base(path)), "-agents.md") {
			for _, dest := range rule.Destinations {
				switch strings.ToLower(filepath.Base(filepath.FromSlash(dest.Path))) {
				case "claude.md", "agents.md", "gemini.md":
					return rules.KindInstructions
				}
			}
		}
	}
	if kind == rules.KindFolder && info.IsDir {
		entries, err := os.ReadDir(path)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				marker, err := os.Stat(filepath.Join(path, entry.Name(), "SKILL.md"))
				if err == nil && marker.Mode().IsRegular() {
					return rules.KindSkill
				}
			}
		}
	}
	switch kind {
	case rules.KindSkill, rules.KindMCP, rules.KindInstructions:
		return kind
	default:
		return uncategorized
	}
}

// GetRuleCategories lists built-in, repository, and local categories.
func (s *Service) GetRuleCategories() ([]rules.Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	repo, local, err := s.loadRuleFilesLocked()
	if err != nil {
		return nil, err
	}
	all := append([]rules.Category(nil), builtinCategories...)
	for _, pair := range []struct {
		file  *rules.File
		scope string
	}{{repo, rules.ScopeRepo}, {local, rules.ScopeLocal}} {
		for _, category := range pair.file.Categories {
			category.Scope = pair.scope
			if category.Icon == "" {
				category.Icon = defaultCategoryIcon(category.ID)
				for _, existing := range all {
					if existing.ID == category.ID {
						category.Icon = existing.Icon
						break
					}
				}
			}
			if categoryExists(builtinCategories, category.ID) {
				for i := range all {
					if all[i].ID == category.ID {
						all[i] = category
						break
					}
				}
			} else {
				all = append(all, category)
			}
		}
	}
	visible := all[:0]
	for _, category := range all {
		if !categoryDeleted(category.ID, repo, local) {
			visible = append(visible, category)
		}
	}
	return visible, nil
}

// SaveRuleCategory creates or updates a category in its chosen scope.
func (s *Service) SaveRuleCategory(category rules.Category, scope string) (rules.Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if scope != rules.ScopeRepo && scope != rules.ScopeLocal {
		return rules.Category{}, fmt.Errorf("invalid category scope")
	}
	if scope == rules.ScopeRepo && s.cfg.RootPath == "" {
		return rules.Category{}, fmt.Errorf("repository root required")
	}
	category.Name = strings.TrimSpace(category.Name)
	if category.Name == "" && !categoryExists(builtinCategories, category.ID) {
		return rules.Category{}, fmt.Errorf("category name required")
	}
	switch category.Color {
	case "blue", "purple", "green", "amber", "teal", "rose", "gray":
	default:
		return rules.Category{}, fmt.Errorf("invalid category color")
	}
	repo, local, err := s.loadRuleFilesLocked()
	if err != nil {
		return rules.Category{}, err
	}
	if category.ID == "" {
		category.ID = rules.NewID("c_")
	}
	if category.ID == uncategorized {
		return rules.Category{}, fmt.Errorf("uncategorized cannot be changed")
	}
	target, other := local, repo
	if scope == rules.ScopeRepo {
		target, other = repo, local
	}
	if !categoryExists(builtinCategories, category.ID) && categoryExists(other.Categories, category.ID) {
		return rules.Category{}, fmt.Errorf("category ID already exists")
	}
	if category.Icon == "" {
		for _, existing := range target.Categories {
			if existing.ID == category.ID {
				category.Icon = existing.Icon
				break
			}
		}
		if category.Icon == "" && categoryExists(builtinCategories, category.ID) {
			for _, existing := range other.Categories {
				if existing.ID == category.ID {
					category.Icon = existing.Icon
					break
				}
			}
		}
		if category.Icon == "" {
			category.Icon = defaultCategoryIcon(category.ID)
		}
	}
	switch category.Icon {
	case "folder", "book", "code", "terminal", "puzzle", "bot", "shield", "star", "tag", "settings":
	default:
		return rules.Category{}, fmt.Errorf("invalid category icon")
	}
	category.Scope = ""
	category.Deleted = false
	found := false
	for i := range target.Categories {
		if target.Categories[i].ID == category.ID {
			target.Categories[i] = category
			found = true
			break
		}
	}
	if !found {
		target.Categories = append(target.Categories, category)
	}
	if scope == rules.ScopeRepo {
		err = rules.SaveRepoRules(s.cfg.RootPath, repo)
	} else {
		err = rules.SaveLocalRules(local)
	}
	if err != nil {
		return rules.Category{}, err
	}
	s.emitRulesChangedLocked()
	category.Scope = scope
	return category, nil
}

// DeleteRuleCategory removes a category and clears its rule assignments.
func (s *Service) DeleteRuleCategory(id, scope string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	builtin := categoryExists(builtinCategories, id)
	if !builtin && scope != rules.ScopeRepo && scope != rules.ScopeLocal {
		return fmt.Errorf("invalid category scope")
	}
	if id == uncategorized {
		return fmt.Errorf("uncategorized cannot be deleted")
	}
	if builtin {
		scope = rules.ScopeLocal
		if s.cfg.RootPath != "" {
			scope = rules.ScopeRepo
		}
	}
	repo, local, err := s.loadRuleFilesLocked()
	if err != nil {
		return err
	}
	target, other := local, repo
	if scope == rules.ScopeRepo {
		target, other = repo, local
	}
	if !builtin && categoryExists(other.Categories, id) {
		return fmt.Errorf("category ID exists in both scopes")
	}
	index := -1
	for i, category := range target.Categories {
		if category.ID == id {
			index = i
			break
		}
	}
	if builtin {
		if index >= 0 {
			target.Categories[index].Deleted = true
		} else {
			target.Categories = append(target.Categories, rules.Category{ID: id, Icon: defaultCategoryIcon(id), Deleted: true})
		}
	} else {
		if index < 0 {
			return fmt.Errorf("category not found")
		}
		target.Categories = append(target.Categories[:index], target.Categories[index+1:]...)
	}
	localChanged, repoChanged := false, false
	for i := range local.Rules {
		if local.Rules[i].CategoryID == id {
			local.Rules[i].CategoryID = uncategorized
			localChanged = true
		}
	}
	for i := range repo.Rules {
		if repo.Rules[i].CategoryID == id {
			repo.Rules[i].CategoryID = uncategorized
			repoChanged = true
		}
	}
	if scope == rules.ScopeLocal || localChanged {
		if err := rules.SaveLocalRules(local); err != nil {
			return err
		}
	}
	if scope == rules.ScopeRepo || repoChanged {
		if err := rules.SaveRepoRules(s.cfg.RootPath, repo); err != nil {
			return err
		}
	}
	s.emitRulesChangedLocked()
	return nil
}

// MigrateRuleCategories assigns categories once, leaving unavailable sources for a future retry.
func (s *Service) MigrateRuleCategories() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	repo, local, err := s.loadRuleFilesLocked()
	if err != nil {
		return err
	}
	env, set := s.rulesEnvLocked(), s.recognizersLocked()
	changed := false
	for _, pair := range []struct {
		file  *rules.File
		scope string
	}{{repo, rules.ScopeRepo}, {local, rules.ScopeLocal}} {
		assignments := make(map[int]rules.CategoryAssignment)
		for i := range pair.file.Rules {
			rule := &pair.file.Rules[i]
			if rule.CategoryID != "" {
				continue
			}
			if category := inferRuleCategory(*rule, env, set); category != "" {
				assignments[i] = rules.CategoryAssignment{RuleID: rule.ID, CategoryID: availableCategory(category, repo, local)}
			}
		}
		if len(assignments) == 0 {
			continue
		}
		path := rules.LocalRulesPath()
		if pair.scope == rules.ScopeRepo {
			path = filepath.Join(s.cfg.RootPath, rules.RulesFileName)
		}
		err = rules.SaveCategoryAssignments(path, assignments)
		if err != nil {
			return err
		}
		changed = true
	}
	if changed {
		s.emitRulesChangedLocked()
	}
	return nil
}

// SuggestRuleCategory detects the available category for a source without modifying any files.
func (s *Service) SuggestRuleCategory(source rules.Source, kind string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	repo, local, err := s.loadRuleFilesLocked()
	if err != nil {
		return "", err
	}
	category := inferRuleCategory(rules.Rule{Source: source, Kind: kind}, s.rulesEnvLocked(), s.recognizersLocked())
	if category == "" {
		return uncategorized, nil
	}
	return availableCategory(category, repo, local), nil
}
