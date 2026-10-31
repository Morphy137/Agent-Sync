// Package mcpserver exposes AgentSync operations as Model Context Protocol tools over stdio so AI
// agents (Claude Code, Codex, Gemini CLI, ...) can set up and run syncs on the user's behalf.
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/ops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/recognize"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/service"
	"agentsync/internal/subscriptions"
)

// ServerName is the implementation name reported to MCP clients.
const ServerName = "agentsync"

// Instructions tells the model how the tools fit together.
const Instructions = `AgentSync keeps agent configuration (AGENTS.md/CLAUDE.md instructions, skills, MCP configs, any file or folder) in one git repository and copies it to many places on this PC through "rules".
A rule = one source (file/folder in the repo, on this PC, or in a subscription) -> N destinations. Each destination is a location alias (a named folder on this PC such as home, claude, codex) plus a relative path with the final name.

Typical flow when the user asks to sync something:
1. status - check the repository root is set (set_root if not) and see pending work.
2. list_locations - see which aliases exist; set_location to map a missing one (e.g. claude -> C:\Users\me\.claude).
3. list_repo_tree / detect_source - find the source and what kind it is (skill, mcp, instructions...).
4. suggest_placement (optional) - see where AgentSync would put it; add_rule fills omitted destination paths with this suggestion.
5. add_rule, then sync_rules to write files now, or plan_sync + sync to also commit/push/pull the repository.
Destructive tools (delete_rule, remove_destination, remove_subscription, discard_repo_file) require confirm=true: ask the user first.
Errors look like "[code] {params}"; codes are stable identifiers (e.g. location_unmapped, rule_not_found).`

// Server wraps an MCP server bound to an ops.API.
type Server struct {
	api *ops.API
	mcp *mcp.Server
}

// New builds the MCP server with every AgentSync tool registered.
func New(api *ops.API, version string) *Server {
	impl := &mcp.Implementation{Name: ServerName, Title: "AgentSync", Version: version}
	s := &Server{api: api, mcp: mcp.NewServer(impl, &mcp.ServerOptions{Instructions: Instructions})}
	s.registerTools()
	return s
}

// MCP returns the underlying SDK server (used by tests to connect in-memory transports).
func (s *Server) MCP() *mcp.Server {
	return s.mcp
}

