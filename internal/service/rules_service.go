package service

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/recognize"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/transform"
)

const (
	rulesChangedEvent = "rules:changed"
	maxAliasLength    = 40
	repoTreeMaxDepth  = 6
)

var aliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// LocationInfo describes a location alias as seen on this PC.
type LocationInfo struct {
	Alias        string `json:"alias"`
	Path         string `json:"path"`
	Builtin      bool   `json:"builtin"`
	Exists       bool   `json:"exists"`
	ProfileID    string `json:"profileId"`
	ProfileLabel string `json:"profileLabel"`
	RuleCount    int    `json:"ruleCount"`
}

// SaveRuleResult returns the stored rule and any collisions it is involved in.
type SaveRuleResult struct {
	Rule       rules.Rule        `json:"rule"`
	Collisions []rules.Collision `json:"collisions"`
}

// SourceTreeEntry is a selectable source (file or folder) with its detected kind.
type SourceTreeEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
	Kind  string `json:"kind"`
}

// transformAdapter wires internal/transform into the rulesync engine.
type transformAdapter struct{}

func toTransformSpec(spec rules.Transform) transform.Spec {
	return transform.Spec{Type: spec.Type, Key: spec.Key}
}

func (transformAdapter) Render(spec rules.Transform, source, existing []byte, prevManaged []string) ([]byte, []string, []byte, error) {
	res, err := transform.Render(toTransformSpec(spec), source, existing, prevManaged)
	if err != nil {
		return nil, nil, nil, err
	}
	return res.Output, res.Managed, res.ManagedView, nil
}

func (transformAdapter) SourceView(spec rules.Transform, source []byte) ([]byte, error) {
	return transform.SourceView(toTransformSpec(spec), source)
}

func (transformAdapter) TargetView(spec rules.Transform, existing []byte, managed []string) ([]byte, error) {
	return transform.TargetView(toTransformSpec(spec), existing, managed)
}

func (s *Service) rulesEnvLocked() rules.Env {
	locations := make(map[string]string, len(s.cfg.Locations))
	for _, loc := range s.cfg.Locations {
		locations[loc.Alias] = loc.Path
	}
	subRoots := make(map[string]string, len(s.cfg.Subscriptions))
	for _, sub := range s.cfg.Subscriptions {
		subRoots[sub.ID] = subscriptionsRoot(sub.ID)
	}
	return rules.Env{
		RepoRoot:          s.cfg.RootPath,
		Locations:         locations,
		HomeDir:           rules.DefaultHomeDir(),
		SubscriptionRoots: subRoots,
	}
}

// loadRuleFilesLocked returns the repo and local rule files (repo file is empty without a root).
func (s *Service) loadRuleFilesLocked() (*rules.File, *rules.File, error) {
	repoFile := &rules.File{Version: rules.CurrentVersion}
	if s.cfg.RootPath != "" {
		loaded, err := rules.LoadRepoRules(s.cfg.RootPath)
		if err != nil {
			return nil, nil, err
		}
		repoFile = loaded
	}
	localFile, err := rules.LoadLocalRules()
	if err != nil {
		return nil, nil, err
	}
	return repoFile, localFile, nil
}

func (s *Service) allRulesLocked() ([]rules.Rule, error) {
	repoFile, localFile, err := s.loadRuleFilesLocked()
	if err != nil {
		return nil, err
	}
	all := make([]rules.Rule, 0, len(repoFile.Rules)+len(localFile.Rules))
	for _, r := range repoFile.Rules {
		r.Scope = rules.ScopeRepo
		all = append(all, r)
	}
	for _, r := range localFile.Rules {
		r.Scope = rules.ScopeLocal
		all = append(all, r)
	}
	return all, nil
}

func (s *Service) recognizersLocked() *recognize.Set {
	set, err := recognize.Load()
	if err == nil {
		return set
	}
	defaults, defErr := recognize.Defaults()
	if defErr != nil {
		return &recognize.Set{}
	}
	return defaults
}

