package cli

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agentsync/internal/rulesync"
	"agentsync/internal/service"
)

func TestParityDestinationPatchPreservesIDsAndClearsExplicitly(t *testing.T) {
	f := newFixture(t)
	mustJSON(t, nil, "root", "set", f.root)
	mustWrite(t, filepath.Join(f.root, "mcp.json"), `{"mcpServers":{"demo":{"command":"demo"}}}`)
	var saved service.SaveRuleResult
	mustJSON(t, &saved, "rules", "add", "--source", "mcp.json", "--to", "home:config.json", "--transform", "json_merge:mcpServers")
	id, destID := saved.Rule.ID, saved.Rule.Destinations[0].ID
	old := stdin
	t.Cleanup(func() { stdin = old })
	stdin = strings.NewReader(`{"mode":"invalid"}`)
	code, _ := runJSON(t, nil, "destinations", "update", id, destID, "--input", "-")
	if code == ExitOK {
		t.Fatal("transform masked invalid mode")
	}
	stdin = strings.NewReader(`{"enabled":false,"ignore":["*.tmp"]}`)
	saved = service.SaveRuleResult{}
	mustJSON(t, &saved, "destinations", "update", id, destID, "--input", "-")
	d := saved.Rule.Destinations[0]
	if d.Enabled || d.ID != destID || d.Transform == nil || d.Path != "config.json" || len(d.Ignore) != 1 {
		t.Fatalf("patch lost fields: %+v", d)
	}
	file := filepath.Join(t.TempDir(), "patch.json")
	mustWrite(t, file, `{"enabled":true,"ignore":[],"clearTransform":true}`)
	saved = service.SaveRuleResult{}
	mustJSON(t, &saved, "destinations", "update", id, destID, "--input", file)
	d = saved.Rule.Destinations[0]
	if !d.Enabled || d.Transform != nil || len(d.Ignore) != 0 || d.ID != destID || d.Mode != "one_way" {
		t.Fatalf("clear changed identity or mode: %+v", d)
	}
	expected := saved.Rule
	for _, patch := range []string{`{"path":"../escape"}`, `{"mode":"invalid"}`, `{"clearTransform":true,"transform":{"type":"json_merge","key":"mcpServers"}}`, `{"id":"new"}`, `{"transform":{"type":"invalid","key":"x"}}`} {
		stdin = strings.NewReader(patch)
		code, _ := runJSON(t, nil, "destinations", "update", id, destID, "--input", "-")
		if code == ExitOK {
			t.Fatalf("invalid destination accepted: %s", patch)
		}
		var got rulesync.RuleStatus
		mustJSON(t, &got, "rules", "show", id)
		// Report may infer a category; compare stored fields rather than presentation metadata.
		got.Rule.CategoryID = expected.CategoryID
		if !reflect.DeepEqual(got.Rule, expected) {
			t.Fatalf("invalid patch mutated destination: %+v != %+v", got.Rule, expected)
		}
	}
	for _, ids := range [][2]string{{id, "missing"}, {"missing", destID}} {
		stdin = strings.NewReader(`{"path":"created.json"}`)
		code, env := runJSON(t, nil, "destinations", "update", ids[0], ids[1], "--input", "-")
		if code != ExitError || (env.Error.Code != "destination_not_found" && env.Error.Code != "rule_not_found") {
			t.Fatalf("missing id: %d %+v", code, env.Error)
		}
	}
}
