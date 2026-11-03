package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
)

const (
	CurrentVersion = 1
	StateFileName  = "state.json"
)

// State holds sync baselines and timestamps.
type State struct {
	Version     int                          `json:"version"`
	Baselines   map[string]map[string]string `json:"baselines"`
	LastSyncAt  string                       `json:"lastSyncAt,omitempty"`
	LastCheckAt string                       `json:"lastCheckAt,omitempty"`
	mu          sync.RWMutex                 `json:"-"`
}

// NewState returns an initialized empty State.
func NewState() *State {
	return &State{
		Version:   CurrentVersion,
		Baselines: make(map[string]map[string]string),
	}
}

// Load loads the state from disk, or returns a new State if it doesn't exist.
func Load() (*State, error) {
	configDir := config.GetConfigDir()
	statePath := filepath.Join(configDir, StateFileName)

	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return NewState(), nil
		}
		return nil, apperr.Wrap(apperr.WriteFailed, err)
	}

	st := NewState()
	if err := json.Unmarshal(data, st); err != nil {
		return nil, apperr.Wrap(apperr.WriteFailed, err)
	}

	if st.Baselines == nil {
		st.Baselines = make(map[string]map[string]string)
	}

	return st, nil
}

// Save writes the state atomically to disk.
func (s *State) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	configDir := config.GetConfigDir()
	if err := os.MkdirAll(configDir, fsops.DefaultDirPerms); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}

	statePath := filepath.Join(configDir, StateFileName)
	return fsops.AtomicWrite(statePath, data, fsops.DefaultFilePerms)
}

// GetBaseline retrieves the baseline hash stored under key for a relative path.
func (s *State) GetBaseline(key, relPath string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, ok := s.Baselines[key]
	if !ok {
		return ""
	}
	return entries[relPath]
}

// SetBaseline sets or updates the baseline hash stored under key for a relative path.
func (s *State) SetBaseline(key, relPath, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Baselines == nil {
		s.Baselines = make(map[string]map[string]string)
	}
	if _, ok := s.Baselines[key]; !ok {
		s.Baselines[key] = make(map[string]string)
	}
	s.Baselines[key][relPath] = hash
}

// RemoveBaseline removes a specific file baseline stored under key.
func (s *State) RemoveBaseline(key, relPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entries, ok := s.Baselines[key]; ok {
		delete(entries, relPath)
	}
}

// GetBaselines returns a copy of all baselines stored under key.
func (s *State) GetBaselines(key string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range s.Baselines[key] {
		result[k] = v
	}
	return result
}

// RemoveBaselines deletes all baselines stored under key.
func (s *State) RemoveBaselines(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Baselines, key)
}

// PruneBaselines deletes every baseline group whose key keep rejects and reports whether any
// was removed. It drops entries left behind by features that no longer exist.
func (s *State) PruneBaselines(keep func(key string) bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	pruned := false
	for key := range s.Baselines {
		if !keep(key) {
			delete(s.Baselines, key)
			pruned = true
		}
	}
	return pruned
}

// SetTimestamps updates lastSyncAt and lastCheckAt.
func (s *State) SetTimestamps(lastSync, lastCheck string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lastSync != "" {
		s.LastSyncAt = lastSync
	}
	if lastCheck != "" {
		s.LastCheckAt = lastCheck
	}
}