func (s *Service) engineLocked() *rulesync.Engine {
	set := s.recognizersLocked()
	return &rulesync.Engine{
		Env:         s.rulesEnvLocked(),
		State:       s.st,
		BackupRoot:  s.cfg.BackupPath,
		Retention:   s.cfg.BackupRetention,
		Transformer: transformAdapter{},
		Legacy:      s.cfg.Connections,
		DetectKind: func(absPath string) string {
			info, err := set.DetectSource(absPath)
			if err != nil {
				return rules.KindFile
			}
			return info.Kind
		},
	}
}

// managedRepoPathsLocked lists repo paths owned by repo-origin rules plus the rules file itself.
func (s *Service) managedRepoPathsLocked() []string {
	paths := []string{rules.RulesFileName}
	all, err := s.allRulesLocked()
	if err != nil {
		return paths
	}
	for _, r := range all {
		if r.Source.Origin == rules.OriginRepo && r.Source.Path != "" {
			paths = append(paths, filepath.ToSlash(r.Source.Path))
		}
	}
	return paths
}

// syncOrchestratorRulesLocked keeps the orchestrator aware of rule-owned repo paths.
func (s *Service) syncOrchestratorRulesLocked() {
	s.orch.ManagedRepoPaths = s.managedRepoPathsLocked()
}

// annotatePlanLocked adds rule counters to a plan so the UI can enable Sync for rule-only changes.
func (s *Service) annotatePlanLocked(planPending, planConflicts *int) {
	all, err := s.allRulesLocked()
	if err != nil {
		return
	}
	report := s.engineLocked().Status(all)
	*planPending = report.Pending
	*planConflicts = report.Conflicts
}

func (s *Service) emitRulesChangedLocked() {
	if s.emitFunc != nil {
		s.emitFunc(rulesChangedEvent, nil)
	}
}

// GetRulesReport returns every rule with per-destination and per-file status, collisions and unmapped aliases.
func (s *Service) GetRulesReport() (*rulesync.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.allRulesLocked()
	if err != nil {
		return nil, err
	}
	report := s.engineLocked().Status(all)
	return &report, nil
}

func findRuleIndex(f *rules.File, id string) int {
	for i := range f.Rules {
		if f.Rules[i].ID == id {
			return i
		}
	}
	return -1
}

func collisionsForRule(all []rules.Collision, ruleID string) []rules.Collision {
	related := make([]rules.Collision, 0)
	for _, c := range all {
		for _, ref := range c.Refs {
			if ref.RuleID == ruleID {
				related = append(related, c)
				break
			}
		}
	}
	return related
}

func normalizeRule(r *rules.Rule) {
	if r.ID == "" {
		r.ID = rules.NewID("r_")
	}
	if r.Kind == "" {
		r.Kind = rules.KindAuto
	}
	r.Source.Path = strings.TrimSpace(r.Source.Path)
	if r.Source.Origin != rules.OriginLocal {
		r.Source.Path = filepath.ToSlash(r.Source.Path)
	}
	for i := range r.Destinations {
		d := &r.Destinations[i]
		if d.ID == "" {
			d.ID = rules.NewID("d_")
		}
		d.Path = filepath.ToSlash(strings.TrimSpace(d.Path))
		if d.Mode == "" {
			d.Mode = rules.ModeTwoWay
		}
		if d.Transform != nil {
			d.Mode = rules.ModeOneWay
		}
	}
}

