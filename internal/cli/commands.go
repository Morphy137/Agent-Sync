package cli

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"

	"agentsync/internal/ops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
)

// command is one leaf of the command tree.
type command struct {
	path  string // e.g. "rules add"
	usage string // arguments synopsis
	desc  string // message key of the description
	run   func(e *env, args []string) (any, error)
	raw   func(e *env, args []string) error // commands that own stdout (mcp)
}

// stdinPath means "read the JSON input from stdin".
const stdinPath = "-"

// Toggle values accepted by "subscriptions auto-update".
const (
	toggleOn  = "on"
	toggleOff = "off"
)

// Message is the result of commands that only report success.
type Message struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

func okMessage(key string) (any, error) {
	return Message{OK: true, Message: key}, nil
}

var commands []command

func init() {
	commands = []command{
		{path: "status", desc: "cmd.status", run: cmdStatus},
		{path: "plan", desc: "cmd.plan", run: cmdPlan},
		{path: "sync", usage: "[--rules-only] [--fingerprint F] [--resolve PROFILE:RELPATH=keep_repo|keep_local]...", desc: "cmd.sync", run: cmdSync},
		{path: "root set", usage: "<folder>", desc: "cmd.root_set", run: cmdRootSet},

		{path: "rules list", desc: "cmd.rules_list", run: cmdRulesList},
		{path: "rules show", usage: "<ruleId>", desc: "cmd.rules_show", run: cmdRulesShow},
		{path: "rules add", usage: "--source PATH --to LOCATION[:PATH]... [--origin repo|local|subscription] [--subscription ID] [--name N] [--scope repo|local] [--kind K] [--mode M] [--transform TYPE:KEY] [--exclude GLOB]... [--disabled] | --input FILE|-", desc: "cmd.rules_add", run: cmdRulesAdd},
		{path: "rules remove", usage: "<ruleId> --yes", desc: "cmd.rules_remove", run: cmdRulesRemove},
		{path: "rules enable", usage: "<ruleId>", desc: "cmd.rules_enable", run: cmdRulesToggle(true)},
		{path: "rules disable", usage: "<ruleId>", desc: "cmd.rules_disable", run: cmdRulesToggle(false)},
		{path: "rules diff", usage: "<ruleId> <destId> [relPath]", desc: "cmd.rules_diff", run: cmdRulesDiff},
		{path: "rules resolve", usage: "<ruleId> <destId> <relPath> --choice use_source|use_target|keep_local|custom [--content-file FILE]", desc: "cmd.rules_resolve", run: cmdRulesResolve},

		{path: "destinations add", usage: "<ruleId> --location L [--path P] [--mode M] [--transform TYPE:KEY] [--ignore GLOB]... [--disabled]", desc: "cmd.dest_add", run: cmdDestAdd},
		{path: "destinations remove", usage: "<ruleId> <destId> --yes", desc: "cmd.dest_remove", run: cmdDestRemove},

		{path: "locations list", desc: "cmd.loc_list", run: cmdLocList},
		{path: "locations set", usage: "<alias> <folder>", desc: "cmd.loc_set", run: cmdLocSet},
		{path: "locations remove", usage: "<alias>", desc: "cmd.loc_remove", run: cmdLocRemove},

		{path: "detect", usage: "<path> [--origin O] [--subscription ID]", desc: "cmd.detect", run: cmdDetect},
		{path: "suggest", usage: "<path> --location L [--origin O] [--subscription ID]", desc: "cmd.suggest", run: cmdSuggest},

		{path: "subscriptions list", desc: "cmd.sub_list", run: cmdSubList},
		{path: "subscriptions add", usage: "<url> [--name N]", desc: "cmd.sub_add", run: cmdSubAdd},
		{path: "subscriptions check", usage: "<id>", desc: "cmd.sub_check", run: cmdSubCheck},
		{path: "subscriptions update", usage: "<id>", desc: "cmd.sub_update", run: cmdSubUpdate},
		{path: "subscriptions auto-update", usage: "<id> on|off", desc: "cmd.sub_auto", run: cmdSubAuto},
		{path: "subscriptions remove", usage: "<id> --yes", desc: "cmd.sub_remove", run: cmdSubRemove},
		{path: "subscriptions tree", usage: "<id>", desc: "cmd.sub_tree", run: cmdSubTree},

		{path: "repo tree", desc: "cmd.repo_tree", run: cmdRepoTree},
		{path: "repo changes", usage: "[--fetch]", desc: "cmd.repo_changes", run: cmdRepoChanges},
		{path: "repo diff", usage: "<path> [--mode working|incoming]", desc: "cmd.repo_diff", run: cmdRepoDiff},
		{path: "repo discard", usage: "<path> --yes", desc: "cmd.repo_discard", run: cmdRepoDiscard},
		{path: "repo connect", usage: "<url>", desc: "cmd.repo_connect", run: cmdRepoConnect},

		{path: "repo gitignore show", desc: "cmd.gitignore_show", run: cmdGitignoreShow},
		{path: "repo gitignore exclude", usage: "<path>", desc: "cmd.gitignore_exclude", run: cmdGitignoreExclude},
		{path: "repo gitignore add-recommended", desc: "cmd.gitignore_recommended", run: cmdGitignoreRecommended},
		{path: "repo gitignore save", usage: "--file FILE|- --yes", desc: "cmd.gitignore_save", run: cmdGitignoreSave},

		{path: "mcp", desc: "cmd.mcp", raw: runMCP},
		{path: "version", desc: "cmd.version", run: func(*env, []string) (any, error) { return map[string]string{"version": Version}, nil }},
	}
}

