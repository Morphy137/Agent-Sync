// Package ops is the headless facade over internal/service used by the CLI and the MCP server.
// It adds agent-friendly composite operations (e.g. "add a rule and suggest where each destination
// goes") on top of the exact same Service the Wails GUI binds, so no sync logic is duplicated.
package ops

import (
	"path"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/recognize"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/service"
	"agentsync/internal/subscriptions"
)

// Repo diff modes accepted by RepoFileDiff.
const (
	DiffModeWorking  = "working"
	DiffModeIncoming = "incoming"
)

// API exposes AgentSync operations for headless callers.
type API struct {
	svc *service.Service
}

// New wraps an existing Service.
func New(svc *service.Service) *API {
	return &API{svc: svc}
}

// NewDefault creates a Service backed by the user's config dir (or AGENTSYNC_CONFIG_DIR).
func NewDefault() *API {
	return New(service.New(nil))
}

// Language returns the UI language stored in the config ("system", "en" or "es").
func (a *API) Language() string {
	cfg, err := a.svc.GetConfig()
	if err != nil {
		return config.DefaultLanguage
	}
	return cfg.Language
}

// ---- Status and sync ----

// RulesSummary counts rules and their pending work.
type RulesSummary struct {
	Total      int      `json:"total"`
	Enabled    int      `json:"enabled"`
	Pending    int      `json:"pending"`
	Conflicts  int      `json:"conflicts"`
	Collisions int      `json:"collisions"`
	Unmapped   []string `json:"unmapped"`
}

// StatusReport is a one-shot overview of the app state on this PC.
type StatusReport struct {
	ConfigDir     string                  `json:"configDir"`
	RootPath      string                  `json:"rootPath"`
	Git           *service.GitInfoPayload `json:"git,omitempty"`
	Plan          *orchestrator.SyncPlan  `json:"plan,omitempty"`
	PlanError     string                  `json:"planError,omitempty"`
	Rules         RulesSummary            `json:"rules"`
	Locations     int                     `json:"locations"`
	Subscriptions int                     `json:"subscriptions"`
}

// Status returns the local status without contacting the remote.
func (a *API) Status() (*StatusReport, error) {
	cfg, err := a.svc.GetConfig()
	if err != nil {
		return nil, err
	}
	report := &StatusReport{
		ConfigDir:     config.GetConfigDir(),
		RootPath:      cfg.RootPath,
		Locations:     len(cfg.Locations),
		Subscriptions: len(cfg.Subscriptions),
		Rules:         RulesSummary{Unmapped: []string{}},
	}
	if cfg.RootPath == "" {
		return report, nil
	}
	if git, err := a.svc.GetGitInfo(); err == nil {
		report.Git = git
	}
	if plan, err := a.svc.GetStatus(); err == nil {
		report.Plan = plan
	} else {
		report.PlanError = err.Error()
	}
	if rr, err := a.svc.GetRulesReport(); err == nil {
		report.Rules = summarizeRules(rr)
	}
	return report, nil
}

func summarizeRules(rr *rulesync.Report) RulesSummary {
	sum := RulesSummary{
		Total:      len(rr.Rules),
		Pending:    rr.Pending,
		Conflicts:  rr.Conflicts,
		Collisions: len(rr.Collisions),
		Unmapped:   nonNil(rr.Unmapped),
	}
	for _, r := range rr.Rules {
		if r.Rule.Enabled {
			sum.Enabled++
		}
	}
	return sum
}

// PlanSync fetches the remote and returns what a full sync would do (no changes are made).
func (a *API) PlanSync() (*orchestrator.SyncPlan, error) {
	return a.svc.PlanSync()
}

// SyncOutcome is the plan that was executed (when planned here) and its result.
type SyncOutcome struct {
	Plan   *orchestrator.SyncPlan   `json:"plan,omitempty"`
	Result *orchestrator.SyncResult `json:"result"`
}

// Sync runs a full sync (git pull/commit/push + rules). With an empty fingerprint it plans first and
// executes that plan; with a fingerprint from a previous PlanSync it fails if the plan changed since.
func (a *API) Sync(fingerprint string, resolutions []orchestrator.ConflictResolution) (*SyncOutcome, error) {
	var plan *orchestrator.SyncPlan
	if fingerprint == "" {
		p, err := a.svc.PlanSync()
		if err != nil {
			return nil, err
		}
		plan = p
		fingerprint = p.Fingerprint
	}
	if resolutions == nil {
		resolutions = []orchestrator.ConflictResolution{}
	}
	res, err := a.svc.ExecuteSync(fingerprint, resolutions)
	if err != nil {
		return nil, err
	}
	return &SyncOutcome{Plan: plan, Result: res}, nil
}

