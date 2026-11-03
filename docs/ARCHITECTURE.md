# AgentSync — How the program works

AgentSync keeps configuration files (agent instructions, skills, MCP configs, or any other
file/folder) in **one git repository** and writes them to **many places** on each computer.
You edit a file once; every destination gets it with the name it needs, versioned and backed up.

This document is the map. Topic guides:

| Topic | Document |
|---|---|
| Sync rules, locations, modes, collisions, the rules files | [SYNC_RULES.md](SYNC_RULES.md) |
| Content/agent detection and `recognizers.json` | [RECOGNIZERS.md](RECOGNIZERS.md) |
| Subscriptions (shared repos), connecting to a remote, reviewing diffs | [REMOTES_AND_SUBSCRIPTIONS.md](REMOTES_AND_SUBSCRIPTIONS.md) |
| CLI and MCP server for AI agents, running next to the GUI | [CLI_AND_MCP.md](CLI_AND_MCP.md) |
| Binding implementation contract (developer reference) | [design/sync-rules-spec.md](design/sync-rules-spec.md) |

---

## 1. Concepts

```
                  ┌─────────────── repository (git) ────────────────┐
                  │ shared/AGENTS.md   skills/…   mcp.json          │
                  │ agentsync.rules.json   (shared rules)           │
                  └───────────────┬─────────────────────────────────┘
                                  │ rules: one source → many destinations
          ┌───────────────────────┼─────────────────────────┐
          ▼                       ▼                         ▼
  claude → ~/.claude/CLAUDE.md   gemini → ~/.gemini/GEMINI.md   obsidian → D:\Vault\IA\Agents.md
  (location aliases are mapped per PC in config.json)

  subscriptions (local clones of other people's repos) can also be rule sources
```

| Concept | Meaning | Stored in | Shared across your PCs |
|---|---|---|---|
| **Repository root** | Folder (normally a git repo) holding your source files | `config.json` → `rootPath` | — |
| **Rule** | One source (file/folder) → N destinations, each with its own path/name, mode and optional transform | Shared: `<repo>/agentsync.rules.json` · This PC only: `<configDir>/rules.local.json` | Shared rules: yes, through git |
| **Location** | Alias → folder on *this* PC (`claude` → `C:\Users\me\.claude`). Builtin `home` = your user folder | `config.json` → `locations` | No — each PC maps its own paths |
| **Subscription** | Local clone of a remote repo used as a rule source; local edits are not pushed | `config.json` → `subscriptions`, clone in `<configDir>/subscriptions/<id>/repo` | No |
| **Recognizers** | Data describing content kinds (skill, MCP, instructions) and known agent folders, used to *suggest* placements | Embedded defaults + `<configDir>/recognizers.json` | No |
| **Baselines** | Last synced hash per file, used for 3-way change detection | `<configDir>/state.json` | No |
| **Backups** | Snapshot of every file before AgentSync overwrites/deletes it | `<backupPath>` (Settings) | No |

`<configDir>` is `%APPDATA%\AgentSync` on Windows (`os.UserConfigDir()/AgentSync` elsewhere),
overridable with the `AGENTSYNC_CONFIG_DIR` environment variable (used by tests and the sandbox).
`AGENTSYNC_HOME_OVERRIDE` overrides what the builtin `home` location resolves to.

---

## 2. Code map

| Layer | Package / folder | Responsibility |
|---|---|---|
| UI | `frontend/src` (Svelte 5) | Views: Sync rules, Files, Subscriptions, Locations; modals (rule editor, diff, sync preview, connect remote) |
| Bindings | `app.go` | Thin Wails wrappers over `internal/service` (generated TS in `frontend/wailsjs`) |
| Desktop shell | `lifecycle.go`, `tray*.go`, `main.go`, `internal/autostart` | Close-to-tray, tray icon, single instance, start with Windows (see "Background behaviour") |
| Headless | `cmd/agentsync-cli`, `internal/cli`, `internal/mcpserver`, `internal/ops` | CLI and stdio MCP server over the same `internal/service` (see [CLI_AND_MCP.md](CLI_AND_MCP.md)) |
| Service | `internal/service` | Single owner of config + state, guarded by a mutex plus a cross-process file lock (`internal/filelock`) shared with the CLI/MCP server; orchestrates everything below; emits `status:changed` and `rules:changed` events |
| Rules | `internal/rules` | Rule model, storage, validation, folder walking, path resolution, collision detection |
| Rule sync | `internal/rulesync` | 3-way status, sync (write/capture/delete) with backups, conflict resolution, diffs |
| Detection | `internal/recognize` | Content-kind detection, agent-folder detection, placement suggestions (`recognizers.json`) |
| Transforms | `internal/transform` | `json_merge` / `toml_merge` of managed entries (e.g. MCP servers) |
| Git | `internal/gitops` | Hardened git CLI wrapper, URL validation, `ConnectRemote` |
| Subscriptions | `internal/subscriptions` | Clone / check / update / tree / incoming diffs of shared repos |
| Global sync | `internal/orchestrator` | Commit managed edits → pull → push (rules run afterwards, in the service) |
| Infra | `internal/config`, `internal/state`, `internal/backup`, `internal/fsops`, `internal/apperr`, `internal/scheduler`, `internal/autostart` | Persistence, baselines, snapshots, atomic file ops, typed errors, periodic checks |

