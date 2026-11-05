package service_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/gitops"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

const (
	claudeSource   = "AGENTS/Claude/CLAUDE-AGENTS.md"
	geminiSource   = "AGENTS/Gemini/GEMINI-AGENTS.md"
	skillSource    = "skills/review"
	reviewSkillDoc = "---\nname: review\ndescription: review code\n---\nbody\n"
)

type suggestionsFixture struct {
	rulesFixture
	subID string
	work  string
}

// newSuggestionsFixture subscribes to a published repository that ships the given suggestions file
// (omitted when empty).
func newSuggestionsFixture(t *testing.T, suggestionsJSON string) suggestionsFixture {
	t.Helper()
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := newRulesFixture(t)
	files := map[string]string{
		claudeSource:                    "# claude instructions\n",
		geminiSource:                    "# gemini instructions\n",
		skillSource + "/SKILL.md":       reviewSkillDoc,
		skillSource + "/notes.md":       "notes\n",
		"README.md":                     "readme\n",
		"docs/" + geminiSource + ".bak": "x\n",
	}
	if suggestionsJSON != "" {
		files[rules.SuggestionsFileName] = suggestionsJSON
	}
	bare, work := publishedRepo(t, files)
	sub, err := f.svc.AddSubscription(fileURL(bare), "Friend")
	if err != nil {
		t.Fatal(err)
	}
	return suggestionsFixture{rulesFixture: f, subID: sub.ID, work: work}
}