// Run serves over stdin/stdout until the client disconnects.
func (s *Server) Run(ctx context.Context) error {
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

// ---- Shared input/output types ----

// NoInput is used by tools without arguments.
type NoInput struct{}

// Done is returned by tools that only report success.
type Done struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// SourceInput identifies a rule source.
type SourceInput struct {
	Origin       string `json:"origin,omitempty" jsonschema:"repo (default; path relative to the repository root), local (absolute path on this PC) or subscription"`
	Subscription string `json:"subscription,omitempty" jsonschema:"subscription id when origin is subscription"`
	Path         string `json:"path" jsonschema:"file or folder path"`
}

func (in SourceInput) source() rules.Source {
	return rules.Source{Origin: in.Origin, Subscription: in.Subscription, Path: in.Path}
}

// RuleIDInput selects a rule.
type RuleIDInput struct {
	RuleID string `json:"ruleId" jsonschema:"rule id (see list_rules)"`
}

// ConfirmedRuleIDInput selects a rule for a destructive action.
type ConfirmedRuleIDInput struct {
	RuleID  string `json:"ruleId" jsonschema:"rule id (see list_rules)"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true; ask the user before deleting"`
}

// SetRuleEnabledInput switches a rule on or off.
type SetRuleEnabledInput struct {
	RuleID  string `json:"ruleId" jsonschema:"rule id (see list_rules)"`
	Enabled bool   `json:"enabled" jsonschema:"true to enable, false to disable"`
}

// AddDestinationInput adds a destination to a rule.
type AddDestinationInput struct {
	RuleID      string               `json:"ruleId" jsonschema:"rule id (see list_rules)"`
	Destination ops.DestinationInput `json:"destination" jsonschema:"the destination to add"`
}

// RemoveDestinationInput removes a destination from a rule.
type RemoveDestinationInput struct {
	RuleID        string `json:"ruleId" jsonschema:"rule id (see list_rules)"`
	DestinationID string `json:"destinationId" jsonschema:"destination id (see get_rule)"`
	Confirm       bool   `json:"confirm,omitempty" jsonschema:"must be true; ask the user before removing"`
}

// RuleFileInput selects one file of a rule destination.
type RuleFileInput struct {
	RuleID        string `json:"ruleId" jsonschema:"rule id"`
	DestinationID string `json:"destinationId" jsonschema:"destination id"`
	RelPath       string `json:"relPath" jsonschema:"file path relative to the source/destination root as reported by get_rule (empty for a single-file rule)"`
}

// ResolveRuleFileInput decides a conflicting or drifted rule file.
type ResolveRuleFileInput struct {
	RuleID        string `json:"ruleId" jsonschema:"rule id"`
	DestinationID string `json:"destinationId" jsonschema:"destination id"`
	RelPath       string `json:"relPath" jsonschema:"file path as reported by get_rule"`
	Choice        string `json:"choice" jsonschema:"use_source (overwrite the destination), use_target (copy the destination back into the source), keep_local (stop syncing this file here) or custom (write content to both)"`
	Content       string `json:"content,omitempty" jsonschema:"merged content when choice is custom"`
}

// LocationInput maps an alias to a folder.
type LocationInput struct {
	Alias string `json:"alias" jsonschema:"lowercase alias (letters, digits, - and _), e.g. claude"`
	Path  string `json:"path" jsonschema:"absolute folder path on this PC, e.g. C:\\Users\\me\\.claude"`
}

// AliasInput selects a location alias.
type AliasInput struct {
	Alias string `json:"alias" jsonschema:"location alias"`
}

// SuggestInput asks where a source should go in a location.
type SuggestInput struct {
	Source   SourceInput `json:"source" jsonschema:"the source to place"`
	Location string      `json:"location" jsonschema:"location alias (see list_locations)"`
}

// PathInput carries a single path.
type PathInput struct {
	Path string `json:"path" jsonschema:"folder path"`
}

// SubscriptionAddInput adds a subscription.
type SubscriptionAddInput struct {
	URL  string `json:"url" jsonschema:"git URL (https or ssh) of the repository to follow read-only"`
	Name string `json:"name,omitempty" jsonschema:"display name; defaults to the repository name"`
}

// SubscriptionIDInput selects a subscription.
type SubscriptionIDInput struct {
	ID string `json:"id" jsonschema:"subscription id (see list_subscriptions)"`
}

// SubscriptionAutoUpdateInput toggles automatic updates.
type SubscriptionAutoUpdateInput struct {
	ID      string `json:"id" jsonschema:"subscription id"`
	Enabled bool   `json:"enabled" jsonschema:"true to pull and apply updates automatically during periodic checks"`
}

// ConfirmedSubscriptionInput removes a subscription.
type ConfirmedSubscriptionInput struct {
	ID      string `json:"id" jsonschema:"subscription id"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true; ask the user first (rules fed by it are deleted too)"`
}

// URLInput carries a remote URL.
type URLInput struct {
	URL string `json:"url" jsonschema:"git remote URL (https or ssh)"`
}

// RepoChangesInput lists repository changes.
type RepoChangesInput struct {
	Fetch bool `json:"fetch,omitempty" jsonschema:"contact the remote first to include incoming changes"`
}

// RepoFileDiffInput compares one repository file.
type RepoFileDiffInput struct {
	Path string `json:"path" jsonschema:"repository-relative path with forward slashes"`
	Mode string `json:"mode,omitempty" jsonschema:"working (default: last commit vs disk) or incoming (last commit vs remote)"`
}

// DiscardRepoFileInput reverts one repository file.
type DiscardRepoFileInput struct {
	Path    string `json:"path" jsonschema:"repository-relative path with forward slashes"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true; ask the user first (a backup is taken)"`
}

// SyncInput executes a full sync.
type SyncInput struct {
	Fingerprint string                            `json:"fingerprint,omitempty" jsonschema:"fingerprint from plan_sync; the sync aborts with plan_changed if things changed since. Omit to plan and run in one step"`
	Resolutions []orchestrator.ConflictResolution `json:"resolutions,omitempty" jsonschema:"decisions for plan conflicts: {profile, relPath, resolution: keep_repo or keep_local}"`
}

// RulesOutput lists rules with status.
type RulesOutput struct {
	Report *rulesync.Report `json:"report"`
}

// LocationsOutput lists locations.
type LocationsOutput struct {
	Locations []service.LocationInfo `json:"locations"`
}

// TreeOutput lists source entries.
type TreeOutput struct {
	Entries []service.SourceTreeEntry `json:"entries"`
}

// SubscriptionsOutput lists subscriptions.
type SubscriptionsOutput struct {
	Subscriptions []service.SubscriptionInfo `json:"subscriptions"`
}

func requireConfirm(confirm bool) error {
	if confirm {
		return nil
	}
	return apperr.New(apperr.ConfirmationRequired).WithParam("hint", "ask the user, then call again with confirm=true")
}

func done(message string) Done {
	return Done{OK: true, Message: message}
}

func boolPtr(v bool) *bool { return &v }

var (
	readOnly    = &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)}
	readNetwork = &mcp.ToolAnnotations{ReadOnlyHint: true}
	additive    = &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)}
	idempotent  = &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)}
	writesFiles = &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)}
	network     = &mcp.ToolAnnotations{DestructiveHint: boolPtr(true)}
	destructive = &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), OpenWorldHint: boolPtr(false)}
)

