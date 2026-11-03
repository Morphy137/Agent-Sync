package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"agentsync/internal/gitops"
	"agentsync/internal/orchestrator"
	"agentsync/internal/rules"
	"agentsync/internal/service"
	"agentsync/internal/subscriptions"
)

const (
	// readDeadline is far above a normal read and far below the time a blocked read would wait.
	readDeadline = 5 * time.Second
	// checkDeadline bounds how long a test waits for a background check to finish.
	checkDeadline = 30 * time.Second
	// gateStartDeadline bounds how long a test waits for a gated fetch to begin.
	gateStartDeadline = 20 * time.Second
	// gatePollInterval is how often waiters look at gate marker files.
	gatePollInterval = 20 * time.Millisecond
	// unreachableRemote points at a closed local port: a network failure without real network access.
	unreachableRemote = "http://127.0.0.1:1/repo.git"
)

// fetchGate turns a repository's remote into a slow one: git fetch starts the remote command, which
// signals "started" and then waits until the test opens the gate. No production hook is involved.
type fetchGate struct {
	started string
	release string
	once    sync.Once
}

func newFetchGate(t *testing.T, repoDir string) *fetchGate {
	t.Helper()
	dir := t.TempDir()
	gate := &fetchGate{started: filepath.Join(dir, "started"), release: filepath.Join(dir, "release")}
	script := filepath.Join(dir, "gate.sh")
	body := "#!/bin/sh\n" +
		"touch '" + filepath.ToSlash(gate.started) + "'\n" +
		"i=0\n" +
		"while [ ! -f '" + filepath.ToSlash(gate.release) + "' ] && [ $i -lt 1200 ]; do sleep 0.05; i=$((i+1)); done\n" +
		"exec git upload-pack \"$@\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repoDir, "config", "remote.origin.uploadpack", "sh '"+filepath.ToSlash(script)+"'")
	t.Cleanup(gate.open)
	return gate
}

func (g *fetchGate) open() {
	g.once.Do(func() { _ = os.WriteFile(g.release, nil, 0o644) })
}

func (g *fetchGate) waitStarted(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(gateStartDeadline)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(g.started); err == nil {
			return
		}
		time.Sleep(gatePollInterval)
	}
	t.Fatal("the gated fetch never started")
}

type checkOutcome struct {
	plan *orchestrator.SyncPlan
	ok   bool
}

func startBackgroundCheck(svc *service.Service) <-chan checkOutcome {
	done := make(chan checkOutcome, 1)
	go func() {
		plan, ok := svc.TryBackgroundCheck()
		done <- checkOutcome{plan: plan, ok: ok}
	}()
	return done
}

func awaitCheck(t *testing.T, done <-chan checkOutcome) checkOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-time.After(checkDeadline):
		t.Fatal("background check did not finish")
		return checkOutcome{}
	}
}

// publishedRepo creates a bare remote seeded with files plus the working clone used to push more commits.
func publishedRepo(t *testing.T, files map[string]string) (bare, work string) {
	t.Helper()
	tmp := t.TempDir()
	bare = filepath.Join(tmp, "published.git")
	work = filepath.Join(tmp, "work")
	runGit(t, tmp, "init", "--bare", "-b", "main", bare)
	runGit(t, tmp, "clone", bare, work)
	runGit(t, work, "config", "user.name", "Friend")
	runGit(t, work, "config", "user.email", "friend@example.com")
	pushFiles(t, work, files, "initial")
	return bare, work
}

func pushFiles(t *testing.T, work string, files map[string]string, message string) {
	t.Helper()
	for rel, content := range files {
		writeFile(t, filepath.Join(work, filepath.FromSlash(rel)), content)
	}
	runGit(t, work, "add", "-A")
	runGit(t, work, "commit", "-m", message)
	runGit(t, work, "push", "origin", "HEAD:main")
}

// gitRootFixture is a rules fixture whose root folder is a repository with a pushed remote.
type gitRootFixture struct {
	rulesFixture
	bare string
}

func newGitRootFixture(t *testing.T, gitignore string) gitRootFixture {
	t.Helper()
	f := newRulesFixture(t)
	bare := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, filepath.Dir(bare), "init", "--bare", "-b", "main", bare)
	runGit(t, f.root, "init", "-b", "main")
	runGit(t, f.root, "config", "user.name", "Tester")
	runGit(t, f.root, "config", "user.email", "tester@example.com")
	if gitignore != "" {
		writeFile(t, filepath.Join(f.root, ".gitignore"), gitignore)
	}
	runGit(t, f.root, "add", "-A")
	runGit(t, f.root, "commit", "-m", "initial")
	runGit(t, f.root, "remote", "add", "origin", bare)
	runGit(t, f.root, "push", "-u", "origin", "main")
	return gitRootFixture{rulesFixture: f, bare: bare}
}

