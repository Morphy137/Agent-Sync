package filelock

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const helperEnv = "AGENTSYNC_FILELOCK_HELPER"

// TestHelperProcess runs in a child process and tries to take the lock without blocking.
func TestHelperProcess(t *testing.T) {
	path := os.Getenv(helperEnv)
	if path == "" {
		t.Skip("helper only")
	}
	ok, err := New(path).TryLock()
	if err != nil {
		os.Exit(2)
	}
	if ok {
		os.Exit(0)
	}
	os.Exit(1)
}

func tryFromOtherProcess(t *testing.T, path string) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), helperEnv+"="+path)
	err := cmd.Run()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	t.Fatalf("helper failed: %v", err)
	return -1
}

func TestLockExcludesOtherProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "agentsync.lock")
	l := New(path)
	if err := l.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if code := tryFromOtherProcess(t, path); code != 1 {
		t.Fatalf("expected other process to be blocked (exit 1), got %d", code)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if code := tryFromOtherProcess(t, path); code != 0 {
		t.Fatalf("expected other process to acquire the lock (exit 0), got %d", code)
	}
}

func TestTryLockAndDoubleUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentsync.lock")
	l := New(path)
	ok, err := l.TryLock()
	if err != nil || !ok {
		t.Fatalf("TryLock = %v, %v", ok, err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("second unlock must be a no-op: %v", err)
	}
}
