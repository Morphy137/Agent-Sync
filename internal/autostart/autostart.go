// Package autostart manages the "start with Windows" entry. The entry lives behind the Store
// interface so tests use a fake store and never touch the real registry.
package autostart

import "errors"

const (
	// ValueName is the name of the entry in the current user's Run registry key.
	ValueName = "AgentSync"
	// MinimizedFlag is passed to the program when Windows starts it.
	MinimizedFlag = "--minimized"
)

// ErrUnsupported is returned when this build or platform cannot register a startup entry.
var ErrUnsupported = errors.New("start with Windows is not supported here")

// Store reads and writes the startup command.
type Store interface {
	// Get returns the stored command and whether an entry exists.
	Get() (command string, exists bool, err error)
	Set(command string) error
	Remove() error
}

// Status is what Settings shows.
type Status struct {
	// Supported is false for development builds (the executable path is temporary) and other platforms.
	Supported bool `json:"supported"`
	// Enabled is true when the entry really exists.
	Enabled bool `json:"enabled"`
}

// Manager registers the running executable as a startup program.
type Manager struct {
	store      Store
	executable string
	supported  bool
}

// NewManager builds a manager for executable. A nil store, an empty path or supported=false makes
// every change fail with ErrUnsupported, while Status still reports it.
func NewManager(store Store, executable string, supported bool) *Manager {
	return &Manager{store: store, executable: executable, supported: supported && store != nil && executable != ""}
}

// Command is the exact value written to the Run key: the quoted path (backslashes stay as they are,
// the registry does not use escapes) followed by the minimized flag.
func (m *Manager) Command() string {
	return `"` + m.executable + `" ` + MinimizedFlag
}

// Status reads the real state from the store.
func (m *Manager) Status() Status {
	if !m.supported {
		return Status{}
	}
	_, exists, err := m.store.Get()
	return Status{Supported: true, Enabled: err == nil && exists}
}

// Enable writes (or rewrites) the startup entry.
func (m *Manager) Enable() error {
	if !m.supported {
		return ErrUnsupported
	}
	return m.store.Set(m.Command())
}

// Disable removes the startup entry; a missing entry is not an error.
func (m *Manager) Disable() error {
	if !m.supported {
		return ErrUnsupported
	}
	return m.store.Remove()
}

// Apply makes the entry match wanted.
func (m *Manager) Apply(wanted bool) error {
	if wanted {
		return m.Enable()
	}
	return m.Disable()
}

// Repair keeps an existing entry pointing at the running executable (for example after the program
// was moved). It reports false when it had nothing to do: no entry, or the entry is already right.
func (m *Manager) Repair() (repaired bool, err error) {
	if !m.supported {
		return false, nil
	}
	command, exists, err := m.store.Get()
	if err != nil || !exists || command == m.Command() {
		return false, err
	}
	return true, m.store.Set(m.Command())
}