// findCommand matches the longest command path at the start of args.
func findCommand(args []string) (*command, []string) {
	var best *command
	bestLen := 0
	for i := range commands {
		words := strings.Fields(commands[i].path)
		if len(words) > len(args) || len(words) <= bestLen {
			continue
		}
		match := true
		for j, w := range words {
			if args[j] != w {
				match = false
				break
			}
		}
		if match {
			best, bestLen = &commands[i], len(words)
		}
	}
	if best == nil {
		return nil, nil
	}
	return best, args[bestLen:]
}

// ---- Status and sync ----

func cmdStatus(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("status"), args); err != nil {
		return nil, err
	}
	return e.API().Status()
}

func cmdPlan(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("plan"), args); err != nil {
		return nil, err
	}
	return e.API().PlanSync()
}

func cmdSync(e *env, args []string) (any, error) {
	fs := newFlags("sync")
	rulesOnly := fs.Bool("rules-only", false, "")
	fingerprint := fs.String("fingerprint", "", "")
	var resolves stringList
	fs.Var(&resolves, "resolve", "")
	if _, err := expectArgs(fs, args); err != nil {
		return nil, err
	}
	if *rulesOnly {
		return e.API().SyncRules()
	}
	resolutions, err := parseResolutions(resolves)
	if err != nil {
		return nil, err
	}
	return e.API().Sync(*fingerprint, resolutions)
}

// parseResolutions reads PROFILE:RELPATH=CHOICE values.
func parseResolutions(values []string) ([]orchestrator.ConflictResolution, error) {
	out := make([]orchestrator.ConflictResolution, 0, len(values))
	for _, v := range values {
		eq := strings.LastIndex(v, "=")
		colon := strings.Index(v, ":")
		if eq < 0 || colon < 0 || colon > eq {
			return nil, usagef("--resolve %q: expected PROFILE:RELPATH=keep_repo|keep_local", v)
		}
		out = append(out, orchestrator.ConflictResolution{Profile: v[:colon], RelPath: v[colon+1 : eq], Resolution: v[eq+1:]})
	}
	return out, nil
}

func cmdRootSet(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("root set"), args, "<folder>")
	if err != nil {
		return nil, err
	}
	return e.API().SetRoot(pos[0])
}

// ---- Rules ----

func cmdRulesList(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("rules list"), args); err != nil {
		return nil, err
	}
	return e.API().ListRules()
}

func cmdRulesShow(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("rules show"), args, "<ruleId>")
	if err != nil {
		return nil, err
	}
	return e.API().GetRule(pos[0])
}

