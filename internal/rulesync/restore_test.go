package rulesync_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
	"agentsync/internal/fsops"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
)

const (
	instructionsFile = "AGENTS.md"
	manifestPerm     = 0o644
)

// backupByOwner returns the newest listed backup of an owner.
func (e *env) backupByOwner(ownerID string) backup.Entry {
	e.t.Helper()
	list, err := backup.ListAll(e.backups)
	if err != nil {
		e.t.Fatal(err)
	}
	for _, entry := range list {
		if entry.OwnerID == ownerID {
			return entry
		}
	}
	e.t.Fatalf("no backup for owner %s in %+v", ownerID, list)
	return backup.Entry{}
}

func (e *env) restore(all []rules.Rule, entry backup.Entry, rels ...string) (rulesync.RestoreOutcome, error) {
	e.t.Helper()
	selected, err := entry.Select(rels)
	if err != nil {
		e.t.Fatal(err)
	}
	return e.engine.RestoreFiles(all, entry, selected)
}

// overwrittenInstructions syncs v1, then v2 so a backup of v1 exists and the target holds v2.
func overwrittenInstructions(t *testing.T) (*env, []rules.Rule, string, string) {
	t.Helper()
	e := newEnv(t)
	src := filepath.Join(e.repo, instructionsFile)
	target := filepath.Join(e.locA, instructionsFile)
	all := []rules.Rule{repoRule("r1", instructionsFile, dest("d1", aliasA, instructionsFile, rules.ModeTwoWay))}
	e.write(src, "v1")
	e.sync(all)
	e.write(src, "v2")
	if res := e.sync(all); res.Written != 1 || len(res.Backups) != 1 {
		t.Fatalf("expected an overwrite with backup, got %+v", res)
	}
	return e, all, src, target
}

func TestRestoreReturnsFileAndBacksUpCurrentVersion(t *testing.T) {
	e, all, src, target := overwrittenInstructions(t)
	entry := e.backupByOwner("rule:r1:d1")
	if entry.Kind != backup.KindRule {
		t.Fatalf("unexpected kind %s", entry.Kind)
	}

	outcome, err := e.restore(all, entry)
	if err != nil {
		t.Fatal(err)
	}
	if e.read(target) != "v1" || e.read(src) != "v2" {
		t.Fatalf("restore must only touch the destination: target=%q source=%q", e.read(target), e.read(src))
	}
	if outcome.Root != e.locA || len(outcome.Restored) != 1 {
		t.Fatalf("unexpected outcome %+v", outcome)
	}

	// The replaced version (v2) was backed up first, so the restore can be undone.
	undo, err := backup.Open(e.backups, outcome.UndoBackupID)
	if err != nil {
		t.Fatalf("undo backup not found: %v", err)
	}
	if undo.Reason != backup.ReasonRestore {
		t.Errorf("undo reason = %q", undo.Reason)
	}
	saved, err := os.ReadFile(filepath.Join(undo.Path, instructionsFile))
	if err != nil || string(saved) != "v2" {
		t.Fatalf("undo backup should hold the replaced v2: %q %v", saved, err)
	}
	undoOutcome, err := e.restore(all, undo)
	if err != nil || e.read(target) != "v2" {
		t.Fatalf("undoing the restore failed: %v, target=%q", err, e.read(target))
	}
	_ = undoOutcome
}

