package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/ops"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

type fixture struct {
	root string
	home string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base := t.TempDir()
	f := &fixture{root: filepath.Join(base, "repo"), home: filepath.Join(base, "home")}
	t.Setenv(config.ConfigEnvVar, filepath.Join(base, "config"))
	t.Setenv(rules.HomeOverrideEnvVar, f.home)
	t.Setenv(LangEnvVar, "")
	for _, dir := range []string{f.root, filepath.Join(f.home, ".claude"), filepath.Join(f.root, "skills", "demo")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite(t, filepath.Join(f.root, "AGENTS.md"), "# shared\n")
	mustWrite(t, filepath.Join(f.root, "skills", "demo", "SKILL.md"), "---\nname: demo\ndescription: demo\n---\n")
	return f
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// factory builds a fresh Service per run, like separate CLI invocations.
func factory() *ops.API {
	return ops.New(service.New(nil))
}

type result struct {
	code   int
	stdout string
	stderr string
}

func run(args ...string) result {
	var out, errOut bytes.Buffer
	code := Run(context.Background(), args, &out, &errOut, factory)
	return result{code: code, stdout: out.String(), stderr: errOut.String()}
}

// runJSON runs with --json and decodes the envelope; data is decoded into into when non-nil.
func runJSON(t *testing.T, into any, args ...string) (int, Envelope) {
	t.Helper()
	res := run(append(args, flagJSON)...)
	var env struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error *ErrorBody      `json:"error"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &env); err != nil {
		t.Fatalf("%v: invalid JSON %q: %v", args, res.stdout, err)
	}
	if into != nil && env.OK {
		if err := json.Unmarshal(env.Data, into); err != nil {
			t.Fatalf("%v: decode data: %v", args, err)
		}
	}
	return res.code, Envelope{OK: env.OK, Error: env.Error}
}

func mustJSON(t *testing.T, into any, args ...string) {
	t.Helper()
	code, env := runJSON(t, into, args...)
	if code != ExitOK || !env.OK {
		t.Fatalf("%v: exit %d, error %+v", args, code, env.Error)
	}
}

func TestHelpAndUsageErrors(t *testing.T) {
	newFixture(t)
	res := run("help")
	if res.code != ExitOK || !strings.Contains(res.stdout, "rules add") || !strings.Contains(res.stdout, "mcp") {
		t.Fatalf("help: %d %q", res.code, res.stdout)
	}
	res = run("--lang", "es", "help", "rules")
	if !strings.Contains(res.stdout, "Comandos:") || strings.Contains(res.stdout, "locations set") {
		t.Fatalf("spanish topic help: %q", res.stdout)
	}

	code, env := runJSON(t, nil, "frobnicate")
	if code != ExitUsage || env.Error == nil || env.Error.Code != "usage" {
		t.Fatalf("unknown command: %d %+v", code, env.Error)
	}
	code, env = runJSON(t, nil, "rules", "show")
	if code != ExitUsage || env.Error.Code != "usage" {
		t.Fatalf("missing arg: %d %+v", code, env.Error)
	}
	res = run("rules", "show")
	if res.code != ExitUsage || !strings.Contains(res.stderr, "help") {
		t.Fatalf("human usage error: %d %q", res.code, res.stderr)
	}
}

func TestStatusWithoutRoot(t *testing.T) {
	newFixture(t)
	var status ops.StatusReport
	mustJSON(t, &status, "status")
	if status.RootPath != "" || status.ConfigDir == "" {
		t.Fatalf("unexpected status: %+v", status)
	}
	res := run("status")
	if res.code != ExitOK || !strings.Contains(res.stdout, "root set") {
		t.Fatalf("human status should explain how to set the root: %q", res.stdout)
	}
}

func TestRuleWorkflowAcrossInvocations(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "set", f.root)
	var locs []service.LocationInfo
	mustJSON(t, &locs, "locations", "set", "claude", filepath.Join(f.home, ".claude"))
	if len(locs) < 2 {
		t.Fatalf("locations: %+v", locs)
	}

	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--source", "AGENTS.md", "--to", "claude:CLAUDE.md", "--name", "Shared")
	if saved.Rule.ID == "" || saved.Rule.Name != "Shared" || saved.Rule.Destinations[0].Path != "CLAUDE.md" {
		t.Fatalf("saved: %+v", saved.Rule)
	}
	ruleID := saved.Rule.ID

	mustJSON(t, nil, "sync", "--rules-only")
	data, err := os.ReadFile(filepath.Join(f.home, ".claude", "CLAUDE.md"))
	if err != nil || string(data) != "# shared\n" {
		t.Fatalf("destination: %q %v", data, err)
	}

	// Flags may follow positional arguments.
	mustJSON(t, &saved, "rules", "disable", ruleID)
	if saved.Rule.Enabled {
		t.Fatal("rule should be disabled")
	}

	var dest service.SaveRuleResult
	mustJSON(t, &dest, "destinations", "add", ruleID, "--location", "home", "--path", "notes/AGENTS.md", "--mode", rules.ModeOneWay)
	if len(dest.Rule.Destinations) != 2 || dest.Rule.Destinations[1].Mode != rules.ModeOneWay {
		t.Fatalf("destinations: %+v", dest.Rule.Destinations)
	}
	destID := dest.Rule.Destinations[1].ID
	code, env := runJSON(t, nil, "destinations", "remove", ruleID, destID)
	if code != ExitError || env.Error.Code != "confirmation_required" {
		t.Fatalf("remove without --yes: %d %+v", code, env.Error)
	}
	mustJSON(t, &dest, "destinations", "remove", ruleID, destID, "--yes")
	if len(dest.Rule.Destinations) != 1 {
		t.Fatalf("destination not removed: %+v", dest.Rule.Destinations)
	}

	code, env = runJSON(t, nil, "rules", "remove", ruleID)
	if code != ExitError || env.Error.Code != "confirmation_required" {
		t.Fatalf("rules remove without --yes: %d %+v", code, env.Error)
	}
	mustJSON(t, nil, "rules", "remove", "--yes", ruleID)

	var report struct {
		Rules []any `json:"rules"`
	}
	mustJSON(t, &report, "rules", "list")
	if len(report.Rules) != 0 {
		t.Fatalf("rules left: %+v", report.Rules)
	}
	code, env = runJSON(t, nil, "rules", "show", ruleID)
	if code != ExitError || env.Error.Code != "rule_not_found" {
		t.Fatalf("show deleted: %d %+v", code, env.Error)
	}
}

func TestRulesAddFromJSONInputAndSuggestion(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "set", f.root)

	var info struct {
		Kind string `json:"kind"`
	}
	mustJSON(t, &info, "detect", "skills/demo")
	if info.Kind != rules.KindSkill {
		t.Fatalf("detect: %+v", info)
	}
	var suggestion struct {
		Path string `json:"path"`
	}
	mustJSON(t, &suggestion, "suggest", "skills/demo", "--location", "home")
	if suggestion.Path == "" {
		t.Fatal("suggest returned no path")
	}

	input := `{"sourcePath":"skills/demo","destinations":[{"location":"home"}]}`
	stdin = strings.NewReader(input)
	t.Cleanup(func() { stdin = os.Stdin })
	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--input", "-")
	if got := saved.Rule.Destinations[0].Path; got != suggestion.Path {
		t.Fatalf("omitted path should use the suggestion %q, got %q", suggestion.Path, got)
	}
	if saved.Rule.Name != "demo" {
		t.Fatalf("default name: %q", saved.Rule.Name)
	}

	res := run("rules", "list")
	if res.code != ExitOK || !strings.Contains(res.stdout, saved.Rule.ID) {
		t.Fatalf("human rules list: %q", res.stdout)
	}
}

func TestSubscriptionAndRepoGuards(t *testing.T) {
	newFixture(t)
	code, env := runJSON(t, nil, "subscriptions", "remove", "s_missing")
	if code != ExitError || env.Error.Code != "confirmation_required" {
		t.Fatalf("subscriptions remove: %d %+v", code, env.Error)
	}
	code, env = runJSON(t, nil, "subscriptions", "remove", "s_missing", "--yes")
	if code != ExitError || env.Error.Code != "subscription_not_found" {
		t.Fatalf("subscriptions remove missing: %d %+v", code, env.Error)
	}
	code, env = runJSON(t, nil, "subscriptions", "auto-update", "s_missing", "maybe")
	if code != ExitUsage {
		t.Fatalf("auto-update invalid toggle: %d %+v", code, env.Error)
	}
	code, env = runJSON(t, nil, "repo", "discard", "AGENTS.md")
	if code != ExitError || env.Error.Code != "confirmation_required" {
		t.Fatalf("repo discard: %d %+v", code, env.Error)
	}
	code, env = runJSON(t, nil, "repo", "diff", "AGENTS.md", "--mode", "sideways")
	if code != ExitError || env.Error.Code != "invalid_argument" {
		t.Fatalf("repo diff bad mode: %d %+v", code, env.Error)
	}
}

func TestGitignoreCommands(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "set", f.root)

	var gi service.GitignorePayload
	mustJSON(t, &gi, "repo", "gitignore", "exclude", "cache")
	if !strings.Contains(gi.Content, "cache") {
		t.Fatalf("exclude: %q", gi.Content)
	}
	code, env := runJSON(t, nil, "repo", "gitignore", "save", "--file", "-")
	if code != ExitError || env.Error.Code != "confirmation_required" {
		t.Fatalf("save without --yes: %d %+v", code, env.Error)
	}
	stdin = strings.NewReader("replaced\n")
	t.Cleanup(func() { stdin = os.Stdin })
	mustJSON(t, &gi, "repo", "gitignore", "save", "--file", "-", "--yes")
	mustJSON(t, &gi, "repo", "gitignore", "show")
	if strings.TrimSpace(gi.Content) != "replaced" {
		t.Fatalf("show after save: %q", gi.Content)
	}
}

func TestParseHelpers(t *testing.T) {
	res, err := parseResolutions([]string{"CLAUDE:sub/a.md=keep_local"})
	if err != nil || len(res) != 1 || res[0].Profile != "CLAUDE" || res[0].RelPath != "sub/a.md" || res[0].Resolution != "keep_local" {
		t.Fatalf("parseResolutions: %+v %v", res, err)
	}
	if _, err := parseResolutions([]string{"nonsense"}); err == nil {
		t.Fatal("expected error for malformed --resolve")
	}
	tf, err := parseTransform("json_merge:mcpServers")
	if err != nil || tf.Type != rules.TransformJSONMerge || tf.Key != "mcpServers" {
		t.Fatalf("parseTransform: %+v %v", tf, err)
	}
	if _, err := parseTransform("json_merge"); err == nil {
		t.Fatal("expected error for transform without key")
	}
	if d := parseTarget("codex:prompts/a.md"); d.Location != "codex" || d.Path != "prompts/a.md" {
		t.Fatalf("parseTarget: %+v", d)
	}
	if d := parseTarget("home"); d.Location != "home" || d.Path != "" {
		t.Fatalf("parseTarget without path: %+v", d)
	}
	jsonOut, lang, help, rest := splitGlobalFlags([]string{"rules", "--json", "list", "--lang=es"})
	if !jsonOut || lang != LangES || help || strings.Join(rest, " ") != "rules list" {
		t.Fatalf("splitGlobalFlags: %v %q %v %v", jsonOut, lang, help, rest)
	}
}

func TestLanguageResolution(t *testing.T) {
	t.Setenv(LangEnvVar, "")
	for _, name := range systemLangEnvVars {
		t.Setenv(name, "")
	}
	if got := resolveLang("", ""); got != LangEN {
		t.Fatalf("default: %s", got)
	}
	if got := resolveLang("", "es"); got != LangES {
		t.Fatalf("configured: %s", got)
	}
	t.Setenv("LANG", "es_ES.UTF-8")
	if got := resolveLang("", "system"); got != LangES {
		t.Fatalf("system locale: %s", got)
	}
	if got := resolveLang("en", "es"); got != LangEN {
		t.Fatalf("flag wins: %s", got)
	}
}

func TestMessageCatalogParity(t *testing.T) {
	keys := func(lang string) []string {
		out := make([]string, 0, len(messages[lang]))
		for k := range messages[lang] {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	en, es := keys(LangEN), keys(LangES)
	if strings.Join(en, ",") != strings.Join(es, ",") {
		t.Fatalf("en/es keys differ:\nen=%v\nes=%v", en, es)
	}
	for _, c := range commands {
		for _, lang := range []string{LangEN, LangES} {
			if _, ok := messages[lang][c.desc]; !ok {
				t.Errorf("missing %s description %s for %q", lang, c.desc, c.path)
			}
		}
	}
}

func TestRulesAddWithFileSelection(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(filepath.Join(f.root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(f.root, "notes", "a.md"), "a")
	mustWrite(t, filepath.Join(f.root, "notes", "b.md"), "b")
	mustWrite(t, filepath.Join(f.root, "notes", "skip.md"), "s")
	mustJSON(t, nil, "root", "set", f.root)
	mustJSON(t, nil, "locations", "set", "claude", filepath.Join(f.home, ".claude"))

	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--source", "notes", "--include", "a.md", "--include", "b.md", "--to", "claude:picked")
	if got := saved.Rule.Source.Include; len(got) != 2 {
		t.Fatalf("include not stored: %+v", saved.Rule.Source)
	}
	mustJSON(t, nil, "sync", "--rules-only")
	for name, want := range map[string]bool{"a.md": true, "b.md": true, "skip.md": false} {
		_, err := os.Stat(filepath.Join(f.home, ".claude", "picked", name))
		if (err == nil) != want {
			t.Fatalf("%s present=%v, want %v", name, err == nil, want)
		}
	}
}