func TestBackgroundCheckDoesNotBlockReadsDuringFetch(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := newGitRootFixture(t, "")
	subBare, _ := publishedRepo(t, map[string]string{"skills/a.md": "a"})
	if _, err := f.svc.AddSubscription(fileURL(subBare), ""); err != nil {
		t.Fatal(err)
	}
	gate := newFetchGate(t, f.root)

	done := startBackgroundCheck(f.svc)
	gate.waitStarted(t)

	reads := []struct {
		name string
		call func() error
	}{
		{"GetConfig", func() error { _, err := f.svc.GetConfig(); return err }},
		{"ListProfiles", func() error { _, err := f.svc.ListProfiles(); return err }},
		{"GetStatus", func() error { _, err := f.svc.GetStatus(); return err }},
		{"ListSubscriptions", func() error { _, err := f.svc.ListSubscriptions(); return err }},
		{"GetGitInfo", func() error { _, err := f.svc.GetGitInfo(); return err }},
	}
	for _, read := range reads {
		finished := make(chan error, 1)
		start := time.Now()
		go func() { finished <- read.call() }()
		select {
		case err := <-finished:
			if err != nil {
				t.Errorf("%s: %v", read.name, err)
			}
			t.Logf("%s returned in %v while the fetch was in progress", read.name, time.Since(start))
		case <-time.After(readDeadline):
			t.Errorf("%s blocked while a background fetch was in progress", read.name)
		}
	}
	select {
	case <-done:
		t.Fatal("the check finished although its fetch was still gated")
	default:
	}

	gate.open()
	if outcome := awaitCheck(t, done); !outcome.ok || outcome.plan == nil || outcome.plan.IsOffline {
		t.Fatalf("check after the fetch finished: %+v", outcome)
	}
}

func TestBackgroundCheckReportsOffline(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := newGitRootFixture(t, "")
	subBare, _ := publishedRepo(t, map[string]string{"skills/a.md": "a"})
	sub, err := f.svc.AddSubscription(fileURL(subBare), "")
	if err != nil {
		t.Fatal(err)
	}

	online, ok := f.svc.TryBackgroundCheck()
	if !ok || online.IsOffline {
		t.Fatalf("reachable remote reported offline: %+v ok=%v", online, ok)
	}

	runGit(t, f.root, "remote", "set-url", "origin", unreachableRemote)
	runGit(t, subscriptions.Root(sub.ID), "remote", "set-url", "origin", unreachableRemote)
	offline, ok := f.svc.TryBackgroundCheck()
	if !ok || !offline.IsOffline || offline.Direction != orchestrator.DirectionOffline {
		t.Fatalf("unreachable remote not reported offline: %+v ok=%v", offline, ok)
	}
	infos, err := f.svc.ListSubscriptions()
	if err != nil || len(infos) != 1 || infos[0].Update == nil || !infos[0].Update.Offline {
		t.Fatalf("subscription not marked offline: %+v %v", infos, err)
	}
}

