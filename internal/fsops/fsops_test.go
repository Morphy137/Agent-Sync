package fsops_test

import (
	"os"
	"path/filepath"
	"testing"

	"agentsync/internal/fsops"
)

func TestWalkManagedAndHash(t *testing.T) {
	tempDir := t.TempDir()
	profileDir := filepath.Join(tempDir, "CLAUDE")
	if err := os.MkdirAll(filepath.Join(profileDir, "commands"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(profileDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create valid files
	claudeMd := filepath.Join(profileDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMd, []byte("claude content"), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewMd := filepath.Join(profileDir, "commands", "review.md")
	if err := os.WriteFile(reviewMd, []byte("review command"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create ignored files
	if err := os.WriteFile(filepath.Join(profileDir, ".DS_Store"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, ".gitkeep"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, ".git", "config"), []byte("git config"), 0o644); err != nil {
		t.Fatal(err)
	}

	managed, err := fsops.WalkManaged(profileDir)
	if err != nil {
		t.Fatalf("WalkManaged failed: %v", err)
	}

	if len(managed) != 2 {
		t.Fatalf("expected 2 managed files, got %d: %v", len(managed), managed)
	}

	if _, ok := managed["CLAUDE.md"]; !ok {
		t.Errorf("expected CLAUDE.md in managed map")
	}
	if _, ok := managed["commands/review.md"]; !ok {
		t.Errorf("expected commands/review.md in managed map")
	}

	// Test HashFile
	hash1, err := fsops.HashFile(claudeMd)
	if err != nil {
		t.Fatalf("HashFile failed: %v", err)
	}
	if hash1 == "" {
		t.Errorf("expected non-empty hash")
	}
}

func TestAtomicCopyAndWrite(t *testing.T) {
	tempDir := t.TempDir()
	src := filepath.Join(tempDir, "src.txt")
	dst := filepath.Join(tempDir, "sub", "dst.txt")

	content := []byte("hello atomic world")
	if err := fsops.AtomicWrite(src, content, 0o644); err != nil {
		t.Fatalf("AtomicWrite failed: %v", err)
	}

	read, err := os.ReadFile(src)
	if err != nil || string(read) != string(content) {
		t.Fatalf("expected content %s, got %s", content, read)
	}

	if err := fsops.AtomicCopy(src, dst); err != nil {
		t.Fatalf("AtomicCopy failed: %v", err)
	}

	copied, err := os.ReadFile(dst)
	if err != nil || string(copied) != string(content) {
		t.Fatalf("expected copied content %s, got %s", content, copied)
	}
}
