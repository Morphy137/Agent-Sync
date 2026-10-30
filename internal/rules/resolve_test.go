package rules_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/rules"
)

type fixture struct {
	repo, loc, home, sub string
	env                  rules.Env
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	base := t.TempDir()
	f := fixture{
		repo: filepath.Join(base, "repo"),
		loc:  filepath.Join(base, "loc"),
		home: filepath.Join(base, "home"),
		sub:  filepath.Join(base, "sub"),
	}
	f.env = rules.Env{
		RepoRoot:          f.repo,
		Locations:         map[string]string{"claude": f.loc},
		HomeDir:           f.home,
		SubscriptionRoots: map[string]string{"s_1": f.sub},
	}
	return f
}

func relPaths(pairs []rules.Pair) []string {
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, p.RelPath)
	}
	return out
}

func TestResolveFileSource(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.repo, "AGENTS.md"), "hi")
	r := sampleRule()
	r.Source.Path = "AGENTS.md"
	d := rules.Destination{ID: "d_1", Location: "claude", Path: "sub/CLAUDE.md", Enabled: true}

	res, err := rules.Resolve(f.env, r, d)
	if err != nil {
		t.Fatal(err)
	}
	if res.SourceIsDir || res.Unmapped || res.SourceMissing {
		t.Fatalf("unexpected flags: %+v", res)
	}
	want := rules.Pair{
		RelPath:   "CLAUDE.md",
		SourceAbs: filepath.Join(f.repo, "AGENTS.md"),
		TargetAbs: filepath.Join(f.loc, "sub", "CLAUDE.md"),
	}
	if len(res.Pairs) != 1 || res.Pairs[0] != want {
		t.Fatalf("unexpected pairs: %+v", res.Pairs)
	}
	if res.TargetRoot != filepath.Join(f.loc, "sub") {
		t.Fatalf("unexpected target root %s", res.TargetRoot)
	}

	d.Ignore = []string{"CLAUDE.md"}
	res, err = rules.Resolve(f.env, r, d)
	if err != nil || len(res.Pairs) != 0 {
		t.Fatalf("ignored file should produce no pairs: %+v %v", res, err)
	}

	d.Path = ""
	d.Ignore = nil
	_, err = rules.Resolve(f.env, r, d)
	assertReason(t, err, apperr.InvalidRule, rules.ReasonFileDestNeedsPath)
}

func TestResolveFolderSource(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(f.repo, "skills")
	for _, rel := range []string{
		"a/SKILL.md", "a/notes.txt", "b/SKILL.md", "b/cache/x.bin", "c/secret.pem",
		"auth.json", "c/id_rsa", ".hidden", "keep.log", "ignored/one.md", "solo.md",
		".claude/settings.json", ".git/config", "a/.DS_Store", "Thumbs.db", "desktop.ini",
		"a/tmp.md" + fsops.TempFileSuffix, ".env", ".env.local", "c/.credentials.json",
	} {
		writeFile(t, filepath.Join(src, filepath.FromSlash(rel)), rel)
	}
	r := sampleRule()
	r.Exclude = []string{"*.txt", "cache", "keep.log"}
	d := rules.Destination{ID: "d_1", Location: "claude", Path: "skills", Enabled: true, Ignore: []string{"ignored", "solo.md"}}

	res, err := rules.Resolve(f.env, r, d)
	if err != nil {
		t.Fatal(err)
	}
	if !res.SourceIsDir || res.TargetRoot != filepath.Join(f.loc, "skills") {
		t.Fatalf("unexpected resolution %+v", res)
	}
	want := []string{".claude/settings.json", ".hidden", "a/SKILL.md", "b/SKILL.md"}
	if got := relPaths(res.Pairs); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	if res.Pairs[2].TargetAbs != filepath.Join(f.loc, "skills", "a", "SKILL.md") {
		t.Fatalf("unexpected target %s", res.Pairs[0].TargetAbs)
	}
}

