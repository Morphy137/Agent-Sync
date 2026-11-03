package service_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/service"
	"agentsync/internal/state"
)

// legacyFixture is a config dir written the way versions with folder connections left it.
type legacyFixture struct {
	t         *testing.T
	configDir string
	rootDir   string
	tempDir   string
}

type legacyConn struct {
	ID         string `json:"id"`
	Profile    string `json:"profile"`
	TargetPath string `json:"targetPath"`
}

func newLegacyFixture(t *testing.T) *legacyFixture {
	t.Helper()
	tempDir := t.TempDir()
	f := &legacyFixture{
		t:         t,
		tempDir:   tempDir,
		configDir: filepath.Join(tempDir, "config"),
		rootDir:   filepath.Join(tempDir, "repo"),
	}
	t.Setenv(config.ConfigEnvVar, f.configDir)
	if err := os.MkdirAll(f.configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.rootDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *legacyFixture) dir(parts ...string) string {
	f.t.Helper()
	path := filepath.Join(append([]string{f.tempDir}, parts...)...)
	if err := os.MkdirAll(path, 0o755); err != nil {
		f.t.Fatal(err)
	}
	return path
}

func (f *legacyFixture) profile(name string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Join(f.rootDir, name), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

func (f *legacyFixture) writeJSON(path string, v any) {
	f.t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *legacyFixture) writeConfig(rootPath string, conns []legacyConn, locations []config.Location) {
	f.t.Helper()
	f.writeJSON(filepath.Join(f.configDir, config.ConfigFileName), map[string]any{
		"version":     1,
		"rootPath":    rootPath,
		"connections": conns,
		"locations":   locations,
		"theme":       "dark",
		"futureField": "kept out of the way",
	})
}

func (f *legacyFixture) writeBaselines(baselines map[string]map[string]string) {
	f.t.Helper()
	f.writeJSON(filepath.Join(f.configDir, state.StateFileName), map[string]any{
		"version":   1,
		"baselines": baselines,
	})
}

func (f *legacyFixture) rawConfig() map[string]any {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.configDir, config.ConfigFileName))
	if err != nil {
		f.t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		f.t.Fatal(err)
	}
	return raw
}

func (f *legacyFixture) localRules() []rules.Rule {
	f.t.Helper()
	file, err := rules.LoadLocalRules()
	if err != nil {
		f.t.Fatal(err)
	}
	return file.Rules
}

func ruleForProfile(t *testing.T, all []rules.Rule, profile string) rules.Rule {
	t.Helper()
	for _, r := range all {
		if r.Source.Path == profile {
			return r
		}
	}
	t.Fatalf("no rule for profile %q in %+v", profile, all)
	return rules.Rule{}
}

func TestLegacyConnectionsBecomeLocalRules(t *testing.T) {
	f := newLegacyFixture(t)
	f.profile("CLAUDE")
	f.profile("CODEX")
	claudeTarget := f.dir("home", ".claude")
	codexTarget := f.dir("home", ".codex")
	f.writeConfig(f.rootDir,
		[]legacyConn{
			{ID: "c1", Profile: "CLAUDE", TargetPath: claudeTarget},
			{ID: "c2", Profile: "CODEX", TargetPath: codexTarget},
			{ID: "c3", Profile: "GONE", TargetPath: f.dir("home", ".gone")},
		},
		[]config.Location{{Alias: "codex-home", Path: codexTarget}},
	)
	f.writeBaselines(map[string]map[string]string{
		"c1":              {"CLAUDE.md": "hash-claude", "commands/review.md": "hash-review"},
		"c2":              {"AGENTS.md": "hash-codex"},
		"orphan":          {"x.md": "stale"},
		"rule:other:dest": {"keep.md": "kept"},
	})

	svc := service.New(nil)
	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatal(err)
	}

	all := f.localRules()
	if len(all) != 2 {
		t.Fatalf("expected one rule per existing profile folder, got %+v", all)
	}

	claude := ruleForProfile(t, all, "CLAUDE")
	if claude.Name != "CLAUDE" || !claude.Enabled || claude.Source.Origin != rules.OriginRepo || claude.Kind != rules.KindFolder {
		t.Errorf("unexpected migrated rule: %+v", claude)
	}
	if len(claude.Destinations) != 1 {
		t.Fatalf("expected one destination, got %+v", claude.Destinations)
	}
	claudeDest := claude.Destinations[0]
	if claudeDest.Location != "claude" || claudeDest.Path != "." || claudeDest.Mode != rules.ModeTwoWay || !claudeDest.Enabled {
		t.Errorf("unexpected destination: %+v", claudeDest)
	}
	if err := rules.Validate(claude, rules.ScopeLocal); err != nil {
		t.Errorf("migrated rule must be valid: %v", err)
	}

	codex := ruleForProfile(t, all, "CODEX")
	if got := codex.Destinations[0].Location; got != "codex-home" {
		t.Errorf("expected the existing location with the same path to be reused, got %q", got)
	}

	locations := map[string]string{}
	for _, loc := range cfg.Locations {
		locations[loc.Alias] = loc.Path
	}
	if len(locations) != 2 || locations["claude"] != claudeTarget || locations["codex-home"] != codexTarget {
		t.Errorf("unexpected locations: %+v", cfg.Locations)
	}

	raw := f.rawConfig()
	if _, stillThere := raw["connections"]; stillThere {
		t.Errorf("config.json must no longer store connections: %+v", raw)
	}
	if raw["theme"] != "dark" {
		t.Errorf("other settings must survive the migration: %+v", raw)
	}

	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	key := rulesync.BaselineKey(claude.ID, claudeDest.ID)
	if st.GetBaseline(key, "CLAUDE.md") != "hash-claude" || st.GetBaseline(key, "commands/review.md") != "hash-review" {
		t.Errorf("baselines were not carried to the rule destination: %+v", st.GetBaselines(key))
	}
	if st.GetBaseline(rulesync.BaselineKey(codex.ID, codex.Destinations[0].ID), "AGENTS.md") != "hash-codex" {
		t.Errorf("codex baselines were not carried over")
	}
	for _, legacyKey := range []string{"c1", "c2", "orphan"} {
		if len(st.GetBaselines(legacyKey)) != 0 {
			t.Errorf("legacy state entry %q must be dropped", legacyKey)
		}
	}
	if st.GetBaseline("rule:other:dest", "keep.md") != "kept" {
		t.Errorf("rule baselines must be kept")
	}
}

