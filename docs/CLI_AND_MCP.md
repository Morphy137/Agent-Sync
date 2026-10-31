# AgentSync CLI and MCP server

`agentsync-cli` runs AgentSync without the window. It has two uses:

- **CLI** for you and your scripts: `agentsync-cli rules add --source AGENTS.md --to claude:CLAUDE.md`.
- **MCP server** for AI agents: `agentsync-cli mcp` lets Claude Code, Codex, Gemini CLI (or any MCP
  client) set up and run syncs when you ask in plain words, e.g. *"sync my `skills/review` skill to
  Claude and Codex"*.

Both call the same `internal/service` the GUI binds (through the thin `internal/ops` facade), so rules,
detection, suggestions, backups, collision checks and git handling behave exactly like in the app.

---

## 1. Install

```powershell
# from the repository checkout
go build -o build/bin/agentsync-cli.exe ./cmd/agentsync-cli
# or put it on PATH (%USERPROFILE%\go\bin)
go install ./cmd/agentsync-cli
```

It is a separate console binary because the Wails GUI executable is built for the Windows GUI
subsystem (no console attached), which is unsuitable for a terminal tool or a stdio server.

The CLI uses the same configuration as the app (`%APPDATA%\AgentSync`, or `AGENTSYNC_CONFIG_DIR`).
Set the repository root once if you have not opened the app yet:

```powershell
agentsync-cli root set C:\Users\me\agent-configs
```

---

## 2. Connect your AI agent (MCP)

Replace the path with where your `agentsync-cli.exe` lives (omit the path if it is on `PATH`).

### Claude Code

```powershell
claude mcp add --scope user agentsync -- C:\Tools\agentsync-cli.exe mcp
```

Use `--scope project` to store it in the project's `.mcp.json` instead. Check with `claude mcp list`.

### Codex (`~/.codex/config.toml`)

```toml
[mcp_servers.agentsync]
command = 'C:\Tools\agentsync-cli.exe'
args = ["mcp"]
```

### Gemini CLI (`~/.gemini/settings.json`)

```json
{
  "mcpServers": {
    "agentsync": {
      "command": "C:\\Tools\\agentsync-cli.exe",
      "args": ["mcp"]
    }
  }
}
```

### Any other MCP client

Command `agentsync-cli`, arguments `["mcp"]`, transport **stdio**. Optional environment variables:
`AGENTSYNC_CONFIG_DIR` (use another config folder, e.g. a sandbox) and `AGENTSYNC_HOME_OVERRIDE`
(what the builtin `home` location points to).

### What to ask

- "Sync `AGENTS.md` from my AgentSync repo to Claude as `CLAUDE.md` and to Gemini as `GEMINI.md`."
- "Add the `skills/pdf` skill to Claude Code and Codex, then sync."
- "Subscribe to https://github.com/acme/agent-skills and sync its `review` skill to Claude."
- "What does AgentSync have pending? Show me the diff of my changed files."

The server sends the model a short guide (status → locations → detect/suggest → add rule → sync).
Destructive tools need `confirm: true`, which the model is told to request only after asking you.

---

## 3. MCP tools

| Tool | What it does | Notes |
|---|---|---|
| `status` | Root, git state, pending work, rule counts, unmapped aliases | read-only, no network |
| `set_root` | Set the repository root folder | |
| `plan_sync` | Fetch and preview a full sync; returns a `fingerprint` | read-only, network |
| `sync` | Pull, apply rules, commit, push (pass the plan `fingerprint`; `resolutions` for conflicts) | network |
| `sync_rules` | Apply pending rule changes only (no git) | writes files, with backups |
| `list_rules` / `get_rule` | Rules with destination states and per-file status | read-only |
| `add_rule` | New rule; destinations without `path` get AgentSync's suggestion | |
| `set_rule_enabled` | Enable/disable a rule | |
| `add_destination` | Add a destination to a rule (suggested path when omitted) | |
| `remove_destination` | Remove a destination | **`confirm: true`** |
| `delete_rule` | Delete a rule (written files stay) | **`confirm: true`** |
| `rule_diff` / `resolve_rule_file` | Inspect and resolve a conflicting/drifted file | |
| `list_locations` / `set_location` / `remove_location` | Location aliases on this PC | |
| `detect_source` / `suggest_placement` / `list_repo_tree` | What a source is and where it should go | read-only |
| `list_subscriptions` / `add_subscription` / `check_subscription` / `update_subscription` / `set_subscription_auto_update` / `list_subscription_tree` | Shared repositories | |
| `remove_subscription` | Remove a subscription and its rules | **`confirm: true`** |
| `connect_remote` | Attach the repository to a remote without losing local files | network |
| `list_repo_changes` / `repo_file_diff` | Repository changes and per-file diffs | read-only |
| `discard_repo_file` | Revert a local repository change (backup first) | **`confirm: true`** |

