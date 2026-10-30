package rulesync_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/state"
)

func TestFolderRuleLifecycle(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "skills", "x", "SKILL.md"), "v1")
	e.write(filepath.Join(e.repo, "skills", "y.md"), "y")
	e.write(filepath.Join(e.repo, "skills", "secret.pem"), "nope")
	all := []rules.Rule{repoRule("r1", "skills", dest("d1", aliasA, "skills", rules.ModeTwoWay))}

	rep := e.engine.Status(all)
	if rep.Pending != 2 || rep.Conflicts != 0 {
		t.Fatalf("expected 2 pending, got %+v", rep)
	}
	expectStatus(t, rep, "r1", "d1", "x/SKILL.md", rulesync.StatusNewInSource)
	if rep.Rules[0].DetectedKind != rules.KindFolder || rep.Rules[0].Scope != rules.ScopeRepo {
		t.Fatalf("unexpected rule metadata %+v", rep.Rules[0])
	}

	res := e.sync(all)
	if res.Written != 2 || len(res.Backups) != 0 {
		t.Fatalf("unexpected result %+v", res)
	}
	target := filepath.Join(e.locA, "skills", "x", "SKILL.md")
	if e.read(target) != "v1" || e.exists(filepath.Join(e.locA, "skills", "secret.pem")) {
		t.Fatalf("unexpected target contents")
	}
	if e.engine.State.GetBaseline("rule:r1:d1", "x/SKILL.md") == "" {
		t.Fatalf("baseline not recorded")
	}
	rep = e.engine.Status(all)
	if rep.Pending != 0 {
		t.Fatalf("expected in sync, got %+v", rep)
	}

	// The state is persisted in the config dir.
	loaded, err := state.Load()
	if err != nil || loaded.GetBaseline("rule:r1:d1", "y.md") == "" {
		t.Fatalf("state not saved: %v", err)
	}

	// Source change -> source_changed -> written with a backup.
	e.write(filepath.Join(e.repo, "skills", "x", "SKILL.md"), "v2")
	expectStatus(t, e.engine.Status(all), "r1", "d1", "x/SKILL.md", rulesync.StatusSourceChanged)
	res = e.sync(all)
	if res.Written != 1 || len(res.Backups) != 1 {
		t.Fatalf("expected one write with backup, got %+v", res)
	}
	if !strings.HasPrefix(res.Backups[0], rulesync.BackupRootFor(e.backups, "r1", "d1")) {
		t.Fatalf("backup outside rule backup root: %s", res.Backups[0])
	}
	if e.read(filepath.Join(res.Backups[0], "x", "SKILL.md")) != "v1" {
		t.Fatalf("backup should contain previous content")
	}
	if e.read(target) != "v2" {
		t.Fatalf("target not updated")
	}

	// Target removed -> target_missing -> restored.
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "x/SKILL.md", rulesync.StatusTargetMissing)
	e.sync(all)
	if e.read(target) != "v2" {
		t.Fatalf("target not restored")
	}

	// Source file deleted -> source_deleted -> target deleted with backup, empty dir removed.
	if err := os.Remove(filepath.Join(e.repo, "skills", "x", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "x/SKILL.md", rulesync.StatusSourceDeleted)
	res = e.sync(all)
	if res.Deleted != 1 || len(res.Backups) != 1 || e.exists(target) || e.exists(filepath.Dir(target)) {
		t.Fatalf("expected deletion with backup, got %+v", res)
	}
	if e.engine.State.GetBaseline("rule:r1:d1", "x/SKILL.md") != "" {
		t.Fatalf("baseline should be removed after delete")
	}
}

func TestSourceDeletedButTargetEditedIsConflict(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "skills", "a.md"), "a")
	all := []rules.Rule{repoRule("r1", "skills", dest("d1", aliasA, "skills", rules.ModeTwoWay))}
	e.sync(all)
	os.Remove(filepath.Join(e.repo, "skills", "a.md"))
	e.write(filepath.Join(e.locA, "skills", "a.md"), "edited")
	expectStatus(t, e.engine.Status(all), "r1", "d1", "a.md", rulesync.StatusConflict)
	res := e.sync(all)
	if res.Skipped != 1 || !e.exists(filepath.Join(e.locA, "skills", "a.md")) {
		t.Fatalf("conflict must be skipped, got %+v", res)
	}

	// use_source on a deleted source removes the destination file.
	if _, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "a.md", Choice: rulesync.UseSource}); err != nil {
		t.Fatal(err)
	}
	if e.exists(filepath.Join(e.locA, "skills", "a.md")) {
		t.Fatalf("use_source should delete the destination")
	}
}