func suggestionsJSONFor(t *testing.T, suggestions ...rules.Suggestion) string {
	t.Helper()
	data, err := json.Marshal(rules.SuggestionsFile{Version: rules.SuggestionsVersion, Suggestions: suggestions})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func claudeSuggestion() rules.Suggestion {
	return rules.Suggestion{
		Name:         "Claude instructions",
		Description:  "Main file",
		Source:       claudeSource,
		Destinations: []rules.SuggestedDestination{{Location: "claude", Path: "CLAUDE.md", Mode: rules.ModeOneWay}},
	}
}

func geminiSuggestion() rules.Suggestion {
	return rules.Suggestion{
		Name:         "Gemini instructions",
		Source:       geminiSource,
		Destinations: []rules.SuggestedDestination{{Location: "gemini", Path: "GEMINI.md"}},
	}
}

func skillSuggestion(location string) rules.Suggestion {
	return rules.Suggestion{
		Name:         "Review skill",
		Source:       skillSource,
		Kind:         rules.KindSkill,
		Destinations: []rules.SuggestedDestination{{Location: location, Path: "skills/review", Mode: rules.ModeCreateOnly}},
	}
}

func findSuggestion(t *testing.T, list []service.SuggestionInfo, name string) service.SuggestionInfo {
	t.Helper()
	for _, info := range list {
		if info.Name == name {
			return info
		}
	}
	t.Fatalf("suggestion %q not found in %+v", name, list)
	return service.SuggestionInfo{}
}

func resultFor(t *testing.T, outcome *service.SuggestionsApplyOutcome, id string) service.SuggestionApplyResult {
	t.Helper()
	for _, r := range outcome.Results {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no result for %s in %+v", id, outcome.Results)
	return service.SuggestionApplyResult{}
}

func TestListSuggestionsWithoutFileIsEmpty(t *testing.T) {
	f := newSuggestionsFixture(t, "")
	list, err := f.svc.ListSubscriptionSuggestions(f.subID)
	if err != nil || len(list) != 0 {
		t.Fatalf("got %+v, %v", list, err)
	}
	infos, _ := f.svc.ListSubscriptions()
	if infos[0].SuggestionsPending != 0 {
		t.Fatalf("pending = %d", infos[0].SuggestionsPending)
	}
}

func TestListSuggestionsUnknownSubscription(t *testing.T) {
	f := newRulesFixture(t)
	if _, err := f.svc.ListSubscriptionSuggestions("missing"); !apperr.HasCode(err, apperr.SubscriptionNotFound) {
		t.Fatalf("got %v", err)
	}
	if _, err := f.svc.ApplySubscriptionSuggestions("missing", nil, nil); !apperr.HasCode(err, apperr.SubscriptionNotFound) {
		t.Fatalf("got %v", err)
	}
	if _, err := f.svc.ExportSubscriptionSuggestions("missing"); !apperr.HasCode(err, apperr.SubscriptionNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestListSuggestionsMalformedFileReturnsCodedError(t *testing.T) {
	f := newSuggestionsFixture(t, `{"version": 1, "suggestions": [`)
	_, err := f.svc.ListSubscriptionSuggestions(f.subID)
	if !apperr.HasCode(err, apperr.InvalidSuggestions) {
		t.Fatalf("got %v", err)
	}
	infos, listErr := f.svc.ListSubscriptions()
	if listErr != nil || infos[0].SuggestionsPending != 0 {
		t.Fatalf("listing must survive a malformed file: %+v, %v", infos, listErr)
	}
}

func TestListSuggestionsDescribesThisPC(t *testing.T) {
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, claudeSuggestion(), skillSuggestion("codex"),
		rules.Suggestion{Name: "Missing", Source: "nope/none.md", Destinations: []rules.SuggestedDestination{{Location: "claude", Path: "x.md"}}}))
	if err := os.WriteFile(filepath.Join(f.claudeDir, "CLAUDE.md"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	codexHome := filepath.Join(os.Getenv("AGENTSYNC_HOME_OVERRIDE"), ".codex")
	writeFile(t, filepath.Join(codexHome, "config.toml"), "")

	list, err := f.svc.ListSubscriptionSuggestions(f.subID)
	if err != nil || len(list) != 3 {
		t.Fatalf("got %+v, %v", list, err)
	}

	claude := findSuggestion(t, list, "Claude instructions")
	if !claude.Valid || !claude.SourceExists || claude.Applied || claude.Description != "Main file" {
		t.Fatalf("claude: %+v", claude)
	}
	dest := claude.Destinations[0]
	if !dest.Mapped || dest.TargetPath != filepath.Join(f.claudeDir, "CLAUDE.md") || !dest.TargetExists || dest.Mode != rules.ModeOneWay {
		t.Fatalf("claude destination: %+v", dest)
	}

	skill := findSuggestion(t, list, "Review skill")
	skillDest := skill.Destinations[0]
	if !skill.Valid || skillDest.Mapped || skillDest.TargetPath != "" {
		t.Fatalf("skill: %+v", skill)
	}
	if skillDest.DetectedPath != codexHome {
		t.Fatalf("detected path = %q, want %q", skillDest.DetectedPath, codexHome)
	}

	missing := findSuggestion(t, list, "Missing")
	if missing.Valid || missing.SourceExists || missing.InvalidReason != "source_not_found" {
		t.Fatalf("missing: %+v", missing)
	}

	infos, _ := f.svc.ListSubscriptions()
	if infos[0].SuggestionsPending != 2 {
		t.Fatalf("pending = %d, want 2", infos[0].SuggestionsPending)
	}
}

func TestApplySuggestionsCreatesPendingLocalRulesWithoutWritingFiles(t *testing.T) {
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, claudeSuggestion(), geminiSuggestion()))
	list, _ := f.svc.ListSubscriptionSuggestions(f.subID)
	ids := []string{findSuggestion(t, list, "Claude instructions").ID, findSuggestion(t, list, "Gemini instructions").ID}

	outcome, err := f.svc.ApplySubscriptionSuggestions(f.subID, ids, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied != 2 {
		t.Fatalf("outcome: %+v", outcome)
	}

	local, err := rules.LoadLocalRules()
	if err != nil || len(local.Rules) != 2 {
		t.Fatalf("local rules: %+v, %v", local, err)
	}
	for _, r := range local.Rules {
		if r.Source.Origin != rules.OriginSubscription || r.Source.Subscription != f.subID || !r.Enabled || r.Kind != rules.KindAuto {
			t.Fatalf("unexpected rule: %+v", r)
		}
	}
	if _, err := os.Stat(filepath.Join(f.root, rules.RulesFileName)); !os.IsNotExist(err) {
		t.Fatalf("applying suggestions wrote repository rules: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.claudeDir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("applying wrote a file before Sync: %v", err)
	}

	report, err := f.svc.GetRulesReport()
	if err != nil || report.Pending == 0 {
		t.Fatalf("new rules must be pending: %+v, %v", report, err)
	}

	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(f.claudeDir, "CLAUDE.md")); got != "# claude instructions\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	if got := readFile(t, filepath.Join(f.geminiDir, "GEMINI.md")); got != "# gemini instructions\n" {
		t.Fatalf("GEMINI.md = %q", got)
	}
}

func TestApplySuggestionsSkipsAlreadyAppliedAndDuplicates(t *testing.T) {
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, claudeSuggestion(), geminiSuggestion()))
	list, _ := f.svc.ListSubscriptionSuggestions(f.subID)
	claudeID := findSuggestion(t, list, "Claude instructions").ID
	geminiID := findSuggestion(t, list, "Gemini instructions").ID

	// An equivalent rule created by hand counts as applied.
	manual := rules.Rule{
		Name: "My own", Enabled: true,
		Source:       rules.Source{Origin: rules.OriginSubscription, Subscription: f.subID, Path: geminiSource},
		Destinations: []rules.Destination{{Location: "gemini", Path: "GEMINI.md", Mode: rules.ModeOneWay, Enabled: true}},
	}
	if _, err := f.svc.SaveRule(manual, rules.ScopeLocal); err != nil {
		t.Fatal(err)
	}

	outcome, err := f.svc.ApplySubscriptionSuggestions(f.subID, []string{claudeID, claudeID, geminiID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Results) != 2 || outcome.Applied != 1 {
		t.Fatalf("outcome: %+v", outcome)
	}
	if resultFor(t, outcome, claudeID).Status != service.SuggestionApplied || resultFor(t, outcome, geminiID).Status != service.SuggestionSkipped {
		t.Fatalf("outcome: %+v", outcome)
	}

	again, _ := f.svc.ApplySubscriptionSuggestions(f.subID, []string{claudeID, geminiID}, nil)
	if again.Applied != 0 || resultFor(t, again, claudeID).Status != service.SuggestionSkipped {
		t.Fatalf("second apply must skip everything: %+v", again)
	}
	local, _ := rules.LoadLocalRules()
	if len(local.Rules) != 2 {
		t.Fatalf("rule count = %d, want 2 (manual + claude)", len(local.Rules))
	}

	after, _ := f.svc.ListSubscriptionSuggestions(f.subID)
	for _, info := range after {
		if !info.Applied {
			t.Fatalf("%s should be marked applied", info.Name)
		}
	}
	infos, _ := f.svc.ListSubscriptions()
	if infos[0].SuggestionsPending != 0 {
		t.Fatalf("pending = %d", infos[0].SuggestionsPending)
	}
}

func TestApplySuggestionsAddsOnlyMissingDestinations(t *testing.T) {
	both := rules.Suggestion{
		Name:   "Both",
		Source: claudeSource,
		Destinations: []rules.SuggestedDestination{
			{Location: "claude", Path: "CLAUDE.md"},
			{Location: "gemini", Path: "GEMINI.md"},
		},
	}
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, both))
	manual := rules.Rule{
		Name: "Existing", Enabled: true,
		Source:       rules.Source{Origin: rules.OriginSubscription, Subscription: f.subID, Path: claudeSource},
		Destinations: []rules.Destination{{Location: "claude", Path: "CLAUDE.md", Mode: rules.ModeOneWay, Enabled: true}},
	}
	if _, err := f.svc.SaveRule(manual, rules.ScopeLocal); err != nil {
		t.Fatal(err)
	}
	list, _ := f.svc.ListSubscriptionSuggestions(f.subID)
	info := findSuggestion(t, list, "Both")
	if info.Applied || !info.Destinations[0].Applied || info.Destinations[1].Applied {
		t.Fatalf("partial state wrong: %+v", info)
	}
	outcome, err := f.svc.ApplySubscriptionSuggestions(f.subID, []string{info.ID}, nil)
	if err != nil || outcome.Applied != 1 {
		t.Fatalf("outcome: %+v, %v", outcome, err)
	}
	local, _ := rules.LoadLocalRules()
	if len(local.Rules) != 2 || len(local.Rules[1].Destinations) != 1 || local.Rules[1].Destinations[0].Location != "gemini" {
		t.Fatalf("expected a second rule with only the gemini destination: %+v", local.Rules)
	}
}

