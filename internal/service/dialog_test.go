package service_test

import (
	"os"
	"path/filepath"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/service"
)

func TestSelectFolderResolvesNearestExistingStartDir(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, filepath.Join(tempDir, "config"))
	existing := filepath.Join(tempDir, "existing")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(existing, "file.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty input", "", ""},
		{"existing directory", existing, existing},
		{"missing nested path walks up", filepath.Join(existing, "missing", "deeper"), existing},
		{"file path uses parent", filePath, existing},
		{"relative path ignored", "relative/path", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received string
			svc := service.New(nil)
			svc.SetDialogFunc(func(_, defaultDir string) (string, error) {
				received = defaultDir
				return "", nil
			})
			if _, err := svc.SelectFolder("title", tc.input); err != nil {
				t.Fatalf("SelectFolder failed: %v", err)
			}
			if received != tc.want {
				t.Errorf("default dir = %q, want %q", received, tc.want)
			}
		})
	}
}