func TestCaptureUpdatesOtherDestinations(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(e.repo, "agents", "AGENTS.md")
	e.write(src, "v1")
	all := []rules.Rule{repoRule("r1", "agents/AGENTS.md",
		dest("d1", aliasA, "CLAUDE.md", rules.ModeTwoWay),
		dest("d2", aliasB, "GEMINI.md", rules.ModeTwoWay),
	)}
	e.sync(all)

	e.write(filepath.Join(e.locA, "CLAUDE.md"), "edited in A")
	rep := e.engine.Status(all)
	expectStatus(t, rep, "r1", "d1", "CLAUDE.md", rulesync.StatusTargetChanged)
	expectStatus(t, rep, "r1", "d2", "GEMINI.md", rulesync.StatusInSync)

	res := e.sync(all)
	if res.Captured != 1 || res.Written != 1 {
		t.Fatalf("expected capture + propagation, got %+v", res)
	}
	if !reflect.DeepEqual(res.CapturedRepoPaths, []string{"agents/AGENTS.md"}) {
		t.Fatalf("unexpected captured paths %v", res.CapturedRepoPaths)
	}
	if e.read(src) != "edited in A" || e.read(filepath.Join(e.locB, "GEMINI.md")) != "edited in A" {
		t.Fatalf("capture did not propagate")
	}
	if len(res.Backups) != 2 {
		t.Fatalf("expected source and destination backups, got %v", res.Backups)
	}
	if rep := e.engine.Status(all); rep.Pending != 0 || rep.Conflicts != 0 {
		t.Fatalf("expected clean status, got %+v", rep)
	}
}

func TestCaptureSkippedWhenDestinationsDisagree(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "a.md"), "v1")
	all := []rules.Rule{repoRule("r1", "a.md",
		dest("d1", aliasA, "a.md", rules.ModeTwoWay),
		dest("d2", aliasB, "a.md", rules.ModeTwoWay),
	)}
	e.sync(all)
	e.write(filepath.Join(e.locA, "a.md"), "A")
	e.write(filepath.Join(e.locB, "a.md"), "B")
	res := e.sync(all)
	if res.Captured != 0 || res.Skipped != 2 || e.read(filepath.Join(e.repo, "a.md")) != "v1" {
		t.Fatalf("diverging captures must be skipped, got %+v", res)
	}
}

func TestOneWayAndSubscriptionDrift(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "a.md"), "repo")
	e.write(filepath.Join(e.sub, "b.md"), "sub")
	sub := rules.Rule{
		ID: "r2", Name: "sub", Enabled: true, Scope: rules.ScopeLocal,
		Source:       rules.Source{Origin: rules.OriginSubscription, Subscription: "s_1", Path: "b.md"},
		Destinations: []rules.Destination{dest("d1", aliasA, "b.md", rules.ModeTwoWay)},
	}
	all := []rules.Rule{repoRule("r1", "a.md", dest("d1", aliasA, "a.md", rules.ModeOneWay)), sub}
	e.sync(all)

	e.write(filepath.Join(e.locA, "a.md"), "local edit")
	e.write(filepath.Join(e.locA, "b.md"), "local edit")
	rep := e.engine.Status(all)
	expectStatus(t, rep, "r1", "d1", "a.md", rulesync.StatusDrift)
	expectStatus(t, rep, "r2", "d1", "b.md", rulesync.StatusDrift)
	if rep.Conflicts != 2 {
		t.Fatalf("drift should count as conflicts, got %+v", rep)
	}
	res := e.sync(all)
	if res.Captured != 0 || res.Skipped != 2 || e.read(filepath.Join(e.repo, "a.md")) != "repo" {
		t.Fatalf("drift must not be captured, got %+v", res)
	}

	// use_target on a read-only subscription source is refused.
	_, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r2", DestID: "d1", RelPath: "b.md", Choice: rulesync.UseTarget})
	expectCode(t, err, apperr.SourceReadOnly)
	_, _, err = e.engine.Resolve(all, rulesync.Resolution{RuleID: "r2", DestID: "d1", RelPath: "b.md", Choice: rulesync.UseCustom, Content: "x"})
	expectCode(t, err, apperr.SourceReadOnly)

	// use_source restores the destination.
	if _, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "a.md", Choice: rulesync.UseSource}); err != nil {
		t.Fatal(err)
	}
	if e.read(filepath.Join(e.locA, "a.md")) != "repo" {
		t.Fatalf("use_source did not restore")
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "a.md", rulesync.StatusInSync)
}