func TestRestoredFileIsReviewedNotOverwrittenBySync(t *testing.T) {
	e, all, src, target := overwrittenInstructions(t)
	entry := e.backupByOwner("rule:r1:d1")
	// Worst case: the restored content equals the stored baseline, which would otherwise
	// look like an untouched destination and be overwritten by the source on the next sync.
	hash, err := fsops.HashFile(filepath.Join(entry.Path, instructionsFile))
	if err != nil {
		t.Fatal(err)
	}
	e.engine.State.SetBaseline(rulesync.BaselineKey("r1", "d1"), instructionsFile, hash)

	if _, err := e.restore(all, entry); err != nil {
		t.Fatal(err)
	}
	rep := e.engine.Status(all)
	expectStatus(t, rep, "r1", "d1", instructionsFile, rulesync.StatusUntrackedDiff)
	if rep.Conflicts != 1 || rep.Pending != 0 {
		t.Fatalf("restored file must be a conflict to review: %+v", rep)
	}

	res := e.sync(all)
	if res.Written != 0 || res.Captured != 0 || res.Skipped != 1 {
		t.Fatalf("sync must skip the restored file, got %+v", res)
	}
	if e.read(target) != "v1" || e.read(src) != "v2" {
		t.Fatalf("sync changed files: target=%q source=%q", e.read(target), e.read(src))
	}

	// The user decides: keeping the source version is an explicit choice with its own backup.
	if _, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: instructionsFile, Choice: rulesync.UseSource}); err != nil {
		t.Fatal(err)
	}
	if e.read(target) != "v2" {
		t.Fatalf("use_source should rewrite the destination")
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", instructionsFile, rulesync.StatusInSync)
}

func TestRestoredFileMatchingSourceIsInSync(t *testing.T) {
	e, all, src, target := overwrittenInstructions(t)
	entry := e.backupByOwner("rule:r1:d1")
	e.write(src, "v1")

	if _, err := e.restore(all, entry); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", instructionsFile, rulesync.StatusInSync)
	if got := e.engine.State.GetBaseline(rulesync.BaselineKey("r1", "d1"), instructionsFile); got == "" {
		t.Fatal("a restored file identical to the source must be recorded as synced")
	}
	if e.read(target) != "v1" {
		t.Fatal("target should hold v1")
	}
}

func TestRestoreSelectedFilesOnly(t *testing.T) {
	e := newEnv(t)
	all := []rules.Rule{repoRule("r1", "skills", dest("d1", aliasA, "skills", rules.ModeOneWay))}
	e.write(filepath.Join(e.repo, "skills", "a.md"), "a1")
	e.write(filepath.Join(e.repo, "skills", "sub", "b.md"), "b1")
	e.sync(all)
	e.write(filepath.Join(e.repo, "skills", "a.md"), "a2")
	e.write(filepath.Join(e.repo, "skills", "sub", "b.md"), "b2")
	e.sync(all)
	entry := e.backupByOwner("rule:r1:d1")
	if len(entry.Files) != 2 {
		t.Fatalf("expected both files in the backup, got %v", entry.Files)
	}

	outcome, err := e.restore(all, entry, "sub/b.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(outcome.Restored) != 1 || outcome.Restored[0] != "sub/b.md" {
		t.Fatalf("unexpected restored files %v", outcome.Restored)
	}
	if e.read(filepath.Join(e.locA, "skills", "a.md")) != "a2" || e.read(filepath.Join(e.locA, "skills", "sub", "b.md")) != "b1" {
		t.Fatal("only the selected file should have been restored")
	}
	rep := e.engine.Status(all)
	expectStatus(t, rep, "r1", "d1", "sub/b.md", rulesync.StatusUntrackedDiff)
	expectStatus(t, rep, "r1", "d1", "a.md", rulesync.StatusInSync)

	outcome, err = e.restore(all, entry)
	if err != nil || len(outcome.Restored) != 2 {
		t.Fatalf("restoring all files: %v %+v", err, outcome)
	}
}

func TestRestoreRecreatesFileDeletedBySync(t *testing.T) {
	e := newEnv(t)
	all := []rules.Rule{repoRule("r1", "skills", dest("d1", aliasA, "skills", rules.ModeOneWay))}
	e.write(filepath.Join(e.repo, "skills", "a.md"), "a")
	e.write(filepath.Join(e.repo, "skills", "gone.md"), "gone")
	e.sync(all)
	if err := os.Remove(filepath.Join(e.repo, "skills", "gone.md")); err != nil {
		t.Fatal(err)
	}
	if res := e.sync(all); res.Deleted != 1 {
		t.Fatalf("expected a deletion, got %+v", res)
	}
	target := filepath.Join(e.locA, "skills", "gone.md")
	if e.exists(target) {
		t.Fatal("file should have been deleted")
	}

	entry := e.backupByOwner("rule:r1:d1")
	outcome, err := e.restore(all, entry)
	if err != nil {
		t.Fatal(err)
	}
	if e.read(target) != "gone" || outcome.UndoBackupID != "" {
		t.Fatalf("deleted file not restored (undo %q): %q", outcome.UndoBackupID, e.read(target))
	}
	// Nothing replaced, so no undo backup; the file is not managed anymore, so sync leaves it alone.
	e.sync(all)
	if e.read(target) != "gone" {
		t.Fatal("sync must not delete the restored file again")
	}
}

