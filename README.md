# AgentSync

AgentSync is a high-performance, lightweight desktop application built with Go, Wails v2, and Svelte 5. It keeps your AI agent configuration files (`CLAUDE.md`, `GEMINI.md`, `AGENTS.md`, prompt templates, custom slash commands, etc.) synchronized across multiple machines via a versioned Git root folder.

Edit a file once and AgentSync writes it everywhere it is needed — with the name each destination
expects (`AGENTS.md` → `CLAUDE.md`, `GEMINI.md`, …) — versioned in git and backed up before every change.
It is not limited to AI agents: any file or folder can be synced to any folder (another project, an
Obsidian vault, …).

## Documentation

| Guide | What it covers |
|---|---|
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | How the program works: concepts, code map, sync flow, safety guarantees |
| [docs/SYNC_RULES.md](docs/SYNC_RULES.md) | Sync rules, locations, scopes, modes, transforms, collisions, rule files |
| [docs/RECOGNIZERS.md](docs/RECOGNIZERS.md) | Content/agent detection and the `recognizers.json` file |
| [docs/REMOTES_AND_SUBSCRIPTIONS.md](docs/REMOTES_AND_SUBSCRIPTIONS.md) | Remote connection, shared repositories (subscriptions), reviewing diffs |
| [docs/design/sync-rules-spec.md](docs/design/sync-rules-spec.md) | Developer contract for the rules engine packages |

---

## Architecture & Conceptual Diagram

```
                 +--------------------------------------------+
                 |            Remote Git Repository           |
                 |     (GitHub, GitLab, Gitea, Bare Repo)     |
                 +---------------------+----------------------+
                                       |
                                git push / pull
                                       |
                 +---------------------v----------------------+
                 |            AgentSync Root Folder           |
                 |               (Local Git Repo)             |
                 |  e.g. D:\agent-configs                     |
                 |   ├── CLAUDE/                              |
                 |   │   ├── CLAUDE.md                        |
                 |   │   └── commands/review.md               |
                 |   ├── GEMINI/                              |
                 |   │   └── GEMINI.md                        |
                 |   └── CODEX/                               |
                 |       └── AGENTS.md                        |
                 +---------------------+----------------------+
                                       |
                           Bidirectional Sync Engine
                           & Automated Backup System
                                       |
         +-----------------------------+-----------------------------+
         |                                                           |
         v                                                           v
+-----------------------------------+     +-----------------------------------+
| Profile: CLAUDE                   |     | Profile: GEMINI                   |
| Target: C:\Users\<User>\.claude   |     | Target: C:\Users\<User>\.gemini   |
|   ├── CLAUDE.md       (managed)   |     |   ├── GEMINI.md       (managed)   |
|   ├── commands/       (managed)   |     |   ├── cache/          (untouched) |
|   └── settings.json   (UNTOUCHED) |     |   └── auth.json       (UNTOUCHED) |
+-----------------------------------+     +-----------------------------------+
```

---

## Key Features

- **Generic Sync Rules (one source → many destinations):** Any file or folder in the repository (or on this PC, or in a subscribed repo) can be written to any number of destinations, each with its own final name, mode (two-way, one-way, create-only) and optional transform. Destinations use per-PC **location aliases**, so shared rules in `agentsync.rules.json` work on every computer. See [docs/SYNC_RULES.md](docs/SYNC_RULES.md).
- **Detection Without Hardcoding:** Skills, MCP configs and instruction files are detected from their content, and agent folders (Claude Code, Codex, Gemini CLI, OpenCode, Antigravity…) are recognized from data in an editable `recognizers.json` to suggest the right path, name and transform. See [docs/RECOGNIZERS.md](docs/RECOGNIZERS.md).
- **MCP Config Merging:** `json_merge` / `toml_merge` transforms write only the MCP servers AgentSync owns into `~/.claude.json`, `settings.json` or `config.toml`, preserving everything else.
- **Collision Warnings:** Two rules targeting the same file, overlapping folders, cycles or case conflicts are always reported and block the involved destinations — never resolved silently.
- **Subscriptions:** Paste the URL of a shared repository, pick only the folders/files you want (e.g. one skill) and stay updated; incoming changes are listed with diffs. See [docs/REMOTES_AND_SUBSCRIPTIONS.md](docs/REMOTES_AND_SUBSCRIPTIONS.md).
- **Lossless Remote Connection & Diff Review:** Connect a folder to a remote even if it is not a repo yet; local history is kept in a backup branch and local differences are reviewed file by file in a diff view (keep yours, take the repository version, or edit a merged version).
- **Connection-Centric Dashboard & Step-by-Step Connection Wizard:** Focuses on active connections between repository profiles and local target directories. Includes a 3-step Connection Wizard (repository profile selection/creation -> target folder selection with live validation & selective file import -> diff preview & conflict resolution) with pre-action safety backups.
- **PathField Component & Universal Folder Navigation:** Every path input throughout the app (onboarding, wizard, settings) supports direct manual typing, live inspection and validation, native system folder picker dialogs, suggestions chips (Claude Code, Gemini, Codex, OpenCode), and inline folder creation.
- **Flexible Onboarding (Select, Create, Clone):** Welcome screen supports selecting an existing repository, creating and initializing a new Git repository folder, or cloning a remote repository directly from HTTPS/SSH URLs.
- **Three-Way State Tracking:** Uses SHA-256 baselines (Source, Target, Baseline) to accurately detect whether local files, repository files, or both have changed, including locked/unreadable file detection.
- **Single-State Service Layer & Concurrency Lock:** `internal/service` maintains a single in-memory `*config.Config` and `*state.State` guarded by a global `sync.Mutex` (`TryLock` for background scheduler checks), preventing race conditions and overlapping Git index operations.
- **Plan Fingerprinting & Safe Sync Order:** `PlanSync` produces a deterministic SHA-256 fingerprint verified by `ExecuteSync` under lock before applying changes. `ExecuteSync` follows a safe 11-step sequence (capture non-conflicting uploads, discard `keep_repo` conflicts, commit, `pull --rebase` before capturing `keep_local` conflicts, push with automatic rebase-retry on non-fast-forward rejection, and apply downloads).
- **Strict Boundary Protection:** Only files explicitly present in a profile are managed. Unmanaged files in target directories (such as API keys, `settings.json`, session histories, and local caches) are never touched, read, or committed.
- **Mandatory Automated Backups:** Target files are backed up in a timestamped snapshot before any overwrite, restore, or discarded repo edit. If backup creation fails, the write is immediately aborted. Restoring a snapshot leaves files as `target_changed` so the next sync uploads the restored version to the repository.
- **Frameless Window & Monochrome Soft Elevation Design System:** Custom frameless titlebar with native window controls, zero-flash theme initialization (`localStorage` + inline boot script), tactile multi-layer shadows, and recessed sunken wells.
- **Bilingual Interface (i18n):** 100% key parity between English (`en`) and Spanish (`es`), including native dialog titles, ARIA labels, and dynamic `<html lang>`.