func TestResolveSpecialCases(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.sub, "mcp.json"), "{}")
	r := sampleRule()

	t.Run("unmapped", func(t *testing.T) {
		res, err := rules.Resolve(f.env, r, rules.Destination{ID: "d", Location: "nowhere", Path: "x"})
		if err != nil || !res.Unmapped {
			t.Fatalf("expected unmapped, got %+v %v", res, err)
		}
	})
	t.Run("home alias", func(t *testing.T) {
		sr := r
		sr.Source = rules.Source{Origin: rules.OriginSubscription, Subscription: "s_1", Path: "mcp.json"}
		res, err := rules.Resolve(f.env, sr, rules.Destination{ID: "d", Location: rules.BuiltinHomeAlias, Path: ".claude.json"})
		if err != nil {
			t.Fatal(err)
		}
		if res.Pairs[0].TargetAbs != filepath.Join(f.home, ".claude.json") {
			t.Fatalf("unexpected target %s", res.Pairs[0].TargetAbs)
		}
	})
	t.Run("home unset is unmapped", func(t *testing.T) {
		env := f.env
		env.HomeDir = ""
		res, err := rules.Resolve(env, r, rules.Destination{ID: "d", Location: rules.BuiltinHomeAlias, Path: "x"})
		if err != nil || !res.Unmapped {
			t.Fatalf("expected unmapped, got %+v %v", res, err)
		}
	})
	t.Run("source missing", func(t *testing.T) {
		mr := r
		mr.Kind = rules.KindSkill
		res, err := rules.Resolve(f.env, mr, rules.Destination{ID: "d", Location: "claude", Path: "skills/x"})
		if err != nil || !res.SourceMissing || !res.SourceIsDir {
			t.Fatalf("expected missing folder source, got %+v %v", res, err)
		}
		if res.TargetRoot != filepath.Join(f.loc, "skills", "x") {
			t.Fatalf("unexpected root %s", res.TargetRoot)
		}
	})
	t.Run("dest escapes", func(t *testing.T) {
		_, err := rules.Resolve(f.env, r, rules.Destination{ID: "d", Location: "claude", Path: "../out"})
		assertReason(t, err, apperr.InvalidRule, rules.ReasonInvalidDestPath)
	})
	t.Run("relative location", func(t *testing.T) {
		env := f.env
		env.Locations = map[string]string{"claude": "relative/dir"}
		_, err := rules.Resolve(env, r, rules.Destination{ID: "d", Location: "claude", Path: "x"})
		assertReason(t, err, apperr.LocationInvalid, "")
	})
	t.Run("unknown subscription", func(t *testing.T) {
		sr := r
		sr.Source = rules.Source{Origin: rules.OriginSubscription, Subscription: "s_x", Path: "a"}
		_, err := rules.Resolve(f.env, sr, rules.Destination{ID: "d", Location: "claude", Path: "x"})
		assertReason(t, err, apperr.SubscriptionNotFound, "")
	})
	t.Run("transform on folder", func(t *testing.T) {
		writeFile(t, filepath.Join(f.repo, "skills", "a.md"), "a")
		_, err := rules.Resolve(f.env, r, rules.Destination{ID: "d", Location: "claude", Path: "x",
			Transform: &rules.Transform{Type: rules.TransformJSONMerge, Key: "k"}})
		assertReason(t, err, apperr.InvalidRule, rules.ReasonTransformNeedsFile)
	})
}

func TestIsExcluded(t *testing.T) {
	cases := map[string]bool{
		"a/b.pem":             true,
		"deep/.env.local":     true,
		"x/id_rsa.pub":        true,
		"node_modules/a/b.js": true,
		"src/main.go":         false,
		"docs/readme.md":      false,
	}
	for rel, want := range cases {
		if got := rules.IsExcluded(rel, []string{"node_modules"}); got != want {
			t.Errorf("%s: expected %v, got %v", rel, want, got)
		}
	}
}

