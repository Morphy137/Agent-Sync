package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestResolveImportDestinationRootTermination(t *testing.T) {
	const childRoot = "AGENTSYNC_TEST_RESOLVE_IMPORT_ROOT"
	if root := os.Getenv(childRoot); root != "" {
		got, err := resolveImportDestination(root, ".")
		if err != nil {
			t.Fatal(err)
		}
		if rel, err := filepath.Rel(root, got); err != nil || rel != "." {
			t.Fatalf("root resolution = %q: rel=%q err=%v", got, rel, err)
		}
		return
	}

	root := t.TempDir()
	volume := filepath.VolumeName(root)
	roots := []struct{ name, path string }{
		{"repository", root},
		{"filesystem", volume + string(filepath.Separator)},
	}
	if runtime.GOOS == "windows" && len(volume) == 2 && volume[1] == ':' {
		roots = append(roots, struct{ name, path string }{
			"UNC-share", `\\localhost\` + strings.ToUpper(volume[:1]) + `$\`,
		})
	}
	for _, tc := range roots {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); err != nil {
				if tc.name == "UNC-share" {
					t.Skipf("localhost administrative share unavailable (share/access permissions): %v", err)
				}
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestResolveImportDestinationRootTermination$", "-test.v", "-test.timeout=5s")
			cmd.Env = append(os.Environ(), childRoot+"="+tc.path)
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("root resolution did not terminate: %v\n%s", ctx.Err(), output)
			}
			if err != nil {
				t.Fatalf("root resolution: %v\n%s", err, output)
			}
		})
	}
}