func TestApplySuggestionsUnmappedLocationNeedsMapping(t *testing.T) {
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, skillSuggestion("codex")))
	list, _ := f.svc.ListSubscriptionSuggestions(f.subID)
	id := list[0].ID

	outcome, err := f.svc.ApplySubscriptionSuggestions(f.subID, []string{id}, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := resultFor(t, outcome, id)
	if res.Status != service.SuggestionFailed || res.ErrorCode != apperr.LocationUnmapped || res.ErrorAlias != "codex" {
		t.Fatalf("result: %+v", res)
	}
	if local, _ := rules.LoadLocalRules(); len(local.Rules) != 0 {
		t.Fatalf("a rejected suggestion must not create a rule: %+v", local.Rules)
	}

	notAFolder := filepath.Join(t.TempDir(), "missing")
	outcome, _ = f.svc.ApplySubscriptionSuggestions(f.subID, []string{id}, map[string]string{"codex": notAFolder})
	if res := resultFor(t, outcome, id); res.Status != service.SuggestionFailed || res.ErrorCode != apperr.TargetNotFound || res.ErrorAlias != "codex" {
		t.Fatalf("a bad folder must be rejected: %+v", res)
	}
	if local, _ := rules.LoadLocalRules(); len(local.Rules) != 0 {
		t.Fatalf("a bad folder must not create a rule: %+v", local.Rules)
	}

	codexDir := filepath.Join(t.TempDir(), "codex-home")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outcome, err = f.svc.ApplySubscriptionSuggestions(f.subID, []string{id}, map[string]string{"codex": codexDir})
	if err != nil || outcome.Applied != 1 {
		t.Fatalf("outcome: %+v, %v", outcome, err)
	}
	locations, _ := f.svc.ListLocations()
	mapped := false
	for _, l := range locations {
		mapped = mapped || (l.Alias == "codex" && l.Path == codexDir)
	}
	if !mapped {
		t.Fatalf("the mapping was not saved: %+v", locations)
	}
	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(codexDir, "skills", "review", "SKILL.md")); got != reviewSkillDoc {
		t.Fatalf("skill not synced: %q", got)
	}
}