---

## 3. Sync flow

### Rule status (per destination file)

Each file pair (source → destination) compares three hashes: **source**, **destination** and
**baseline** (last synced). The result is one of:

| Status | Meaning | What Sync does |
|---|---|---|
| `in_sync` | Nothing to do | — |
| `new_in_source` / `source_changed` / `target_missing` | Source is newer | Writes the destination (backup first) |
| `target_changed` | Destination edited, source unchanged | Two-way: copies it back into the source (capture). Otherwise: drift |
| `source_deleted` | File removed from a folder source, destination untouched | Deletes the destination file (backup first) |
| `conflict` / `untracked_diff` | Both sides changed / first link with different content | Nothing — you decide in the diff view |
| `drift` | One-way, transformed or subscription destination edited locally | Nothing — you decide in the diff view |
| `ignored` | You chose "keep local, stop syncing this file" | Never touched |

Destinations can also be **blocked** (collision), **unmapped** (location not set on this PC),
**disabled** or have a **missing source**; those are skipped and shown in the Rules view.

### Global Sync button

1. Revalidate the plan (fetch remote, fingerprint check).
2. Commit pending local edits inside managed paths (repo-origin rule sources and `agentsync.rules.json`).
   Edits elsewhere in the repository are reported as "outside managed paths" and never committed; if
   the remote also changed one of them, Sync stops with `git_dirty_worktree` until you clean it.
3. Pull (fast-forward, or rebase when there are local commits), then push (one retry if the remote moved).
4. **Apply rules** with the freshly pulled content; rule captures (destination edits flowing back
   into repo sources) are committed and pushed.

When the remote changed a file you also edited locally inside a managed path, Sync lists it as a
conflict and asks for a decision: **keep repository** (the local edit is backed up and discarded) or
**keep this PC** (the local version is backed up, set aside while pulling and committed on top).

The Rules view also has its own **Sync rules** button that only applies rules (captured repo
changes are committed on the next global Sync).

### Periodic check (scheduler)

Every *N* minutes (Settings): fetch the main remote, fetch every subscription (auto-update
the ones that opted in), recompute the plan and refresh the UI. Nothing is written to your
destinations unless a subscription has auto-update on or **Apply subscription updates
automatically** (`autoApplyUpdates`, off by default) is enabled; conflicting files are always skipped.

With `autoApplyUpdates`, the apply phase of `TryBackgroundCheck` (under the service lock, skipped when
another operation holds it) runs `rulesync.Engine.SyncMatching` for each subscription's rules. That is
the engine path of "Sync rules", restricted to rules whose origin is `subscription`: overwritten files
are backed up first, conflicts, drift and local edits are skipped, and nothing is committed or pushed.
When files were written the service emits `rules:auto_applied` and the UI shows a bell notification.

### Background behaviour (tray, start with Windows, single instance)

Three settings, all off by default, live in **Settings → Background**:

- **Close to tray** (`closeToTray`): while on, `trayIcon` (`tray_windows.go`, built on `fyne.io/systray`)
  shows a notification-area icon with Open, Sync now and Quit, in the configured language. Wails v2
  has no tray API, so the tray runs on its own OS-thread-locked goroutine and message loop. The icon
  appears and disappears live when the setting changes. Left click opens the window. *Sync now*
  runs a remote check and opens the window when something is waiting to be applied.
  `OnBeforeClose` hides the window instead of closing it only while the tray icon is on screen, so
  without a tray the close button still quits. The scheduler keeps running while hidden.
