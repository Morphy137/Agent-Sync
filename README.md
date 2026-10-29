# AgentSync

AgentSync is a high-performance, lightweight desktop application built with Go, Wails v2, and Svelte 5. It keeps your AI agent configuration files (`CLAUDE.md`, `GEMINI.md`, `AGENTS.md`, prompt templates, custom slash commands, etc.) synchronized across multiple machines via a versioned Git root folder.

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
                 |       └── instructions.md                  |
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

- **Three-Way State Tracking:** Uses SHA-256 baselines (Source, Target, Baseline) to accurately detect whether local files, repository files, or both have changed.
- **Strict Boundary Protection:** Only files explicitly present in a profile are managed. Unmanaged files in target directories (such as API keys, `settings.json`, session histories, and local caches) are never touched, read, or committed.
- **Safety First with Automated Backups:** Any target file before being overwritten is backed up in a timestamped snapshot. Backups can be restored with a single click. Configurable retention pruning automatically maintains the desired number of snapshots.
- **Conflict Resolution UI:** Visual diffs allow choosing between keeping repository versions (`keep_repo`) or keeping local workstation versions (`keep_local`).
- **Atomic File Operations:** All file writes use `.agentsync.tmp` files, synced to disk (`fsync`), and atomically replaced via filesystem rename.
- **First-Class Git Integration:** Supports Git init, branch checkout, commit, push, pull, remote branch tracking (`ahead` / `behind`), and remote URL assignment through the system Git CLI without third-party dependencies or external credentials configuration.
- **Bilingual Interface (i18n):** Zero hardcoded user strings; full parity between English (`en`) and Spanish (`es`).
- **Modern Theme System:** Custom CSS design tokens with support for Light, Dark, and System modes.
- **Lightweight Single Binary:** Produces a single standalone Windows executable (~11.8 MB) using native WebView2, requiring no Electron runtime.

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

To compile a standalone production binary:

```powershell
# In the repository root:
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
    └── instructions.md
```
- Folders starting with `.` (such as `.git`) are automatically ignored.
- Ignored files across profiles: `.gitkeep`, `.DS_Store`, `Thumbs.db`, `desktop.ini`.

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
- `sandbox/pc-a/home/`: Simulated target directories for PC-A (`.claude`, `.codex`, `.gemini`).
- `sandbox/pc-b/agent-configs`: Cloned Git root for PC-B.
- `sandbox/pc-b/home/`: Simulated target directories for PC-B.

### 2. Launch AgentSync in Sandbox Isolation
To run the app against the sandbox without touching your real machine configuration:
```powershell
$env:AGENTSYNC_CONFIG_DIR = "$PWD\sandbox\config-pc-a"
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
# Run all tests with code coverage
go test -cover ./internal/...
```

Coverage highlights:
- `internal/syncer`: **92.2%**
- `internal/orchestrator`: **89.7%**
- `internal/backup`: **81.0%**
- `internal/state`: **84.2%**
- `internal/gitops`: **74.8%**
- `internal/profiles`: **75.9%**
- `internal/config`: **70.8%**

### Frontend Lint, Type Check & Vitest
```powershell
cd frontend
npm run lint    # ESLint check
npm run check   # Svelte-check diagnostics
npm run test    # Vitest suite (including i18n key parity)
npm run build   # Production Vite bundle verification
cd ..
```

---

## Code Quality Standards

The codebase adheres strictly to:
- Conventional Commits (`feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`).
- `gofmt -l .` with zero formatting deviations.
- `go vet ./...` with zero issues.
- `staticcheck ./...` with zero warnings.
- 100% key parity between English and Spanish localizations.
