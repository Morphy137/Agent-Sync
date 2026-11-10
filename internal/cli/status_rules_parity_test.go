package cli

import (
	"os"
	"path/filepath"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/ops"
	"agentsync/internal/rules"
)

func TestParityStatusCountsLocalRulesWithoutRoot(t *testing.T) {
	f := newFixture(t)
	source := filepath.Join(t.TempDir(), "source.md")
	mustWrite(t, source, "local content")
	mustJSON(t, nil, "rules", "add", "--origin", "local", "--source", source, "--to", "home:local.md", "--mode", "one_way")
	var status ops.StatusReport
	mustJSON(t, &status, "status")
	if status.RootPath != "" || status.Rules.Total != 1 || status.Rules.Enabled != 1 || status.Rules.Pending != 1 {
		t.Fatalf("local rule missing: %+v (home %s)", status, f.home)
	}
}

func TestParityStatusSurfacesRuleReportErrors(t *testing.T) {
	for _, withRoot := range []bool{false, true} {
		t.Run(map[bool]string{false: "no_root", true: "with_root"}[withRoot], func(t *testing.T) {
			f := newFixture(t)
			if withRoot {
				mustJSON(t, nil, "root", "set", f.root)
			}
			if err := os.MkdirAll(config.GetConfigDir(), 0o755); err != nil {
				t.Fatal(err)
			}
			mustWrite(t, filepath.Join(config.GetConfigDir(), rules.LocalRulesFileName), "broken json")
			code, env := runJSON(t, nil, "status")
			if code != ExitError || env.Error == nil {
				t.Fatalf("rule report error swallowed: %d %+v", code, env)
			}
		})
	}
}
