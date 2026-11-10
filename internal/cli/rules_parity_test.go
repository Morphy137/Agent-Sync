package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/service"
)

func TestParityRuleCreateCategoryAndStrictJSON(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "set", f.root)
	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--source", "AGENTS.md", "--to", "home:AGENTS.md", "--category", "instructions")
	if saved.Rule.CategoryID != "instructions" {
		t.Fatalf("category: %+v", saved.Rule)
	}
	old := stdin
	t.Cleanup(func() { stdin = old })
	for _, input := range []string{
		`{"sourcePath":"AGENTS.md","typo":true,"destinations":[{"location":"home","path":"other.md"}]}`,
		`{"sourcePath":"AGENTS.md","destinations":[{"location":"home","path":"other.md","typo":true}]}`,
	} {
		stdin = strings.NewReader(input)
		code, _ := runJSON(t, nil, "rules", "add", "--input", "-")
		if code != ExitUsage {
			t.Fatalf("unknown fields accepted: %s", input)
		}
	}
	input := filepath.Join(t.TempDir(), "create.json")
	mustWrite(t, input, `{"sourcePath":"AGENTS.md","categoryId":"instructions","destinations":[{"location":"home","path":"another.md"}]}`)
	mustJSON(t, &saved, "rules", "add", "--input", input)
	if saved.Rule.CategoryID != "instructions" {
		t.Fatalf("JSON category: %+v", saved.Rule)
	}
}

func TestParityRulePatchPreservesStoredState(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "set", f.root)
	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--source", "AGENTS.md", "--to", "home:AGENTS.md", "--exclude", "*.tmp", "--disabled")
	id, dest := saved.Rule.ID, saved.Rule.Destinations[0].ID
	mustJSON(t, nil, "rules", "enable", id)
	mustJSON(t, nil, "sync", "--rules-only")
	statePath := filepath.Join(config.GetConfigDir(), "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	old := stdin
	t.Cleanup(func() { stdin = old })
	stdin = strings.NewReader(`{"name":"Renamed","enabled":false,"exclude":[]}`)
	saved = service.SaveRuleResult{}
	mustJSON(t, &saved, "rules", "update", id, "--input", "-")
	if saved.Rule.ID != id || saved.Rule.Destinations[0].ID != dest || saved.Rule.Enabled || len(saved.Rule.Exclude) != 0 || saved.Rule.Source.Path != "AGENTS.md" {
		t.Fatalf("patch lost fields: %+v", saved.Rule)
	}
	after, err := os.ReadFile(statePath)
	if err != nil || string(before) != string(after) {
		t.Fatalf("baseline changed: %v", err)
	}
	patch := filepath.Join(t.TempDir(), "patch.json")
	mustWrite(t, patch, `{"enabled":true}`)
	mustJSON(t, &saved, "rules", "update", id, "--input", patch)
	if !saved.Rule.Enabled || saved.Rule.Name != "Renamed" {
		t.Fatalf("omitted fields reset: %+v", saved.Rule)
	}
	for _, input := range []string{`{"kind":"invalid"}`, `{"sourcePath":"../escape"}`, `{"typo":true}`, `{"id":"replacement"}`, `{} {}`} {
		stdin = strings.NewReader(input)
		code, _ := runJSON(t, nil, "rules", "update", id, "--input", "-")
		if code == ExitOK {
			t.Fatalf("accepted invalid patch: %s", input)
		}
		var got rulesync.RuleStatus
		mustJSON(t, &got, "rules", "show", id)
		if got.Rule.Name != "Renamed" || !got.Rule.Enabled || got.Rule.Source.Path != "AGENTS.md" {
			t.Fatalf("invalid patch mutated: %+v", got.Rule)
		}
	}
	stdin = strings.NewReader(`{"name":"missing"}`)
	code, env := runJSON(t, nil, "rules", "update", "missing", "--input", "-")
	if code != ExitError || env.Error.Code != "rule_not_found" {
		t.Fatalf("missing rule: %d %+v", code, env.Error)
	}
	rf, err := rules.LoadRepoRules(f.root)
	if err != nil || len(rf.Rules) != 1 {
		t.Fatalf("created missing id: %+v %v", rf, err)
	}
}
