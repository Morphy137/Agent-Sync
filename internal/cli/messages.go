package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Supported languages.
const (
	LangEN = "en"
	LangES = "es"
)

// systemLangEnvVars are checked in order when no language is configured.
var systemLangEnvVars = []string{"LC_ALL", "LC_MESSAGES", "LANG"}

// messages holds every user-facing CLI string. Keys must stay in parity across languages.
var messages = map[string]map[string]string{
	LangEN: {
		"help.title":                "AgentSync CLI - sync agent configs (instructions, skills, MCP configs, any file) from your repository to every place that needs them.",
		"help.usage":                "Usage: agentsync-cli <command> [arguments] [--json] [--lang en|es]",
		"help.commands":             "Commands:",
		"help.global":               "Global flags: --json prints {\"ok\":true,\"data\":...} or {\"ok\":false,\"error\":{...}} for scripts and AI agents; --lang sets the language.",
		"help.mcp":                  "MCP: run \"agentsync-cli mcp\" as a stdio MCP server (see docs/CLI_AND_MCP.md).",
		"hint.help":                 "Run \"agentsync-cli help\" for usage.",
		"err.unknown_command":       "unknown command",
		"label.error":               "Error",
		"label.root":                "Repository root",
		"label.config_dir":          "Config folder",
		"label.not_set":             "(not set - run: agentsync-cli root set <folder>)",
		"label.branch":              "Branch",
		"label.remote":              "Remote",
		"label.ahead_behind":        "Ahead/behind",
		"label.direction":           "Sync direction",
		"label.rules":               "Rules",
		"label.rules_counts":        "%d total, %d enabled, %d pending, %d conflicts, %d collisions",
		"label.unmapped":            "Unmapped locations",
		"label.locations":           "Locations",
		"label.subscriptions":       "Subscriptions",
		"label.fingerprint":         "Fingerprint",
		"label.downloads":           "Downloads",
		"label.uploads":             "Uploads",
		"label.conflicts":           "Conflicts",
		"label.commit":              "To commit",
		"label.outside":             "Changes outside managed paths",
		"label.applied":             "Applied",
		"label.pulled":              "Pulled",
		"label.pushed":              "Pushed",
		"label.written":             "Written",
		"label.captured":            "Captured",
		"label.deleted":             "Deleted",
		"label.skipped":             "Skipped",
		"label.backups":             "Backups",
		"label.collisions":          "Collisions",
		"label.kind":                "Kind",
		"label.name":                "Name",
		"label.location":            "Location",
		"label.path":                "Path",
		"label.mode":                "Mode",
		"label.transform":           "Transform",
		"label.detected":            "Agent detected",
		"label.generic":             "generic copy",
		"label.behind":              "Behind",
		"label.incoming":            "Incoming files",
		"label.changed":             "Changed files",
		"label.backup_branch":       "Previous history kept in branch",
		"label.local_changes":       "Local changes to review",
		"label.binary":              "(binary or too large to show)",
		"label.none":                "(none)",
		"label.yes":                 "yes",
		"label.no":                  "no",
		"label.saved_rule":          "Saved rule %s (%s), scope %s",
		"label.id":                  "ID",
		"label.scope":               "Scope",
		"label.enabled":             "Enabled",
		"label.source":              "Source",
		"label.destinations":        "Destinations",
		"label.alias":               "Alias",
		"label.exists":              "Exists",
		"label.url":                 "URL",
		"label.auto_update":         "Auto-update",
		"cmd.gitignore_show":        "Show the repository .gitignore and missing recommended patterns.",
		"cmd.gitignore_exclude":     "Add a repository file or folder to .gitignore.",
		"cmd.gitignore_recommended": "Append the recommended .gitignore patterns that are missing.",
		"cmd.gitignore_save":        "Replace the whole .gitignore (needs --yes).",
		"label.missing_recommended": "Missing recommended patterns",
		"label.offline":             "offline",
		"msg.rule_removed":          "Rule removed. Files already written stay in place.",
		"msg.file_resolved":         "File resolved.",
		"msg.subscription_removed":  "Subscription removed.",
		"msg.change_discarded":      "Change discarded (a backup was taken).",
		"cmd.status":                "Show repository, git and rule status (no network).",
		"cmd.plan":                  "Fetch the remote and show what sync would do.",
		"cmd.sync":                  "Pull, apply rules, commit and push (or --rules-only to just apply rules).",
		"cmd.root_set":              "Set the repository root folder.",
		"cmd.rules_list":            "List rules with destination and file status.",
		"cmd.rules_show":            "Show one rule.",
		"cmd.rules_add":             "Create a rule; destinations without a path use AgentSync's suggestion.",
		"cmd.rules_remove":          "Delete a rule (needs --yes).",
		"cmd.rules_enable":          "Enable a rule.",
		"cmd.rules_disable":         "Disable a rule.",
		"cmd.rules_diff":            "Show source vs destination for a rule file.",
		"cmd.rules_resolve":         "Resolve a conflicting or drifted rule file.",
		"cmd.dest_add":              "Add a destination to a rule.",
		"cmd.dest_remove":           "Remove a destination from a rule (needs --yes).",
		"cmd.loc_list":              "List location aliases on this PC.",
		"cmd.loc_set":               "Map an alias to a folder on this PC.",
		"cmd.loc_remove":            "Remove an alias mapping.",
		"cmd.detect":                "Detect the kind of a source (skill, mcp, instructions, file, folder).",
		"cmd.suggest":               "Suggest where a source goes inside a location.",
		"cmd.sub_list":              "List subscriptions.",
		"cmd.sub_add":               "Subscribe to a remote repository (read-only).",
		"cmd.sub_check":             "Check a subscription for incoming changes.",
		"cmd.sub_update":            "Pull a subscription and apply its rules.",
		"cmd.sub_auto":              "Turn automatic updates on or off.",
		"cmd.sub_remove":            "Remove a subscription and its rules (needs --yes).",
		"cmd.sub_tree":              "List a subscription's files.",
		"cmd.repo_tree":             "List repository files with detected kinds.",
		"cmd.repo_changes":          "List repository changes (--fetch to include incoming).",
		"cmd.repo_diff":             "Show the diff of a repository file.",
		"cmd.repo_discard":          "Discard a local repository change (needs --yes).",
		"cmd.repo_connect":          "Connect the repository to a remote URL.",
		"cmd.mcp":                   "Run the MCP server over stdio for AI agents.",
		"cmd.version":               "Print the version.",
	},
	LangES: {
		"help.title":                "AgentSync CLI - sincroniza configuraciones de agentes (instrucciones, skills, configs MCP, cualquier archivo) desde tu repositorio a todos los lugares que las necesitan.",
		"help.usage":                "Uso: agentsync-cli <comando> [argumentos] [--json] [--lang en|es]",
		"help.commands":             "Comandos:",
		"help.global":               "Flags globales: --json imprime {\"ok\":true,\"data\":...} o {\"ok\":false,\"error\":{...}} para scripts y agentes de IA; --lang fija el idioma.",
		"help.mcp":                  "MCP: ejecuta \"agentsync-cli mcp\" como servidor MCP por stdio (ver docs/CLI_AND_MCP.md).",
		"hint.help":                 "Ejecuta \"agentsync-cli help\" para ver el uso.",
		"err.unknown_command":       "comando desconocido",
		"label.error":               "Error",
		"label.root":                "Raíz del repositorio",
		"label.config_dir":          "Carpeta de configuración",
		"label.not_set":             "(sin definir - ejecuta: agentsync-cli root set <carpeta>)",
		"label.branch":              "Rama",
		"label.remote":              "Remoto",
		"label.ahead_behind":        "Adelante/atrás",
		"label.direction":           "Dirección de sincronización",
		"label.rules":               "Reglas",
		"label.rules_counts":        "%d en total, %d activas, %d pendientes, %d conflictos, %d colisiones",
		"label.unmapped":            "Ubicaciones sin mapear",
		"label.locations":           "Ubicaciones",
		"label.subscriptions":       "Suscripciones",
		"label.fingerprint":         "Huella",
		"label.downloads":           "Descargas",
		"label.uploads":             "Subidas",
		"label.conflicts":           "Conflictos",
		"label.commit":              "Por confirmar",
		"label.outside":             "Cambios fuera de rutas gestionadas",
		"label.applied":             "Aplicados",
		"label.pulled":              "Descargados",
		"label.pushed":              "Subidos",
		"label.written":             "Escritos",
		"label.captured":            "Capturados",
		"label.deleted":             "Eliminados",
		"label.skipped":             "Omitidos",
		"label.backups":             "Copias de seguridad",
		"label.collisions":          "Colisiones",
		"label.kind":                "Tipo",
		"label.name":                "Nombre",
		"label.location":            "Ubicación",
		"label.path":                "Ruta",
		"label.mode":                "Modo",
		"label.transform":           "Transformación",
		"label.detected":            "Agente detectado",
		"label.generic":             "copia genérica",
		"label.behind":              "Atrás",
		"label.incoming":            "Archivos entrantes",
		"label.changed":             "Archivos cambiados",
		"label.backup_branch":       "Historial anterior guardado en la rama",
		"label.local_changes":       "Cambios locales para revisar",
		"label.binary":              "(binario o demasiado grande para mostrar)",
		"label.none":                "(ninguno)",
		"label.yes":                 "sí",
		"label.no":                  "no",
		"label.saved_rule":          "Regla guardada %s (%s), ámbito %s",
		"label.id":                  "ID",
		"label.scope":               "Ámbito",
		"label.enabled":             "Activa",
		"label.source":              "Origen",
		"label.destinations":        "Destinos",
		"label.alias":               "Alias",
		"label.exists":              "Existe",
		"label.url":                 "URL",
		"label.auto_update":         "Actualización automática",
		"cmd.gitignore_show":        "Muestra el .gitignore del repositorio y los patrones recomendados que faltan.",
		"cmd.gitignore_exclude":     "Añade un archivo o carpeta del repositorio al .gitignore.",
		"cmd.gitignore_recommended": "Añade los patrones recomendados del .gitignore que faltan.",
		"cmd.gitignore_save":        "Reemplaza todo el .gitignore (requiere --yes).",
		"label.missing_recommended": "Patrones recomendados que faltan",
		"label.offline":             "sin conexión",
		"msg.rule_removed":          "Regla eliminada. Los archivos ya escritos se conservan.",
		"msg.file_resolved":         "Archivo resuelto.",
		"msg.subscription_removed":  "Suscripción eliminada.",
		"msg.change_discarded":      "Cambio descartado (se hizo una copia de seguridad).",
		"cmd.status":                "Muestra el estado del repositorio, git y reglas (sin red).",
		"cmd.plan":                  "Consulta el remoto y muestra lo que haría la sincronización.",
		"cmd.sync":                  "Descarga, aplica reglas, confirma y sube (o --rules-only para solo aplicar reglas).",
		"cmd.root_set":              "Define la carpeta raíz del repositorio.",
		"cmd.rules_list":            "Lista las reglas con el estado de destinos y archivos.",
		"cmd.rules_show":            "Muestra una regla.",
		"cmd.rules_add":             "Crea una regla; los destinos sin ruta usan la sugerencia de AgentSync.",
		"cmd.rules_remove":          "Elimina una regla (requiere --yes).",
		"cmd.rules_enable":          "Activa una regla.",
		"cmd.rules_disable":         "Desactiva una regla.",
		"cmd.rules_diff":            "Muestra origen y destino de un archivo de una regla.",
		"cmd.rules_resolve":         "Resuelve un archivo de regla en conflicto o modificado.",
		"cmd.dest_add":              "Añade un destino a una regla.",
		"cmd.dest_remove":           "Quita un destino de una regla (requiere --yes).",
		"cmd.loc_list":              "Lista los alias de ubicación de este PC.",
		"cmd.loc_set":               "Asocia un alias a una carpeta de este PC.",
		"cmd.loc_remove":            "Quita la asociación de un alias.",
		"cmd.detect":                "Detecta el tipo de un origen (skill, mcp, instrucciones, archivo, carpeta).",
		"cmd.suggest":               "Sugiere dónde va un origen dentro de una ubicación.",
		"cmd.sub_list":              "Lista las suscripciones.",
		"cmd.sub_add":               "Suscribe a un repositorio remoto (solo lectura).",
		"cmd.sub_check":             "Busca cambios entrantes en una suscripción.",
		"cmd.sub_update":            "Actualiza una suscripción y aplica sus reglas.",
		"cmd.sub_auto":              "Activa o desactiva las actualizaciones automáticas.",
		"cmd.sub_remove":            "Elimina una suscripción y sus reglas (requiere --yes).",
		"cmd.sub_tree":              "Lista los archivos de una suscripción.",
		"cmd.repo_tree":             "Lista los archivos del repositorio con su tipo detectado.",
		"cmd.repo_changes":          "Lista los cambios del repositorio (--fetch para incluir entrantes).",
		"cmd.repo_diff":             "Muestra el diff de un archivo del repositorio.",
		"cmd.repo_discard":          "Descarta un cambio local del repositorio (requiere --yes).",
		"cmd.repo_connect":          "Conecta el repositorio a una URL remota.",
		"cmd.mcp":                   "Ejecuta el servidor MCP por stdio para agentes de IA.",
		"cmd.version":               "Muestra la versión.",
	},
}

