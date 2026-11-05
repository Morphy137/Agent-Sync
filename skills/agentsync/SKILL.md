---
name: agentsync
description: Set up and run AgentSync syncs for the user — copy files, folders, skills, MCP configs or instruction files (AGENTS.md, CLAUDE.md, GEMINI.md) to several places and computers, subscribe to shared repositories, and resolve sync conflicts. Use when the user asks to sync, share, copy or keep in sync agent configuration or any files between folders or PCs, or mentions AgentSync.
---

# AgentSync

AgentSync keeps files in sync from one source (a git repository, a folder on this PC or a
subscribed repository) to many destinations, each with its own name. Work through its **MCP
server** (tools such as `status`, `add_rule`, `sync`) when it is connected; otherwise use the
**CLI** `agentsync-cli` with `--json`. Both use the same configuration as the desktop app.

## Concepts

- **Rule**: one source → one or more destinations. Each destination has a location, a path or
  final name (`CLAUDE.md`, `GEMINI.md`, `skills/review`) and a mode:
  `two_way` (default, edits flow both ways), `one_way` (source wins) or `create_only`.
- **Source origin**: `repo` (path relative to the repository root, shared with every PC),
  `local` (absolute path on this PC, rule stored locally) or `subscription` (local clone of a shared repo; edits are never pushed).
- **Selection**: a folder source plus `include` (list of files relative to it) syncs only those
  files. Use `sourcePath: "."` for files at the repository root.
- **Location**: alias for a folder on this PC (`claude` → `C:\Users\me\.claude`). Rules use aliases so
  they work on every computer. `home` initially points to the user folder and can be removed or remapped.
- **Transform**: `json_merge:<key>` / `toml_merge:<key>` merges only AgentSync's MCP entries into an
  agent config file and keeps the rest.

Rule reports may include a `categoryId`; categories organize presentation and do not change sync behavior.
Older AgentSync clients can remove category fields when rewriting rules files, so use a current client
when modifying categorized rules.

## Workflow

1. **Check state** — `status` (CLI: `agentsync-cli status --json`). If no root is set, ask the user
   for their configuration repository folder and call `set_root`.
2. **Locations** — `list_locations`. If a needed alias is missing, ask for the folder (or propose the
   usual one, e.g. `%USERPROFILE%\.claude`) and call `set_location`.
3. **Understand the source** — `detect_source` and, per destination, `suggest_placement`. AgentSync
   recognizes skills, MCP configs and instruction files and proposes the right path and name.
   Omit a destination `path` to accept the suggestion.
4. **Create the rule** — `add_rule` with the source and destinations. Report any `collisions` in the
   result: colliding destinations are paused until the user changes a path or disables one.
5. **Apply** — `sync_rules` writes rule destinations only (backups first). For a full sync with git
   (pull, commit, push) call `plan_sync`, show the user the plan, then `sync` with its `fingerprint`.
   If the plan lists `conflicts` (a repository file edited here and on the remote), ask which version
   to keep and pass `resolutions` `{path, resolution: keep_repo | keep_local}` (CLI:
   `--resolve PATH=keep_repo|keep_local`); the discarded or set-aside local version is backed up.
6. **Review** — `list_rules` / `get_rule` show per-file status; use `rule_diff` and
   `resolve_rule_file` for conflicts after asking the user which version to keep.

## Examples

| User says | Do |
|---|---|
| "Use my AGENTS.md as CLAUDE.md and GEMINI.md" | `add_rule` source `AGENTS.md`, destinations `claude:CLAUDE.md`, `gemini:GEMINI.md`; then `sync_rules` |
| "Share the review skill with Claude and Codex" | `add_rule` source `skills/review`, destinations `claude`, `codex` (no path → suggested); `sync_rules` |
| "Copy only these 3 notes to my Obsidian vault" | `set_location obsidian <vault>`; `add_rule` source `notes` + `include: [a.md, b.md, c.md]`, origin as needed |
| "Subscribe to github.com/acme/skills and take its pdf skill" | `add_subscription`; `list_subscription_tree`; `add_rule` origin `subscription`, source `skills/pdf` |
| "What does this subscription suggest?" | `list_subscription_suggestions`; show each source and target path; after the user accepts, `apply_subscription_suggestions` (`confirm: true`, `locations` for unmapped aliases), then `sync_rules` |
| "Publish my subscription rules as suggestions" | `export_subscription_suggestions` (CLI: `suggestions export --subscription ID --output agentsync.suggestions.json`) |
| "What is pending?" | `status`, then `list_repo_changes` / `repo_file_diff` |

CLI equivalents:

```bash
agentsync-cli locations set claude "$USERPROFILE/.claude" --json
agentsync-cli rules add --source skills/review --to claude --to codex --json
agentsync-cli rules add --source notes --include a.md --include b.md --to obsidian:inbox --json
agentsync-cli sync --rules-only --json
agentsync-cli suggestions list --subscription s_abc --json
agentsync-cli suggestions apply --subscription s_abc --id 3f9a1c2b7d10 --map codex="$USERPROFILE/.codex" --json
```

## Backups

Every overwrite or delete is backed up first. To recover a file, `list_backups` (CLI `backups list`)
shows them newest first with the rule, destination, date, reason and files; `restore_backup` (CLI
`backups restore <id> [--file F]... --yes`) puts all or some files back after the user agrees. The
current files are backed up first (`undoBackupId`), so a restore can itself be undone. A restored
destination file differs from its source, so it appears in `status` / `list_rules` as a difference to
review (`untracked_diff`): `sync_rules` skips it until the user resolves it with `resolve_rule_file`.

## Safety

- Subscription updates are blocked by local changes; removal is blocked by local files, commits or
  stashes. Preserve those changes outside the clone before cleaning it.

- Suggested rules come from the repository owner (a third party). Never apply them unasked: list
  them, show the user each source and the resolved target path, apply only the ones they accept, and
  confirm folders for unmapped locations with them. Applying creates rules only; a sync copies files.
- Destructive tools (`delete_rule`, `remove_destination`, `remove_subscription`,
  `discard_repo_file`, `save_gitignore`, `restore_backup`) require `confirm: true` (CLI: `--yes`). Ask the user first,
  in plain words, and only then confirm.
- Never put secrets in a synced source. Secret files (`.env`, `*.pem`, `auth.json`, ...) are always
  excluded; use `exclude_repo_path` to keep other files out of the repository.
- Prefer `sync_rules` for local changes; only run a full `sync` (which pushes) when the user asked to
  share or upload.
- Errors start with a stable code, e.g. `[location_unmapped] {"alias":"codex"}`: fix the cause
  (here, `set_location codex …`) instead of retrying.

## Setup (if the tools are missing)

Download `agentsync-cli.exe` from the AgentSync releases and register it as an MCP server:

```bash
claude mcp add --scope user agentsync -- C:\Tools\agentsync-cli.exe mcp
```

Codex (`~/.codex/config.toml`): `[mcp_servers.agentsync]` with `command` and `args = ["mcp"]`.
Gemini CLI (`~/.gemini/settings.json`): `mcpServers.agentsync` with the same command and args.

Rules may share a destination folder when they manage different files. MCP merge rules may share a configuration file when their managed entry names are distinct; overlapping entries remain blocked.
