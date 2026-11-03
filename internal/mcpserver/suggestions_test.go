package mcpserver

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

const suggestionsFile = `{ "version": 1, "suggestions": [
  { "name": "Instructions", "source": "AGENTS/CLAUDE.md",
    "destinations": [ { "location": "home", "path": "CLAUDE.md" } ] }
] }`

func subscribeToSuggestingRepo(t *testing.T, f *fixture) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo := t.TempDir()
	git := func(args ...string) {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-b", "main")
	git("config", "user.name", "Friend")
	git("config", "user.email", "friend@example.com")
	writeFile(t, filepath.Join(repo, rules.SuggestionsFileName), suggestionsFile)
	if err := os.MkdirAll(filepath.Join(repo, "AGENTS"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "AGENTS", "CLAUDE.md"), "# claude\n")
	git("add", "-A")
	git("commit", "-m", "initial")

	var sub config.Subscription
	f.mustCall(t, "add_subscription", map[string]any{"url": repo}, &sub)
	return sub.ID
}

func TestSuggestionTools(t *testing.T) {
	f := newFixture(t)
	subID := subscribeToSuggestingRepo(t, f)

	var listed SuggestionsOutput
	f.mustCall(t, "list_subscription_suggestions", map[string]any{"id": subID}, &listed)
	if len(listed.Suggestions) != 1 || !listed.Suggestions[0].Valid {
		t.Fatalf("listed = %+v", listed)
	}
	id := listed.Suggestions[0].ID

	res := f.call(t, "apply_subscription_suggestions", map[string]any{"id": subID, "suggestions": []string{id}}, nil)
	if !res.IsError || !strings.Contains(errorText(res), "confirmation_required") {
		t.Fatalf("apply without confirm: %s", errorText(res))
	}
	if local, _ := rules.LoadLocalRules(); len(local.Rules) != 0 {
		t.Fatalf("apply without confirm created rules: %+v", local.Rules)
	}

	var outcome service.SuggestionsApplyOutcome
	f.mustCall(t, "apply_subscription_suggestions", map[string]any{"id": subID, "suggestions": []string{id}, "confirm": true}, &outcome)
	if outcome.Applied != 1 {
		t.Fatalf("outcome = %+v", outcome)
	}
	if _, err := os.Stat(filepath.Join(f.home, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("apply wrote a file before sync: %v", err)
	}

	var exported rules.SuggestionsFile
	f.mustCall(t, "export_subscription_suggestions", map[string]any{"id": subID}, &exported)
	if len(exported.Suggestions) != 1 || exported.Suggestions[0].Source != "AGENTS/CLAUDE.md" {
		t.Fatalf("exported = %+v", exported)
	}

	res = f.call(t, "apply_subscription_suggestions", map[string]any{"id": subID, "suggestions": []string{}, "confirm": true}, nil)
	if !res.IsError || !strings.Contains(errorText(res), "invalid_argument") {
		t.Fatalf("an empty selection must be refused, got %s", errorText(res))
	}
}