// SyncRules applies pending rule changes only (no git operations).
func (a *API) SyncRules() (*rulesync.ApplyResult, error) {
	return a.svc.SyncRules()
}

// SetRoot sets the repository root folder and returns the new status.
func (a *API) SetRoot(rootPath string) (*StatusReport, error) {
	if _, err := a.svc.SetRootPath(strings.TrimSpace(rootPath)); err != nil {
		return nil, err
	}
	return a.Status()
}

// ---- Rules ----

// DestinationInput describes one destination. Empty Path/Mode/Transform are filled from the
// placement AgentSync suggests for the source in that location.
type DestinationInput struct {
	Location  string           `json:"location" jsonschema:"location alias the destination is relative to, e.g. home, claude, codex (see list_locations)"`
	Path      string           `json:"path,omitempty" jsonschema:"path relative to the location including the final name, e.g. .claude/CLAUDE.md or skills/my-skill; omit to use AgentSync's suggestion"`
	Mode      string           `json:"mode,omitempty" jsonschema:"two_way (default), one_way (repo always wins) or create_only (written once, never updated)"`
	Transform *rules.Transform `json:"transform,omitempty" jsonschema:"merge into an existing config file instead of replacing it: type json_merge or toml_merge, key e.g. mcpServers"`
	Disabled  bool             `json:"disabled,omitempty" jsonschema:"create the destination switched off"`
	Ignore    []string         `json:"ignore,omitempty" jsonschema:"glob patterns ignored in this destination"`
}

// RuleInput describes a new rule.
type RuleInput struct {
	Name         string             `json:"name,omitempty" jsonschema:"display name; defaults to the source file or folder name"`
	Scope        string             `json:"scope,omitempty" jsonschema:"repo (default: stored in the repository and shared with your other PCs) or local (this PC only; default when origin is local)"`
	Origin       string             `json:"origin,omitempty" jsonschema:"where the source lives: repo (default; path relative to the repository root), local (absolute path on this PC) or subscription"`
	Subscription string             `json:"subscription,omitempty" jsonschema:"subscription id when origin is subscription"`
	SourcePath   string             `json:"sourcePath" jsonschema:"source file or folder (relative to the repository or subscription root, absolute for origin local)"`
	Kind         string             `json:"kind,omitempty" jsonschema:"auto (default), file, folder, skill, mcp or instructions"`
	Exclude      []string           `json:"exclude,omitempty" jsonschema:"glob patterns excluded from a folder source"`
	Disabled     bool               `json:"disabled,omitempty" jsonschema:"create the rule switched off"`
	Destinations []DestinationInput `json:"destinations" jsonschema:"one or more destinations"`
}

func (in RuleInput) source() rules.Source {
	origin := in.Origin
	if origin == "" {
		origin = rules.OriginRepo
	}
	return rules.Source{Origin: origin, Subscription: in.Subscription, Path: strings.TrimSpace(in.SourcePath)}
}

func (in RuleInput) scope() string {
	if in.Scope != "" {
		return in.Scope
	}
	if in.Origin == rules.OriginLocal {
		return rules.ScopeLocal
	}
	return rules.ScopeRepo
}

func defaultRuleName(src rules.Source) string {
	clean := strings.TrimRight(filepath.ToSlash(src.Path), "/")
	if base := path.Base(clean); base != "." && base != "/" && base != "" {
		return base
	}
	return src.Path
}

// buildDestination turns an input into a stored destination, filling gaps from the suggestion.
func (a *API) buildDestination(src rules.Source, in DestinationInput) (rules.Destination, error) {
	dest := rules.Destination{
		Location:  strings.TrimSpace(in.Location),
		Path:      strings.TrimSpace(in.Path),
		Mode:      in.Mode,
		Transform: in.Transform,
		Enabled:   !in.Disabled,
		Ignore:    in.Ignore,
	}
	if dest.Location == "" {
		return rules.Destination{}, apperr.New(apperr.InvalidArgument).WithParam("field", "location")
	}
	if dest.Path != "" && dest.Mode != "" {
		return dest, nil
	}
	suggestion, err := a.svc.SuggestPlacement(src, dest.Location)
	if err != nil {
		if dest.Path == "" {
			return rules.Destination{}, err
		}
		return dest, nil
	}
	if dest.Path == "" {
		dest.Location = suggestion.Location
		dest.Path = suggestion.Path
		if dest.Transform == nil {
			dest.Transform = suggestion.Transform
		}
	}
	if dest.Mode == "" {
		dest.Mode = suggestion.Mode
	}
	return dest, nil
}