func TestRestoreRefusesUnmappedGoneAndLegacyOwners(t *testing.T) {
	e, all, _, target := overwrittenInstructions(t)
	entry := e.backupByOwner("rule:r1:d1")

	_, err := e.engine.RestoreFiles(nil, entry, entry.Files)
	expectCode(t, err, apperr.BackupNotRestorable)

	otherDest := []rules.Rule{repoRule("r1", instructionsFile, dest("d9", aliasA, instructionsFile, rules.ModeTwoWay))}
	_, err = e.engine.RestoreFiles(otherDest, entry, entry.Files)
	expectCode(t, err, apperr.BackupNotRestorable)

	delete(e.engine.Env.Locations, aliasA)
	_, err = e.engine.RestoreFiles(all, entry, entry.Files)
	expectCode(t, err, apperr.LocationUnmapped)
	e.engine.Env.Locations[aliasA] = e.locA

	if err := os.RemoveAll(e.locA); err != nil {
		t.Fatal(err)
	}
	_, err = e.engine.RestoreFiles(all, entry, entry.Files)
	expectCode(t, err, apperr.TargetNotFound)
	if e.exists(target) {
		t.Fatal("a missing location folder must not be recreated")
	}

	legacy := entry
	legacy.OwnerID = "c_old"
	_, err = e.engine.RestoreFiles(all, legacy, legacy.Files)
	expectCode(t, err, apperr.BackupNotRestorable)
}

func TestRestoreIgnoresTamperedManifest(t *testing.T) {
	e, all, _, target := overwrittenInstructions(t)
	entry := e.backupByOwner("rule:r1:d1")
	outside := filepath.Join(filepath.Dir(e.backups), "outside")
	if err := os.MkdirAll(outside, testDirPerm); err != nil {
		t.Fatal(err)
	}

	tampered := map[string]any{
		"ownerId":    entry.OwnerID,
		"name":       entry.Name,
		"targetPath": outside,
		"createdAt":  entry.CreatedAt,
		"reason":     entry.Reason,
		"files":      []string{instructionsFile, "../../escape.md", "/abs.md", "sub/../../up.md"},
	}
	data, _ := json.Marshal(tampered)
	if err := os.WriteFile(filepath.Join(entry.Path, backup.ManifestFileName), data, manifestPerm); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"escape.md", "up.md"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(entry.Path), name), []byte("x"), manifestPerm); err != nil {
			t.Fatal(err)
		}
	}

	reopened, err := backup.Open(e.backups, entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.Files) != 1 || reopened.Files[0] != instructionsFile {
		t.Fatalf("unsafe stored paths must be dropped, got %v", reopened.Files)
	}
	if _, err := reopened.Select([]string{"../../escape.md"}); err == nil {
		t.Fatal("selecting an escaping path must fail")
	}
	if _, err := e.engine.RestoreFiles(all, reopened, reopened.Files); err != nil {
		t.Fatal(err)
	}
	if e.read(target) != "v1" {
		t.Fatalf("restore must land in the rule destination, got %q", e.read(target))
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("restore wrote to the manifest path: %v", entries)
	}
}

