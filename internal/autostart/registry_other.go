//go:build !windows

package autostart

// NewRegistryStore returns nil: only Windows has a Run registry key.
func NewRegistryStore() Store { return nil }