func TestBackgroundCheckAutoUpdatesSubscriptionsAndReportsBlocked(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := newRulesFixture(t)
	var mu sync.Mutex
	var blocked []string
	f.svc = service.New(func(eventName string, data any) {
		if eventName != "subscription:blocked" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if name, ok := data.(string); ok {
			blocked = append(blocked, name)
		}
	})
	bare, work := publishedRepo(t, map[string]string{"skills/a.md": "v1"})
	sub, err := f.svc.AddSubscription(fileURL(bare), "Friend")
	if err != nil {
		t.Fatal(err)
	}
	clonedFile := filepath.Join(subscriptions.Root(sub.ID), "skills", "a.md")

	// Without auto-update the check only reports what is incoming.
	pushFiles(t, work, map[string]string{"skills/a.md": "v2"}, "second")
	if _, ok := f.svc.TryBackgroundCheck(); !ok {
		t.Fatal("check skipped")
	}
	infos, _ := f.svc.ListSubscriptions()
	if len(infos) != 1 || infos[0].Update == nil || infos[0].Update.Behind != 1 {
		t.Fatalf("incoming commit not reported: %+v", infos)
	}
	if got := readFile(t, clonedFile); got != "v1" {
		t.Fatalf("clone changed without auto-update: %q", got)
	}

	// With auto-update the fetched commit is applied without another fetch.
	if _, err := f.svc.SetSubscriptionAutoUpdate(sub.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.svc.TryBackgroundCheck(); !ok {
		t.Fatal("check skipped")
	}
	if got := readFile(t, clonedFile); got != "v2" {
		t.Fatalf("auto-update did not apply: %q", got)
	}

	// Local edits in the clone block the update and are reported.
	writeFile(t, clonedFile, "local edit")
	pushFiles(t, work, map[string]string{"skills/a.md": "v3"}, "third")
	if _, ok := f.svc.TryBackgroundCheck(); !ok {
		t.Fatal("check skipped")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(blocked) != 1 || blocked[0] != "Friend" {
		t.Fatalf("blocked events = %v", blocked)
	}
	if got := readFile(t, clonedFile); got != "local edit" {
		t.Fatalf("blocked update overwrote local edit: %q", got)
	}
}

func TestBackgroundCheckDiscardsResultWhenRootChangesDuringFetch(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := newGitRootFixture(t, "")
	gate := newFetchGate(t, f.root)
	otherRoot := t.TempDir()

	done := startBackgroundCheck(f.svc)
	gate.waitStarted(t)
	if _, err := f.svc.SetRootPath(otherRoot); err != nil {
		t.Fatal(err)
	}
	gate.open()

	if outcome := awaitCheck(t, done); outcome.ok {
		t.Fatalf("a check for the previous root must be discarded: %+v", outcome)
	}
	cfg, err := f.svc.GetConfig()
	if err != nil || filepath.Clean(cfg.RootPath) != filepath.Clean(otherRoot) {
		t.Fatalf("root = %q, %v", cfg.RootPath, err)
	}
}

func TestUserSyncWaitsForBackgroundFetchWithoutDeadlock(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := newGitRootFixture(t, "")
	gate := newFetchGate(t, f.root)

	done := startBackgroundCheck(f.svc)
	gate.waitStarted(t)

	planned := make(chan error, 1)
	go func() {
		_, err := f.svc.PlanSync()
		planned <- err
	}()
	time.Sleep(10 * gatePollInterval)
	gate.open()

	select {
	case err := <-planned:
		if err != nil {
			t.Fatalf("PlanSync: %v", err)
		}
	case <-time.After(checkDeadline):
		t.Fatal("PlanSync never finished while a background fetch was running")
	}
	awaitCheck(t, done)
}

func TestIgnoredRulesFileDoesNotBreakSync(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := newGitRootFixture(t, rules.RulesFileName+"\n")
	if _, err := f.svc.SaveRule(instructionsRule(), rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(f.root, rules.RulesFileName)); err != nil {
		t.Fatalf("rules file was not written: %v", err)
	}

	plan, err := f.svc.PlanSync()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append(append([]string{}, plan.CommitPreview...), plan.DirtyOutside...) {
		if strings.Contains(path, rules.RulesFileName) {
			t.Fatalf("ignored rules file reported as a pending change: %v", plan)
		}
	}
	for _, upload := range plan.Uploads {
		if strings.Contains(upload.FullPath, rules.RulesFileName) {
			t.Fatalf("ignored rules file reported as an upload: %v", plan.Uploads)
		}
	}
	if _, err := f.svc.ExecuteSync(plan.Fingerprint, nil); err != nil {
		t.Fatalf("ExecuteSync: %v", err)
	}

	// A change made at a destination is captured into the repository source and pushed.
	writeFile(t, filepath.Join(f.geminiDir, "GEMINI.md"), "# edited in gemini\n")
	plan, err = f.svc.PlanSync()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.ExecuteSync(plan.Fingerprint, nil); err != nil {
		t.Fatalf("ExecuteSync after edit: %v", err)
	}
	if got := runGit(t, f.root, "--git-dir", f.bare, "show", "main:shared/AGENTS.md"); got != "# edited in gemini" {
		t.Errorf("remote source = %q", got)
	}
	if tracked := runGit(t, f.root, "ls-files"); strings.Contains(tracked, rules.RulesFileName) {
		t.Errorf("ignored rules file was committed:\n%s", tracked)
	}
	if status := runGit(t, f.root, "status", "--porcelain"); status != "" {
		t.Errorf("worktree should be clean, got %q", status)
	}
}

func TestSubscriptionListingsHideOwnerRulesFile(t *testing.T) {
	if !gitops.IsInstalled() {
		t.Skip("git is not installed")
	}
	f := newRulesFixture(t)
	bare, _ := publishedRepo(t, map[string]string{
		rules.RulesFileName: "{}",
		"agents/AGENTS.md":  "# agents",
	})
	sub, err := f.svc.AddSubscription(fileURL(bare), "")
	if err != nil {
		t.Fatal(err)
	}

	tree, err := f.svc.GetSubscriptionTree(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundAgents := false
	for _, entry := range tree {
		if entry.Path == rules.RulesFileName {
			t.Errorf("tree lists the owner's rules file: %+v", tree)
		}
		foundAgents = foundAgents || entry.Path == "agents"
	}
	if !foundAgents {
		t.Errorf("tree lost regular entries: %+v", tree)
	}

	for _, rel := range []string{"", "."} {
		dir, err := f.svc.ListFileDirectory(sub.ID, rel)
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(dir))
		for _, entry := range dir {
			names = append(names, entry.Name)
		}
		if len(names) != 1 || names[0] != "agents" {
			t.Errorf("browser root %q lists %v, want only agents", rel, names)
		}
	}
}
