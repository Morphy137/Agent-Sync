package syncer_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"agentsync/internal/config"
	"agentsync/internal/state"
	"agentsync/internal/syncer"
)

func TestComputeStatusUnreadableLockedFile(t *testing.T) {
	tempDir := t.TempDir()
	rootDir := filepath.Join(tempDir, "root")
	targetDir := filepath.Join(tempDir, "target")
	profileDir := filepath.Join(rootDir, "CLAUDE")

	_ = os.MkdirAll(profileDir, 0o755)
	_ = os.MkdirAll(targetDir, 0o755)

	sourceFile := filepath.Join(profileDir, "CLAUDE.md")
	targetFile := filepath.Join(targetDir, "CLAUDE.md")
	_ = os.WriteFile(sourceFile, []byte("source"), 0o644)
	_ = os.WriteFile(targetFile, []byte("target"), 0o644)

	// Open targetFile with exclusive share mode (0) on Windows so HashFile fails
	pathPtr, err := syscall.UTF16PtrFromString(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(
		pathPtr,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, // no sharing allowed -> exclusive lock
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("failed to lock file exclusively: %v", err)
	}
	defer syscall.CloseHandle(handle)

	conn := config.Connection{
		ID:         "conn-unreadable",
		Profile:    "CLAUDE",
		TargetPath: targetDir,
	}
	st := state.NewState()

	status, err := syncer.ComputeStatus(rootDir, conn, st)
	if err != nil {
		t.Fatalf("ComputeStatus failed: %v", err)
	}

	if status.Status != syncer.ConnUnreadable {
		t.Errorf("expected connection status %s, got %s", syncer.ConnUnreadable, status.Status)
	}
	if len(status.Files) != 1 || status.Files[0].Status != syncer.FileUnreadable {
		t.Errorf("expected file status %s, got %+v", syncer.FileUnreadable, status.Files)
	}
}
