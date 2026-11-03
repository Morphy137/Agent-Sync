# Installing and updating AgentSync

## Ways to run it

| Option | Where the program lives | Where your data lives | Updates |
|---|---|---|---|
| **Installer** (`agentsync-setup.exe`, recommended) | `%LOCALAPPDATA%\Programs\AgentSync` (or the folder you choose) | `%APPDATA%\AgentSync` | Automatic, from inside the app |
| **Portable** (`agentsync.exe` + an empty file named `portable` next to it) | Any folder (USB drive, synced folder…) | `data\` next to the program | Download the new `agentsync.exe` by hand |
| **Loose exe** (`agentsync.exe` alone) | Any folder | `%APPDATA%\AgentSync` | Download by hand |

`AGENTSYNC_CONFIG_DIR` overrides all of them (used by tests and sandboxes).

Data means `config.json`, `state.json`, `rules.local.json`, `recognizers.json`, subscriptions and,
by default, backups (`backups\`). The backup folder can be changed in **Settings → Backups**.

### Why data is not stored in the program folder by default

- The installer replaces the program folder on every update and removes it on uninstall.
- Windows does not let normal users write into `Program Files`.

So the installed app keeps data in `%APPDATA%\AgentSync`, which survives updates, reinstalls and
uninstalls. Portable mode exists for people who want everything in one folder.

## The installer

- Per-user install: no administrator prompt.
- Adds Start menu and desktop shortcuts, an entry in *Apps & features*, `agentsync-cli.exe` next to
  the app, and the WebView2 runtime if it is missing.
- English or Spanish, following the system language.
- Uninstalling keeps `%APPDATA%\AgentSync`.

Silent install (IT/scripts): `agentsync-setup.exe /S` (add `/D=C:\path` as the last argument to pick
the folder). Silent updates reuse the folder of the first install (`HKCU\Software\AgentSync\InstallDir`).

## Running in the background

**Settings → Background** has three optional switches, all off by default:

- **Apply subscription updates automatically**: the periodic check also copies new, conflict-free
  subscription files into your folders (backup first, nothing committed or pushed).
- **Keep running in the system tray**: the close button hides the window; the tray icon opens it,
  runs a check (*Sync now*) or quits. Without this option the close button quits.
- **Start with Windows**: adds an `AgentSync` entry to `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
  that starts `"<exe>" --minimized`: hidden in the tray when the tray option is on, otherwise
  minimized to the taskbar so the window is always reachable. The switch shows the real state of
  the entry, and the entry follows the program if you move it. It is unavailable in `wails dev`
  builds, and portable copies can use it. Uninstalling removes the entry.

Only one AgentSync runs per data folder: launching it again shows the existing window.

## Automatic updates

1. At startup and every 6 hours the app reads the releases of
   [Morphy137/Agent-Sync](https://github.com/Morphy137/Agent-Sync/releases) (turn this off in
   **Settings → Help & about**).
2. When a newer version exists, the bell shows it with **What's new** and **Install update**.
3. Installing downloads `agentsync-setup.exe`, checks its SHA-256 against `SHA256SUMS.txt` from the
   same release, runs it silently with `/relaunch` and closes the app; the installer reopens AgentSync.
4. Portable and loose copies open the release page instead.

Closing to the tray never blocks an update: installing quits the app for real before the installer
replaces its files, and the installer's `/relaunch` opens the new version (visible, without
`--minimized`) once the old process is gone.

Downloads are only accepted over HTTPS from GitHub hosts; a file whose checksum does not match is
deleted. Draft releases are ignored; pre-releases count while AgentSync is 0.x.

## Publishing a release

1. Move the `## [Unreleased]` entries of `CHANGELOG.md` to `## [x.y.z] - YYYY-MM-DD`.
2. Set the same version in `wails.json` (`info.productVersion`) and `frontend/package.json`.
3. Merge into `main`, then tag and push: `git tag -a vX.Y.Z -m "AgentSync X.Y.Z"` and
   `git push origin vX.Y.Z`.
4. The **Release** workflow (`.github/workflows/release.yml`) tests, runs
   `scripts/build-release.ps1` and publishes the installer, both executables and `SHA256SUMS.txt`
   with notes taken from the changelog (`scripts/release-notes.ps1`).

To build locally: `./scripts/build-release.ps1` (needs Go, Node, Wails and NSIS).