// AddRule validates and stores a new rule. Destinations without a path get AgentSync's suggestion.
func (a *API) AddRule(in RuleInput) (*service.SaveRuleResult, error) {
	src := in.source()
	if src.Path == "" {
		return nil, apperr.New(apperr.InvalidArgument).WithParam("field", "sourcePath")
	}
	if len(in.Destinations) == 0 {
		return nil, apperr.New(apperr.InvalidArgument).WithParam("field", "destinations")
	}
	rule := rules.Rule{
		Name:    strings.TrimSpace(in.Name),
		Enabled: !in.Disabled,
		Source:  src,
		Kind:    in.Kind,
		Exclude: in.Exclude,
	}
	if rule.Name == "" {
		rule.Name = defaultRuleName(src)
	}
	for _, d := range in.Destinations {
		dest, err := a.buildDestination(src, d)
		if err != nil {
			return nil, err
		}
		rule.Destinations = append(rule.Destinations, dest)
	}
	return a.svc.SaveRule(rule, in.scope())
}

// ListRules returns every rule with per-destination status, collisions and unmapped aliases.
func (a *API) ListRules() (*rulesync.Report, error) {
	return a.svc.GetRulesReport()
}

// GetRule returns one rule with its status.
func (a *API) GetRule(id string) (*rulesync.RuleStatus, error) {
	rr, err := a.svc.GetRulesReport()
	if err != nil {
		return nil, err
	}
	for i := range rr.Rules {
		if rr.Rules[i].Rule.ID == id {
			return &rr.Rules[i], nil
		}
	}
	return nil, apperr.New(apperr.RuleNotFound).WithParam("rule", id)
}

// RemoveRule deletes a rule. Files already written to destinations stay in place.
func (a *API) RemoveRule(id string) error {
	return a.svc.DeleteRule(id)
}

// SetRuleEnabled switches a rule on or off.
func (a *API) SetRuleEnabled(id string, enabled bool) (*service.SaveRuleResult, error) {
	return a.svc.UpdateRule(id, func(r *rules.Rule) error {
		r.Enabled = enabled
		return nil
	})
}

// AddDestination appends a destination to an existing rule.
func (a *API) AddDestination(ruleID string, in DestinationInput) (*service.SaveRuleResult, error) {
	current, err := a.GetRule(ruleID)
	if err != nil {
		return nil, err
	}
	dest, err := a.buildDestination(current.Rule.Source, in)
	if err != nil {
		return nil, err
	}
	return a.svc.UpdateRule(ruleID, func(r *rules.Rule) error {
		r.Destinations = append(r.Destinations, dest)
		return nil
	})
}

// RemoveDestination deletes one destination of a rule. Files already written stay in place.
func (a *API) RemoveDestination(ruleID, destID string) (*service.SaveRuleResult, error) {
	return a.svc.UpdateRule(ruleID, func(r *rules.Rule) error {
		for i := range r.Destinations {
			if r.Destinations[i].ID == destID {
				r.Destinations = append(r.Destinations[:i], r.Destinations[i+1:]...)
				return nil
			}
		}
		return apperr.New(apperr.DestinationNotFound).WithParam("destination", destID)
	})
}

// RuleDiff compares source and destination content of one rule file.
func (a *API) RuleDiff(ruleID, destID, relPath string) (*rulesync.Diff, error) {
	return a.svc.GetRuleDiff(ruleID, destID, relPath)
}

// ResolveRuleFile applies a decision for a conflicting or drifted rule file.
func (a *API) ResolveRuleFile(res rulesync.Resolution) error {
	return a.svc.ResolveRuleFile(res)
}

// ---- Locations and detection ----

// ListLocations returns the location aliases known on this PC (plus builtin home).
func (a *API) ListLocations() ([]service.LocationInfo, error) {
	return a.svc.ListLocations()
}

// SetLocation maps an alias to a folder on this PC and returns the updated list.
func (a *API) SetLocation(alias, folder string) ([]service.LocationInfo, error) {
	if _, err := a.svc.SaveLocation(strings.TrimSpace(alias), strings.TrimSpace(folder)); err != nil {
		return nil, err
	}
	return a.svc.ListLocations()
}

// RemoveLocation removes an alias mapping and returns the updated list.
func (a *API) RemoveLocation(alias string) ([]service.LocationInfo, error) {
	if _, err := a.svc.RemoveLocation(strings.TrimSpace(alias)); err != nil {
		return nil, err
	}
	return a.svc.ListLocations()
}

func withDefaultOrigin(src rules.Source) rules.Source {
	if src.Origin == "" {
		src.Origin = rules.OriginRepo
	}
	return src
}

// DetectSource reports the content kind of a source (skill, mcp, instructions, file, folder).
func (a *API) DetectSource(src rules.Source) (*recognize.SourceInfo, error) {
	return a.svc.DetectSource(withDefaultOrigin(src))
}