func TestApplySuggestionsNeverOverwritesExistingMapping(t *testing.T) {
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, claudeSuggestion()))
	list, _ := f.svc.ListSubscriptionSuggestions(f.subID)
	elsewhere := t.TempDir()
	outcome, err := f.svc.ApplySubscriptionSuggestions(f.subID, []string{list[0].ID}, map[string]string{"claude": elsewhere})
	if err != nil || outcome.Applied != 1 {
		t.Fatalf("outcome: %+v, %v", outcome, err)
	}
	locations, _ := f.svc.ListLocations()
	for _, l := range locations {
		if l.Alias == "claude" && l.Path != f.claudeDir {
			t.Fatalf("existing mapping was replaced: %+v", l)
		}
	}
}

func TestSuggestionsFromUntrustedRepoAreRejected(t *testing.T) {
	bad := func(name, source string, dest rules.SuggestedDestination) rules.Suggestion {
		return rules.Suggestion{Name: name, Source: source, Destinations: []rules.SuggestedDestination{dest}}
	}
	malicious := []rules.Suggestion{
		bad("source escapes", "../outside.md", rules.SuggestedDestination{Location: "claude", Path: "a.md"}),
		bad("source absolute", filepath.ToSlash(filepath.Join(t.TempDir(), "secret.md")), rules.SuggestedDestination{Location: "claude", Path: "a.md"}),
		bad("source windows absolute", `C:\Windows\win.ini`, rules.SuggestedDestination{Location: "claude", Path: "a.md"}),
		bad("source in git", ".git/config", rules.SuggestedDestination{Location: "claude", Path: "a.md"}),
		bad("dest escapes", claudeSource, rules.SuggestedDestination{Location: "claude", Path: "../../.bashrc"}),
		bad("dest backslash escapes", claudeSource, rules.SuggestedDestination{Location: "claude", Path: `..\..\.bashrc`}),
		bad("dest absolute", claudeSource, rules.SuggestedDestination{Location: "claude", Path: "/etc/cron.d/x"}),
		bad("dest windows absolute", claudeSource, rules.SuggestedDestination{Location: "claude", Path: `C:\Users\Public\x.md`}),
		bad("dest git hooks", claudeSource, rules.SuggestedDestination{Location: "claude", Path: "repo/.git/hooks/pre-commit"}),
		bad("two way", claudeSource, rules.SuggestedDestination{Location: "claude", Path: "a.md", Mode: rules.ModeTwoWay}),
		bad("bad mode", claudeSource, rules.SuggestedDestination{Location: "claude", Path: "a.md", Mode: "overwrite_all"}),
		bad("bad alias", claudeSource, rules.SuggestedDestination{Location: "../etc", Path: "a.md"}),
		bad("upper alias", claudeSource, rules.SuggestedDestination{Location: "Claude", Path: "a.md"}),
	}
	good := claudeSuggestion()
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, append(malicious, good)...))

	list, err := f.svc.ListSubscriptionSuggestions(f.subID)
	if err != nil || len(list) != len(malicious)+1 {
		t.Fatalf("got %d entries, %v", len(list), err)
	}
	ids := make([]string, 0, len(list))
	for _, info := range list {
		ids = append(ids, info.ID)
		isGood := info.Name == good.Name
		if info.Valid != isGood {
			t.Errorf("%q: valid = %v", info.Name, info.Valid)
		}
		if !info.Valid && info.InvalidReason == "" {
			t.Errorf("%q: missing reason", info.Name)
		}
		for _, d := range info.Destinations {
			if !info.Valid && d.TargetPath != "" {
				t.Errorf("%q: resolved a target for an invalid suggestion: %q", info.Name, d.TargetPath)
			}
		}
	}

	outcome, err := f.svc.ApplySubscriptionSuggestions(f.subID, append(ids, "feedfacefeed"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Applied != 1 {
		t.Fatalf("only the legitimate suggestion may apply: %+v", outcome)
	}
	for _, res := range outcome.Results {
		if res.Status == service.SuggestionFailed && res.ErrorCode == "" {
			t.Errorf("failed result without code: %+v", res)
		}
	}
	local, _ := rules.LoadLocalRules()
	if len(local.Rules) != 1 || local.Rules[0].Name != good.Name {
		t.Fatalf("unexpected rules: %+v", local.Rules)
	}
	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	for _, escaped := range []string{filepath.Join(filepath.Dir(f.claudeDir), ".bashrc"), filepath.Join(f.claudeDir, "a.md")} {
		if _, err := os.Stat(escaped); !os.IsNotExist(err) {
			t.Fatalf("a rejected suggestion wrote %s", escaped)
		}
	}
}

func TestSuggestionSourceSymlinkOutsideCloneIsRejected(t *testing.T) {
	f := newSuggestionsFixture(t, suggestionsJSONFor(t,
		rules.Suggestion{Name: "Linked", Source: "linked", Kind: rules.KindFolder, Destinations: []rules.SuggestedDestination{{Location: "claude", Path: "linked"}}}))
	outside := t.TempDir()
	writeFile(t, filepath.Join(outside, "secret.txt"), "top secret")
	clone := filepath.Join(os.Getenv(config.ConfigEnvVar), "subscriptions", f.subID, "repo")
	if err := os.Symlink(outside, filepath.Join(clone, "linked")); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	list, err := f.svc.ListSubscriptionSuggestions(f.subID)
	if err != nil {
		t.Fatal(err)
	}
	info := findSuggestion(t, list, "Linked")
	if info.Valid || info.InvalidReason != "source_outside_repository" {
		t.Fatalf("a symlink leaving the clone must be refused: %+v", info)
	}
	outcome, _ := f.svc.ApplySubscriptionSuggestions(f.subID, []string{info.ID}, nil)
	if outcome.Applied != 0 {
		t.Fatalf("outcome: %+v", outcome)
	}
}

func TestSuggestionsFileIsHiddenFromSubscriptionListings(t *testing.T) {
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, claudeSuggestion()))
	tree, err := f.svc.GetSubscriptionTree(f.subID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range tree {
		if entry.Path == rules.SuggestionsFileName {
			t.Fatalf("tree lists the suggestions file: %+v", tree)
		}
	}
	dir, err := f.svc.ListFileDirectory(f.subID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range dir {
		if entry.Name == rules.SuggestionsFileName {
			t.Fatalf("file browser lists the suggestions file: %+v", dir)
		}
	}
}

func TestExportSuggestionsRoundTrip(t *testing.T) {
	original := []rules.Suggestion{claudeSuggestion(), skillSuggestion("claude")}
	f := newSuggestionsFixture(t, suggestionsJSONFor(t, original...))
	list, _ := f.svc.ListSubscriptionSuggestions(f.subID)
	ids := []string{list[0].ID, list[1].ID}
	if outcome, err := f.svc.ApplySubscriptionSuggestions(f.subID, ids, nil); err != nil || outcome.Applied != 2 {
		t.Fatalf("outcome: %+v, %v", outcome, err)
	}
	// Rules that are not fed by the subscription, or that use a transform, never leak into the export.
	repoRule := instructionsRule()
	if _, err := f.svc.SaveRule(repoRule, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	withTransform := rules.Rule{
		Name: "MCP", Enabled: true,
		Source:       rules.Source{Origin: rules.OriginSubscription, Subscription: f.subID, Path: "README.md"},
		Destinations: []rules.Destination{{Location: "claude", Path: "mcp.json", Mode: rules.ModeOneWay, Enabled: true, Transform: &rules.Transform{Type: rules.TransformJSONMerge, Key: "mcpServers"}}},
	}
	if _, err := f.svc.SaveRule(withTransform, rules.ScopeLocal); err != nil {
		t.Fatal(err)
	}
	twoWay := rules.Rule{
		Name: "Two way", Enabled: true,
		Source:       rules.Source{Origin: rules.OriginSubscription, Subscription: f.subID, Path: geminiSource},
		Destinations: []rules.Destination{{Location: "gemini", Path: "GEMINI.md", Mode: rules.ModeTwoWay, Enabled: true}},
	}
	if _, err := f.svc.SaveRule(twoWay, rules.ScopeLocal); err != nil {
		t.Fatal(err)
	}

	exported, err := f.svc.ExportSubscriptionSuggestions(f.subID)
	if err != nil {
		t.Fatal(err)
	}
	if len(exported.Suggestions) != 3 {
		t.Fatalf("exported = %+v", exported.Suggestions)
	}
	data, err := json.Marshal(exported)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := rules.ParseSuggestions(data)
	if err != nil {
		t.Fatalf("exported file must parse: %v", err)
	}
	for _, s := range parsed.Suggestions {
		if err := s.Validate(); err != nil {
			t.Errorf("exported suggestion %q is invalid: %v", s.Name, err)
		}
	}
	byName := map[string]rules.Suggestion{}
	for _, s := range parsed.Suggestions {
		byName[s.Name] = s
	}
	for _, want := range original {
		got, ok := byName[want.Name]
		if !ok || got.Source != want.Source || got.Kind != want.Kind || got.Destinations[0] != want.Destinations[0] {
			t.Errorf("round trip changed %q: got %+v want %+v", want.Name, got, want)
		}
	}
	if byName["Two way"].Destinations[0].Mode != rules.ModeOneWay {
		t.Errorf("two-way must export as one-way: %+v", byName["Two way"])
	}
	if strings.Contains(string(data), "mcp.json") {
		t.Error("transform destinations must not be exported")
	}
}