func TestCreateOnly(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "settings.json"), "defaults")
	e.write(filepath.Join(e.locA, "settings.json"), "user settings")
	all := []rules.Rule{repoRule("r1", "settings.json",
		dest("d1", aliasA, "settings.json", rules.ModeCreateOnly),
		dest("d2", aliasB, "settings.json", rules.ModeCreateOnly),
	)}
	rep := e.engine.Status(all)
	expectStatus(t, rep, "r1", "d1", "settings.json", rulesync.StatusInSync)
	expectStatus(t, rep, "r1", "d2", "settings.json", rulesync.StatusNewInSource)
	res := e.sync(all)
	if res.Written != 1 || e.read(filepath.Join(e.locA, "settings.json")) != "user settings" {
		t.Fatalf("create_only must not overwrite, got %+v", res)
	}
	e.write(filepath.Join(e.repo, "settings.json"), "new defaults")
	expectStatus(t, e.engine.Status(all), "r1", "d2", "settings.json", rulesync.StatusInSync)
}

func TestConflictAndUntrackedDiff(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "a.md"), "repo")
	e.write(filepath.Join(e.locA, "a.md"), "pre-existing")
	all := []rules.Rule{repoRule("r1", "a.md", dest("d1", aliasA, "a.md", rules.ModeTwoWay))}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "a.md", rulesync.StatusUntrackedDiff)
	if res := e.sync(all); res.Skipped != 1 || res.Written != 0 {
		t.Fatalf("untracked diff must be skipped, got %+v", res)
	}

	// use_target adopts the destination content into the repo source.
	_, repoPath, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "a.md", Choice: rulesync.UseTarget})
	if err != nil || repoPath != "a.md" || e.read(filepath.Join(e.repo, "a.md")) != "pre-existing" {
		t.Fatalf("use_target failed: %v %q", err, repoPath)
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "a.md", rulesync.StatusInSync)

	// Both sides edited -> conflict; custom content writes both sides.
	e.write(filepath.Join(e.repo, "a.md"), "repo 2")
	e.write(filepath.Join(e.locA, "a.md"), "local 2")
	expectStatus(t, e.engine.Status(all), "r1", "d1", "a.md", rulesync.StatusConflict)
	_, repoPath, err = e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "a.md", Choice: rulesync.UseCustom, Content: "merged"})
	if err != nil || repoPath != "a.md" {
		t.Fatalf("custom failed: %v", err)
	}
	if e.read(filepath.Join(e.repo, "a.md")) != "merged" || e.read(filepath.Join(e.locA, "a.md")) != "merged" {
		t.Fatalf("custom content not written to both sides")
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "a.md", rulesync.StatusInSync)
}

func TestKeepLocal(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "skills", "a.md"), "a")
	e.write(filepath.Join(e.repo, "skills", "b.md"), "b")
	all := []rules.Rule{repoRule("r1", "skills", dest("d1", aliasA, "skills", rules.ModeTwoWay))}
	e.sync(all)
	e.write(filepath.Join(e.locA, "skills", "a.md"), "mine")
	e.write(filepath.Join(e.repo, "skills", "a.md"), "theirs")

	updated, repoPath, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "a.md", Choice: rulesync.KeepLocal})
	if err != nil || updated == nil || repoPath != "" {
		t.Fatalf("keep_local failed: %v", err)
	}
	if !reflect.DeepEqual(updated.Destinations[0].Ignore, []string{"a.md"}) || len(all[0].Destinations[0].Ignore) != 0 {
		t.Fatalf("expected ignore added on a copy, got %v", updated.Destinations[0].Ignore)
	}
	all = []rules.Rule{*updated}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "a.md", rulesync.StatusIgnored)

	// Ignored files are never deleted even if the source disappears.
	os.Remove(filepath.Join(e.repo, "skills", "a.md"))
	res := e.sync(all)
	if res.Deleted != 0 || e.read(filepath.Join(e.locA, "skills", "a.md")) != "mine" {
		t.Fatalf("ignored file touched: %+v", res)
	}
}

