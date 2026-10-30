package rulesync_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/rulesync"
	"agentsync/internal/state"
)

const (
	testPerm    = 0o644
	testDirPerm = 0o755
	aliasA      = "a"
	aliasB      = "b"
	mcpKey      = "mcpServers"
)

// fakeTransformer is a minimal json_merge implementation for tests.
type fakeTransformer struct{}

func decodeObject(data []byte) (map[string]any, error) {
	obj := map[string]any{}
	if len(data) == 0 {
		return obj, nil
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, apperr.Wrap(apperr.TransformFailed, err)
	}
	return obj, nil
}

func entriesOf(obj map[string]any, key string) map[string]any {
	if m, ok := obj[key].(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func (fakeTransformer) Render(spec rules.Transform, source, existing []byte, prevManaged []string) ([]byte, []string, []byte, error) {
	src, err := decodeObject(source)
	if err != nil {
		return nil, nil, nil, err
	}
	dst, err := decodeObject(existing)
	if err != nil {
		return nil, nil, nil, err
	}
	srcEntries := entriesOf(src, spec.Key)
	dstEntries := entriesOf(dst, spec.Key)
	owned := map[string]bool{}
	for _, n := range prevManaged {
		owned[n] = true
	}
	names := make([]string, 0, len(srcEntries))
	for n := range srcEntries {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if _, exists := dstEntries[n]; exists && !owned[n] {
			return nil, nil, nil, apperr.New(apperr.TransformFailed).WithParam("reason", "entry_conflict").WithParam("entry", n)
		}
	}
	for n := range owned {
		if _, keep := srcEntries[n]; !keep {
			delete(dstEntries, n)
		}
	}
	for _, n := range names {
		dstEntries[n] = srcEntries[n]
	}
	dst[spec.Key] = dstEntries
	out, _ := json.MarshalIndent(dst, "", "  ")
	view, _ := json.Marshal(srcEntries)
	return append(out, '\n'), names, view, nil
}

func (fakeTransformer) SourceView(spec rules.Transform, source []byte) ([]byte, error) {
	src, err := decodeObject(source)
	if err != nil {
		return nil, err
	}
	return json.Marshal(entriesOf(src, spec.Key))
}

func (fakeTransformer) TargetView(spec rules.Transform, existing []byte, managed []string) ([]byte, error) {
	dst, err := decodeObject(existing)
	if err != nil {
		return nil, err
	}
	entries := entriesOf(dst, spec.Key)
	view := map[string]any{}
	for _, n := range managed {
		if v, ok := entries[n]; ok {
			view[n] = v
		}
	}
	return json.Marshal(view)
}

type env struct {
	t       *testing.T
	repo    string
	locA    string
	locB    string
	sub     string
	backups string
	engine  *rulesync.Engine
}

func newEnv(t *testing.T) *env {
	t.Helper()
	base := t.TempDir()
	t.Setenv(config.ConfigEnvVar, filepath.Join(base, "config"))
	e := &env{
		t:       t,
		repo:    filepath.Join(base, "repo"),
		locA:    filepath.Join(base, "locA"),
		locB:    filepath.Join(base, "locB"),
		sub:     filepath.Join(base, "sub"),
		backups: filepath.Join(base, "backups"),
	}
	for _, d := range []string{e.repo, e.locA, e.locB, e.sub} {
		if err := os.MkdirAll(d, testDirPerm); err != nil {
			t.Fatal(err)
		}
	}
	e.engine = &rulesync.Engine{
		Env: rules.Env{
			RepoRoot:          e.repo,
			Locations:         map[string]string{aliasA: e.locA, aliasB: e.locB},
			HomeDir:           filepath.Join(base, "home"),
			SubscriptionRoots: map[string]string{"s_1": e.sub},
		},
		State:       state.NewState(),
		BackupRoot:  e.backups,
		Retention:   config.DefaultBackupRetention,
		Transformer: fakeTransformer{},
	}
	return e
}

func (e *env) write(p, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), testDirPerm); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), testPerm); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) read(p string) string {
	e.t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		e.t.Fatalf("read %s: %v", p, err)
	}
	return string(data)
}

func (e *env) exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (e *env) sync(all []rules.Rule) rulesync.ApplyResult {
	e.t.Helper()
	res, err := e.engine.Sync(all)
	if err != nil {
		e.t.Fatalf("sync: %v", err)
	}
	return res
}

func dest(id, alias, p, mode string) rules.Destination {
	return rules.Destination{ID: id, Location: alias, Path: p, Mode: mode, Enabled: true}
}

func repoRule(id, srcPath string, dests ...rules.Destination) rules.Rule {
	return rules.Rule{
		ID:           id,
		Name:         "rule " + id,
		Enabled:      true,
		Source:       rules.Source{Origin: rules.OriginRepo, Path: srcPath},
		Kind:         rules.KindAuto,
		Destinations: dests,
		Scope:        rules.ScopeRepo,
	}
}

// statuses returns relPath -> status for one destination.
func statuses(t *testing.T, rep rulesync.Report, ruleID, destID string) (string, map[string]string) {
	t.Helper()
	for _, rs := range rep.Rules {
		for _, ds := range rs.Destinations {
			if ds.RuleID == ruleID && ds.DestID == destID {
				out := map[string]string{}
				for _, f := range ds.Files {
					out[f.RelPath] = f.Status
				}
				return ds.State, out
			}
		}
	}
	t.Fatalf("destination %s/%s not in report", ruleID, destID)
	return "", nil
}

func expectStatus(t *testing.T, rep rulesync.Report, ruleID, destID, rel, want string) {
	t.Helper()
	_, files := statuses(t, rep, ruleID, destID)
	if files[rel] != want {
		t.Fatalf("%s/%s %s: expected %s, got %q (all: %v)", ruleID, destID, rel, want, files[rel], files)
	}
}

func expectCode(t *testing.T, err error, code string) {
	t.Helper()
	appErr, ok := err.(*apperr.Error)
	if !ok || appErr.Code != code {
		t.Fatalf("expected apperr %s, got %v", code, err)
	}
}