// parseTransform reads TYPE:KEY (e.g. json_merge:mcpServers).
func parseTransform(v string) (*rules.Transform, error) {
	if v == "" {
		return nil, nil
	}
	typ, key, ok := strings.Cut(v, ":")
	if !ok || typ == "" || key == "" {
		return nil, usagef("--transform %q: expected TYPE:KEY, e.g. json_merge:mcpServers", v)
	}
	return &rules.Transform{Type: typ, Key: key}, nil
}

// parseTarget reads LOCATION[:PATH].
func parseTarget(v string) ops.DestinationInput {
	loc, p, _ := strings.Cut(v, ":")
	return ops.DestinationInput{Location: loc, Path: p}
}

func readInput(path string, stdin io.Reader, out any) error {
	var data []byte
	var err error
	if path == stdinPath {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return usagef("--input: %v", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return usagef("--input: invalid JSON: %v", err)
	}
	return nil
}

// stdin is replaceable in tests.
var stdin io.Reader = os.Stdin

func cmdRulesAdd(e *env, args []string) (any, error) {
	fs := newFlags("rules add")
	input := fs.String("input", "", "")
	source := fs.String("source", "", "")
	origin := fs.String("origin", "", "")
	subscription := fs.String("subscription", "", "")
	name := fs.String("name", "", "")
	scope := fs.String("scope", "", "")
	kind := fs.String("kind", "", "")
	mode := fs.String("mode", "", "")
	transform := fs.String("transform", "", "")
	disabled := fs.Bool("disabled", false, "")
	var targets, excludes stringList
	fs.Var(&targets, "to", "")
	fs.Var(&excludes, "exclude", "")
	if _, err := expectArgs(fs, args); err != nil {
		return nil, err
	}

	var in ops.RuleInput
	if *input != "" {
		if err := readInput(*input, stdin, &in); err != nil {
			return nil, err
		}
		return e.API().AddRule(in)
	}
	if *source == "" || len(targets) == 0 {
		return nil, usagef("rules add: --source and at least one --to are required (or --input)")
	}
	tf, err := parseTransform(*transform)
	if err != nil {
		return nil, err
	}
	in = ops.RuleInput{
		Name: *name, Scope: *scope, Origin: *origin, Subscription: *subscription,
		SourcePath: *source, Kind: *kind, Exclude: excludes, Disabled: *disabled,
	}
	for _, t := range targets {
		d := parseTarget(t)
		d.Mode = *mode
		d.Transform = tf
		in.Destinations = append(in.Destinations, d)
	}
	return e.API().AddRule(in)
}

func cmdRulesRemove(e *env, args []string) (any, error) {
	fs := newFlags("rules remove")
	yes := fs.Bool("yes", false, "")
	pos, err := expectArgs(fs, args, "<ruleId>")
	if err != nil {
		return nil, err
	}
	if err := requireYes("rules remove", *yes); err != nil {
		return nil, err
	}
	if err := e.API().RemoveRule(pos[0]); err != nil {
		return nil, err
	}
	return okMessage("msg.rule_removed")
}

func cmdRulesToggle(enabled bool) func(e *env, args []string) (any, error) {
	return func(e *env, args []string) (any, error) {
		pos, err := expectArgs(newFlags("rules enable/disable"), args, "<ruleId>")
		if err != nil {
			return nil, err
		}
		return e.API().SetRuleEnabled(pos[0], enabled)
	}
}

func cmdRulesDiff(e *env, args []string) (any, error) {
	pos, err := parseArgs(newFlags("rules diff"), args)
	if err != nil {
		return nil, err
	}
	const minArgs, maxArgs = 2, 3
	if len(pos) < minArgs || len(pos) > maxArgs {
		return nil, usagef("rules diff: expected <ruleId> <destId> [relPath]")
	}
	rel := ""
	if len(pos) == maxArgs {
		rel = pos[2]
	}
	return e.API().RuleDiff(pos[0], pos[1], rel)
}

func cmdRulesResolve(e *env, args []string) (any, error) {
	fs := newFlags("rules resolve")
	choice := fs.String("choice", "", "")
	contentFile := fs.String("content-file", "", "")
	pos, err := expectArgs(fs, args, "<ruleId>", "<destId>", "<relPath>")
	if err != nil {
		return nil, err
	}
	if *choice == "" {
		return nil, usagef("rules resolve: --choice is required")
	}
	res := rulesync.Resolution{RuleID: pos[0], DestID: pos[1], RelPath: pos[2], Choice: *choice}
	if *contentFile != "" {
		data, err := os.ReadFile(*contentFile)
		if err != nil {
			return nil, usagef("--content-file: %v", err)
		}
		res.Content = string(data)
	}
	if err := e.API().ResolveRuleFile(res); err != nil {
		return nil, err
	}
	return okMessage("msg.file_resolved")
}

// ---- Destinations ----

func cmdDestAdd(e *env, args []string) (any, error) {
	fs := newFlags("destinations add")
	location := fs.String("location", "", "")
	p := fs.String("path", "", "")
	mode := fs.String("mode", "", "")
	transform := fs.String("transform", "", "")
	disabled := fs.Bool("disabled", false, "")
	var ignores stringList
	fs.Var(&ignores, "ignore", "")
	pos, err := expectArgs(fs, args, "<ruleId>")
	if err != nil {
		return nil, err
	}
	if *location == "" {
		return nil, usagef("destinations add: --location is required")
	}
	tf, err := parseTransform(*transform)
	if err != nil {
		return nil, err
	}
	in := ops.DestinationInput{Location: *location, Path: *p, Mode: *mode, Transform: tf, Disabled: *disabled, Ignore: ignores}
	return e.API().AddDestination(pos[0], in)
}

func cmdDestRemove(e *env, args []string) (any, error) {
	fs := newFlags("destinations remove")
	yes := fs.Bool("yes", false, "")
	pos, err := expectArgs(fs, args, "<ruleId>", "<destId>")
	if err != nil {
		return nil, err
	}
	if err := requireYes("destinations remove", *yes); err != nil {
		return nil, err
	}
	return e.API().RemoveDestination(pos[0], pos[1])
}

// ---- Locations and detection ----

func cmdLocList(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("locations list"), args); err != nil {
		return nil, err
	}
	return e.API().ListLocations()
}

