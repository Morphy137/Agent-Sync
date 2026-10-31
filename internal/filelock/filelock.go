// Package filelock provides an exclusive, cross-process advisory lock backed by a file.
// The GUI, the CLI and the MCP server use it to serialize writes to the shared config and state.
package filelock

import (
	"os"
	"path/filepath"
)

const (
	lockFilePerms = 0o600
	lockDirPerms  = 0o755
)

// Lock is an exclusive lock on a file path. A Lock is not safe for concurrent use by
// multiple goroutines; callers serialize access with their own mutex.
type Lock struct {
	path string
	file *os.File
}

// New returns an unlocked Lock for path. The file is created on first acquisition.
func New(path string) *Lock {
	return &Lock{path: path}
}

// Path returns the lock file path.
func (l *Lock) Path() string {
	return l.path
}

// Lock blocks until the exclusive lock is acquired.
func (l *Lock) Lock() error {
	f, err := l.open()
	if err != nil {
		return err
	}
	if err := lockFile(f, true); err != nil {
		_ = f.Close()
		return err
	}
	l.file = f
	return nil
}

// TryLock acquires the lock without blocking. It reports false when another holder owns it.
func (l *Lock) TryLock() (bool, error) {
	f, err := l.open()
	if err != nil {
		return false, err
	}
	if err := lockFile(f, false); err != nil {
		_ = f.Close()
		if isWouldBlock(err) {
			return false, nil
		}
		return false, err
	}
	l.file = f
	return true, nil
}

// Unlock releases the lock. Unlocking an unlocked Lock is a no-op.
func (l *Lock) Unlock() error {
	if l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	unlockErr := unlockFile(f)
	closeErr := f.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

func (l *Lock) open() (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(l.path), lockDirPerms); err != nil {
		return nil, err
	}
	return os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, lockFilePerms)
}