func TestDetectCollisions(t *testing.T) {
	f := newFixture(t)
	file := func(rule, dest, target string) rules.ResolvedDestination {
		return rules.ResolvedDestination{RuleID: rule, DestID: dest, TargetRoot: filepath.Dir(target),
			Pairs: []rules.Pair{{RelPath: filepath.Base(target), SourceAbs: "x", TargetAbs: target}}}
	}
	folder := func(rule, dest, root string, rels ...string) rules.ResolvedDestination {
		rd := rules.ResolvedDestination{RuleID: rule, DestID: dest, TargetRoot: root, SourceIsDir: true}
		for _, rel := range rels {
			rd.Pairs = append(rd.Pairs, rules.Pair{RelPath: rel, SourceAbs: "x", TargetAbs: filepath.Join(root, filepath.FromSlash(rel))})
		}
		return rd
	}
	ref := func(rule, dest string) rules.DestRef { return rules.DestRef{RuleID: rule, DestID: dest} }

	t.Run("none", func(t *testing.T) {
		got := rules.DetectCollisions(f.env, []rules.ResolvedDestination{
			file("r1", "d1", filepath.Join(f.loc, "a.md")),
			file("r2", "d1", filepath.Join(f.loc, "b.md")),
			{RuleID: "r3", DestID: "d1", Unmapped: true},
		}, []string{filepath.Join(f.repo, "a.md")}, nil)
		if len(got) != 0 {
			t.Fatalf("expected no collisions, got %+v", got)
		}
	})

	t.Run("same target", func(t *testing.T) {
		target := filepath.Join(f.loc, "a.md")
		got := rules.DetectCollisions(f.env, []rules.ResolvedDestination{
			file("r2", "d1", target), file("r1", "d1", target),
		}, nil, nil)
		want := []rules.Collision{{Type: rules.CollisionSameTarget, Path: target, Refs: []rules.DestRef{ref("r1", "d1"), ref("r2", "d1")}}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	})

	t.Run("case conflict", func(t *testing.T) {
		got := rules.DetectCollisions(f.env, []rules.ResolvedDestination{
			file("r1", "d1", filepath.Join(f.loc, "a.md")), file("r2", "d1", filepath.Join(f.loc, "A.md")),
		}, nil, nil)
		if len(got) != 1 || got[0].Type != rules.CollisionCaseConflict || len(got[0].Refs) != 2 {
			t.Fatalf("expected case conflict, got %+v", got)
		}
		got = rules.DetectCollisions(f.env, []rules.ResolvedDestination{
			folder("r1", "d1", filepath.Join(f.loc, "skills"), "a.md", "A.md"),
		}, nil, nil)
		if len(got) != 1 || got[0].Type != rules.CollisionCaseConflict || len(got[0].Refs) != 1 {
			t.Fatalf("expected case conflict within one destination, got %+v", got)
		}
	})

	t.Run("folder overlap", func(t *testing.T) {
		root := filepath.Join(f.loc, "skills")
		inner := filepath.Join(root, "extra.md")
		got := rules.DetectCollisions(f.env, []rules.ResolvedDestination{
			folder("r1", "d1", root, "a.md"),
			file("r2", "d1", inner),
			folder("r3", "d1", filepath.Join(root, "nested")),
		}, nil, nil)
		want := []rules.Collision{
			{Type: rules.CollisionFolderOverlap, Path: inner, Refs: []rules.DestRef{ref("r1", "d1"), ref("r2", "d1")}},
			{Type: rules.CollisionFolderOverlap, Path: filepath.Join(root, "nested"), Refs: []rules.DestRef{ref("r1", "d1"), ref("r3", "d1")}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	})

	t.Run("legacy overlap", func(t *testing.T) {
		legacyDir := filepath.Join(f.loc, "legacy")
		legacy := []config.Connection{{ID: "c1", TargetPath: legacyDir}, {ID: "c2", TargetPath: filepath.Join(f.home, "x", "y")}}
		got := rules.DetectCollisions(f.env, []rules.ResolvedDestination{
			file("r1", "d1", filepath.Join(legacyDir, "a.md")),
			folder("r2", "d1", filepath.Join(f.home, "x")),
		}, nil, legacy)
		want := []rules.Collision{
			{Type: rules.CollisionFolderOverlap, Path: filepath.Join(f.home, "x", "y"), Refs: []rules.DestRef{{ConnectionID: "c2"}, ref("r2", "d1")}},
			{Type: rules.CollisionFolderOverlap, Path: filepath.Join(legacyDir, "a.md"), Refs: []rules.DestRef{{ConnectionID: "c1"}, ref("r1", "d1")}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	})

	t.Run("cycle", func(t *testing.T) {
		srcDir := filepath.Join(f.loc, "src")
		got := rules.DetectCollisions(f.env, []rules.ResolvedDestination{
			file("r1", "d1", filepath.Join(srcDir, "a.md")),
			folder("r2", "d1", f.loc),
		}, []string{srcDir}, nil)
		want := []rules.Collision{
			{Type: rules.CollisionCycle, Path: srcDir, Refs: []rules.DestRef{ref("r2", "d1")}},
			{Type: rules.CollisionCycle, Path: filepath.Join(srcDir, "a.md"), Refs: []rules.DestRef{ref("r1", "d1")}},
			{Type: rules.CollisionFolderOverlap, Path: filepath.Join(srcDir, "a.md"), Refs: []rules.DestRef{ref("r1", "d1"), ref("r2", "d1")}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	})

	t.Run("inside repo", func(t *testing.T) {
		target := filepath.Join(f.repo, "out", "a.md")
		got := rules.DetectCollisions(f.env, []rules.ResolvedDestination{file("r1", "d1", target)}, nil, nil)
		want := []rules.Collision{{Type: rules.CollisionInsideRepo, Path: target, Refs: []rules.DestRef{ref("r1", "d1")}}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %+v, got %+v", want, got)
		}
	})

	t.Run("deterministic", func(t *testing.T) {
		in := []rules.ResolvedDestination{
			file("r1", "d1", filepath.Join(f.repo, "a.md")),
			file("r2", "d1", filepath.Join(f.repo, "a.md")),
			folder("r3", "d1", f.repo),
		}
		first := rules.DetectCollisions(f.env, in, []string{f.repo}, nil)
		for i := 0; i < 10; i++ {
			if got := rules.DetectCollisions(f.env, in, []string{f.repo}, nil); !reflect.DeepEqual(got, first) {
				t.Fatalf("non-deterministic output:\n%+v\n%+v", first, got)
			}
		}
	})
}

func TestWalkSourceSymlinks(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "src")
	writeFile(t, filepath.Join(src, "real.md"), "r")
	writeFile(t, filepath.Join(src, "dir", "inner.md"), "i")
	writeFile(t, filepath.Join(base, "outside.md"), "o")
	if err := os.Symlink(filepath.Join(src, "real.md"), filepath.Join(src, "inside-link.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(base, "outside.md"), filepath.Join(src, "escape.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(src, "dir"), filepath.Join(src, "dirlink")); err != nil {
		t.Fatal(err)
	}
	files, err := rules.WalkSource(src)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(files))
	for rel := range files {
		got = append(got, rel)
	}
	sort.Strings(got)
	want := []string{"dir/inner.md", "inside-link.md", "real.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}
