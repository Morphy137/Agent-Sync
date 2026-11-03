package main

import "strings"

const (
	languageSpanish = "es"
	languageEnglish = "en"
	// trayTooltip is the text shown when pointing at the tray icon; it is the product name, so it is not translated.
	trayTooltip = "AgentSync"
)

// trayLabels are the texts of the tray menu.
type trayLabels struct {
	Open    string
	SyncNow string
	Quit    string
}

// trayHandlers are the actions the tray triggers. Each runs on its own goroutine, never on the tray's thread.
type trayHandlers struct {
	Open    func()
	SyncNow func()
	Quit    func()
}

var trayMessages = map[string]trayLabels{
	languageEnglish: {Open: "Open AgentSync", SyncNow: "Sync now", Quit: "Quit"},
	languageSpanish: {Open: "Abrir AgentSync", SyncNow: "Sincronizar ahora", Quit: "Salir"},
}

// trayLabelsFor returns the menu texts for the configured language ("system" follows the Windows display language).
func trayLabelsFor(configured string) trayLabels {
	lang := normalizeLanguage(configured)
	if lang == "" {
		lang = normalizeLanguage(systemUILanguage())
	}
	if labels, ok := trayMessages[lang]; ok {
		return labels
	}
	return trayMessages[languageEnglish]
}

func normalizeLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch {
	case strings.HasPrefix(value, languageSpanish):
		return languageSpanish
	case strings.HasPrefix(value, languageEnglish):
		return languageEnglish
	}
	return ""
}
