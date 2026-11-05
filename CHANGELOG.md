# Changelog

All notable changes to AgentSync are listed here. The app shows this file in **Settings → Help → What's new**.

## [Unreleased]

### Added
- Organize sync rules into colored, collapsible categories, with automatic suggestions for existing and new rules.
- Choose category icons and colors visually, preview changes, and drag complete categories and rules to reorder them.

### Fixed
- Dragged rules keep their category color around the handle.
- Sync separate MCP servers and skills to the same destination without false conflicts.

## [0.0.8] - 2026-11-03

### Added
- Search your sync rules by name, source, subscription or destination; press Esc to clear.
- Shared repositories can suggest rules: after you subscribe, pick the ones you want and apply them in one step. You see where each file will go before accepting, and nothing is copied until you sync.
- Settings → Backups now has "View and restore": see every backup, pick all or some files and put them back. Your current files are backed up first, and restored files show up as differences to review instead of being overwritten by the next sync.
- Optional setting to apply subscription updates automatically when they have no conflicts or local edits. A backup is made first, nothing is committed or pushed, and the bell tells you how many files changed.
- Optional setting to keep AgentSync in the system tray when you close the window, with Open, Sync now and Quit.
- Optional setting to start AgentSync with Windows.

### Changed
- When a file changed both here and in the repository, you can now keep your version during a sync.
- Opening AgentSync a second time shows the window that is already running instead of starting another copy.
- Rule rows no longer repeat the origin badge: hover the source icon to see where a rule comes from, and file names get more room.

### Removed
- Folder connections are gone; existing ones become sync rules automatically.

## [0.0.7] - 2026-11-03

### Changed
- Settings now save on their own; the Save button is gone.
- Sync rules, subscriptions, connections and locations use tidy, aligned rows that keep their size.
- Disabled rules look dimmed but keep the same layout.
- Synced files open from a small arrow on each row instead of an extra line.
- The app opens faster and stays responsive while checking for remote changes.
- The rules file of a shared repository no longer appears in its file list.
- Markdown files in the Files tab open in reading mode; use the eye button to edit them.
- Pages use the full width of larger windows, and rule names are no longer cut short on wide screens.
- The Review button now appears in place of the sync mode on rules that need review.
- Subscription cards are more compact: the branch opens the repository page, and the actions fit on one row.
- Long folder paths in Locations are shortened in the middle and fit on a single line.
- The Locations description now says each computer picks its own folders.

### Added
- A button to restore all settings to their defaults.
- A button on each sync rule opens a read-only view of its source files, with a file list for folders and skills.
- Click a file in an expanded rule to read its source.
- Published the project under the MIT license.

### Fixed
- The Sync button now appears as soon as you create, enable, disable or delete a rule.

## [0.0.6] - 2026-11-03

### Added
- Browse and edit sync rule source files directly in the new Files tab.
- Preview Markdown files while editing them.
- Navigate repository folders and format JSON files in the file editor.

### Changed
- Show subscription sources clearly and keep sync rules in a single list layout.
- Allow removing the built-in home location and reassigning it later.
- Protect local subscription edits when updating or removing a subscription.
- Keep warnings and errors in the notification center for review.
- Show subscription names in sync rules.
- Removed the folder connections tab.
- Clarified interface text and improved singular and plural messages in English and Spanish.
- Made the sync rules list more compact, with paths and actions on the same row.

### Fixed
- Avoid flashing setup and file navigation controls while the app loads.
- Keep destination folders accessible when their rules are disabled.
- Keep the navigation and headings aligned when switching tabs.
- The file editor now expands with the application window.
- Kept the navigation tabs the same size in the file editor.
- Kept rule destinations consistently sized when disabled.

## [0.0.5] - 2026-10-31

### Changed
- Rounded the app icon across the application and installer.

## [0.0.4] - 2026-10-31

### Changed
- Refined the rule editor, subscription guidance, locations, notifications and GitHub star in Settings.
- Updated the app icon.

### Fixed
- Improved keyboard navigation and focus in dialogs and controls.
- Fixed local repository paths, restore confirmations and several labels and layout issues.

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