// SaveRule creates or updates a rule in the given scope ("repo" or "local").
// Moving a rule between scopes removes it from the other file. Collisions do not block saving;
// they are returned so the UI can warn, and the involved destinations stay blocked until fixed.
func (s *Service) SaveRule(rule rules.Rule, scope string) (*SaveRuleResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if scope != rules.ScopeRepo && scope != rules.ScopeLocal {
		return nil, apperr.New(apperr.InvalidRule).WithParam("reason", "invalid_scope")
	}
	if scope == rules.ScopeRepo && s.cfg.RootPath == "" {
		return nil, apperr.New(apperr.RootNotFound)
	}
	normalizeRule(&rule)
	if err := rules.Validate(rule, scope); err != nil {
		return nil, err
	}

	repoFile, localFile, err := s.loadRuleFilesLocked()
	if err != nil {
		return nil, err
	}
	target, other := localFile, repoFile
	if scope == rules.ScopeRepo {
		target, other = repoFile, localFile
	}
	stored := rule
	stored.Scope = ""
	if idx := findRuleIndex(target, rule.ID); idx >= 0 {
		target.Rules[idx] = stored
	} else {
		target.Rules = append(target.Rules, stored)
	}
	if idx := findRuleIndex(other, rule.ID); idx >= 0 {
		other.Rules = append(other.Rules[:idx], other.Rules[idx+1:]...)
	}

	if s.cfg.RootPath != "" {
		if err := rules.SaveRepoRules(s.cfg.RootPath, repoFile); err != nil {
			return nil, err
		}
	}
	if err := rules.SaveLocalRules(localFile); err != nil {
		return nil, err
	}

	s.syncOrchestratorRulesLocked()
	all, err := s.allRulesLocked()
	if err != nil {
		return nil, err
	}
	report := s.engineLocked().Status(all)
	s.emitRulesChangedLocked()
	rule.Scope = scope
	return &SaveRuleResult{Rule: rule, Collisions: collisionsForRule(report.Collisions, rule.ID)}, nil
}

// DeleteRule removes a rule and its baselines. Files already written to destinations are left in place.
func (s *Service) DeleteRule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	repoFile, localFile, err := s.loadRuleFilesLocked()
	if err != nil {
		return err
	}
	var removed *rules.Rule
	if idx := findRuleIndex(repoFile, id); idx >= 0 {
		r := repoFile.Rules[idx]
		removed = &r
		repoFile.Rules = append(repoFile.Rules[:idx], repoFile.Rules[idx+1:]...)
		if err := rules.SaveRepoRules(s.cfg.RootPath, repoFile); err != nil {
			return err
		}
	} else if idx := findRuleIndex(localFile, id); idx >= 0 {
		r := localFile.Rules[idx]
		removed = &r
		localFile.Rules = append(localFile.Rules[:idx], localFile.Rules[idx+1:]...)
		if err := rules.SaveLocalRules(localFile); err != nil {
			return err
		}
	}
	if removed == nil {
		return apperr.New(apperr.RuleNotFound).WithParam("rule", id)
	}

	for _, d := range removed.Destinations {
		s.st.RemoveConnection(rulesync.BaselinePrefix + removed.ID + ":" + d.ID)
	}
	if err := s.st.Save(); err != nil {
		return err
	}
	s.syncOrchestratorRulesLocked()
	s.emitRulesChangedLocked()
	return nil
}

// SyncRules applies every non-conflicting rule change (writes, captures and deletions) with backups.
// Captured repo files are committed on the next global Sync.
func (s *Service) SyncRules() (*rulesync.ApplyResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.allRulesLocked()
	if err != nil {
		return nil, err
	}
	res, err := s.engineLocked().Sync(all)
	if err != nil {
		return nil, err
	}
	s.emitStatusLocked()
	s.emitRulesChangedLocked()
	return &res, nil
}

// ResolveRuleFile applies the user's decision for one conflicting/drifted file.
func (s *Service) ResolveRuleFile(res rulesync.Resolution) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.allRulesLocked()
	if err != nil {
		return err
	}
	updated, _, err := s.engineLocked().Resolve(all, res)
	if err != nil {
		return err
	}
	if updated != nil {
		if err := s.persistRuleLocked(*updated); err != nil {
			return err
		}
	}
	s.emitStatusLocked()
	s.emitRulesChangedLocked()
	return nil
}

