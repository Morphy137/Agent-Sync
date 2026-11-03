//go:build !windows

package main

import (
	"os"
	"time"
)

// trayIcon is a no-op outside Windows: there is no tray, so closing the window always quits.
type trayIcon struct{}

func newTrayIcon(trayHandlers) *trayIcon { return &trayIcon{} }

func (*trayIcon) Start(trayLabels)             {}
func (*trayIcon) SetLabels(trayLabels)         {}
func (*trayIcon) Stop()                        {}
func (*trayIcon) Active() bool                 { return false }
func (*trayIcon) WaitReady(time.Duration) bool { return false }

// systemUILanguage falls back to the locale environment variables.
func systemUILanguage() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
