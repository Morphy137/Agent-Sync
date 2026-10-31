package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

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

// tabwriter layout.
const (
	tabMinWidth = 0
	tabWidth    = 4
	tabPadding  = 2
	tabPadChar  = ' '
)

// printer writes localized human output.
type printer struct {
	w    io.Writer
	lang string
}

func (p printer) line(format string, args ...any) {
	fmt.Fprintf(p.w, format+"\n", args...)
}

func (p printer) field(key string, value any) {
	p.line("%s: %v", tr(p.lang, key), value)
}

func (p printer) list(key string, items []string) {
	if len(items) == 0 {
		return
	}
	p.line("%s (%d):", tr(p.lang, key), len(items))
	for _, it := range items {
		p.line("  %s", it)
	}
}

func (p printer) table(header []string, rows [][]string) {
	if len(rows) == 0 {
		p.line("%s", tr(p.lang, "label.none"))
		return
	}
	tw := tabwriter.NewWriter(p.w, tabMinWidth, tabWidth, tabPadding, tabPadChar, 0)
	fmt.Fprintln(tw, strings.Join(header, "\t"))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	_ = tw.Flush()
}

func (p printer) headers(keys ...string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = tr(p.lang, k)
	}
	return out
}

func (p printer) yesNo(v bool) string {
	if v {
		return tr(p.lang, "label.yes")
	}
	return tr(p.lang, "label.no")
}

// render prints a command result for humans; unknown types fall back to indented JSON.
func render(w io.Writer, lang string, result any) {
	p := printer{w: w, lang: lang}
	switch v := result.(type) {
	case Message:
		p.line("%s", tr(lang, v.Message))
	case map[string]string:
		for k, val := range v {
			p.line("%s %s", k, val)
		}
	case *ops.StatusReport:
		p.status(v)
	case *orchestrator.SyncPlan:
		p.plan(v)
	case *ops.SyncOutcome:
		if v.Plan != nil {
			p.plan(v.Plan)
		}
		p.syncResult(v.Result)
	case *rulesync.ApplyResult:
		p.applyResult(v)
	case *rulesync.Report:
		p.rulesReport(v)
	case *rulesync.RuleStatus:
		p.ruleStatus(v)
	case *service.SaveRuleResult:
		p.line("%s", tr(lang, "label.saved_rule", v.Rule.ID, v.Rule.Name, v.Rule.Scope))
		p.destinations(v.Rule.Destinations)
		p.collisions(v.Collisions)
	case []service.LocationInfo:
		p.locations(v)
	case *recognize.SourceInfo:
		p.field("label.kind", v.Kind)
		p.field("label.name", v.Name)
	case *recognize.Suggestion:
		p.suggestion(v)
	case []service.SourceTreeEntry:
		rows := make([][]string, 0, len(v))
		for _, e := range v {
			rows = append(rows, []string{e.Path, e.Kind})
		}
		p.table(p.headers("label.path", "label.kind"), rows)
	case []service.SubscriptionInfo:
		p.subscriptions(v)
	case *config.Subscription:
		p.line("%s  %s  %s", v.ID, v.Name, v.URL)
	case *subscriptions.UpdateInfo:
		p.field("label.behind", v.Behind)
		p.list("label.incoming", v.IncomingFiles)
	case *service.SubscriptionUpdateResult:
		p.list("label.changed", v.ChangedFiles)
		p.applyResult(&v.Applied)
	case *ops.RepoChanges:
		p.repoChanges(v)
	case *rulesync.Diff:
		p.diff(v)
	case *gitops.ConnectRemoteResult:
		p.field("label.branch", v.Branch)
		if v.BackupBranch != "" {
			p.field("label.backup_branch", v.BackupBranch)
		}
		changes := make([]string, 0, len(v.LocalChanges))
		for _, c := range v.LocalChanges {
			changes = append(changes, c.Status+" "+c.Path)
		}
		p.list("label.local_changes", changes)
	default:
		writeJSON(w, result)
	}
}

func (p printer) status(s *ops.StatusReport) {
	p.field("label.config_dir", s.ConfigDir)
	if s.RootPath == "" {
		p.field("label.root", tr(p.lang, "label.not_set"))
		return
	}
	p.field("label.root", s.RootPath)
	if s.Git != nil && s.Git.IsRepo {
		p.field("label.branch", s.Git.Branch)
		if s.Git.RemoteURL != "" {
			p.field("label.remote", s.Git.RemoteURL)
		}
		p.field("label.ahead_behind", fmt.Sprintf("%d/%d", s.Git.Ahead, s.Git.Behind))
	}
	if s.Plan != nil {
		p.field("label.direction", s.Plan.Direction)
	}
	r := s.Rules
	p.field("label.rules", tr(p.lang, "label.rules_counts", r.Total, r.Enabled, r.Pending, r.Conflicts, r.Collisions))
	if len(r.Unmapped) > 0 {
		p.field("label.unmapped", strings.Join(r.Unmapped, ", "))
	}
	p.field("label.locations", s.Locations)
	p.field("label.subscriptions", s.Subscriptions)
}

func syncItemPaths(items []orchestrator.SyncItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.FullPath)
	}
	return out
}

func conflictPaths(items []orchestrator.ConflictItem) []string {
	out := make([]string, 0, len(items))
	for _, c := range items {
		out = append(out, fmt.Sprintf("%s:%s (%s)", c.Profile, c.RelPath, c.Reason))
	}
	return out
}

