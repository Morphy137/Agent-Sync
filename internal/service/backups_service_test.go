package service_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

const (
	statusEvent = "status:changed"
	rulesEvent  = "rules:changed"
)

func expectAppErr(t *testing.T, err error, code string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("expected apperr %s, got %v", code, err)
	}
}

type eventLog struct {
	mu     sync.Mutex
	events map[string]int
}

func (l *eventLog) emit(name string, _ any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events[name]++
}

func (l *eventLog) count(name string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.events[name]
}

// overwrittenRuleFixture syncs a rule twice so a backup of the first version exists.
func overwrittenRuleFixture(t *testing.T) (rulesFixture, *eventLog, rules.Rule) {
	t.Helper()
	f := newRulesFixture(t)
	rule := instructionsRule()
	rule.Destinations = rule.Destinations[:1]
	rule.Destinations[0].Mode = rules.ModeOneWay
	saved, err := f.svc.SaveRule(rule, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	log := &eventLog{events: map[string]int{}}
	f.svc = service.New(log.emit)
	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.root, "shared", "AGENTS.md"), "# changed\n")
	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	return f, log, saved.Rule
}

func TestListAndRestoreRuleBackup(t *testing.T) {
	f, log, rule := overwrittenRuleFixture(t)
	target := filepath.Join(f.claudeDir, "CLAUDE.md")

	list, err := f.svc.ListBackups()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListBackups: %v %+v", err, list)
	}
	item := list[0]
	if item.Kind != backup.KindRule || item.Owner != rule.Name || item.Destination != "claude/CLAUDE.md" ||
		item.FileCount != 1 || item.TotalSize == 0 || !item.Restorable || item.Reason != backup.ReasonSync {
		t.Fatalf("unexpected entry %+v", item)
	}

	// A renamed rule shows its current name.
	renamed := rule
	renamed.Name = "Renamed rule"
	if _, err := f.svc.SaveRule(renamed, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	list, _ = f.svc.ListBackups()
	if list[0].Owner != "Renamed rule" {
		t.Fatalf("expected the current rule name, got %q", list[0].Owner)
	}

	before := log.count(rulesEvent)
	res, err := f.svc.RestoreBackup(item.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "# shared instructions\n" {
		t.Fatalf("restored content = %q", got)
	}
	if res.Target != f.claudeDir || len(res.Restored) != 1 || res.UndoBackupID == "" {
		t.Fatalf("unexpected result %+v", res)
	}
	if log.count(rulesEvent) <= before || log.count(statusEvent) == 0 {
		t.Fatalf("restore must emit rules and status events: %v", log.events)
	}

	// The undo backup is listed and holds the version the restore replaced.
	list, _ = f.svc.ListBackups()
	if len(list) != 2 || list[0].ID != res.UndoBackupID || list[0].Reason != backup.ReasonRestore {
		t.Fatalf("undo backup missing from the list: %+v", list)
	}

	// The restored file shows up as a difference to review, and a sync leaves it untouched.
	report, err := f.svc.GetRulesReport()
	if err != nil {
		t.Fatal(err)
	}
	if report.Conflicts != 1 || report.Pending != 0 {
		t.Fatalf("restored file must be a conflict to review: pending=%d conflicts=%d", report.Pending, report.Conflicts)
	}
	if _, err := f.svc.SyncRules(); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "# shared instructions\n" {
		t.Fatalf("sync overwrote the restored file: %q", got)
	}

	if _, err := f.svc.RestoreBackup(res.UndoBackupID, nil); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "# changed\n" {
		t.Fatalf("undo restore content = %q", got)
	}
}

func TestRestoreBackupValidation(t *testing.T) {
	f, _, rule := overwrittenRuleFixture(t)
	list, _ := f.svc.ListBackups()
	id := list[0].ID

	for _, bad := range []string{"", "../x", "rules/x/y/z", "nope/nope"} {
		_, err := f.svc.RestoreBackup(bad, nil)
		expectAppErr(t, err, apperr.BackupNotFound)
	}
	_, err := f.svc.RestoreBackup(id, []string{"../CLAUDE.md"})
	expectAppErr(t, err, apperr.BackupNotFound)
	_, err = f.svc.RestoreBackup(id, []string{"other.md"})
	expectAppErr(t, err, apperr.BackupNotFound)

	if err := f.svc.DeleteRule(rule.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = f.svc.ListBackups()
	if len(list) != 1 || list[0].Restorable || list[0].Owner != rule.Name {
		t.Fatalf("a backup of a deleted rule is listed as not restorable with its stored name: %+v", list)
	}
	_, err = f.svc.RestoreBackup(id, nil)
	expectAppErr(t, err, apperr.BackupNotRestorable)
}

func TestRestoreBackupRefusesUnmappedLocation(t *testing.T) {
	f, _, _ := overwrittenRuleFixture(t)
	list, _ := f.svc.ListBackups()
	if _, err := f.svc.RemoveLocation("claude"); err != nil {
		t.Fatal(err)
	}
	_, err := f.svc.RestoreBackup(list[0].ID, nil)
	expectAppErr(t, err, apperr.LocationUnmapped)
}

func TestRestoreRepoBackupPutsEditsBack(t *testing.T) {
	f := newRulesFixture(t)
	cfg, err := f.svc.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(f.root, "notes", "todo.md")
	writeFile(t, notes, "my manual edit\n")
	if _, err := backup.CreateRepoBackup(cfg.BackupPath, f.root, []string{"notes/todo.md"}, backup.ReasonDiscardRepoEdit, 0); err != nil {
		t.Fatal(err)
	}
	writeFile(t, notes, "committed version\n")

	list, err := f.svc.ListBackups()
	if err != nil || len(list) != 1 || list[0].Kind != backup.KindRepo || !list[0].Restorable {
		t.Fatalf("ListBackups: %v %+v", err, list)
	}
	res, err := f.svc.RestoreBackup(list[0].ID, []string{"notes/todo.md"})
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, notes); got != "my manual edit\n" {
		t.Fatalf("repo file not restored: %q", got)
	}
	if res.Target != f.root || res.UndoBackupID == "" || !strings.HasPrefix(res.UndoBackupID, backup.RepoSnapshotID+"/") {
		t.Fatalf("unexpected result %+v", res)
	}
	undo, err := backup.Open(cfg.BackupPath, res.UndoBackupID)
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(filepath.Join(undo.Path, "notes", "todo.md"))
	if strings.ReplaceAll(string(saved), "\r\n", "\n") != "committed version\n" {
		t.Fatalf("undo backup should hold the replaced file: %q", saved)
	}
}

func TestListBackupsEmptyAndLegacy(t *testing.T) {
	f := newRulesFixture(t)
	list, err := f.svc.ListBackups()
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("empty backup folder must list an empty slice: %v %v", err, list)
	}
	cfg, _ := f.svc.GetConfig()
	legacy := filepath.Join(cfg.BackupPath, "Claude", "010125-010101PM")
	writeFile(t, filepath.Join(legacy, "a.txt"), "old")
	writeFile(t, filepath.Join(legacy, backup.ManifestFileName),
		`{"connectionId":"c1","profile":"Claude","targetPath":"C:/old","createdAt":"2020-01-01T00:00:00Z","reason":"sync","files":["a.txt"]}`)
	list, err = f.svc.ListBackups()
	if err != nil || len(list) != 1 || list[0].Kind != backup.KindLegacy || list[0].Restorable || list[0].Owner != "Claude" {
		t.Fatalf("legacy backup listing: %v %+v", err, list)
	}
	_, err = f.svc.RestoreBackup(list[0].ID, nil)
	expectAppErr(t, err, apperr.BackupNotRestorable)
}
