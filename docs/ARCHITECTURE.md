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
                  │ CLAUDE/ CODEX/ …   (legacy folder connections)  │
                  └───────────────┬─────────────────────────────────┘
                                  │ rules: one source → many destinations
          ┌───────────────────────┼─────────────────────────┐
          ▼                       ▼                         ▼
  claude → ~/.claude/CLAUDE.md   gemini → ~/.gemini/GEMINI.md   obsidian → D:\Vault\IA\Agents.md
  (location aliases are mapped per PC in config.json)

  subscriptions (read-only clones of other people's repos) can also be rule sources
```

| Concept | Meaning | Stored in | Shared across your PCs |
|---|---|---|---|
| **Repository root** | Folder (normally a git repo) holding your source files | `config.json` → `rootPath` | — |
| **Rule** | One source (file/folder) → N destinations, each with its own path/name, mode and optional transform | Shared: `<repo>/agentsync.rules.json` · This PC only: `<configDir>/rules.local.json` | Shared rules: yes, through git |
| **Location** | Alias → folder on *this* PC (`claude` → `C:\Users\me\.claude`). Builtin `home` = your user folder | `config.json` → `locations` | No — each PC maps its own paths |
| **Subscription** | Read-only clone of a remote repo used as a rule source | `config.json` → `subscriptions`, clone in `<configDir>/subscriptions/<id>/repo` | No |
| **Recognizers** | Data describing content kinds (skill, MCP, instructions) and known agent folders, used to *suggest* placements | Embedded defaults + `<configDir>/recognizers.json` | No |
| **Folder connection** (legacy) | Repo profile folder (e.g. `CLAUDE/`) mirrored to one target folder | `config.json` → `connections` | — |
| **Baselines** | Last synced hash per file, used for 3-way change detection | `<configDir>/state.json` | No |
| **Backups** | Snapshot of every file before AgentSync overwrites/deletes it | `<backupPath>` (Settings) | No |

`<configDir>` is `%APPDATA%\AgentSync` on Windows (`os.UserConfigDir()/AgentSync` elsewhere),
overridable with the `AGENTSYNC_CONFIG_DIR` environment variable (used by tests and the sandbox).
`AGENTSYNC_HOME_OVERRIDE` overrides what the builtin `home` location resolves to.

---

## 2. Code map

| Layer | Package / folder | Responsibility |
|---|---|---|
| UI | `frontend/src` (Svelte 5) | Views: Sync rules, Folder connections, Subscriptions, Locations; modals (rule editor, diff, sync preview, connect remote) |
| Bindings | `app.go` | Thin Wails wrappers over `internal/service` (generated TS in `frontend/wailsjs`) |
| Headless | `cmd/agentsync-cli`, `internal/cli`, `internal/mcpserver`, `internal/ops` | CLI and stdio MCP server over the same `internal/service` (see [CLI_AND_MCP.md](CLI_AND_MCP.md)) |
| Service | `internal/service` | Single owner of config + state, guarded by a mutex plus a cross-process file lock (`internal/filelock`) shared with the CLI/MCP server; orchestrates everything below; emits `status:changed` and `rules:changed` events |
| Rules | `internal/rules` | Rule model, storage, validation, folder walking, path resolution, collision detection |
| Rule sync | `internal/rulesync` | 3-way status, sync (write/capture/delete) with backups, conflict resolution, diffs |
| Detection | `internal/recognize` | Content-kind detection, agent-folder detection, placement suggestions (`recognizers.json`) |
| Transforms | `internal/transform` | `json_merge` / `toml_merge` of managed entries (e.g. MCP servers) |
| Git | `internal/gitops` | Hardened git CLI wrapper, URL validation, `ConnectRemote` |
| Subscriptions | `internal/subscriptions` | Clone / check / update / tree / incoming diffs of read-only repos |
| Legacy connections | `internal/syncer`, `internal/profiles` | Folder-mirror connections (profile ↔ target) |
| Global sync | `internal/orchestrator` | Commit → pull → apply connections → capture → push |
| Infra | `internal/config`, `internal/state`, `internal/backup`, `internal/fsops`, `internal/apperr`, `internal/scheduler` | Persistence, baselines, snapshots, atomic file ops, typed errors, periodic checks |

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
2. Commit pending local edits inside managed paths (profile folders, rule sources, `agentsync.rules.json`).
3. Pull (fast-forward or rebase), then apply folder connections.
4. Capture folder-connection edits, commit, push.
5. **Apply rules** with the freshly pulled content; rule captures (destination edits flowing back
   into repo sources) are committed and pushed.

The Rules view also has its own **Sync rules** button that only applies rules (captured repo
changes are committed on the next global Sync).

### Periodic check (scheduler)

Every *N* minutes (Settings): fetch the main remote, fetch every subscription (auto-update
the ones that opted in), recompute the plan and refresh the UI. Nothing is written to your
destinations unless a subscription has auto-update on; conflicting files are always skipped.

---

## 4. Safety guarantees

- **Backups before every overwrite or delete** (rules, connections, discards). Rule backups live in
  `<backupPath>/rules/<ruleId>__<destId>/`; source snapshots taken before captures use the `_source` slot.
- **Atomic writes** (`*.agentsync.tmp` + rename).
- **No silent priority**: collisions block every involved destination until you change the rules.
- **Transforms touch only managed entries** (e.g. only the MCP servers AgentSync wrote) and keep
  the rest of `~/.claude.json`, `settings.json` or `config.toml` untouched.
- **Secrets are never picked up from folder sources**: `.credentials.json`, `auth.json`, `*.pem`,
  `*.key`, `.env`, `.env.*`, `id_rsa*`, `*.p12` are always excluded.
- **Subscriptions are read-only**: their files are never written back; local edits show as drift.
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

---

## 6. i18n

UI strings live in `frontend/src/lib/i18n/locales/<locale>.json` (core) and
`locales/<locale>/<namespace>.json` (feature namespaces, merged at load). English is the base,
Spanish is complete; the default language is the system language. Tests enforce key parity,
no duplicate or overlapping keys, and no hardcoded text in components.
