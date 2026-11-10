package service_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/service"
)

// Junctions exercise the same escape on Windows without symlink privileges.
func importDirectoryLink(t *testing.T, link, target string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		if output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Fatalf("create junction: %v: %s", err, output)
		}
		t.Log("using Windows directory junction")
	}
	t.Cleanup(func() { _ = os.Remove(link) })
}

func TestImportIntoRepoAllowsNewNestedDestinations(t *testing.T) {
	for _, linked := range []bool{false, true} {
		for _, folder := range []bool{false, true} {
			name := "file"
			if folder {
				name = "folder"
			}
			if linked {
				name += "/inside-link"
			}
			t.Run(name, func(t *testing.T) {
				base := t.TempDir()
				t.Setenv(config.ConfigEnvVar, filepath.Join(base, "config"))
				root := filepath.Join(base, "repo")
				if err := os.MkdirAll(filepath.Join(root, "actual"), 0o755); err != nil {
					t.Fatal(err)
				}
				dir := "new/nested"
				physicalDir := filepath.Join(root, filepath.FromSlash(dir))
				insideJunction := false
				if linked {
					link := filepath.Join(root, "linked")
					importDirectoryLink(t, link, filepath.Join(root, "actual"))
					info, err := os.Lstat(link)
					if err != nil {
						t.Fatal(err)
					}
					insideJunction = info.Mode()&os.ModeIrregular != 0
					dir = "linked/new/nested"
					physicalDir = filepath.Join(root, "actual", "new", "nested")
				}
				src := filepath.Join(base, "source.txt")
				contentRel := "source.txt"
				if folder {
					src = filepath.Join(base, "source")
					if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(src, "nested", "file.txt"), []byte("contents"), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(src, "sibling.txt"), []byte("sibling"), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(src, ".env"), []byte("secret"), 0o600); err != nil {
						t.Fatal(err)
					}
					importDirectoryLink(t, filepath.Join(src, "outside-link"), root)
					outsideFile := filepath.Join(base, "private.txt")
					if err := os.WriteFile(outsideFile, []byte("private"), 0o600); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(outsideFile, filepath.Join(src, "outside-file.txt")); err != nil {
						t.Logf("file symlink unavailable: %v", err)
					}
					contentRel = filepath.Join("source", "nested", "file.txt")
				} else if err := os.WriteFile(src, []byte("contents"), 0o600); err != nil {
					t.Fatal(err)
				}
				svc := service.New(nil)
				if _, err := svc.SetRootPath(root); err != nil {
					t.Fatal(err)
				}
				got, err := svc.ImportIntoRepo(src, dir)
				if insideJunction {
					// Junctions are explicitly rejected, even for in-root targets.
					if !apperr.HasCode(err, apperr.InvalidRule) {
						t.Fatalf("inside junction not explicitly rejected: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				want := dir + "/" + filepath.Base(src)
				if got != want {
					t.Fatalf("path = %q, want %q", got, want)
				}
				data, err := os.ReadFile(filepath.Join(physicalDir, contentRel))
				if err != nil || string(data) != "contents" {
					t.Fatalf("import: %q, %v", data, err)
				}
				if folder {
					data, err := os.ReadFile(filepath.Join(physicalDir, "source", "sibling.txt"))
					if err != nil || string(data) != "sibling" {
						t.Fatalf("sibling import: %q, %v", data, err)
					}
					for _, excluded := range []string{".env", "outside-link", "outside-file.txt"} {
						if _, err := os.Lstat(filepath.Join(physicalDir, "source", excluded)); !os.IsNotExist(err) {
							t.Fatalf("excluded source %q copied: %v", excluded, err)
						}
					}
				}
				if _, err := svc.ImportIntoRepo(src, dir); !apperr.HasCode(err, apperr.FolderExists) {
					t.Fatalf("overwrite accepted: %v", err)
				}
			})
		}
	}
}

func TestImportIntoRepoRejectsDestinationEscape(t *testing.T) {
	testImportDestinationEscape(t, importDirectoryLink)
}

func TestImportIntoRepoRejectsDestinationJunctionEscape(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction regression")
	}
	testImportDestinationEscape(t, func(t *testing.T, link, target string) {
		t.Helper()
		if output, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Fatalf("create junction: %v: %s", err, output)
		}
		t.Cleanup(func() { _ = os.Remove(link) })
	})
}

func TestImportIntoRepoRejectsSymlinkToJunctionEscape(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows junction regression")
	}
	testImportDestinationEscape(t, func(t *testing.T, link, target string) {
		t.Helper()
		junction := filepath.Join(filepath.Dir(link), "redirect")
		if output, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
			t.Fatalf("create junction: %v: %s", err, output)
		}
		t.Cleanup(func() { _ = os.Remove(junction) })
		if err := os.Symlink(junction, link); err != nil {
			t.Skipf("symlink privilege unavailable: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(link) })
	})
}

func testImportDestinationEscape(t *testing.T, makeLink func(*testing.T, string, string)) {
	t.Helper()
	for _, folder := range []bool{false, true} {
		for _, dir := range []string{"linked", "linked/new/nested"} {
			name := "file/" + dir
			if folder {
				name = "folder/" + dir
			}
			t.Run(name, func(t *testing.T) {
				base := t.TempDir()
				t.Setenv(config.ConfigEnvVar, filepath.Join(base, "config"))
				root, outside := filepath.Join(base, "repo"), filepath.Join(base, "outside")
				for _, path := range []string{root, outside} {
					if err := os.MkdirAll(path, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				src := filepath.Join(base, "source.txt")
				if folder {
					src = filepath.Join(base, "source")
					if err := os.MkdirAll(filepath.Join(src, "nested"), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(filepath.Join(src, "nested", "file.txt"), []byte("secret"), 0o600); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(src, []byte("secret"), 0o600); err != nil {
					t.Fatal(err)
				}
				svc := service.New(nil)
				if _, err := svc.SetRootPath(root); err != nil {
					t.Fatal(err)
				}
				makeLink(t, filepath.Join(root, "linked"), outside)
				_, err := svc.ImportIntoRepo(src, dir)
				if !apperr.HasCode(err, apperr.InvalidRule) {
					t.Errorf("destination escape must be rejected, got %v", err)
				}
				entries, err := os.ReadDir(outside)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 0 {
					t.Errorf("import wrote outside repository: %v", entries)
				}
			})
		}
	}
}