func TestLegacyMigrationIsIdempotent(t *testing.T) {
	f := newLegacyFixture(t)
	f.profile("CLAUDE")
	f.writeConfig(f.rootDir, []legacyConn{{ID: "c1", Profile: "CLAUDE", TargetPath: f.dir("home", ".claude")}}, nil)

	svc := service.New(nil)
	first := f.localRules()
	if len(first) != 1 {
		t.Fatalf("expected one rule after the first start, got %+v", first)
	}

	service.New(nil)
	if err := svc.ReloadFromDisk(); err != nil {
		t.Fatal(err)
	}
	second := f.localRules()
	if len(second) != 1 || second[0].ID != first[0].ID {
		t.Fatalf("a second start must not migrate again: %+v", second)
	}
	cfg, _ := svc.GetConfig()
	if len(cfg.Locations) != 1 {
		t.Errorf("a second start must not add locations: %+v", cfg.Locations)
	}
}

func TestLegacyMigrationRetryDoesNotDuplicateRules(t *testing.T) {
	f := newLegacyFixture(t)
	f.profile("CLAUDE")
	target := f.dir("home", ".claude")
	conns := []legacyConn{{ID: "c1", Profile: "CLAUDE", TargetPath: target}}
	f.writeConfig(f.rootDir, conns, nil)

	service.New(nil)
	first := f.localRules()

	// Simulates a start that stopped after writing the rules but before saving the config.
	f.writeConfig(f.rootDir, conns, []config.Location{{Alias: "claude", Path: target}})
	service.New(nil)

	if second := f.localRules(); len(second) != 1 || second[0].ID != first[0].ID {
		t.Fatalf("a retried migration must reuse the existing rule: %+v", second)
	}
}

func TestLegacyConnectionsWithoutRootAreDropped(t *testing.T) {
	f := newLegacyFixture(t)
	f.profile("CLAUDE")
	f.writeConfig("", []legacyConn{{ID: "c1", Profile: "CLAUDE", TargetPath: f.dir("home", ".claude")}}, nil)

	svc := service.New(nil)
	cfg, err := svc.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(f.localRules()) != 0 || len(cfg.Locations) != 0 {
		t.Errorf("without a repository nothing can be migrated: rules=%+v locations=%+v", f.localRules(), cfg.Locations)
	}
	if _, stillThere := f.rawConfig()["connections"]; stillThere {
		t.Errorf("connections must be cleared even when none could be migrated")
	}
}

func TestLegacyMigrationAvoidsAliasCollisions(t *testing.T) {
	f := newLegacyFixture(t)
	f.profile("Docs")
	f.profile("Home")
	f.profile("Docs Archive")
	f.writeConfig(f.rootDir,
		[]legacyConn{
			{ID: "c1", Profile: "Docs", TargetPath: f.dir("work", "docs")},
			{ID: "c2", Profile: "Home", TargetPath: f.dir("work", "home")},
			{ID: "c3", Profile: "Docs Archive", TargetPath: f.dir("work", "archive")},
		},
		[]config.Location{{Alias: "docs", Path: f.dir("other", "docs")}},
	)

	svc := service.New(nil)
	cfg, _ := svc.GetConfig()

	aliases := map[string]string{}
	for _, loc := range cfg.Locations {
		aliases[loc.Alias] = loc.Path
	}
	all := f.localRules()
	docs := ruleForProfile(t, all, "Docs").Destinations[0].Location
	home := ruleForProfile(t, all, "Home").Destinations[0].Location
	archive := ruleForProfile(t, all, "Docs Archive").Destinations[0].Location

	if docs != "docs-2" || aliases["docs-2"] != f.dir("work", "docs") {
		t.Errorf("expected docs-2 for a taken alias, got %q (%+v)", docs, aliases)
	}
	if home != "home-2" || aliases["home-2"] != f.dir("work", "home") {
		t.Errorf("the builtin home alias must stay free, got %q (%+v)", home, aliases)
	}
	if archive != "docs-archive" {
		t.Errorf("expected characters outside the alias alphabet to become dashes, got %q", archive)
	}
	for alias := range aliases {
		if alias != strings.ToLower(alias) {
			t.Errorf("aliases must be lower case: %q", alias)
		}
	}
	if len(aliases) != 4 {
		t.Errorf("expected the original and three new aliases, got %+v", aliases)
	}
}

func TestStateWithLegacyBaselinesLoadsAndIsPrunedWithoutConnections(t *testing.T) {
	f := newLegacyFixture(t)
	f.writeConfig(f.rootDir, nil, nil)
	f.writeBaselines(map[string]map[string]string{
		"old-connection": {"a.md": "h"},
		"rule:r1:d1":     {"b.md": "h2"},
	})

	service.New(nil)

	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.GetBaselines("old-connection")) != 0 || st.GetBaseline("rule:r1:d1", "b.md") != "h2" {
		t.Errorf("expected only rule baselines to remain: %+v", st.Baselines)
	}
}
