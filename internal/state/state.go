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

// GetBaseline retrieves the baseline hash for a connection and relative path.
func (s *State) GetBaseline(connectionID, relPath string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connMap, ok := s.Baselines[connectionID]
	if !ok {
		return ""
	}
	return connMap[relPath]
}

// SetBaseline sets or updates the baseline hash for a connection and relative path.
func (s *State) SetBaseline(connectionID, relPath, hash string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Baselines == nil {
		s.Baselines = make(map[string]map[string]string)
	}
	if _, ok := s.Baselines[connectionID]; !ok {
		s.Baselines[connectionID] = make(map[string]string)
	}
	s.Baselines[connectionID][relPath] = hash
}

// RemoveBaseline removes a specific file baseline for a connection.
func (s *State) RemoveBaseline(connectionID, relPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if connMap, ok := s.Baselines[connectionID]; ok {
		delete(connMap, relPath)
	}
}

// GetConnectionBaselines returns a copy of all baselines for a connection.
func (s *State) GetConnectionBaselines(connectionID string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	if connMap, ok := s.Baselines[connectionID]; ok {
		for k, v := range connMap {
			result[k] = v
		}
	}
	return result
}

// RemoveConnection deletes all baselines for a given connection ID.
func (s *State) RemoveConnection(connectionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Baselines, connectionID)
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