// SuggestPlacement proposes the destination path, mode and transform for a source in a location.
func (a *API) SuggestPlacement(src rules.Source, alias string) (*recognize.Suggestion, error) {
	return a.svc.SuggestPlacement(withDefaultOrigin(src), strings.TrimSpace(alias))
}

// RepoTree lists repository files and folders with their detected kinds.
func (a *API) RepoTree() ([]service.SourceTreeEntry, error) {
	return a.svc.ListRepoTree()
}

// ---- Subscriptions ----

// ListSubscriptions returns subscriptions with their last update check.
func (a *API) ListSubscriptions() ([]service.SubscriptionInfo, error) {
	return a.svc.ListSubscriptions()
}

// AddSubscription clones a remote repository as a read-only rule source.
func (a *API) AddSubscription(url, name string) (*config.Subscription, error) {
	return a.svc.AddSubscription(url, name)
}

// CheckSubscription fetches and reports incoming changes without applying them.
func (a *API) CheckSubscription(id string) (*subscriptions.UpdateInfo, error) {
	return a.svc.CheckSubscription(id)
}

// UpdateSubscription pulls remote changes and applies the rules fed by the subscription.
func (a *API) UpdateSubscription(id string) (*service.SubscriptionUpdateResult, error) {
	return a.svc.UpdateSubscription(id)
}

// SetSubscriptionAutoUpdate toggles automatic updates during periodic checks.
func (a *API) SetSubscriptionAutoUpdate(id string, enabled bool) ([]service.SubscriptionInfo, error) {
	if _, err := a.svc.SetSubscriptionAutoUpdate(id, enabled); err != nil {
		return nil, err
	}
	return a.svc.ListSubscriptions()
}

// RemoveSubscription deletes a subscription and every rule fed by it. Files already copied stay.
func (a *API) RemoveSubscription(id string) error {
	return a.svc.RemoveSubscription(id)
}

// SubscriptionTree lists the files of a subscription with detected kinds.
func (a *API) SubscriptionTree(id string) ([]service.SourceTreeEntry, error) {
	return a.svc.GetSubscriptionTree(id)
}

// ---- Repository remote ----

// ConnectRemote attaches the repository root to a remote URL without losing local files.
func (a *API) ConnectRemote(url string) (*gitops.ConnectRemoteResult, error) {
	return a.svc.ConnectRemote(url)
}

// RepoChanges lists what differs between the repository, its remote and the destinations.
type RepoChanges struct {
	Direction      string                      `json:"direction"`
	Ahead          int                         `json:"ahead"`
	Behind         int                         `json:"behind"`
	HasRemote      bool                        `json:"hasRemote"`
	IsOffline      bool                        `json:"isOffline"`
	LocalChanges   []string                    `json:"localChanges"`
	OutsideChanges []string                    `json:"outsideChanges"`
	Downloads      []orchestrator.SyncItem     `json:"downloads"`
	Uploads        []orchestrator.SyncItem     `json:"uploads"`
	Conflicts      []orchestrator.ConflictItem `json:"conflicts"`
}

// ListRepoChanges reports repository changes. With fetch it contacts the remote first.
func (a *API) ListRepoChanges(fetch bool) (*RepoChanges, error) {
	var plan *orchestrator.SyncPlan
	var err error
	if fetch {
		plan, err = a.svc.PlanSync()
	} else {
		plan, err = a.svc.GetStatus()
	}
	if err != nil {
		return nil, err
	}
	return &RepoChanges{
		Direction:      plan.Direction,
		Ahead:          plan.Ahead,
		Behind:         plan.Behind,
		HasRemote:      plan.HasRemote,
		IsOffline:      plan.IsOffline,
		LocalChanges:   nonNil(plan.CommitPreview),
		OutsideChanges: nonNil(plan.DirtyOutside),
		Downloads:      nonNil(plan.Downloads),
		Uploads:        nonNil(plan.Uploads),
		Conflicts:      nonNil(plan.Conflicts),
	}, nil
}

// RepoFileDiff compares a repo file: "working" = last commit vs disk, "incoming" = last commit vs remote.
func (a *API) RepoFileDiff(relPath, mode string) (*rulesync.Diff, error) {
	if mode == "" {
		mode = DiffModeWorking
	}
	if mode != DiffModeWorking && mode != DiffModeIncoming {
		return nil, apperr.New(apperr.InvalidArgument).WithParam("field", "mode")
	}
	return a.svc.GetRepoFileDiff(relPath, mode)
}

// DiscardRepoFile reverts a local repository change to the committed version (backup first).
func (a *API) DiscardRepoFile(relPath string) error {
	return a.svc.DiscardRepoFile(relPath)
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
