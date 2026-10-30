package rules_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/rules"
)

const testPerm = 0o644

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), testPerm); err != nil {
		t.Fatal(err)
	}
}

func assertReason(t *testing.T, err error, code, reason string) {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperr.Error, got %v", err)
	}
	if appErr.Code != code {
		t.Fatalf("expected code %s, got %s (%v)", code, appErr.Code, err)
	}
	if reason != "" && appErr.Params["reason"] != reason {
		t.Fatalf("expected reason %s, got %s", reason, appErr.Params["reason"])
	}
}

func sampleRule() rules.Rule {
	return rules.Rule{
		ID:      "r_1",
		Name:    "Skills",
		Enabled: true,
		Source:  rules.Source{Origin: rules.OriginRepo, Path: "skills"},
		Kind:    rules.KindAuto,
		Destinations: []rules.Destination{
			{ID: "d_1", Location: "claude", Path: "skills", Mode: rules.ModeTwoWay, Enabled: true},
		},
	}
}

func TestRepoRulesRoundTrip(t *testing.T) {
	repo := t.TempDir()

	f, err := rules.LoadRepoRules(repo)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if f.Version != rules.CurrentVersion || len(f.Rules) != 0 {
		t.Fatalf("expected empty file, got %+v", f)
	}

	r := sampleRule()
	r.Scope = rules.ScopeRepo
	r.Name = "Skills <&>"
	if err := rules.SaveRepoRules(repo, &rules.File{Rules: []rules.Rule{r}}); err != nil {
		t.Fatalf("save: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(repo, rules.RulesFileName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.HasSuffix(text, "}\n") {
		t.Fatalf("expected trailing newline, got %q", text[len(text)-3:])
	}
	if !strings.Contains(text, "\n  \"rules\": [") {
		t.Fatalf("expected 2-space indent:\n%s", text)
	}
	if strings.Contains(text, "\"scope\"") {
		t.Fatalf("scope must be stripped before saving:\n%s", text)
	}
	if !strings.Contains(text, "Skills <&>") {
		t.Fatalf("html characters must not be escaped:\n%s", text)
	}

	loaded, err := rules.LoadRepoRules(repo)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].Scope != rules.ScopeRepo {
		t.Fatalf("expected repo scope on load, got %+v", loaded.Rules)
	}
	loaded.Rules[0].Scope = ""
	r.Scope = ""
	if !reflect.DeepEqual(loaded.Rules[0], r) {
		t.Fatalf("round trip mismatch:\n%+v\n%+v", loaded.Rules[0], r)
	}
}

func TestLocalRulesUseConfigDir(t *testing.T) {
	cfgDir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, cfgDir)

	r := sampleRule()
	r.Source = rules.Source{Origin: rules.OriginLocal, Path: filepath.Join(cfgDir, "src")}
	if err := rules.SaveLocalRules(&rules.File{Version: rules.CurrentVersion, Rules: []rules.Rule{r}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(cfgDir, rules.LocalRulesFileName)); err != nil {
		t.Fatalf("expected local rules file: %v", err)
	}
	loaded, err := rules.LoadLocalRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].Scope != rules.ScopeLocal {
		t.Fatalf("expected local scope, got %+v", loaded.Rules)
	}
}

func TestLoadMalformedRules(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, rules.RulesFileName), "{not json")
	_, err := rules.LoadRepoRules(repo)
	assertReason(t, err, apperr.InvalidRule, rules.ReasonMalformedFile)

	writeFile(t, filepath.Join(repo, rules.RulesFileName), `{"version": 99, "rules": []}`)
	_, err = rules.LoadRepoRules(repo)
	assertReason(t, err, apperr.InvalidRule, rules.ReasonUnsupportedVersion)
}

func TestNewID(t *testing.T) {
	pattern := regexp.MustCompile(`^r_[0-9a-f]{10}$`)
	a, b := rules.NewID(rules.RuleIDPrefix), rules.NewID(rules.RuleIDPrefix)
	if !pattern.MatchString(a) || !pattern.MatchString(b) {
		t.Fatalf("unexpected id format: %s %s", a, b)
	}
	if a == b {
		t.Fatalf("ids must be unique")
	}
}

