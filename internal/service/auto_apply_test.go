package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agentsync/internal/gitops"
	"agentsync/internal/rules"
	"agentsync/internal/service"
	"agentsync/internal/subscriptions"
)

const (
	autoApplyFile = "skills/a.md"
	autoApplyDest = "a.md"
)

// autoApplyFixture is a subscription whose single rule copies a file into the claude location.
type autoApplyFixture struct {
	rulesFixture
	work   string
	subID  string
	target string

	mu     sync.Mutex
	events []service.AutoAppliedPayload
}

func newAutoApplyFixture(t *testing.T) *autoApplyFixture {
	t.Helper()
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := &autoApplyFixture{rulesFixture: newRulesFixture(t)}
	f.svc = service.New(func(eventName string, data any) {
		payload, ok := data.(service.AutoAppliedPayload)
		if eventName != "rules:auto_applied" || !ok {
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		f.events = append(f.events, payload)
	})
	bare, work := publishedRepo(t, map[string]string{autoApplyFile: "v1"})
	f.work = work
	sub, err := f.svc.AddSubscription(fileURL(bare), "Friend")
	if err != nil {
		t.Fatal(err)
	}
	f.subID = sub.ID
	rule := rules.Rule{
		Name:         "Friend file",
		Enabled:      true,
		Source:       rules.Source{Origin: rules.OriginSubscription, Subscription: sub.ID, Path: autoApplyFile},
		Destinations: []rules.Destination{{Location: "claude", Path: autoApplyDest, Enabled: true}},
	}
	if _, err := f.svc.SaveRule(rule, rules.ScopeLocal); err != nil {
		t.Fatal(err)
	}
	f.target = filepath.Join(f.claudeDir, autoApplyDest)
	return f
}

// setAutoApply flips the option through the settings API, keeping every other setting.
func (f *autoApplyFixture) setAutoApply(t *testing.T, on bool) {
	t.Helper()
	cfg, err := f.svc.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = f.svc.UpdateSettings(service.SettingsPayload{
		Language:             cfg.Language,
		Theme:                cfg.Theme,
		BackupPath:           cfg.BackupPath,
		BackupRetention:      cfg.BackupRetention,
		CheckIntervalMinutes: cfg.CheckIntervalMinutes,
		ConfirmBeforeSync:    cfg.ConfirmBeforeSync,
		AutoApplyUpdates:     on,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// pullIntoClone moves the subscription clone forward without applying any rule, the way a manual
// "git pull" would, so the rule shows a pending source change.
func (f *autoApplyFixture) pullIntoClone(t *testing.T, content string) {
	t.Helper()
	pushFiles(t, f.work, map[string]string{autoApplyFile: content}, "update "+content)
	runGit(t, subscriptions.Root(f.subID), "pull", "--ff-only")
}

func (f *autoApplyFixture) check(t *testing.T) {
	t.Helper()
	if _, ok := f.svc.TryBackgroundCheck(); !ok {
		t.Fatal("background check was skipped")
	}
}

func (f *autoApplyFixture) autoAppliedEvents() []service.AutoAppliedPayload {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]service.AutoAppliedPayload(nil), f.events...)
}

func TestAutoApplyWritesCleanChangeAndBacksUp(t *testing.T) {
	f := newAutoApplyFixture(t)
	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, f.target); got != "v1" {
		t.Fatalf("initial sync wrote %q", got)
	}
	f.pullIntoClone(t, "v2")
	f.setAutoApply(t, true)

	f.check(t)

	if got := readFile(t, f.target); got != "v2" {
		t.Fatalf("destination = %q, want v2", got)
	}
	cfg, _ := f.svc.GetConfig()
	if !backupHolds(t, cfg.BackupPath, "v1") {
		t.Errorf("no backup of the overwritten file under %s", cfg.BackupPath)
	}
	events := f.autoAppliedEvents()
	if len(events) != 1 || events[0].Count != 1 || len(events[0].Sources) != 1 || events[0].Sources[0] != "Friend" {
		t.Fatalf("events = %+v", events)
	}
}

func TestAutoApplyAppliesRulesCreatedAfterTheClone(t *testing.T) {
	f := newAutoApplyFixture(t)
	f.setAutoApply(t, true)

	f.check(t)

	if got := readFile(t, f.target); got != "v1" {
		t.Fatalf("destination = %q, want v1", got)
	}
}

func TestAutoApplyLeavesConflictsAndLocalEditsUntouched(t *testing.T) {
	f := newAutoApplyFixture(t)
	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, f.target, "my edit")
	f.pullIntoClone(t, "v2")
	f.setAutoApply(t, true)

	f.check(t)

	if got := readFile(t, f.target); got != "my edit" {
		t.Fatalf("conflicting file was overwritten: %q", got)
	}
	if events := f.autoAppliedEvents(); len(events) != 0 {
		t.Fatalf("nothing was written but events = %+v", events)
	}

	// A local edit without a new source version is not touched either.
	f2 := newAutoApplyFixture(t)
	if _, err := f2.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, f2.target, "my edit")
	f2.setAutoApply(t, true)
	f2.check(t)
	if got := readFile(t, f2.target); got != "my edit" {
		t.Fatalf("local edit was overwritten: %q", got)
	}
}

func TestAutoApplyIgnoresRepoAndLocalRules(t *testing.T) {
	f := newAutoApplyFixture(t)
	if _, err := f.svc.SaveRule(instructionsRule(), rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	localSource := filepath.Join(t.TempDir(), "notes.md")
	writeFile(t, localSource, "local notes")
	localRule := rules.Rule{
		Name:         "Local notes",
		Enabled:      true,
		Source:       rules.Source{Origin: rules.OriginLocal, Path: localSource},
		Destinations: []rules.Destination{{Location: "gemini", Path: "NOTES.md", Enabled: true}},
	}
	if _, err := f.svc.SaveRule(localRule, rules.ScopeLocal); err != nil {
		t.Fatal(err)
	}
	f.setAutoApply(t, true)

	f.check(t)

	for _, untouched := range []string{
		filepath.Join(f.claudeDir, "CLAUDE.md"),
		filepath.Join(f.geminiDir, "GEMINI.md"),
		filepath.Join(f.geminiDir, "NOTES.md"),
	} {
		if _, err := os.Stat(untouched); err == nil {
			t.Errorf("%s was written by automatic apply", untouched)
		}
	}
	if got := readFile(t, f.target); got != "v1" {
		t.Errorf("subscription rule not applied: %q", got)
	}
}

func TestAutoApplyIsOffByDefault(t *testing.T) {
	f := newAutoApplyFixture(t)
	f.check(t)
	if _, err := os.Stat(f.target); err == nil {
		t.Fatal("a pending rule was applied although the option is off")
	}
	if events := f.autoAppliedEvents(); len(events) != 0 {
		t.Fatalf("events = %+v", events)
	}

	f.setAutoApply(t, true)
	f.setAutoApply(t, false)
	f.check(t)
	if _, err := os.Stat(f.target); err == nil {
		t.Fatal("a pending rule was applied after the option was turned off")
	}
}

// backupHolds reports whether any file under dir has exactly the given content.
func backupHolds(t *testing.T, dir, content string) bool {
	t.Helper()
	found := false
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if data, readErr := os.ReadFile(path); readErr == nil && strings.TrimSpace(string(data)) == content {
			found = true
		}
		return nil
	})
	return found
}