func (p printer) plan(plan *orchestrator.SyncPlan) {
	direction := plan.Direction
	if plan.IsOffline {
		direction += " (" + tr(p.lang, "label.offline") + ")"
	}
	p.field("label.direction", direction)
	p.field("label.ahead_behind", fmt.Sprintf("%d/%d", plan.Ahead, plan.Behind))
	p.field("label.fingerprint", plan.Fingerprint)
	p.list("label.downloads", syncItemPaths(plan.Downloads))
	p.list("label.uploads", syncItemPaths(plan.Uploads))
	p.list("label.conflicts", conflictPaths(plan.Conflicts))
	p.list("label.commit", plan.CommitPreview)
	p.list("label.outside", plan.DirtyOutside)
	p.field("label.rules", tr(p.lang, "label.rules_counts", 0, 0, plan.RulesPending, plan.RulesConflicts, 0))
}

func (p printer) syncResult(r *orchestrator.SyncResult) {
	if r == nil {
		return
	}
	p.field("label.applied", r.Applied)
	p.field("label.captured", r.Captured)
	p.field("label.pulled", r.Pulled)
	p.field("label.pushed", r.Pushed)
	p.field("label.written", r.RulesWritten)
	p.field("label.skipped", r.RulesSkipped)
}

func (p printer) applyResult(r *rulesync.ApplyResult) {
	p.field("label.written", r.Written)
	p.field("label.captured", r.Captured)
	p.field("label.deleted", r.Deleted)
	p.field("label.skipped", r.Skipped)
	p.list("label.backups", r.Backups)
}

func (p printer) rulesReport(r *rulesync.Report) {
	rows := make([][]string, 0, len(r.Rules))
	for _, rs := range r.Rules {
		states := make([]string, 0, len(rs.Destinations))
		for _, d := range rs.Destinations {
			states = append(states, d.State)
		}
		rows = append(rows, []string{rs.Rule.ID, rs.Rule.Name, rs.Scope, p.yesNo(rs.Rule.Enabled), rs.Rule.Source.Path, strings.Join(states, ",")})
	}
	p.table(p.headers("label.id", "label.name", "label.scope", "label.enabled", "label.source", "label.destinations"), rows)
	p.collisions(r.Collisions)
	if len(r.Unmapped) > 0 {
		p.field("label.unmapped", strings.Join(r.Unmapped, ", "))
	}
}

func (p printer) ruleStatus(rs *rulesync.RuleStatus) {
	p.line("%s  %s  (%s, %s)", rs.Rule.ID, rs.Rule.Name, rs.Scope, rs.DetectedKind)
	p.field("label.source", rs.Rule.Source.Origin+" "+rs.Rule.Source.Path)
	p.destinations(rs.Rule.Destinations)
	for _, d := range rs.Destinations {
		for _, f := range d.Files {
			p.line("  %s  %s  %s", d.DestID, f.Status, f.TargetAbs)
		}
	}
}

func transformLabel(t *rules.Transform) string {
	if t == nil {
		return "-"
	}
	return t.Type + ":" + t.Key
}

func (p printer) destinations(dests []rules.Destination) {
	rows := make([][]string, 0, len(dests))
	for _, d := range dests {
		rows = append(rows, []string{d.ID, d.Location, d.Path, d.Mode, transformLabel(d.Transform), p.yesNo(d.Enabled)})
	}
	p.table(p.headers("label.id", "label.location", "label.path", "label.mode", "label.transform", "label.enabled"), rows)
}

func (p printer) collisions(cs []rules.Collision) {
	items := make([]string, 0, len(cs))
	for _, c := range cs {
		items = append(items, c.Type+": "+c.Path)
	}
	p.list("label.collisions", items)
}

func (p printer) locations(locs []service.LocationInfo) {
	rows := make([][]string, 0, len(locs))
	for _, l := range locs {
		rows = append(rows, []string{l.Alias, l.Path, p.yesNo(l.Exists), l.ProfileLabel, fmt.Sprint(l.RuleCount)})
	}
	p.table(p.headers("label.alias", "label.path", "label.exists", "label.detected", "label.rules"), rows)
}

func (p printer) suggestion(s *recognize.Suggestion) {
	label := s.Label
	if !s.Detected {
		label = tr(p.lang, "label.generic")
	}
	p.field("label.detected", label)
	p.field("label.location", s.Location)
	p.field("label.path", s.Path)
	p.field("label.mode", s.Mode)
	p.field("label.transform", transformLabel(s.Transform))
}

func (p printer) subscriptions(subs []service.SubscriptionInfo) {
	rows := make([][]string, 0, len(subs))
	for _, s := range subs {
		behind := "-"
		if s.Update != nil {
			behind = fmt.Sprint(s.Update.Behind)
		}
		rows = append(rows, []string{s.ID, s.Name, s.URL, p.yesNo(s.AutoUpdate), behind, fmt.Sprint(s.RuleCount)})
	}
	p.table(p.headers("label.id", "label.name", "label.url", "label.auto_update", "label.behind", "label.rules"), rows)
}

func (p printer) repoChanges(c *ops.RepoChanges) {
	p.field("label.direction", c.Direction)
	p.field("label.ahead_behind", fmt.Sprintf("%d/%d", c.Ahead, c.Behind))
	p.list("label.commit", c.LocalChanges)
	p.list("label.outside", c.OutsideChanges)
	p.list("label.downloads", syncItemPaths(c.Downloads))
	p.list("label.uploads", syncItemPaths(c.Uploads))
	p.list("label.conflicts", conflictPaths(c.Conflicts))
}

func (p printer) diff(d *rulesync.Diff) {
	if d.Binary {
		p.line("%s", tr(p.lang, "label.binary"))
		return
	}
	p.line("--- %s", d.LeftLabel)
	p.line("+++ %s", d.RightLabel)
	for _, l := range strings.Split(d.Left, "\n") {
		p.line("-%s", l)
	}
	for _, l := range strings.Split(d.Right, "\n") {
		p.line("+%s", l)
	}
}