- **Real quits** call `App.quit`, which raises a flag before `runtime.Quit`, because every Wails
  `Quit` goes through `OnBeforeClose`. Tray Quit and the updater's `InstallUpdate` use it, so an
  installer never runs against a hidden, running app. The title-bar X calls `Quit` and follows the
  setting. `OnShutdown` stops the scheduler and removes the tray icon.
- **Start with Windows** (`startWithWindows`): `internal/autostart` writes
  `"<exe>" --minimized` to the `AgentSync` value of `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
  through a `Store` interface (tests use a fake, never the registry). Settings shows the real registry
  state. At launch the entry is rewritten when the program moved, and the stored option is cleared
  when the entry was removed outside the app. `--minimized` hides the window only when
  `closeToTray` is on; otherwise it starts minimized. Development builds (`wails dev`, tag `dev`)
  cannot use it because their executable path is temporary.
- **Single instance**: Wails' `SingleInstanceLock` shows the running window when AgentSync is
  launched again. The lock id includes a hash of the data folder (and a suffix for development
  builds), so portable copies, sandboxes and dev builds with their own data do not block each other.

### Upgrading from folder connections

Earlier versions mirrored a top-level repository folder (the "profile", e.g. `CLAUDE/`) to one folder
on the PC (`config.json` → `connections`). Rules replaced that feature, and on the first start after
upgrading each old connection becomes a **local** rule (this PC only):

- source: the profile folder in the repository (origin `repo`, kind `folder`);
- one destination, mode `two_way` (the old mirror copied repo changes to the PC and PC edits back, and
  asked on conflicts, which is what a two-way rule does), at the root of a location;
- the location reuses an existing alias that has the same path, otherwise a new alias is made from the
  profile name in lower case (`CLAUDE` → `claude`, `claude-2` when taken);
- the last synced hashes carry over, so the first rule sync still knows which side changed;
- profile folders that no longer exist are skipped, and `connections` plus the old baselines are
  then removed from `config.json` and `state.json`. Running it again does nothing.

One behaviour differs: removing a file from the profile folder now deletes it from the destination
(with a backup), instead of leaving it behind.

---

## 4. Safety guarantees

- **Backups before every overwrite or delete** (rules, discards). Rule backups live in
  `<backupPath>/rules/<ruleId>__<destId>/`; source snapshots taken before captures use the `_source` slot.
  Backups can be listed and restored from Settings, the CLI and MCP (`internal/backup/browse.go`,
  `rulesync.Engine.RestoreFiles`, `service.ListBackups` / `RestoreBackup`); a restore backs up the
  current files first and leaves restored files as differences to review (see SYNC_RULES.md).
- **Atomic writes** (`*.agentsync.tmp` + rename).
- **No silent priority**: collisions block every involved destination until you change the rules.
- **Transforms touch only managed entries** (e.g. only the MCP servers AgentSync wrote) and keep
  the rest of `~/.claude.json`, `settings.json` or `config.toml` untouched.
- **Secrets are never picked up from folder sources**: `.credentials.json`, `auth.json`, `*.pem`,
  `*.key`, `.env`, `.env.*`, `id_rsa*`, `*.p12` are always excluded.
- **Subscription edits stay local**: they are never pushed to the source repository, and updates are blocked while local files have changes.
- **Git hardening**: `-c protocol.ext.allow=never`, `GIT_TERMINAL_PROMPT=0`, URL validation
  (rejects `ext::`, option injection, control characters), URLs passed after `--`.
- **Connecting a remote never loses local files**: local history is saved in a backup branch and
  local differences stay as uncommitted changes for you to review.

---

## 5. Events between backend and UI

| Event | Payload | Emitted when |
|---|---|---|
| `status:changed` | `orchestrator.SyncPlan` (includes `rulesPending`, `rulesConflicts`) | Any change affecting the global plan |
| `rules:changed` | none | Rules, locations or subscriptions changed; UI reloads rule data |
| `rules:auto_applied` | `{count, sources}` | A periodic check wrote subscription files on its own (`autoApplyUpdates`) |

---

## 6. i18n

UI strings live in `frontend/src/lib/i18n/locales/<locale>.json` (core) and
`locales/<locale>/<namespace>.json` (feature namespaces, merged at load). English is the base,
Spanish is complete; the default language is the system language. Tests enforce key parity,
no duplicate or overlapping keys, and no hardcoded text in components.