// tr returns the message for key in lang (falling back to English, then to the key itself).
func tr(lang, key string, args ...any) string {
	msg, ok := messages[lang][key]
	if !ok {
		msg, ok = messages[LangEN][key]
	}
	if !ok {
		msg = key
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// resolveLang picks the language: --lang flag, AGENTSYNC_LANG, configured language, system locale.
func resolveLang(flagValue, configured string) string {
	for _, candidate := range []string{flagValue, os.Getenv(LangEnvVar), configured} {
		if lang := normalizeLang(candidate); lang != "" {
			return lang
		}
	}
	for _, name := range systemLangEnvVars {
		if lang := normalizeLang(os.Getenv(name)); lang != "" {
			return lang
		}
	}
	return LangEN
}

func normalizeLang(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch {
	case strings.HasPrefix(v, LangES):
		return LangES
	case strings.HasPrefix(v, LangEN):
		return LangEN
	}
	return ""
}

// printHelp prints the full help, or only the commands under a prefix (e.g. "rules").
func printHelp(w io.Writer, lang string, topic []string) {
	prefix := strings.Join(topic, " ")
	fmt.Fprintln(w, tr(lang, "help.title"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, tr(lang, "help.usage"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, tr(lang, "help.commands"))
	for _, c := range commands {
		if prefix != "" && !strings.HasPrefix(c.path, prefix) {
			continue
		}
		line := c.path
		if c.usage != "" {
			line += " " + c.usage
		}
		fmt.Fprintf(w, "  %s\n      %s\n", line, tr(lang, c.desc))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, tr(lang, "help.global"))
	fmt.Fprintln(w, tr(lang, "help.mcp"))
}