func TestValidate(t *testing.T) {
	absLocal := filepath.Join(t.TempDir(), "src")
	cases := []struct {
		name   string
		mutate func(r *rules.Rule)
		scope  string
		reason string
	}{
		{"valid", func(r *rules.Rule) {}, rules.ScopeRepo, ""},
		{"valid empty kind and mode", func(r *rules.Rule) { r.Kind = ""; r.Destinations[0].Mode = "" }, rules.ScopeRepo, ""},
		{"valid dest path with inner dots", func(r *rules.Rule) { r.Destinations[0].Path = "a/../b" }, rules.ScopeRepo, ""},
		{"valid empty dest path", func(r *rules.Rule) { r.Destinations[0].Path = "" }, rules.ScopeRepo, ""},
		{"missing id", func(r *rules.Rule) { r.ID = " " }, rules.ScopeRepo, rules.ReasonMissingID},
		{"missing name", func(r *rules.Rule) { r.Name = "" }, rules.ScopeRepo, rules.ReasonMissingName},
		{"bad scope", func(r *rules.Rule) {}, "nope", rules.ReasonInvalidScope},
		{"bad origin", func(r *rules.Rule) { r.Source.Origin = "web" }, rules.ScopeRepo, rules.ReasonInvalidOrigin},
		{"repo path escapes", func(r *rules.Rule) { r.Source.Path = "a/../../x" }, rules.ScopeRepo, rules.ReasonInvalidSourcePath},
		{"repo path dotdot segment", func(r *rules.Rule) { r.Source.Path = "a/../b" }, rules.ScopeRepo, rules.ReasonInvalidSourcePath},
		{"repo path absolute", func(r *rules.Rule) { r.Source.Path = absLocal }, rules.ScopeRepo, rules.ReasonInvalidSourcePath},
		{"repo path root", func(r *rules.Rule) { r.Source.Path = "." }, rules.ScopeRepo, rules.ReasonInvalidSourcePath},
		{"local origin in repo", func(r *rules.Rule) { r.Source = rules.Source{Origin: rules.OriginLocal, Path: absLocal} }, rules.ScopeRepo, rules.ReasonLocalOriginInRepo},
		{"local origin relative", func(r *rules.Rule) { r.Source = rules.Source{Origin: rules.OriginLocal, Path: "rel"} }, rules.ScopeLocal, rules.ReasonInvalidSourcePath},
		{"local origin ok", func(r *rules.Rule) { r.Source = rules.Source{Origin: rules.OriginLocal, Path: absLocal} }, rules.ScopeLocal, ""},
		{"subscription without id", func(r *rules.Rule) { r.Source = rules.Source{Origin: rules.OriginSubscription, Path: "x"} }, rules.ScopeLocal, rules.ReasonMissingSubscription},
		{"bad kind", func(r *rules.Rule) { r.Kind = "zip" }, rules.ScopeRepo, rules.ReasonInvalidKind},
		{"bad exclude", func(r *rules.Rule) { r.Exclude = []string{"[a"} }, rules.ScopeRepo, rules.ReasonInvalidExclude},
		{"missing dest id", func(r *rules.Rule) { r.Destinations[0].ID = "" }, rules.ScopeRepo, rules.ReasonMissingDestinationID},
		{"duplicate dest id", func(r *rules.Rule) { r.Destinations = append(r.Destinations, r.Destinations[0]) }, rules.ScopeRepo, rules.ReasonDuplicateDestination},
		{"missing location", func(r *rules.Rule) { r.Destinations[0].Location = "" }, rules.ScopeRepo, rules.ReasonMissingLocation},
		{"dest escapes", func(r *rules.Rule) { r.Destinations[0].Path = "a/../../b" }, rules.ScopeRepo, rules.ReasonInvalidDestPath},
		{"dest absolute", func(r *rules.Rule) { r.Destinations[0].Path = absLocal }, rules.ScopeRepo, rules.ReasonInvalidDestPath},
		{"bad mode", func(r *rules.Rule) { r.Destinations[0].Mode = "sideways" }, rules.ScopeRepo, rules.ReasonInvalidMode},
		{"bad transform type", func(r *rules.Rule) { r.Destinations[0].Transform = &rules.Transform{Type: "yaml", Key: "k"} }, rules.ScopeRepo, rules.ReasonInvalidTransform},
		{"transform without key", func(r *rules.Rule) { r.Destinations[0].Transform = &rules.Transform{Type: rules.TransformJSONMerge} }, rules.ScopeRepo, rules.ReasonInvalidTransform},
		{"transform on folder", func(r *rules.Rule) {
			r.Kind = rules.KindFolder
			r.Destinations[0].Transform = &rules.Transform{Type: rules.TransformJSONMerge, Key: "mcpServers"}
		}, rules.ScopeRepo, rules.ReasonTransformNeedsFile},
		{"transform on file ok", func(r *rules.Rule) {
			r.Kind = rules.KindMCP
			r.Destinations[0].Transform = &rules.Transform{Type: rules.TransformTOMLMerge, Key: "mcp_servers"}
		}, rules.ScopeRepo, ""},
		{"bad ignore", func(r *rules.Rule) { r.Destinations[0].Ignore = []string{"../x"} }, rules.ScopeRepo, rules.ReasonInvalidIgnore},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := sampleRule()
			tc.mutate(&r)
			err := rules.Validate(r, tc.scope)
			if tc.reason == "" {
				if err != nil {
					t.Fatalf("expected valid, got %v", err)
				}
				return
			}
			assertReason(t, err, apperr.InvalidRule, tc.reason)
		})
	}
}

func TestEffectiveModeAndKind(t *testing.T) {
	d := rules.Destination{}
	if d.EffectiveMode() != rules.ModeTwoWay {
		t.Fatalf("empty mode should default to two_way")
	}
	d.Mode = rules.ModeCreateOnly
	d.Transform = &rules.Transform{Type: rules.TransformJSONMerge, Key: "k"}
	if d.EffectiveMode() != rules.ModeOneWay {
		t.Fatalf("transform should force one_way")
	}
	if (rules.Rule{}).EffectiveKind() != rules.KindAuto {
		t.Fatalf("empty kind should be auto")
	}
}

func TestDefaultHomeDirOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv(rules.HomeOverrideEnvVar, home)
	if got := rules.DefaultHomeDir(); got != home {
		t.Fatalf("expected %s, got %s", home, got)
	}
}
