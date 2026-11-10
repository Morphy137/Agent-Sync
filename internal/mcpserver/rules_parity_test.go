package mcpserver

import (
	"testing"

	"agentsync/internal/service"
)

func TestParityRuleAndDestinationPatchThroughMCP(t *testing.T) {
	f := newFixture(t)
	f.mustCall(t, "set_root", map[string]any{"path": f.root}, nil)
	var saved service.SaveRuleResult
	f.mustCall(t, "add_rule", map[string]any{"sourcePath": "AGENTS.md", "categoryId": "instructions", "destinations": []map[string]any{{"location": "home", "path": "AGENTS.md"}}}, &saved)
	id, destID := saved.Rule.ID, saved.Rule.Destinations[0].ID
	if saved.Rule.CategoryID != "instructions" {
		t.Fatalf("category lost: %+v", saved.Rule)
	}
	f.mustCall(t, "update_rule", map[string]any{"ruleId": id, "patch": map[string]any{"enabled": false, "name": "Patched", "include": []string{}, "exclude": []string{}}}, &saved)
	if saved.Rule.Enabled || saved.Rule.Name != "Patched" || saved.Rule.ID != id || saved.Rule.Destinations[0].ID != destID {
		t.Fatalf("rule patch: %+v", saved.Rule)
	}
	f.mustCall(t, "update_destination", map[string]any{"ruleId": id, "destinationId": destID, "patch": map[string]any{"enabled": false, "ignore": []string{}, "path": "updated.md"}}, &saved)
	d := saved.Rule.Destinations[0]
	if d.Enabled || d.ID != destID || d.Path != "updated.md" || d.Location != "home" {
		t.Fatalf("destination patch: %+v", d)
	}
	f.mustCall(t, "update_rule", map[string]any{"ruleId": id, "patch": map[string]any{"scope": "local"}}, &saved)
	if saved.Rule.Scope != "local" || saved.Rule.Name != "Patched" || saved.Rule.ID != id || saved.Rule.Destinations[0].ID != destID {
		t.Fatalf("MCP migration lost fields: %+v", saved.Rule)
	}
	f.mustCall(t, "update_rule", map[string]any{"ruleId": id, "patch": map[string]any{"name": nil, "enabled": nil}}, &saved)
	if saved.Rule.Name != "Patched" || saved.Rule.Enabled {
		t.Fatalf("null reset fields: %+v", saved.Rule)
	}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"update_rule", map[string]any{"ruleId": "missing", "patch": map[string]any{"name": "Must not create"}}},
		{"update_destination", map[string]any{"ruleId": id, "destinationId": "missing", "patch": map[string]any{"path": "must-not-create.md"}}},
		{"update_rule", map[string]any{"ruleId": id, "patch": map[string]any{"kind": "invalid"}}},
		{"update_rule", map[string]any{"ruleId": id, "patch": map[string]any{"id": "replacement"}}},
		{"update_destination", map[string]any{"ruleId": id, "destinationId": destID, "patch": map[string]any{"path": "../escape"}}},
	} {
		if res := f.call(t, tc.name, tc.args, nil); !res.IsError {
			t.Fatalf("accepted invalid update: %+v", tc)
		}
	}
	var got RulesOutput
	f.mustCall(t, "list_rules", nil, &got)
	if len(got.Report.Rules) != 1 || got.Report.Rules[0].Rule.ID != id || len(got.Report.Rules[0].Rule.Destinations) != 1 || got.Report.Rules[0].Rule.Destinations[0].Path != "updated.md" {
		t.Fatalf("invalid updates mutated rules: %+v", got.Report)
	}
}