func cmdLocSet(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("locations set"), args, "<alias>", "<folder>")
	if err != nil {
		return nil, err
	}
	return e.API().SetLocation(pos[0], pos[1])
}

func cmdLocRemove(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("locations remove"), args, "<alias>")
	if err != nil {
		return nil, err
	}
	return e.API().RemoveLocation(pos[0])
}

func sourceFlags(name string) (*sourceArgs, *flag.FlagSet) {
	fs := newFlags(name)
	sa := &sourceArgs{origin: fs.String("origin", "", ""), subscription: fs.String("subscription", "", "")}
	return sa, fs
}

type sourceArgs struct {
	origin, subscription *string
}

func (s *sourceArgs) source(p string) rules.Source {
	return rules.Source{Origin: *s.origin, Subscription: *s.subscription, Path: p}
}

func cmdDetect(e *env, args []string) (any, error) {
	sa, fs := sourceFlags("detect")
	pos, err := expectArgs(fs, args, "<path>")
	if err != nil {
		return nil, err
	}
	return e.API().DetectSource(sa.source(pos[0]))
}

func cmdSuggest(e *env, args []string) (any, error) {
	sa, fs := sourceFlags("suggest")
	location := fs.String("location", "", "")
	pos, err := expectArgs(fs, args, "<path>")
	if err != nil {
		return nil, err
	}
	if *location == "" {
		return nil, usagef("suggest: --location is required")
	}
	return e.API().SuggestPlacement(sa.source(pos[0]), *location)
}

// ---- Subscriptions ----

func cmdSubList(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("subscriptions list"), args); err != nil {
		return nil, err
	}
	return e.API().ListSubscriptions()
}

func cmdSubAdd(e *env, args []string) (any, error) {
	fs := newFlags("subscriptions add")
	name := fs.String("name", "", "")
	pos, err := expectArgs(fs, args, "<url>")
	if err != nil {
		return nil, err
	}
	return e.API().AddSubscription(pos[0], *name)
}

