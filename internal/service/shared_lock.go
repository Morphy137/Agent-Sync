package service

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"agentsync/internal/config"
	"agentsync/internal/filelock"
	"agentsync/internal/rules"
	"agentsync/internal/state"
)

// LockFileName is the cross-process lock shared by the GUI, the CLI and the MCP server.
const LockFileName = "agentsync.lock"

// DefaultExternalWatchInterval is how often the GUI checks whether another process changed the shared files.
const DefaultExternalWatchInterval = 2 * time.Second

// serviceLock serializes Service operations inside the process (mutex) and, when shared mode is
// enabled, across processes (file lock in the config dir). onAcquire runs after both are held,
// onRelease runs right before the file lock is released.
type serviceLock struct {
	mu        sync.Mutex
	shared    bool
	file      *filelock.Lock
	onAcquire func()
	onRelease func()
}

func (l *serviceLock) Lock() {
	l.mu.Lock()
	if !l.shared {
		return
	}
	l.file = filelock.New(filepath.Join(config.GetConfigDir(), LockFileName))
	if err := l.file.Lock(); err != nil {
		// The config dir is unusable (read-only or missing permissions); fall back to the in-process lock.
		l.file = nil
	}
	if l.onAcquire != nil {
		l.onAcquire()
	}
}

func (l *serviceLock) TryLock() bool {
	if !l.mu.TryLock() {
		return false
	}
	if !l.shared {
		return true
	}
	l.file = filelock.New(filepath.Join(config.GetConfigDir(), LockFileName))
	ok, err := l.file.TryLock()
	if err == nil && !ok {
		l.file = nil
		l.mu.Unlock()
		return false
	}
	if err != nil {
		l.file = nil
	}
	if l.onAcquire != nil {
		l.onAcquire()
	}
	return true
}

func (l *serviceLock) Unlock() {
	if l.shared {
		if l.onRelease != nil {
			l.onRelease()
		}
		if l.file != nil {
			_ = l.file.Unlock()
			l.file = nil
		}
	}
	l.mu.Unlock()
}

// fileStamp identifies one version of a file on disk.
type fileStamp struct {
	exists  bool
	size    int64
	modTime int64
}

// diskSnapshot records the shared files as this process last left them.
type diskSnapshot struct {
	config     fileStamp
	state      fileStamp
	localRules fileStamp
	repoRules  fileStamp
	rootPath   string
}

func stampOf(path string) fileStamp {
	if path == "" {
		return fileStamp{}
	}
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}
	return fileStamp{exists: true, size: fi.Size(), modTime: fi.ModTime().UnixNano()}
}

func takeSnapshot(rootPath string) diskSnapshot {
	dir := config.GetConfigDir()
	snap := diskSnapshot{
		config:     stampOf(filepath.Join(dir, config.ConfigFileName)),
		state:      stampOf(filepath.Join(dir, state.StateFileName)),
		localRules: stampOf(rules.LocalRulesPath()),
		rootPath:   rootPath,
	}
	if rootPath != "" {
		snap.repoRules = stampOf(filepath.Join(rootPath, rules.RulesFileName))
	}
	return snap
}

// enableSharedMode turns on cross-process coordination: every operation takes the file lock and
// reloads config/state when another process (GUI, CLI or MCP server) changed them.
func (s *Service) enableSharedMode() {
	snap := takeSnapshot(s.cfg.RootPath)
	s.lastSnapshot.Store(&snap)
	s.mu.shared = true
	s.mu.onAcquire = s.refreshFromDiskLocked
	s.mu.onRelease = s.rememberDiskLocked
}

// rememberDiskLocked stores the snapshot after this process' own writes so they are not seen as external.
func (s *Service) rememberDiskLocked() {
	snap := takeSnapshot(s.cfg.RootPath)
	s.lastSnapshot.Store(&snap)
}

// refreshFromDiskLocked reloads what another process changed since this process last held the lock.
func (s *Service) refreshFromDiskLocked() {
	prev := s.lastSnapshot.Load()
	if prev == nil {
		return
	}
	cur := takeSnapshot(s.cfg.RootPath)
	if cur == *prev {
		return
	}
	if cur.config != prev.config {
		if cfg, err := config.Load(); err == nil {
			s.cfg = cfg
		}
	}
	if cur.state != prev.state {
		if st, err := state.Load(); err == nil {
			s.st = st
		}
	}
	snap := takeSnapshot(s.cfg.RootPath)
	s.lastSnapshot.Store(&snap)
	s.emitRulesChangedLocked()
	s.emitStatusLocked()
}

// externalChangePending reports whether shared files differ from the last snapshot (no lock needed).
func (s *Service) externalChangePending() bool {
	prev := s.lastSnapshot.Load()
	if prev == nil {
		return false
	}
	return takeSnapshot(prev.rootPath) != *prev
}

// WatchExternalChanges polls the shared files and, when another process changed them, reloads and
// emits the usual change events so an open GUI reflects edits made through the CLI or MCP server.
func (s *Service) WatchExternalChanges(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !s.externalChangePending() {
				continue
			}
			// Taking the lock runs refreshFromDiskLocked; skip this tick when an operation is running.
			if s.mu.TryLock() {
				s.mu.Unlock()
			}
		}
	}
}

// snapshotHolder keeps the last snapshot readable without taking the service lock.
type snapshotHolder = atomic.Pointer[diskSnapshot]