// persistRuleLocked writes an existing rule back to the file it came from.
func (s *Service) persistRuleLocked(rule rules.Rule) error {
	repoFile, localFile, err := s.loadRuleFilesLocked()
	if err != nil {
		return err
	}
	scope := rule.Scope
	rule.Scope = ""
	if scope == rules.ScopeRepo {
		if idx := findRuleIndex(repoFile, rule.ID); idx >= 0 {
			repoFile.Rules[idx] = rule
			return rules.SaveRepoRules(s.cfg.RootPath, repoFile)
		}
	}
	if idx := findRuleIndex(localFile, rule.ID); idx >= 0 {
		localFile.Rules[idx] = rule
		return rules.SaveLocalRules(localFile)
	}
	if idx := findRuleIndex(repoFile, rule.ID); idx >= 0 {
		repoFile.Rules[idx] = rule
		return rules.SaveRepoRules(s.cfg.RootPath, repoFile)
	}
	return apperr.New(apperr.RuleNotFound).WithParam("rule", rule.ID)
}

// GetRuleDiff returns source vs destination content for one file of a rule destination.
func (s *Service) GetRuleDiff(ruleID, destID, relPath string) (*rulesync.Diff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	all, err := s.allRulesLocked()
	if err != nil {
		return nil, err
	}
	diff, err := s.engineLocked().Diff(all, ruleID, destID, relPath)
	if err != nil {
		return nil, err
	}
	return &diff, nil
}

// emitStatusLocked recomputes the global plan (no fetch) and emits it.
func (s *Service) emitStatusLocked() {
	if s.cfg.RootPath == "" {
		return
	}
	s.syncOrchestratorRulesLocked()
	if plan, err := s.orch.PlanSyncWithOptions(s.cfg, s.st, false); err == nil {
		s.annotatePlanLocked(&plan.RulesPending, &plan.RulesConflicts)
		s.emitPlanLocked(plan)
	}
}

// ---- Locations ----

// ListLocations returns user aliases plus the builtin "home", with detected folder profiles.
func (s *Service) ListLocations() ([]LocationInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	set := s.recognizersLocked()
	usage := make(map[string]int)
	if all, err := s.allRulesLocked(); err == nil {
		for _, r := range all {
			for _, d := range r.Destinations {
				usage[d.Location]++
			}
		}
	}

	describe := func(alias, path string, builtin bool) LocationInfo {
		info := LocationInfo{Alias: alias, Path: path, Builtin: builtin, RuleCount: usage[alias]}
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			info.Exists = true
			if prof, ok := set.DetectFolder(path); ok {
				info.ProfileID = prof.ProfileID
				info.ProfileLabel = prof.Label
			}
		}
		return info
	}

	result := []LocationInfo{describe(rules.BuiltinHomeAlias, rules.DefaultHomeDir(), true)}
	known := map[string]bool{rules.BuiltinHomeAlias: true}
	for _, loc := range s.cfg.Locations {
		result = append(result, describe(loc.Alias, loc.Path, false))
		known[loc.Alias] = true
	}
	// Aliases referenced by rules but not mapped on this PC.
	unmapped := make([]string, 0)
	for alias := range usage {
		if !known[alias] {
			unmapped = append(unmapped, alias)
		}
	}
	sort.Strings(unmapped)
	for _, alias := range unmapped {
		result = append(result, LocationInfo{Alias: alias, RuleCount: usage[alias]})
	}
	return result, nil
}

