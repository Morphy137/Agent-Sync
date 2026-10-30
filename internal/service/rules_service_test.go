package service_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/service"
)

type rulesFixture struct {
	svc       *service.Service
	root      string
	claudeDir string
	geminiDir string
}

func newRulesFixture(t *testing.T) rulesFixture {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv(config.ConfigEnvVar, filepath.Join(tmp, "config"))
	t.Setenv("AGENTSYNC_HOME_OVERRIDE", filepath.Join(tmp, "home"))

	f := rulesFixture{
		root:      filepath.Join(tmp, "repo"),
		claudeDir: filepath.Join(tmp, "home", ".claude"),
		geminiDir: filepath.Join(tmp, "home", ".gemini"),
	}
	for _, dir := range []string{filepath.Join(f.root, "shared"), f.claudeDir, f.geminiDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(f.root, "shared", "AGENTS.md"), "# shared instructions\n")

	f.svc = service.New(nil)
	if _, err := f.svc.SetRootPath(f.root); err != nil {
		t.Fatalf("SetRootPath: %v", err)
	}
	if _, err := f.svc.SaveLocation("claude", f.claudeDir); err != nil {
		t.Fatalf("SaveLocation claude: %v", err)
	}
	if _, err := f.svc.SaveLocation("gemini", f.geminiDir); err != nil {
		t.Fatalf("SaveLocation gemini: %v", err)
	}
	return f
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// readFile returns file content with CRLF normalized (git may convert line endings on checkout).
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func instructionsRule() rules.Rule {
	return rules.Rule{
		Name:    "Agent instructions",
		Enabled: true,
		Source:  rules.Source{Origin: rules.OriginRepo, Path: "shared/AGENTS.md"},
		Destinations: []rules.Destination{
			{Location: "claude", Path: "CLAUDE.md", Enabled: true},
			{Location: "gemini", Path: "GEMINI.md", Enabled: true},
		},
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func fileURL(path string) string {
	return "file:///" + strings.TrimPrefix(filepath.ToSlash(path), "/")
}

func TestRulesFanOutWithRenamedDestinationsAndCapture(t *testing.T) {
	f := newRulesFixture(t)

	saved, err := f.svc.SaveRule(instructionsRule(), rules.ScopeRepo)
	if err != nil {
		t.Fatalf("SaveRule: %v", err)
	}
	if len(saved.Collisions) != 0 {
		t.Fatalf("unexpected collisions: %+v", saved.Collisions)
	}
	if _, err := os.Stat(filepath.Join(f.root, rules.RulesFileName)); err != nil {
		t.Fatalf("rules file not written in repo: %v", err)
	}

	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatalf("SyncRules: %v", err)
	}
	if got := readFile(t, filepath.Join(f.claudeDir, "CLAUDE.md")); got != "# shared instructions\n" {
		t.Errorf("CLAUDE.md = %q", got)
	}
	if got := readFile(t, filepath.Join(f.geminiDir, "GEMINI.md")); got != "# shared instructions\n" {
		t.Errorf("GEMINI.md = %q", got)
	}

	// Editing one destination flows back to the source and out to the other destination.
	writeFile(t, filepath.Join(f.claudeDir, "CLAUDE.md"), "# edited in claude\n")
	res, err := f.svc.SyncRules()
	if err != nil {
		t.Fatalf("SyncRules after edit: %v", err)
	}
	if res.Captured != 1 {
		t.Errorf("captured = %d, want 1", res.Captured)
	}
	if got := readFile(t, filepath.Join(f.root, "shared", "AGENTS.md")); got != "# edited in claude\n" {
		t.Errorf("source = %q", got)
	}
	if got := readFile(t, filepath.Join(f.geminiDir, "GEMINI.md")); got != "# edited in claude\n" {
		t.Errorf("GEMINI.md = %q", got)
	}

	report, err := f.svc.GetRulesReport()
	if err != nil {
		t.Fatal(err)
	}
	if report.Pending != 0 || report.Conflicts != 0 {
		t.Errorf("expected clean report, got pending=%d conflicts=%d", report.Pending, report.Conflicts)
	}
}

func TestCollidingRulesAreReportedAndBlocked(t *testing.T) {
	f := newRulesFixture(t)
	writeFile(t, filepath.Join(f.root, "overrides", "CLAUDE.md"), "# claude only\n")

	if _, err := f.svc.SaveRule(instructionsRule(), rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	override := rules.Rule{
		Name:         "Claude override",
		Enabled:      true,
		Source:       rules.Source{Origin: rules.OriginRepo, Path: "overrides/CLAUDE.md"},
		Destinations: []rules.Destination{{Location: "claude", Path: "CLAUDE.md", Enabled: true}},
	}
	saved, err := f.svc.SaveRule(override, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Collisions) == 0 || saved.Collisions[0].Type != rules.CollisionSameTarget {
		t.Fatalf("expected same_target collision, got %+v", saved.Collisions)
	}

	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.claudeDir, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Errorf("blocked destination must not be written")
	}
	if got := readFile(t, filepath.Join(f.geminiDir, "GEMINI.md")); got != "# shared instructions\n" {
		t.Errorf("non-colliding destination should still sync, got %q", got)
	}

	report, err := f.svc.GetRulesReport()
	if err != nil {
		t.Fatal(err)
	}
	blocked := 0
	for _, r := range report.Rules {
		for _, d := range r.Destinations {
			if d.State == rulesync.DestBlocked {
				blocked++
			}
		}
	}
	if blocked != 2 {
		t.Errorf("blocked destinations = %d, want 2", blocked)
	}
}

func TestDetectionSuggestsAgentPlacement(t *testing.T) {
	f := newRulesFixture(t)
	writeFile(t, filepath.Join(f.claudeDir, "settings.json"), "{}\n")

	src := rules.Source{Origin: rules.OriginRepo, Path: "shared/AGENTS.md"}
	info, err := f.svc.DetectSource(src)
	if err != nil {
		t.Fatal(err)
	}
	if info.Kind != rules.KindInstructions {
		t.Fatalf("kind = %q, want instructions", info.Kind)
	}
	suggestion, err := f.svc.SuggestPlacement(src, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if !suggestion.Detected || suggestion.Path != "CLAUDE.md" {
		t.Errorf("suggestion = %+v, want detected CLAUDE.md", suggestion)
	}
}

func TestExecuteSyncCommitsAndPushesRuleCaptures(t *testing.T) {
	f := newRulesFixture(t)
	bare := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v\n%s", err, out)
	}
	runGit(t, f.root, "init", "-b", "main")
	runGit(t, f.root, "config", "user.name", "Tester")
	runGit(t, f.root, "config", "user.email", "tester@example.com")
	runGit(t, f.root, "add", "-A")
	runGit(t, f.root, "commit", "-m", "initial")
	runGit(t, f.root, "remote", "add", "origin", bare)
	runGit(t, f.root, "push", "-u", "origin", "main")

	if _, err := f.svc.SaveRule(instructionsRule(), rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	plan, err := f.svc.PlanSync()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.DirtyOutside) != 0 {
		t.Fatalf("rules file must be managed, dirtyOutside=%v", plan.DirtyOutside)
	}
	if plan.RulesPending == 0 {
		t.Fatalf("expected pending rule writes")
	}
	if _, err := f.svc.ExecuteSync(plan.Fingerprint, nil); err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}
	if got := readFile(t, filepath.Join(f.geminiDir, "GEMINI.md")); got != "# shared instructions\n" {
		t.Fatalf("GEMINI.md = %q", got)
	}

	writeFile(t, filepath.Join(f.geminiDir, "GEMINI.md"), "# edited in gemini\n")
	plan, err = f.svc.PlanSync()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ExecuteSync(plan.Fingerprint, nil); err != nil {
		t.Fatalf("ExecuteSync after edit: %v", err)
	}
	remoteContent := runGit(t, f.root, "--git-dir", bare, "show", "main:shared/AGENTS.md")
	if remoteContent != "# edited in gemini" {
		t.Errorf("remote source = %q", remoteContent)
	}
	if status := runGit(t, f.root, "status", "--porcelain"); status != "" {
		t.Errorf("worktree should be clean, got %q", status)
	}
}

func TestSubscriptionFeedsRuleAndUpdates(t *testing.T) {
	f := newRulesFixture(t)
	tmp := t.TempDir()
	bare := filepath.Join(tmp, "shared.git")
	work := filepath.Join(tmp, "work")
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", bare).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "clone", bare, work).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	runGit(t, work, "config", "user.name", "Friend")
	runGit(t, work, "config", "user.email", "friend@example.com")
	skill := "---\nname: my-skill\ndescription: test skill\n---\nv1\n"
	writeFile(t, filepath.Join(work, "skills", "my-skill", "SKILL.md"), skill)
	runGit(t, work, "add", "-A")
	runGit(t, work, "commit", "-m", "add skill")
	runGit(t, work, "push", "origin", "HEAD:main")

	sub, err := f.svc.AddSubscription(fileURL(bare), "")
	if err != nil {
		t.Fatalf("AddSubscription: %v", err)
	}
	tree, err := f.svc.GetSubscriptionTree(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundSkill := false
	for _, e := range tree {
		if e.Path == "skills/my-skill" && e.Kind == rules.KindSkill {
			foundSkill = true
		}
	}
	if !foundSkill {
		t.Fatalf("skill folder not detected in tree: %+v", tree)
	}

	rule := rules.Rule{
		Name:         "Friend skill",
		Enabled:      true,
		Source:       rules.Source{Origin: rules.OriginSubscription, Subscription: sub.ID, Path: "skills/my-skill"},
		Destinations: []rules.Destination{{Location: "claude", Path: "skills/my-skill", Enabled: true}},
	}
	if _, err := f.svc.SaveRule(rule, rules.ScopeLocal); err != nil {
		t.Fatalf("SaveRule: %v", err)
	}
	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(f.claudeDir, "skills", "my-skill", "SKILL.md")
	if got := readFile(t, target); got != skill {
		t.Fatalf("skill not copied: %q", got)
	}

	updated := strings.Replace(skill, "v1", "v2", 1)
	writeFile(t, filepath.Join(work, "skills", "my-skill", "SKILL.md"), updated)
	runGit(t, work, "commit", "-am", "update skill")
	runGit(t, work, "push", "origin", "HEAD:main")

	info, err := f.svc.CheckSubscription(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Behind != 1 {
		t.Fatalf("behind = %d, want 1", info.Behind)
	}
	if _, err := f.svc.UpdateSubscription(sub.ID); err != nil {
		t.Fatalf("UpdateSubscription: %v", err)
	}
	if got := readFile(t, target); got != updated {
		t.Errorf("skill not updated: %q", got)
	}

	if err := f.svc.RemoveSubscription(sub.ID); err != nil {
		t.Fatal(err)
	}
	report, err := f.svc.GetRulesReport()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Rules) != 0 {
		t.Errorf("subscription rules should be removed, %d rules remain", len(report.Rules))
	}
}
