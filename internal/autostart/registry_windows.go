//go:build windows

package autostart

import (
	"errors"

	"golang.org/x/sys/windows/registry"
)

// runKeyPath is where Windows looks for programs to start when the user signs in.
const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

type registryStore struct{}

// NewRegistryStore returns the store backed by HKCU\Software\Microsoft\Windows\CurrentVersion\Run.
func NewRegistryStore() Store { return registryStore{} }

func (registryStore) Get() (string, bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	defer key.Close()

	value, _, err := key.GetStringValue(ValueName)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (registryStore) Set(command string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(ValueName, command)
}

func (registryStore) Remove() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.DeleteValue(ValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}