func TestRestoreRejectsSymlinkedFolderEscape(t *testing.T) {
	e := newEnv(t)
	all := []rules.Rule{repoRule("r1", "skills", dest("d1", aliasA, "skills", rules.ModeOneWay))}
	e.write(filepath.Join(e.repo, "skills", "sub", "a.md"), "a1")
	e.sync(all)
	e.write(filepath.Join(e.repo, "skills", "sub", "a.md"), "a2")
	e.sync(all)
	entry := e.backupByOwner("rule:r1:d1")

	outside := filepath.Join(filepath.Dir(e.backups), "outside")
	if err := os.MkdirAll(outside, testDirPerm); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(e.locA, "skills", "sub")
	if err := os.RemoveAll(subDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, subDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := e.restore(all, entry)
	expectCode(t, err, apperr.BackupUnsafePath)
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("restore followed a symlink out of the destination: %v", entries)
	}
}

func TestRestoreSourceBackupShowsPendingSourceChange(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(e.repo, instructionsFile)
	target := filepath.Join(e.locA, instructionsFile)
	all := []rules.Rule{repoRule("r1", instructionsFile, dest("d1", aliasA, instructionsFile, rules.ModeTwoWay))}
	e.write(src, "v1")
	e.sync(all)
	e.write(target, "edited in target")
	if res := e.sync(all); res.Captured != 1 {
		t.Fatalf("expected a capture, got %+v", res)
	}
	entry := e.backupByOwner("rule:r1:" + rulesync.SourceBackupID)
	if entry.Kind != backup.KindSource {
		t.Fatalf("unexpected kind %s", entry.Kind)
	}

	if _, err := e.restore(all, entry); err != nil {
		t.Fatal(err)
	}
	if e.read(src) != "v1" || e.read(target) != "edited in target" {
		t.Fatalf("source=%q target=%q", e.read(src), e.read(target))
	}
	// The restored source is a visible pending change; it is applied only by the next sync.
	rep := e.engine.Status(all)
	expectStatus(t, rep, "r1", "d1", instructionsFile, rulesync.StatusSourceChanged)
	if rep.Pending != 1 {
		t.Fatalf("expected one pending change, got %+v", rep)
	}
}

func TestRestoreSourceBackupRefusedForReadOnlySource(t *testing.T) {
	e := newEnv(t)
	all := []rules.Rule{{
		ID: "r1", Name: "sub rule", Enabled: true, Kind: rules.KindAuto,
		Source:       rules.Source{Origin: rules.OriginSubscription, Subscription: "s_1", Path: instructionsFile},
		Destinations: []rules.Destination{dest("d1", aliasA, instructionsFile, rules.ModeTwoWay)},
	}}
	entry := backup.Entry{OwnerID: "rule:r1:" + rulesync.SourceBackupID, Files: []string{instructionsFile}}
	_, err := e.engine.RestoreFiles(all, entry, entry.Files)
	expectCode(t, err, apperr.SourceReadOnly)
}

func TestRestoreTransformDestinationReportsDriftForReview(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(e.repo, "mcp.json")
	target := filepath.Join(e.locA, "claude.json")
	e.write(src, `{"mcpServers": {"git": {"cmd": "git"}}}`)
	e.write(target, `{"mcpServers": {"mine": {"cmd": "x"}}}`)
	d := dest("d1", aliasA, "claude.json", rules.ModeTwoWay)
	d.Transform = &rules.Transform{Type: rules.TransformJSONMerge, Key: mcpKey}
	all := []rules.Rule{repoRule("r1", "mcp.json", d)}
	e.sync(all)
	original := e.backupByOwner("rule:r1:d1")

	if _, err := e.restore(all, original); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.read(target), `"mine"`) || strings.Contains(e.read(target), `"git"`) {
		t.Fatalf("expected the pre-sync file back:\n%s", e.read(target))
	}
	// The managed entry vanished from the file: drift, which a sync never overwrites.
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusDrift)
	if res := e.sync(all); res.Written != 0 || res.Skipped != 1 {
		t.Fatalf("sync must skip a drifted transform destination, got %+v", res)
	}
}

func TestBackupFolderConstantsMatchBackupPackage(t *testing.T) {
	if rulesync.RulesBackupDir != backup.RulesFolder || rulesync.SourceBackupID != backup.SourceSlotID {
		t.Fatal("rulesync and backup must agree on the rules backup layout")
	}
}
