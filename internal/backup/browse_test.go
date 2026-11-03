package backup_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/backup"
)

const (
	testFilePerm = 0o644
	testDirPerm  = 0o755
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), testDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), testFilePerm); err != nil {
		t.Fatal(err)
	}
}

func snapshotOf(t *testing.T, backupRoot, group string, owner backup.Owner, rel, content string) *backup.BackupInfo {
	t.Helper()
	writeTestFile(t, filepath.Join(owner.Dir, rel), content)
	info, err := backup.CreateSnapshot(filepath.Join(backupRoot, group), owner, []string{rel}, backup.ReasonSync, 0)
	if err != nil || info == nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	return info
}

func expectApperr(t *testing.T, err error, code string) {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) || ae.Code != code {
		t.Fatalf("expected apperr %s, got %v", code, err)
	}
}

func TestListAllCoversEveryKindAndSkipsCorruptEntries(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "backups")
	dir := filepath.Join(tmp, "target")

	snapshotOf(t, root, "rules/r1__d1", backup.Owner{ID: "rule:r1:d1", Name: "Instructions", Dir: dir}, "AGENTS.md", "rule")
	snapshotOf(t, root, "rules/r1___source", backup.Owner{ID: "rule:r1:_source", Name: "Instructions", Dir: dir}, "AGENTS.md", "source")
	repoDir := filepath.Join(tmp, "repo")
	writeTestFile(t, filepath.Join(repoDir, "notes.md"), "repo")
	if _, err := backup.CreateRepoBackup(root, repoDir, []string{"notes.md"}, backup.ReasonDiscardRepoEdit, 0); err != nil {
		t.Fatal(err)
	}
	legacyDir := filepath.Join(root, "old", "010125-010101PM")
	writeTestFile(t, filepath.Join(legacyDir, "a.txt"), "legacy")
	writeTestFile(t, filepath.Join(legacyDir, backup.ManifestFileName),
		`{"connectionId":"c1","profile":"Claude","targetPath":"C:/x","createdAt":"2020-01-01T00:00:00Z","reason":"sync","files":["a.txt"]}`)

	// Corrupt and partial entries must be skipped, not fail the listing.
	writeTestFile(t, filepath.Join(root, "rules", "r2__d1", "t", "010101-010101PM", backup.ManifestFileName), "{not json")
	writeTestFile(t, filepath.Join(root, "rules", "r2__d1", "t", "010102-010101PM", "x.txt"), "no manifest")
	writeTestFile(t, filepath.Join(root, "rules", "r3__d1", "t", "010101-010101PM", backup.ManifestFileName),
		`{"ownerId":"rule:r3:d1","name":"Gone files","targetPath":"C:/x","createdAt":"2021-01-01T00:00:00Z","files":["missing.md","../escape.md"]}`)

	list, err := backup.ListAll(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 4 {
		t.Fatalf("expected 4 valid backups, got %d: %+v", len(list), list)
	}
	kinds := map[string]string{}
	for _, e := range list {
		kinds[e.Kind] = e.ID
	}
	for _, kind := range []string{backup.KindRule, backup.KindSource, backup.KindRepo, backup.KindLegacy} {
		if kinds[kind] == "" {
			t.Errorf("kind %s missing from %v", kind, kinds)
		}
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].CreatedAt < list[i].CreatedAt {
			t.Errorf("not sorted newest first: %s before %s", list[i-1].CreatedAt, list[i].CreatedAt)
		}
	}
	if !strings.HasPrefix(kinds[backup.KindRule], "rules/r1__d1/target/") {
		t.Errorf("unexpected rule id %q", kinds[backup.KindRule])
	}

	empty, err := backup.ListAll(filepath.Join(tmp, "nothing"))
	if err != nil || len(empty) != 0 {
		t.Errorf("a missing backup folder lists nothing: %v %v", err, empty)
	}
}

func TestOpenValidatesIDsAndSelectsFiles(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "backups")
	dir := filepath.Join(tmp, "target")
	owner := backup.Owner{ID: "rule:r1:d1", Name: "Instructions", Dir: dir}
	group := filepath.Join(root, "rules", "r1__d1")
	writeTestFile(t, filepath.Join(dir, "AGENTS.md"), "v1")
	writeTestFile(t, filepath.Join(dir, "docs", "b.md"), "b")
	info, err := backup.CreateSnapshot(group, owner, []string{"AGENTS.md", "docs/b.md"}, backup.ReasonSync, 0)
	if err != nil || info == nil {
		t.Fatal(err)
	}

	id := "rules/r1__d1/target/" + info.ID
	entry, err := backup.Open(root, id)
	if err != nil || entry.Kind != backup.KindRule || entry.Size == 0 || len(entry.Files) != 2 {
		t.Fatalf("Open: %v %+v", err, entry)
	}

	for _, bad := range []string{"", "..", "../x", "rules/r1__d1/target/../../..", "rules/r1__d1", "a/b/c",
		`rules\r1__d1\target\x`, "rules/r1__d1/target/missing", "/rules/r1__d1/target/" + info.ID} {
		_, err := backup.Open(root, bad)
		expectApperr(t, err, apperr.BackupNotFound)
	}

	picked, err := entry.Select([]string{"docs/b.md", "docs/b.md"})
	if err != nil || len(picked) != 1 || picked[0] != "docs/b.md" {
		t.Fatalf("Select: %v %v", err, picked)
	}
	if _, err := entry.Select([]string{"../AGENTS.md"}); err == nil {
		t.Error("Select must reject paths outside the backup")
	}
	if _, err := entry.Select([]string{"other.md"}); err == nil {
		t.Error("Select must reject files that are not in the backup")
	}
	everything, err := entry.Select(nil)
	if err != nil || len(everything) != 2 {
		t.Errorf("Select(nil) must return all files: %v %v", err, everything)
	}
}

func TestSafeJoinRejectsEscapes(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	outside := filepath.Join(tmp, "outside")
	for _, d := range []string{root, outside, filepath.Join(root, "sub")} {
		if err := os.MkdirAll(d, testDirPerm); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{"../x", "a/../../x", "/abs", `C:\x`, "c:/x", `a\b`, "", "."} {
		_, err := backup.SafeJoin(root, rel)
		expectApperr(t, err, apperr.BackupUnsafePath)
	}
	got, err := backup.SafeJoin(root, "sub/new/file.md")
	if err != nil || got != filepath.Join(root, "sub", "new", "file.md") {
		t.Errorf("valid nested path rejected: %q %v", got, err)
	}

	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := backup.SafeJoin(root, "link/file.md"); err == nil {
		t.Error("a symlinked folder leading outside must be rejected")
	}
	writeTestFile(t, filepath.Join(outside, "real.md"), "x")
	if err := os.Symlink(filepath.Join(outside, "real.md"), filepath.Join(root, "alias.md")); err == nil {
		if _, err := backup.SafeJoin(root, "alias.md"); err == nil {
			t.Error("a symlinked destination file must be rejected")
		}
	}
}

func TestPrepareRestoreRefusesGitFolderForRepoBackups(t *testing.T) {
	tmp := t.TempDir()
	entry := backup.Entry{Kind: backup.KindRepo, Path: tmp}
	_, err := backup.PrepareRestore(entry, tmp, []string{".git/config"})
	expectApperr(t, err, apperr.BackupUnsafePath)
}