// handler adapts a plain function to the SDK's typed tool handler signature.
func handler[In, Out any](fn func(In) (Out, error)) mcp.ToolHandlerFor[In, Out] {
	return func(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		out, err := fn(in)
		return nil, out, err
	}
}

func deref[T any](v *T, err error) (T, error) {
	var zero T
	if err != nil || v == nil {
		return zero, err
	}
	return *v, nil
}

func (s *Server) registerTools() {
	s.registerStatusTools()
	s.registerRuleTools()
	s.registerLocationTools()
	s.registerSubscriptionTools()
	s.registerRepoTools()
}

func (s *Server) registerStatusTools() {
	a := s.api
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "status",
		Description: "Overview of AgentSync on this PC: repository root, git state, pending sync work, rule counts (pending, conflicts, collisions, unmapped location aliases). No network access. Call this first.",
		Annotations: readOnly,
	}, handler(func(NoInput) (ops.StatusReport, error) { return deref(a.Status()) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "set_root",
		Description: "Set the repository root: the folder (normally a git repository) that holds the source files and agentsync.rules.json.",
		Annotations: idempotent,
	}, handler(func(in PathInput) (ops.StatusReport, error) { return deref(a.SetRoot(in.Path)) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "plan_sync",
		Description: "Fetch the remote and compute what a full sync would do (downloads, uploads, conflicts, commit preview). Makes no changes. Returns a fingerprint to pass to sync.",
		Annotations: readNetwork,
	}, handler(func(NoInput) (orchestrator.SyncPlan, error) { return deref(a.PlanSync()) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "sync",
		Description: "Run a full sync: pull from the remote, apply rules to every destination (with backups), commit and push repository changes. Pass the fingerprint from plan_sync so nothing unexpected runs; conflicts need resolutions.",
		Annotations: network,
	}, handler(func(in SyncInput) (ops.SyncOutcome, error) { return deref(a.Sync(in.Fingerprint, in.Resolutions)) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "sync_rules",
		Description: "Apply pending rule changes now (write destinations, capture edits made in destinations back into the source, delete removed files) with backups. No git commit/push; conflicts are skipped and reported by get_rule.",
		Annotations: writesFiles,
	}, handler(func(NoInput) (rulesync.ApplyResult, error) { return deref(a.SyncRules()) }))
}

func (s *Server) registerRuleTools() {
	a := s.api
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_rules",
		Description: "List every sync rule with per-destination state (ready, location_unmapped, blocked_collision, disabled, source_missing), per-file status, collisions and unmapped aliases.",
		Annotations: readOnly,
	}, handler(func(NoInput) (RulesOutput, error) {
		rr, err := a.ListRules()
		return RulesOutput{Report: rr}, err
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_rule",
		Description: "Get one rule with its destinations (ids, locations, paths, modes) and file statuses.",
		Annotations: readOnly,
	}, handler(func(in RuleIDInput) (rulesync.RuleStatus, error) { return deref(a.GetRule(in.RuleID)) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name: "add_rule",
		Description: "Create a sync rule: one source copied to one or more destinations. Destinations without a path get AgentSync's suggested placement for that location " +
			"(e.g. a skill folder into skills/<name>, an MCP config merged into the agent's config file). Files are not written until sync_rules or sync. Returns the stored rule and any collisions.",
		Annotations: additive,
	}, handler(func(in ops.RuleInput) (service.SaveRuleResult, error) { return deref(a.AddRule(in)) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "delete_rule",
		Description: "Delete a rule. Files already written to its destinations stay in place. Requires confirm=true.",
		Annotations: destructive,
	}, handler(func(in ConfirmedRuleIDInput) (Done, error) {
		if err := requireConfirm(in.Confirm); err != nil {
			return Done{}, err
		}
		if err := a.RemoveRule(in.RuleID); err != nil {
			return Done{}, err
		}
		return done("rule deleted"), nil
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "set_rule_enabled",
		Description: "Enable or disable a rule without deleting it.",
		Annotations: idempotent,
	}, handler(func(in SetRuleEnabledInput) (service.SaveRuleResult, error) {
		return deref(a.SetRuleEnabled(in.RuleID, in.Enabled))
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "add_destination",
		Description: "Add a destination to an existing rule. Omit path to use AgentSync's suggested placement for that location.",
		Annotations: additive,
	}, handler(func(in AddDestinationInput) (service.SaveRuleResult, error) {
		return deref(a.AddDestination(in.RuleID, in.Destination))
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "remove_destination",
		Description: "Remove one destination from a rule. Files already written there stay in place. Requires confirm=true.",
		Annotations: destructive,
	}, handler(func(in RemoveDestinationInput) (service.SaveRuleResult, error) {
		if err := requireConfirm(in.Confirm); err != nil {
			return service.SaveRuleResult{}, err
		}
		return deref(a.RemoveDestination(in.RuleID, in.DestinationID))
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "rule_diff",
		Description: "Show source vs destination content of one rule file (use before resolving a conflict).",
		Annotations: readOnly,
	}, handler(func(in RuleFileInput) (rulesync.Diff, error) {
		return deref(a.RuleDiff(in.RuleID, in.DestinationID, in.RelPath))
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "resolve_rule_file",
		Description: "Resolve a conflicting or drifted rule file. Overwritten files are backed up first.",
		Annotations: writesFiles,
	}, handler(func(in ResolveRuleFileInput) (Done, error) {
		res := rulesync.Resolution{RuleID: in.RuleID, DestID: in.DestinationID, RelPath: in.RelPath, Choice: in.Choice, Content: in.Content}
		if err := a.ResolveRuleFile(res); err != nil {
			return Done{}, err
		}
		return done("file resolved"), nil
	}))
}

func (s *Server) registerLocationTools() {
	a := s.api
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_locations",
		Description: "List location aliases on this PC (builtin home plus user aliases) with their folder, whether it exists, the agent detected there and how many rule destinations use it.",
		Annotations: readOnly,
	}, handler(func(NoInput) (LocationsOutput, error) {
		locs, err := a.ListLocations()
		return LocationsOutput{Locations: locs}, err
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "set_location",
		Description: "Map a location alias to a folder on this PC (creates or updates it). Rules reference aliases so they stay portable across PCs.",
		Annotations: idempotent,
	}, handler(func(in LocationInput) (LocationsOutput, error) {
		locs, err := a.SetLocation(in.Alias, in.Path)
		return LocationsOutput{Locations: locs}, err
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "remove_location",
		Description: "Remove a location alias mapping on this PC. Rules using it become location_unmapped until it is mapped again; no files are deleted.",
		Annotations: idempotent,
	}, handler(func(in AliasInput) (LocationsOutput, error) {
		locs, err := a.RemoveLocation(in.Alias)
		return LocationsOutput{Locations: locs}, err
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "detect_source",
		Description: "Detect what a source is: skill, mcp, instructions, plain file or folder.",
		Annotations: readOnly,
	}, handler(func(in SourceInput) (recognize.SourceInfo, error) { return deref(a.DetectSource(in.source())) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "suggest_placement",
		Description: "Suggest the destination path, mode and transform for a source inside a location, based on the agent folder detected there (falls back to a plain copy).",
		Annotations: readOnly,
	}, handler(func(in SuggestInput) (recognize.Suggestion, error) {
		return deref(a.SuggestPlacement(in.Source.source(), in.Location))
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_repo_tree",
		Description: "List files and folders in the repository root with their detected kind, to pick rule sources.",
		Annotations: readOnly,
	}, handler(func(NoInput) (TreeOutput, error) {
		entries, err := a.RepoTree()
		return TreeOutput{Entries: entries}, err
	}))
}

func (s *Server) registerSubscriptionTools() {
	a := s.api
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_subscriptions",
		Description: "List subscriptions (read-only clones of other repositories usable as rule sources) with their last update check.",
		Annotations: readOnly,
	}, handler(func(NoInput) (SubscriptionsOutput, error) {
		subs, err := a.ListSubscriptions()
		return SubscriptionsOutput{Subscriptions: subs}, err
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "add_subscription",
		Description: "Subscribe to a remote git repository (cloned read-only). Then use origin=subscription in add_rule to sync its files.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false)},
	}, handler(func(in SubscriptionAddInput) (config.Subscription, error) {
		return deref(a.AddSubscription(in.URL, in.Name))
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "check_subscription",
		Description: "Fetch a subscription and report incoming changes without applying them.",
		Annotations: readNetwork,
	}, handler(func(in SubscriptionIDInput) (subscriptions.UpdateInfo, error) {
		return deref(a.CheckSubscription(in.ID))
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "update_subscription",
		Description: "Pull a subscription's remote changes and apply the rules it feeds (with backups).",
		Annotations: network,
	}, handler(func(in SubscriptionIDInput) (service.SubscriptionUpdateResult, error) {
		return deref(a.UpdateSubscription(in.ID))
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "set_subscription_auto_update",
		Description: "Turn automatic updates of a subscription on or off.",
		Annotations: idempotent,
	}, handler(func(in SubscriptionAutoUpdateInput) (SubscriptionsOutput, error) {
		subs, err := a.SetSubscriptionAutoUpdate(in.ID, in.Enabled)
		return SubscriptionsOutput{Subscriptions: subs}, err
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "remove_subscription",
		Description: "Remove a subscription, its clone and every rule fed by it. Files already copied stay in place. Requires confirm=true.",
		Annotations: destructive,
	}, handler(func(in ConfirmedSubscriptionInput) (Done, error) {
		if err := requireConfirm(in.Confirm); err != nil {
			return Done{}, err
		}
		if err := a.RemoveSubscription(in.ID); err != nil {
			return Done{}, err
		}
		return done("subscription removed"), nil
	}))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_subscription_tree",
		Description: "List files and folders of a subscription with their detected kind, to pick rule sources.",
		Annotations: readOnly,
	}, handler(func(in SubscriptionIDInput) (TreeOutput, error) {
		entries, err := a.SubscriptionTree(in.ID)
		return TreeOutput{Entries: entries}, err
	}))
}

func (s *Server) registerRepoTools() {
	a := s.api
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "connect_remote",
		Description: "Connect the repository root to a remote git URL without losing local files (initializes git if needed). Local differences are left uncommitted for review.",
		Annotations: network,
	}, handler(func(in URLInput) (gitops.ConnectRemoteResult, error) { return deref(a.ConnectRemote(in.URL)) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_repo_changes",
		Description: "List repository changes: local edits to commit, edits outside managed paths, incoming/outgoing files, conflicts, ahead/behind counts.",
		Annotations: readNetwork,
	}, handler(func(in RepoChangesInput) (ops.RepoChanges, error) { return deref(a.ListRepoChanges(in.Fetch)) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "repo_file_diff",
		Description: "Show the diff of one repository file: working (last commit vs disk) or incoming (last commit vs remote).",
		Annotations: readOnly,
	}, handler(func(in RepoFileDiffInput) (rulesync.Diff, error) { return deref(a.RepoFileDiff(in.Path, in.Mode)) }))

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "discard_repo_file",
		Description: "Discard a local repository change, restoring the committed version (a backup is taken first). Requires confirm=true.",
		Annotations: destructive,
	}, handler(func(in DiscardRepoFileInput) (Done, error) {
		if err := requireConfirm(in.Confirm); err != nil {
			return Done{}, err
		}
		if err := a.DiscardRepoFile(in.Path); err != nil {
			return Done{}, err
		}
		return done("change discarded"), nil
	}))
}