---

## Requirements

### Operating System
- **Windows 10 / 11** (64-bit). Microsoft Edge WebView2 runtime (pre-installed by default on Windows 10/11).
- **Git** installed and accessible in system `PATH`.

### For Development & Compilation
- **Go 1.23+** (tested with Go 1.27.3)
- **Node.js 18+** & **npm** (tested with Node.js 24)
- **Wails CLI v2** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

---

## Building from Source

To regenerate the application icon and compile a standalone production binary:

```powershell
# Optional: regenerate build/appicon.png from SVG
node scripts/generate-icon.mjs

# Compile production executable
wails build -clean
```

The resulting binary will be placed at:
```
build/bin/agentsync.exe
```

---

## Live Development

To start the application in development mode with hot-module replacement (HMR) for the Svelte 5 frontend and automatic Go recompilation:

```powershell
wails dev
```

---

## Directory Conventions & Setup

### 1. Root Folder (Source Repository)
The root folder is the source of truth for your agent profiles. Each direct subfolder represents a profile:
```
D:\agent-configs\
├── CLAUDE\
│   ├── CLAUDE.md
│   └── commands\
│       ├── test.md
│       └── review.md
├── GEMINI\
│   └── GEMINI.md
└── CODEX\
    └── AGENTS.md
```
- Folders starting with `.` (such as `.git`) are automatically ignored.
- Ignored files across profiles: `.gitkeep`, `.DS_Store`, `Thumbs.db`, `desktop.ini`, `*.agentsync.tmp`.

### 2. Linking Target Directories
Inside AgentSync, link each discovered profile to the corresponding directory on your machine:
- Profile `CLAUDE` → `C:\Users\<User>\.claude`
- Profile `GEMINI` → `C:\Users\<User>\.gemini`
- Profile `CODEX`  → `C:\Users\<User>\.codex`

When connecting for the first time, AgentSync detects if files already exist on target or source and lets you choose whether to import target files into the repository or populate the target from the repository.

---

## Testing with the Sandbox

A self-contained testing sandbox simulates two distinct workstations (`PC-A` and `PC-B`) sharing a bare Git remote:

### 1. Setup Sandbox
```powershell
powershell -ExecutionPolicy Bypass -File scripts/setup-sandbox.ps1
```
This generates:
- `sandbox/remote.git`: Bare central Git repository.
- `sandbox/pc-a/agent-configs`: Cloned Git root for PC-A.
- `sandbox/pc-a/home/`: Simulated target directories for PC-A (`.claude`, `.codex`, `.gemini`, `.config/opencode`).
- `sandbox/pc-b/agent-configs`: Cloned Git root for PC-B.
- `sandbox/pc-b/home/`: Simulated target directories for PC-B.

### 2. Launch AgentSync in Sandbox Isolation
To run the app against the sandbox without touching your real machine configuration:
```powershell
$env:AGENTSYNC_CONFIG_DIR = "$PWD\sandbox\pc-a\config"
.\build\bin\agentsync.exe
```

### 3. Reset Sandbox
```powershell
powershell -ExecutionPolicy Bypass -File scripts/reset-sandbox.ps1
```

---

## Running the Automated Test Suite

### Go Unit, Integration, & E2E Tests
```powershell
# Run all Go packages (excluding frontend/node_modules) with code coverage
go test . ./internal/... -count=1 -cover
```

Coverage highlights:
- `internal/scheduler`: **95.7%**
- `internal/orchestrator`: **89.3%**
- `internal/syncer`: **89.1%**
- `internal/state`: **84.2%**
- `internal/service`: **81.5%**
- `internal/backup`: **77.9%**
- `internal/profiles`: **75.9%**
- `internal/config`: **68.5%**
- `internal/gitops`: **62.4%**
- `internal/fsops`: **54.2%**
- `internal/apperr`: **46.2%**

### Frontend Lint, Type Check & Vitest
```powershell
cd frontend
npm run lint    # ESLint check (0 errors, 0 warnings)
npm run check   # Svelte-check diagnostics (0 errors, 0 warnings)
npm run test    # Vitest suite (17 tests across 6 files, 100% pass)
npm run build   # Production Vite bundle verification
cd ..
```

---

## Code Quality Standards

The codebase adheres strictly to:
- Conventional Commits (`feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`).
- `gofmt -l .` with zero formatting deviations.
- `go vet . ./internal/...` with zero issues.
- `staticcheck . ./internal/...` with zero warnings.
- 100% key parity between English and Spanish localizations.
