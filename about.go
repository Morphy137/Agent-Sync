package main

import (
	_ "embed"
)

// changelog is CHANGELOG.md, embedded so the app can show "What's new" offline.
//
//go:embed CHANGELOG.md
var changelog string

// GetChangelog returns the user-facing changelog (Markdown).
func (a *App) GetChangelog() string {
	return changelog
}
