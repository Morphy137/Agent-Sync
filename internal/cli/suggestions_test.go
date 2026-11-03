package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

const suggestionsFile = `{ "version": 1, "suggestions": [
  { "name": "Claude instructions", "source": "AGENTS/CLAUDE.md",
    "destinations": [ { "location": "claude", "path": "CLAUDE.md", "mode": "one_way" } ] },
  { "name": "Codex instructions", "source": "AGENTS/CLAUDE.md",
    "destinations": [ { "location": "codex", "path": "AGENTS.md" } ] },
  { "name": "Escapes", "source": "AGENTS/CLAUDE.md",
    "destinations": [ { "location": "claude", "path": "../../.bashrc" } ] }
] }`

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// subscribeToSuggestingRepo subscribes the fixture to a local repository that ships a suggestions file.
func subscribeToSuggestingRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	gitIn(t, repo, "init", "-b", "main")
	gitIn(t, repo, "config", "user.name", "Friend")
	gitIn(t, repo, "config", "user.email", "friend@example.com")
	mustWrite(t, filepath.Join(repo, rules.SuggestionsFileName), suggestionsFile)
	if err := os.MkdirAll(filepath.Join(repo, "AGENTS"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, "AGENTS", "CLAUDE.md"), "# claude\n")
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "commit", "-m", "initial")

	var sub config.Subscription
	mustJSON(t, &sub, "subscriptions", "add", repo)
	return sub.ID
}

func TestSuggestionsListApplyAndExport(t *testing.T) {
	f := newFixture(t)
	subID := subscribeToSuggestingRepo(t)

	var listed []service.SuggestionInfo
	mustJSON(t, &listed, "suggestions", "list", "--subscription", subID)
	if len(listed) != 3 {
		t.Fatalf("listed = %+v", listed)
	}
	byName := map[string]service.SuggestionInfo{}
	for _, info := range listed {
		byName[info.Name] = info
	}
	if !byName["Claude instructions"].Valid || byName["Escapes"].Valid {
		t.Fatalf("validity wrong: %+v", listed)
	}
	if res := run("suggestions", "list", "--subscription", subID); res.code != ExitOK || res.stdout == "" {
		t.Fatalf("human output: %+v", res)
	}

	// Applying needs an explicit selection.
	if res := run("suggestions", "apply", "--subscription", subID); res.code != ExitUsage {
		t.Fatalf("apply without a selection must be a usage error: %+v", res)
	}
	if res := run("suggestions", "apply", "--subscription", subID, "--all", "--map", "broken"); res.code != ExitUsage {
		t.Fatalf("bad --map must be a usage error: %+v", res)
	}

	mustJSON(t, nil, "locations", "set", "claude", filepath.Join(f.home, ".claude"))
	claudeID, codexID := byName["Claude instructions"].ID, byName["Codex instructions"].ID
	var outcome service.SuggestionsApplyOutcome
	mustJSON(t, &outcome, "suggestions", "apply", "--subscription", subID, "--id", claudeID, "--id", codexID)
	if outcome.Applied != 1 {
		t.Fatalf("codex has no folder yet, only claude may apply: %+v", outcome)
	}

	codexDir := filepath.Join(f.home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustJSON(t, &outcome, "suggestions", "apply", "--subscription", subID, "--all", "--map", "codex="+codexDir)
	if outcome.Applied != 1 {
		t.Fatalf("--all must apply the remaining valid suggestion only: %+v", outcome)
	}
	skipped := 0
	for _, r := range outcome.Results {
		if r.Status == service.SuggestionSkipped {
			skipped++
		}
	}
	if skipped != 0 {
		t.Fatalf("--all only selects what is not applied yet: %+v", outcome)
	}

	output := filepath.Join(t.TempDir(), "exported.json")
	if res := run("suggestions", "export", "--subscription", subID, "--output", output); res.code != ExitOK {
		t.Fatalf("export: %+v", res)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := rules.ParseSuggestions(data)
	if err != nil || len(exported.Suggestions) != 2 {
		t.Fatalf("exported: %+v, %v", exported, err)
	}
	for _, s := range exported.Suggestions {
		if err := s.Validate(); err != nil {
			t.Errorf("exported %q invalid: %v", s.Name, err)
		}
	}

	if res := run("suggestions", "export", "--subscription", subID); res.code != ExitOK || res.stdout == "" {
		t.Fatalf("export to stdout: %+v", res)
	}
	if res := run("suggestions", "list"); res.code != ExitUsage {
		t.Fatalf("missing --subscription must be a usage error: %+v", res)
	}
}