Input and output JSON schemas are generated from the Go types and published through `tools/list`.
Errors are returned as tool errors whose text starts with a stable code, e.g.
`[location_unmapped] {"alias":"codex"}` or `[confirmation_required] {...}`.

---

## 4. CLI reference

Run `agentsync-cli help` (or `agentsync-cli help rules`) for the full list. Global flags, accepted anywhere:

- `--json` — print `{"ok":true,"data":…}` or `{"ok":false,"error":{"code","params","message"}}`.
- `--lang en|es` — output language (default: `AGENTSYNC_LANG`, then the app setting, then the system locale).

Exit codes: `0` success, `1` operation failed, `2` invalid command line.

| Command | Purpose |
|---|---|
| `status` | Overview (no network) |
| `plan` | Fetch and preview a full sync |
| `sync [--rules-only] [--fingerprint F] [--resolve PROFILE:RELPATH=keep_repo\|keep_local]…` | Run a sync |
| `root set <folder>` | Set the repository root |
| `rules list` · `rules show <id>` | Inspect rules |
| `rules add --source PATH --to LOCATION[:PATH]… [--origin …] [--subscription ID] [--name N] [--scope repo\|local] [--kind K] [--mode M] [--transform TYPE:KEY] [--exclude GLOB]… [--disabled]` | Create a rule |
| `rules add --input FILE\|-` | Create a rule from JSON (same shape as the MCP `add_rule` input) |
| `rules enable <id>` · `rules disable <id>` · `rules remove <id> --yes` | Change or delete a rule |
| `rules diff <rule> <dest> [relPath]` · `rules resolve <rule> <dest> <relPath> --choice C [--content-file F]` | Conflicts |
| `destinations add <rule> --location L [--path P] [--mode M] [--transform TYPE:KEY] [--ignore GLOB]… [--disabled]` | Add a destination |
| `destinations remove <rule> <dest> --yes` | Remove a destination |
| `locations list` · `locations set <alias> <folder>` · `locations remove <alias>` | Location aliases |
| `detect <path> [--origin O] [--subscription ID]` · `suggest <path> --location L` | Detection and placement |
| `subscriptions list\|add <url> [--name N]\|check <id>\|update <id>\|auto-update <id> on\|off\|tree <id>\|remove <id> --yes` | Subscriptions |
| `repo tree` · `repo changes [--fetch]` · `repo diff <path> [--mode working\|incoming]` · `repo discard <path> --yes` · `repo connect <url>` | Repository |
| `mcp` | Run the MCP server on stdio |

Example:

```powershell
agentsync-cli locations set claude $env:USERPROFILE\.claude
agentsync-cli rules add --source skills/review --to claude --to codex --json
agentsync-cli sync --rules-only
```

`--to claude` without a path uses the suggested placement (`skills/review` for a skill in a Claude
folder, a `json_merge` into the agent config for an MCP file, …). Flags may come before or after
positional arguments.

---

## 5. Running next to the GUI

The GUI, any number of CLI invocations and MCP servers can run at the same time:

- Every service operation holds an exclusive **cross-process file lock** (`<configDir>/agentsync.lock`,
  `LockFileEx` on Windows, `flock` elsewhere) in addition to the in-process mutex, so writes to
  `config.json`, `state.json`, `rules.local.json` and `agentsync.rules.json` never interleave.
- On acquiring the lock a process compares those files with what it last wrote; if another process
  changed them it reloads `config.json`/`state.json` before doing anything, so it never overwrites
  newer data with a stale in-memory copy.
- The open GUI polls the same files every 2 s (`Service.WatchExternalChanges`) and refreshes its views
  (`rules:changed`, `status:changed`) when the CLI or MCP server changed something.
- Long operations (a sync with network access) make other processes wait until they finish.

---

## 6. Code map

| Package | Role |
|---|---|
| `cmd/agentsync-cli` | `main` — calls `cli.Main()` |
| `internal/cli` | Command parsing, human (en/es) and `--json` output |
| `internal/mcpserver` | MCP tools over stdio (official `github.com/modelcontextprotocol/go-sdk`) |
| `internal/ops` | Headless facade over `internal/service`: composite agent operations (suggested placement for omitted paths, atomic rule edits) |
| `internal/filelock` | Cross-process exclusive file lock |
| `internal/service/shared_lock.go` | Service lock = mutex + file lock + reload of external changes |

Tests use temporary folders through `AGENTSYNC_CONFIG_DIR` and `AGENTSYNC_HOME_OVERRIDE` and never
touch the real configuration: `go test ./internal/cli/ ./internal/mcpserver/ ./internal/filelock/ ./internal/service/`.
