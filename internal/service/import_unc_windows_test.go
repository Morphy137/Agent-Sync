package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agentsync/internal/config"
)

// Run the import in a killable child: the regression spins while holding the
// service lock, so a goroutine timeout would leak both the loop and the lock.
func TestImportIntoRepoUNCCaseTermination(t *testing.T) {
	const childRoot = "AGENTSYNC_TEST_UNC_IMPORT_ROOT"
	const childSource = "AGENTSYNC_TEST_UNC_IMPORT_SOURCE"
	if root := os.Getenv(childRoot); root != "" {
		svc := &Service{cfg: &config.Config{RootPath: root}}
		got, err := svc.ImportIntoRepo(os.Getenv(childSource), "UNCLink/new")
		if err != nil {
			t.Fatal(err)
		}
		if got != "UNCLink/new/source.txt" {
			t.Fatalf("import path = %q", got)
		}
		return
	}

	base := t.TempDir()
	volume := filepath.VolumeName(base)
	if len(volume) != 2 || volume[1] != ':' {
		t.Skipf("local drive needed for localhost administrative share: %q", base)
	}
	localRoot := filepath.Join(base, "CaseRepo")
	if err := os.MkdirAll(filepath.Join(localRoot, "Actual"), 0o755); err != nil {
		t.Fatal(err)
	}
	root := `\\localhost\` + strings.ToUpper(volume[:1]) + "$" + strings.TrimPrefix(localRoot, volume)
	target := `\\LOCALHOST\` + strings.ToLower(volume[:1]) + "$" + strings.TrimPrefix(filepath.Join(localRoot, "Actual"), volume)
	if _, err := os.Stat(root); err != nil {
		t.Skipf("localhost administrative share unavailable (share/access permissions): %v", err)
	}
	if err := os.Symlink(target, filepath.Join(localRoot, "UNCLink")); err != nil {
		t.Skipf("UNC symlink unavailable (symlink/share permissions): %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(filepath.Join(localRoot, "UNCLink")) })
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	resolvedLink, err := filepath.EvalSymlinks(filepath.Join(root, "UNCLink"))
	if err != nil {
		t.Fatal(err)
	}
	ancestor := filepath.Dir(resolvedLink)
	if rel, err := filepath.Rel(resolvedRoot, ancestor); err != nil || rel != "." {
		t.Fatalf("fixture target must have root as lexical parent: rel=%q err=%v", rel, err)
	}
	if ancestor == resolvedRoot {
		t.Skip("filesystem canonicalized UNC volume case; case-mismatch regression unavailable")
	}
	t.Logf("UNC case mismatch: root=%q target parent=%q", resolvedRoot, ancestor)
	source := filepath.Join(base, "source.txt")
	if err := os.WriteFile(source, []byte("ordinary file"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestImportIntoRepoUNCCaseTermination$", "-test.v")
	cmd.Env = append(os.Environ(), childRoot+"="+root, childSource+"="+source)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("import did not terminate before child deadline: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		t.Fatalf("child import: %v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(localRoot, "Actual", "new", "source.txt"))
	if err != nil || string(data) != "ordinary file" {
		t.Fatalf("in-root symlink import: data=%q err=%v", data, err)
	}
}
