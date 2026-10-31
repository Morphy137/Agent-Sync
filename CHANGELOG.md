# Changelog

All notable changes to AgentSync are listed here. The app shows this file in **Settings → Help → What's new**.

## [Unreleased]

## [0.0.3] - 2026-10-31

### Added
- Installer: installs for your user without administrator rights, with Start menu and desktop shortcuts and the command line tool included.
- Automatic updates: AgentSync checks for new versions and can install them and restart by itself.
- Portable mode: put an empty file named `portable` next to `agentsync.exe` to keep all data beside the program.
- Settings → Help & about shows the version, how this copy is installed and where your data is kept.

### Changed
- "Star on GitHub" is now a clear card in Settings → Help & about.

## [0.0.2] - 2026-10-31

### Added
- Notification bell: repository messages appear briefly as a popup and then stay listed under the bell.
- Themes now change the whole app (background, surfaces and accent): White, Black, Nord, Rose and Green, plus more under "More".
- Pick several files at once as a rule source, shown as a compact grid or list.
- "New location" opens the folder picker and saves the location for you.
- Rules can be shown as a grid or a list.
- "What's new" and a link to the GitHub project in Settings → Help.
- An agent skill that teaches Claude Code, Codex or Gemini CLI to use AgentSync.

### Changed
- Rule cards are more compact; "This PC only" is now "Local".
- Tabs slide smoothly and every view uses the same header and main button size.
- Badges and chips use squared corners; option labels are centered.
- Content detection is shown with a check mark (or a question mark when unknown).
- Softer shadows, especially in dark themes and dialogs.

### Fixed
- The notification popup no longer turns see-through when the window loses focus.

### Removed
- The separate color palette setting (now part of themes).
- The "Generic copy" label in destinations.

## [0.0.1] - 2026-10-31

### Added
- Sync rules: copy one file or folder to many places, each with its own name.
- Locations: name your folders once per PC so rules work on every computer.
- Detection of skills, MCP configs and instruction files with suggested paths.
- MCP config merge into JSON and TOML agent settings.
- Collision warnings and backups before every overwrite.
- Git sync between computers, connect to a remote and review changes with diffs.
- Subscriptions to shared repositories.
- Repository exclusions editor.
- First-run tutorial.
- Command line and MCP server for AI agents.
- English and Spanish.