func TestBlockedUnmappedDisabledAndMissing(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "a.md"), "a")
	e.write(filepath.Join(e.repo, "b.md"), "b")
	disabledDest := dest("d2", aliasA, "z.md", rules.ModeTwoWay)
	disabledDest.Enabled = false
	all := []rules.Rule{
		repoRule("r1", "a.md", dest("d1", aliasA, "same.md", rules.ModeTwoWay), disabledDest),
		repoRule("r2", "b.md", dest("d1", aliasA, "same.md", rules.ModeTwoWay)),
		repoRule("r3", "a.md", dest("d1", "unknown", "a.md", rules.ModeTwoWay)),
		repoRule("r4", "missing.md", dest("d1", aliasB, "m.md", rules.ModeTwoWay)),
	}
	rep := e.engine.Status(all)
	if len(rep.Collisions) != 1 || rep.Collisions[0].Type != rules.CollisionSameTarget {
		t.Fatalf("expected one same_target collision, got %+v", rep.Collisions)
	}
	for _, c := range []struct{ rule, dest, state string }{
		{"r1", "d1", rulesync.DestBlocked},
		{"r1", "d2", rulesync.DestDisabled},
		{"r2", "d1", rulesync.DestBlocked},
		{"r3", "d1", rulesync.DestUnmapped},
		{"r4", "d1", rulesync.DestSourceMissing},
	} {
		if state, _ := statuses(t, rep, c.rule, c.dest); state != c.state {
			t.Fatalf("%s/%s: expected %s, got %s", c.rule, c.dest, c.state, state)
		}
	}
	if !reflect.DeepEqual(rep.Unmapped, []string{"unknown"}) {
		t.Fatalf("unexpected unmapped %v", rep.Unmapped)
	}
	res := e.sync(all)
	if res.Written != 0 || res.Skipped != 2 || e.exists(filepath.Join(e.locA, "same.md")) {
		t.Fatalf("blocked destinations must not be written, got %+v", res)
	}
	_, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "same.md", Choice: rulesync.UseSource})
	expectCode(t, err, apperr.CollisionDetected)
	_, _, err = e.engine.Resolve(all, rulesync.Resolution{RuleID: "r3", DestID: "d1", RelPath: "a.md", Choice: rulesync.UseSource})
	expectCode(t, err, apperr.LocationUnmapped)
	_, _, err = e.engine.Resolve(all, rulesync.Resolution{RuleID: "nope", DestID: "d1", RelPath: "a.md", Choice: rulesync.UseSource})
	expectCode(t, err, apperr.RuleNotFound)
}

func TestLegacyConnectionBlocksRule(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "a.md"), "a")
	e.engine.Legacy = []config.Connection{{ID: "c1", TargetPath: e.locA}}
	all := []rules.Rule{repoRule("r1", "a.md", dest("d1", aliasA, "a.md", rules.ModeTwoWay))}
	if state, _ := statuses(t, e.engine.Status(all), "r1", "d1"); state != rulesync.DestBlocked {
		t.Fatalf("expected blocked by legacy connection, got %s", state)
	}
}