// SaveLocation creates or updates an alias mapping for this PC.
func (s *Service) SaveLocation(alias, path string) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	alias = strings.ToLower(strings.TrimSpace(alias))
	if alias == rules.BuiltinHomeAlias || len(alias) > maxAliasLength || !aliasPattern.MatchString(alias) {
		return nil, apperr.New(apperr.LocationInvalid).WithParam("reason", "invalid_alias")
	}
	abs := filepath.Clean(expandPath(path))
	if !filepath.IsAbs(abs) {
		return nil, apperr.New(apperr.PathNotAbsolute).WithParam("path", path)
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return nil, apperr.New(apperr.TargetNotFound).WithParam("path", abs)
	}

	updated := false
	for i := range s.cfg.Locations {
		if s.cfg.Locations[i].Alias == alias {
			s.cfg.Locations[i].Path = abs
			updated = true
		}
	}
	if !updated {
		s.cfg.Locations = append(s.cfg.Locations, config.Location{Alias: alias, Path: abs})
	}
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}
	s.emitRulesChangedLocked()
	return cloneConfig(s.cfg), nil
}

// RemoveLocation deletes an alias mapping on this PC; rules referencing it become unmapped (skipped).
func (s *Service) RemoveLocation(alias string) (*config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kept := make([]config.Location, 0, len(s.cfg.Locations))
	for _, loc := range s.cfg.Locations {
		if loc.Alias != alias {
			kept = append(kept, loc)
		}
	}
	s.cfg.Locations = kept
	if err := config.Save(s.cfg); err != nil {
		return nil, err
	}
	s.emitRulesChangedLocked()
	return cloneConfig(s.cfg), nil
}

// ---- Detection ----

// sourceAbsLocked resolves a rule source to an absolute path on this PC.
func (s *Service) sourceAbsLocked(src rules.Source) (string, error) {
	rel := filepath.FromSlash(src.Path)
	switch src.Origin {
	case rules.OriginRepo:
		if s.cfg.RootPath == "" {
			return "", apperr.New(apperr.RootNotFound)
		}
		return safeJoin(s.cfg.RootPath, rel)
	case rules.OriginSubscription:
		if !s.hasSubscriptionLocked(src.Subscription) {
			return "", apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", src.Subscription)
		}
		return safeJoin(subscriptionsRoot(src.Subscription), rel)
	case rules.OriginLocal:
		abs := filepath.Clean(expandPath(src.Path))
		if !filepath.IsAbs(abs) {
			return "", apperr.New(apperr.PathNotAbsolute).WithParam("path", src.Path)
		}
		return abs, nil
	}
	return "", apperr.New(apperr.InvalidRule).WithParam("reason", "invalid_origin")
}

// safeJoin joins a relative path under root and rejects escapes.
func safeJoin(root, rel string) (string, error) {
	joined := filepath.Join(root, rel)
	if filepath.IsAbs(rel) || !config.IsSubdirectory(joined, root) {
		return "", apperr.New(apperr.InvalidRule).WithParam("reason", "path_escapes_root")
	}
	return joined, nil
}

// DetectSource reports what kind of content a source is (skill, mcp, instructions, file, folder).
func (s *Service) DetectSource(src rules.Source) (*recognize.SourceInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	abs, err := s.sourceAbsLocked(src)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(abs); err != nil {
		return nil, apperr.New(apperr.SourceNotFound).WithParam("path", abs)
	}
	info, err := s.recognizersLocked().DetectSource(abs)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

// SuggestPlacement proposes where/how a source should land inside a location (detected profile or generic copy).
func (s *Service) SuggestPlacement(src rules.Source, alias string) (*recognize.Suggestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	abs, err := s.sourceAbsLocked(src)
	if err != nil {
		return nil, err
	}
	set := s.recognizersLocked()
	info, err := set.DetectSource(abs)
	if err != nil {
		return nil, err
	}
	env := s.rulesEnvLocked()
	locationAbs := env.Locations[alias]
	if alias == rules.BuiltinHomeAlias {
		locationAbs = env.HomeDir
	}
	if locationAbs == "" {
		return nil, apperr.New(apperr.LocationUnmapped).WithParam("alias", alias)
	}
	suggestion := set.Suggest(info, alias, locationAbs)
	return &suggestion, nil
}

// GetRecognizersPath ensures the user recognizers file exists (seeded with defaults) and returns its path.
func (s *Service) GetRecognizersPath() (string, error) {
	return recognize.WriteUserTemplate()
}

// ListRepoTree lists files and folders of the repository root for picking rule sources.
func (s *Service) ListRepoTree() ([]SourceTreeEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return []SourceTreeEntry{}, nil
	}
	return listSourceTree(s.cfg.RootPath, repoTreeMaxDepth, s.recognizersLocked())
}

