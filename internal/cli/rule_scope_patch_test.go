package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

func TestParityRuleScopePatchMovesWithoutReplacingIDsOrBaselines(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "set", f.root)
	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--source", "AGENTS.md", "--to", "home:AGENTS.md")
	id, dest := saved.Rule.ID, saved.Rule.Destinations[0].ID
	mustJSON(t, nil, "sync", "--rules-only")
	statePath := filepath.Join(config.GetConfigDir(), "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	old := stdin
	t.Cleanup(func() { stdin = old })
	for _, scope := range []string{rules.ScopeLocal, rules.ScopeRepo} {
		stdin = strings.NewReader(`{"scope":"` + scope + `","name":"Moved"}`)
		mustJSON(t, &saved, "rules", "update", id, "--input", "-")
		if saved.Rule.ID != id || saved.Rule.Destinations[0].ID != dest || saved.Rule.Scope != scope || saved.Rule.Name != "Moved" {
			t.Fatalf("migration lost identity: %+v", saved.Rule)
		}
		repo, err := rules.LoadRepoRules(f.root)
		if err != nil {
			t.Fatal(err)
		}
		local, err := rules.LoadLocalRules()
		if err != nil {
			t.Fatal(err)
		}
		if len(repo.Rules)+len(local.Rules) != 1 || (scope == rules.ScopeLocal && len(local.Rules) != 1) || (scope == rules.ScopeRepo && len(repo.Rules) != 1) {
			t.Fatalf("migration duplicated/lost rule: %+v %+v", repo, local)
		}
		after, err := os.ReadFile(statePath)
		if err != nil || string(before) != string(after) {
			t.Fatalf("baseline changed: %v", err)
		}
	}
	stdin = strings.NewReader(`{"scope":"invalid","name":"Must not persist"}`)
	code, _ := runJSON(t, nil, "rules", "update", id, "--input", "-")
	if code == ExitOK {
		t.Fatal("invalid scope accepted")
	}
	repo, err := rules.LoadRepoRules(f.root)
	if err != nil || repo.Rules[0].Name != "Moved" {
		t.Fatalf("invalid migration mutated: %+v %v", repo, err)
	}
	stdin = strings.NewReader(`{"scope":"local","name":"Must not create"}`)
	code, env := runJSON(t, nil, "rules", "update", "missing", "--input", "-")
	if code != ExitError || env.Error.Code != "rule_not_found" {
		t.Fatalf("migration created missing id: %d %+v", code, env.Error)
	}
}

func TestParityInvalidScopeMigrationDoesNotMutateLocalRule(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "set", f.root)
	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--origin", "local", "--source", filepath.Join(f.root, "AGENTS.md"), "--to", "home:local.md", "--name", "Original")
	old := stdin
	t.Cleanup(func() { stdin = old })
	stdin = strings.NewReader(`{"scope":"repo","name":"Must not persist"}`)
	code, _ := runJSON(t, nil, "rules", "update", saved.Rule.ID, "--input", "-")
	if code != ExitError {
		t.Fatal("local origin migrated to repo scope")
	}
	local, err := rules.LoadLocalRules()
	if err != nil || len(local.Rules) != 1 || local.Rules[0].Name != "Original" {
		t.Fatalf("invalid migration mutated local rule: %+v %v", local, err)
	}
	repo, err := rules.LoadRepoRules(f.root)
	if err != nil || len(repo.Rules) != 0 {
		t.Fatalf("invalid migration created repo rule: %+v %v", repo, err)
	}
}