func TestTransformDestination(t *testing.T) {
	e := newEnv(t)
	src := filepath.Join(e.repo, "mcp.json")
	target := filepath.Join(e.locA, "claude.json")
	e.write(src, `{"mcpServers": {"git": {"cmd": "git"}}}`)
	e.write(target, `{"theme": "dark", "mcpServers": {"mine": {"cmd": "x"}}}`)
	d := dest("d1", aliasA, "claude.json", rules.ModeTwoWay)
	d.Transform = &rules.Transform{Type: rules.TransformJSONMerge, Key: mcpKey}
	all := []rules.Rule{repoRule("r1", "mcp.json", d)}

	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusNewInSource)
	res := e.sync(all)
	if res.Written != 1 || len(res.Backups) != 1 {
		t.Fatalf("expected merge with backup, got %+v", res)
	}
	merged := e.read(target)
	if !strings.Contains(merged, `"theme": "dark"`) || !strings.Contains(merged, `"mine"`) || !strings.Contains(merged, `"git"`) {
		t.Fatalf("merge lost content:\n%s", merged)
	}
	if got := e.engine.State.GetBaseline(rulesync.ManagedKey("r1", "d1"), rulesync.ManagedEntriesRel); got != "git" {
		t.Fatalf("managed entries not stored, got %q", got)
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusInSync)

	// Unrelated edits in the destination file are not drift.
	e.write(target, strings.Replace(merged, `"dark"`, `"light"`, 1))
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusInSync)

	// Source change re-renders and removes entries no longer in source.
	e.write(src, `{"mcpServers": {"fs": {"cmd": "fs"}}}`)
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusSourceChanged)
	e.sync(all)
	out := e.read(target)
	if strings.Contains(out, `"git"`) || !strings.Contains(out, `"fs"`) || !strings.Contains(out, `"light"`) {
		t.Fatalf("unexpected render:\n%s", out)
	}

	// Editing a managed entry is drift: never uploaded.
	e.write(target, strings.Replace(out, `"cmd": "fs"`, `"cmd": "hacked"`, 1))
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusDrift)
	if res := e.sync(all); res.Skipped != 1 || res.Captured != 0 {
		t.Fatalf("drift must be skipped, got %+v", res)
	}
	_, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "claude.json", Choice: rulesync.UseTarget})
	expectCode(t, err, apperr.TransformFailed)

	diff, err := e.engine.Diff(all, "r1", "d1", "claude.json")
	if err != nil || !diff.Transform || !strings.Contains(diff.Left, `"cmd": "fs"`) || !strings.Contains(diff.Right, "hacked") {
		t.Fatalf("unexpected transform diff %+v %v", diff, err)
	}

	if _, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "claude.json", Choice: rulesync.UseSource}); err != nil {
		t.Fatal(err)
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusInSync)

	// Custom content edits the source and re-renders.
	_, repoPath, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "claude.json",
		Choice: rulesync.UseCustom, Content: `{"mcpServers": {"fs": {"cmd": "fs2"}}}`})
	if err != nil || repoPath != "mcp.json" || !strings.Contains(e.read(target), "fs2") {
		t.Fatalf("custom transform failed: %v", err)
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusInSync)
}

func TestTransformEntryConflictAndTakeover(t *testing.T) {
	e := newEnv(t)
	target := filepath.Join(e.locA, "claude.json")
	e.write(filepath.Join(e.repo, "mcp.json"), `{"mcpServers": {"git": {"cmd": "ours"}}}`)
	e.write(target, `{"mcpServers": {"git": {"cmd": "theirs"}}}`)
	d := dest("d1", aliasA, "claude.json", "")
	d.Transform = &rules.Transform{Type: rules.TransformJSONMerge, Key: mcpKey}
	all := []rules.Rule{repoRule("r1", "mcp.json", d)}

	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusConflict)
	if res := e.sync(all); res.Skipped != 1 || !strings.Contains(e.read(target), "theirs") {
		t.Fatalf("conflicting entry must not be overwritten, got %+v", res)
	}
	if _, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "claude.json", Choice: rulesync.UseSource}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.read(target), "ours") {
		t.Fatalf("use_source should take over the entry")
	}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusInSync)
}

func TestTransformAlreadyInSyncRecordsBaseline(t *testing.T) {
	e := newEnv(t)
	target := filepath.Join(e.locA, "claude.json")
	e.write(filepath.Join(e.repo, "mcp.json"), `{"mcpServers": {"git": {"cmd": "git"}}}`)
	out, _, _, _ := fakeTransformer{}.Render(rules.Transform{Type: rules.TransformJSONMerge, Key: mcpKey},
		[]byte(`{"mcpServers": {"git": {"cmd": "git"}}}`), nil, nil)
	e.write(target, string(out))
	d := dest("d1", aliasA, "claude.json", "")
	d.Transform = &rules.Transform{Type: rules.TransformJSONMerge, Key: mcpKey}
	all := []rules.Rule{repoRule("r1", "mcp.json", d)}
	expectStatus(t, e.engine.Status(all), "r1", "d1", "claude.json", rulesync.StatusInSync)
	res := e.sync(all)
	if res.Written != 0 || e.engine.State.GetBaseline("rule:r1:d1", "claude.json") == "" {
		t.Fatalf("expected baseline without write, got %+v", res)
	}
}