func cmdSubCheck(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("subscriptions check"), args, "<id>")
	if err != nil {
		return nil, err
	}
	return e.API().CheckSubscription(pos[0])
}

func cmdSubUpdate(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("subscriptions update"), args, "<id>")
	if err != nil {
		return nil, err
	}
	return e.API().UpdateSubscription(pos[0])
}

func cmdSubAuto(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("subscriptions auto-update"), args, "<id>", "on|off")
	if err != nil {
		return nil, err
	}
	if pos[1] != toggleOn && pos[1] != toggleOff {
		return nil, usagef("subscriptions auto-update: expected on or off, got %q", pos[1])
	}
	return e.API().SetSubscriptionAutoUpdate(pos[0], pos[1] == toggleOn)
}

func cmdSubRemove(e *env, args []string) (any, error) {
	fs := newFlags("subscriptions remove")
	yes := fs.Bool("yes", false, "")
	pos, err := expectArgs(fs, args, "<id>")
	if err != nil {
		return nil, err
	}
	if err := requireYes("subscriptions remove", *yes); err != nil {
		return nil, err
	}
	if err := e.API().RemoveSubscription(pos[0]); err != nil {
		return nil, err
	}
	return okMessage("msg.subscription_removed")
}

func cmdSubTree(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("subscriptions tree"), args, "<id>")
	if err != nil {
		return nil, err
	}
	return e.API().SubscriptionTree(pos[0])
}

// ---- Repository ----

func cmdRepoTree(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("repo tree"), args); err != nil {
		return nil, err
	}
	return e.API().RepoTree()
}

func cmdRepoChanges(e *env, args []string) (any, error) {
	fs := newFlags("repo changes")
	fetch := fs.Bool("fetch", false, "")
	if _, err := expectArgs(fs, args); err != nil {
		return nil, err
	}
	return e.API().ListRepoChanges(*fetch)
}

func cmdRepoDiff(e *env, args []string) (any, error) {
	fs := newFlags("repo diff")
	mode := fs.String("mode", ops.DiffModeWorking, "")
	pos, err := expectArgs(fs, args, "<path>")
	if err != nil {
		return nil, err
	}
	return e.API().RepoFileDiff(pos[0], *mode)
}

func cmdRepoDiscard(e *env, args []string) (any, error) {
	fs := newFlags("repo discard")
	yes := fs.Bool("yes", false, "")
	pos, err := expectArgs(fs, args, "<path>")
	if err != nil {
		return nil, err
	}
	if err := requireYes("repo discard", *yes); err != nil {
		return nil, err
	}
	if err := e.API().DiscardRepoFile(pos[0]); err != nil {
		return nil, err
	}
	return okMessage("msg.change_discarded")
}

func cmdRepoConnect(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("repo connect"), args, "<url>")
	if err != nil {
		return nil, err
	}
	return e.API().ConnectRemote(pos[0])
}

// ---- .gitignore ----

func cmdGitignoreShow(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("repo gitignore show"), args); err != nil {
		return nil, err
	}
	return e.API().Gitignore()
}

func cmdGitignoreExclude(e *env, args []string) (any, error) {
	pos, err := expectArgs(newFlags("repo gitignore exclude"), args, "<path>")
	if err != nil {
		return nil, err
	}
	return e.API().ExcludeRepoPath(pos[0])
}

func cmdGitignoreRecommended(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("repo gitignore add-recommended"), args); err != nil {
		return nil, err
	}
	return e.API().AddRecommendedGitignore()
}

func cmdGitignoreSave(e *env, args []string) (any, error) {
	fs := newFlags("repo gitignore save")
	file := fs.String("file", "", "")
	yes := fs.Bool("yes", false, "")
	if _, err := expectArgs(fs, args); err != nil {
		return nil, err
	}
	if *file == "" {
		return nil, usagef("repo gitignore save: --file is required")
	}
	if err := requireYes("repo gitignore save", *yes); err != nil {
		return nil, err
	}
	var data []byte
	var err error
	if *file == stdinPath {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(*file)
	}
	if err != nil {
		return nil, usagef("--file: %v", err)
	}
	return e.API().SaveGitignore(string(data))
}