// treeNoiseNames are never offered as rule sources. Dot-folders such as .claude or .agents stay visible.
var treeNoiseNames = map[string]bool{".git": true, ".ds_store": true, "thumbs.db": true, "desktop.ini": true}

func isTreeNoise(name string) bool {
	lower := strings.ToLower(name)
	return treeNoiseNames[lower] || strings.HasSuffix(lower, fsops.TempFileSuffix)
}

// listSourceTree walks root (skipping .git and ignored names) and annotates entries with detected kinds.
func listSourceTree(root string, maxDepth int, set *recognize.Set) ([]SourceTreeEntry, error) {
	entries := make([]SourceTreeEntry, 0)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || path == root {
			return nil
		}
		if isTreeNoise(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		depth := strings.Count(filepath.ToSlash(rel), "/") + 1
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entry := SourceTreeEntry{Path: filepath.ToSlash(rel), IsDir: d.IsDir()}
		if info, err := d.Info(); err == nil && !d.IsDir() {
			entry.Size = info.Size()
		}
		if detected, err := set.DetectSource(path); err == nil {
			entry.Kind = detected.Kind
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, apperr.Wrap(apperr.Unknown, err)
	}
	return entries, nil
}

// ImportIntoRepo copies a local file or folder into the repository so it can be versioned and shared.
// Returns the repo-relative path of the copy. Existing destinations are never overwritten.
func (s *Service) ImportIntoRepo(sourcePath, repoRelDir string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.RootPath == "" {
		return "", apperr.New(apperr.RootNotFound)
	}
	src := filepath.Clean(expandPath(sourcePath))
	fi, err := os.Stat(src)
	if err != nil {
		return "", apperr.New(apperr.SourceNotFound).WithParam("path", src)
	}
	destDir, err := safeJoin(s.cfg.RootPath, filepath.FromSlash(strings.TrimSpace(repoRelDir)))
	if err != nil {
		return "", err
	}
	dest := filepath.Join(destDir, filepath.Base(src))
	if _, err := os.Stat(dest); err == nil {
		return "", apperr.New(apperr.FolderExists).WithParam("path", dest)
	}

	if !fi.IsDir() {
		if err := fsops.AtomicCopy(src, dest); err != nil {
			return "", apperr.Wrap(apperr.WriteFailed, err)
		}
	} else {
		files, err := fsops.WalkManaged(src)
		if err != nil {
			return "", err
		}
		for rel := range files {
			if matchesAnyPattern(rel, rules.DefaultSecretExcludes) {
				continue
			}
			if err := fsops.AtomicCopy(filepath.Join(src, filepath.FromSlash(rel)), filepath.Join(dest, filepath.FromSlash(rel))); err != nil {
				return "", apperr.Wrap(apperr.WriteFailed, err)
			}
		}
	}

	rel, err := filepath.Rel(s.cfg.RootPath, dest)
	if err != nil {
		return "", apperr.Wrap(apperr.Unknown, err)
	}
	return filepath.ToSlash(rel), nil
}

// matchesAnyPattern matches a forward-slash relative path (or its base name) against glob patterns.
func matchesAnyPattern(rel string, patterns []string) bool {
	base := filepath.Base(filepath.FromSlash(rel))
	for _, p := range patterns {
		if ok, _ := filepath.Match(p, base); ok {
			return true
		}
		if ok, _ := filepath.Match(p, rel); ok {
			return true
		}
	}
	return false
}