func TestDiff(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "skills", "a.md"), "left")
	e.write(filepath.Join(e.locA, "skills", "a.md"), "right")
	e.write(filepath.Join(e.repo, "skills", "bin.dat"), "a\x00b")
	e.write(filepath.Join(e.repo, "skills", "bad.txt"), "\xff\xfe")
	e.write(filepath.Join(e.repo, "skills", "big.txt"), strings.Repeat("x", rulesync.MaxDiffBytes+1))
	all := []rules.Rule{repoRule("r1", "skills", dest("d1", aliasA, "skills", rules.ModeTwoWay))}

	diff, err := e.engine.Diff(all, "r1", "d1", "a.md")
	if err != nil {
		t.Fatal(err)
	}
	want := rulesync.Diff{LeftLabel: "source", RightLabel: "destination", Left: "left", Right: "right"}
	if diff != want {
		t.Fatalf("expected %+v, got %+v", want, diff)
	}
	for _, rel := range []string{"bin.dat", "bad.txt", "big.txt"} {
		diff, err := e.engine.Diff(all, "r1", "d1", rel)
		if err != nil || !diff.Binary || diff.Left != "" || diff.Right != "" {
			t.Fatalf("%s: expected binary diff, got %+v %v", rel, diff, err)
		}
	}
	_, err = e.engine.Diff(all, "r1", "d1", "../escape")
	expectCode(t, err, apperr.InvalidRule)
}

func TestExcludedFileIsNotExposedThroughResolve(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "skills", "a.md"), "a")
	e.write(filepath.Join(e.repo, "skills", "token.key"), "secret")
	all := []rules.Rule{repoRule("r1", "skills", dest("d1", aliasA, "skills", rules.ModeTwoWay))}
	_, _, err := e.engine.Resolve(all, rulesync.Resolution{RuleID: "r1", DestID: "d1", RelPath: "token.key", Choice: rulesync.UseSource})
	expectCode(t, err, apperr.SourceNotFound)
	if e.exists(filepath.Join(e.locA, "skills", "token.key")) {
		t.Fatalf("secret file must never be written")
	}
}

func TestBaselineKeysAndBackupRoot(t *testing.T) {
	if rulesync.BaselineKey("r", "d") != "rule:r:d" || rulesync.ManagedKey("r", "d") != "rule:r:d#managed" {
		t.Fatalf("unexpected keys")
	}
	root := rulesync.BackupRootFor("B", "r:1", "d/2")
	if root != filepath.Join("B", rulesync.RulesBackupDir, "r_1__d_2") {
		t.Fatalf("unexpected backup root %s", root)
	}
}

func TestDetectKindHook(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "skills", "x", "SKILL.md"), "s")
	e.engine.DetectKind = func(string) string { return rules.KindSkill }
	all := []rules.Rule{repoRule("r1", "skills/x", dest("d1", aliasA, "skills/x", rules.ModeTwoWay))}
	if got := e.engine.Status(all).Rules[0].DetectedKind; got != rules.KindSkill {
		t.Fatalf("expected skill, got %s", got)
	}
}

func TestDotFilesInFolderSourceAreSynced(t *testing.T) {
	e := newEnv(t)
	e.write(filepath.Join(e.repo, "profile", ".claude", "settings.json"), "{}")
	e.write(filepath.Join(e.repo, "profile", ".agents", "AGENTS.md"), "agents")
	e.write(filepath.Join(e.repo, "profile", ".git", "HEAD"), "ref")
	e.write(filepath.Join(e.repo, "profile", ".env"), "SECRET=1")
	all := []rules.Rule{repoRule("r1", "profile", dest("d1", aliasA, "out", rules.ModeTwoWay))}
	res := e.sync(all)
	if res.Written != 2 {
		t.Fatalf("expected 2 dot files written, got %+v", res)
	}
	if e.read(filepath.Join(e.locA, "out", ".claude", "settings.json")) != "{}" ||
		e.read(filepath.Join(e.locA, "out", ".agents", "AGENTS.md")) != "agents" {
		t.Fatalf("dot files not synced")
	}
	if e.exists(filepath.Join(e.locA, "out", ".git")) || e.exists(filepath.Join(e.locA, "out", ".env")) {
		t.Fatalf(".git and secrets must never be synced")
	}
}
