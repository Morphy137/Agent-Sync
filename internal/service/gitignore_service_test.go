package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/service"
)

func newGitignoreService(t *testing.T) (*service.Service, string) {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("AGENTSYNC_CONFIG_DIR", filepath.Join(tempDir, "config"))
	root := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(root, "skills", "private"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".DS_Store\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := service.New(func(string, any) {})
	if _, err := svc.SetRootPath(root); err != nil {
		t.Fatal(err)
	}
	return svc, root
}

func TestGitignore_RecommendedPatternsAreMergedOnce(t *testing.T) {
	svc, _ := newGitignoreService(t)

	before, err := svc.GetGitignore()
	if err != nil {
		t.Fatal(err)
	}
	if len(before.MissingRecommended) == 0 {
		t.Fatal("expected recommended patterns to be missing")
	}
	for _, p := range before.MissingRecommended {
		if p == ".DS_Store" {
			t.Fatal(".DS_Store is already present and must not be reported as missing")
		}
	}

	after, err := svc.AddRecommendedGitignore()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.MissingRecommended) != 0 {
		t.Fatalf("expected nothing missing, got %v", after.MissingRecommended)
	}
	if strings.Count(after.Content, ".DS_Store") != 1 {
		t.Fatalf("existing pattern duplicated:\n%s", after.Content)
	}
	if !strings.Contains(after.Content, ".env\n") {
		t.Fatalf("expected secrets to be excluded:\n%s", after.Content)
	}

	again, err := svc.AddRecommendedGitignore()
	if err != nil {
		t.Fatal(err)
	}
	if again.Content != after.Content {
		t.Fatal("adding recommended patterns twice must be a no-op")
	}
}

func TestGitignore_ExcludeRepoPathAnchorsFoldersAndRejectsOutside(t *testing.T) {
	svc, root := newGitignoreService(t)

	res, err := svc.ExcludeRepoPath(filepath.Join(root, "skills", "private"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Content, "/skills/private/\n") {
		t.Fatalf("expected anchored folder pattern:\n%s", res.Content)
	}

	if _, err := svc.ExcludeRepoPath("notes.md"); err != nil {
		t.Fatal(err)
	}
	res, _ = svc.GetGitignore()
	if !strings.Contains(res.Content, "/notes.md\n") {
		t.Fatalf("expected anchored file pattern:\n%s", res.Content)
	}

	if _, err := svc.ExcludeRepoPath(filepath.Dir(root)); err == nil {
		t.Fatal("expected a path outside the repository to be rejected")
	}
	if _, err := svc.ExcludeRepoPath(".git"); err == nil {
		t.Fatal("expected .git to be rejected")
	}
}

func TestGitignore_SaveNormalizesLineEndings(t *testing.T) {
	svc, root := newGitignoreService(t)
	if _, err := svc.SaveGitignore("a\r\nb"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\nb\n" {
		t.Fatalf("unexpected content %q", data)
	}
}

func TestSetTutorialCompleted_Persists(t *testing.T) {
	svc, _ := newGitignoreService(t)
	cfg, err := svc.SetTutorialCompleted(true)
	if err != nil || !cfg.TutorialCompleted {
		t.Fatalf("expected tutorial flag set, err=%v", err)
	}
	reloaded := service.New(func(string, any) {})
	got, _ := reloaded.GetConfig()
	if !got.TutorialCompleted {
		t.Fatal("expected tutorial flag to be persisted")
	}
}
