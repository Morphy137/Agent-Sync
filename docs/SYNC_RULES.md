# Sync rules

A **rule** takes one **source** (a file or a folder) and writes it to any number of
**destinations**. Each destination chooses its own folder, final name, mode and optional transform.

Typical rules:

| Source | Destinations |
|---|---|
| `shared/AGENTS.md` | `claude` → `CLAUDE.md`, `gemini` → `GEMINI.md`, `codex` → `AGENTS.md` |
| `skills/` (folder) | `claude` → `skills/`, `home` → `.agents/skills/` |
| `mcp.json` | `home` → `.claude.json` (json_merge `mcpServers`), `codex` → `config.toml` (toml_merge `mcp_servers`) |
| `docs/README.md` | `web-project` → `README.md`, `api-project` → `README.md` |
| `notes/agents.md` | `obsidian` → `AI/Agents.md` |

Nothing is tied to AI agents: any file or folder can go anywhere. Agent knowledge only exists in
the [recognizers](RECOGNIZERS.md), which *suggest* paths when they recognize content or folders.

---

## Locations (per PC)

Destinations never contain absolute paths. They reference a **location alias** that each PC maps
to its own folder in the **Locations** view (stored in `config.json`):

```json
"locations": [
  { "alias": "claude",   "path": "C:\\Users\\me\\.claude" },
  { "alias": "obsidian", "path": "D:\\Vault" }
]
```

- Alias format: lowercase letters, digits, `-`, `_` (max 40 chars).
- `home` is builtin and always points to the user folder.
- A rule using an alias that is not mapped on this PC is **skipped** (state `location_unmapped`),
  never an error. Map it in Locations and it starts syncing.

---

## Scopes: shared vs. this PC

| Scope | File | Use it for |
|---|---|---|
| `repo` (shared) | `<repo>/agentsync.rules.json` — committed and synced like any other repo file | Rules you want on every PC |
| `local` (this PC only) | `<configDir>/rules.local.json` | Local sources (absolute paths), subscription rules, machine-specific tweaks |

Both files share the same format:

```json
{
  "version": 1,
  "rules": [
    {
      "id": "r_3f9a1c2b7d",
      "name": "Agent instructions",
      "enabled": true,
      "source": { "origin": "repo", "path": "shared/AGENTS.md" },
      "kind": "auto",
      "destinations": [
        { "id": "d_1a2b3c4d5e", "location": "claude", "path": "CLAUDE.md", "mode": "two_way", "enabled": true },
        { "id": "d_6f7a8b9c0d", "location": "gemini", "path": "GEMINI.md", "mode": "two_way", "enabled": true },
        {
          "id": "d_aa11bb22cc",
          "location": "home",
          "path": ".claude.json",
          "mode": "one_way",
          "transform": { "type": "json_merge", "key": "mcpServers" },
          "enabled": true
        }
      ],
      "exclude": ["*.log", "node_modules"]
    }
  ]
}
```

| Field | Values |
|---|---|
| `source.origin` | `repo` (path relative to the repo root), `local` (absolute path, local scope only), `subscription` (path relative to the subscription clone; `source.subscription` = its id) |
| `kind` | `auto` (detected) or forced: `file`, `folder`, `skill`, `mcp`, `instructions` |
| `destinations[].path` | Relative to the location. For a file source it is the **final file name** (e.g. `CLAUDE.md`); for a folder source it is the destination folder |
| `destinations[].mode` | `two_way` (default), `one_way`, `create_only` |
| `destinations[].transform` | Optional `{ "type": "json_merge" \| "toml_merge", "key": "…" }`; forces one-way |
| `destinations[].ignore` | Relative files you chose to keep local ("stop syncing this file") |
| `exclude` | Glob patterns for folder sources, matched against the relative path, the base name and each folder name |

You normally edit rules from the app; the files are plain JSON (2-space indent) so they diff well in git.

---

## Modes

| Mode | Source → destination | Destination edits |
|---|---|---|
| `two_way` | Written when the source changes | Captured back into the source (repo/local sources) and propagated to the other destinations |
| `one_way` | Written when the source changes | Reported as **drift**; you decide (overwrite, keep local, edit) |
| `create_only` | Written only if the destination file is missing | Never touched afterwards |

Subscription sources are read-only, so their destinations behave as one-way.

## Transforms

Transforms merge **only the entries AgentSync owns** into an existing config file:

- `json_merge` — e.g. `~/.claude.json` / `~/.gemini/settings.json`, key `mcpServers`. Every other
  key and every entry you added yourself is preserved.
- `toml_merge` — e.g. `~/.codex/config.toml`, key `mcp_servers`. Entries are written as
  `[mcp_servers.<name>]` tables inside a delimited block:
  ```toml
  # >>> agentsync managed: mcp_servers >>>
  [mcp_servers.github]
  command = "npx"
  args = ["-y", "@modelcontextprotocol/server-github"]
  # <<< agentsync managed: mcp_servers <<<
  ```
  Everything outside the block is kept byte for byte. Comments inside the block are regenerated.

The source must be JSON with the key at the top level, e.g. `{ "mcpServers": { "github": { … } } }`.
Entries removed from the source are removed from the destination; an entry that already exists in
the destination but was not created by AgentSync is a **conflict** (never overwritten silently).
Drift detection only looks at the managed entries, so the rest of the file can change freely.

## Collisions (always warned, never resolved by priority)

| Type | Example |
|---|---|
| `same_target` | Two rules write `claude/CLAUDE.md` |
| `case_conflict` | `CLAUDE.md` vs `claude.md` (same file on Windows/macOS) |
| `folder_overlap` | A folder destination contains another destination, or a legacy folder connection target |
| `cycle` | A destination is (inside) a rule source |
| `inside_repo` | A destination is inside the repository |

Saving a colliding rule succeeds but the app warns immediately, and **every involved destination
is blocked** until you change a path, disable a destination or exclude it. For per-PC/per-agent
overrides (e.g. a special `CLAUDE.md`), create the override file, point a rule at that destination
and remove/disable that destination in the generic rule.

## Resolving conflicts and drift

Each file needing attention has a **Review** button that opens the diff view
(left = source or rendered output, right = destination). Choices:

| Choice | Effect |
|---|---|
| Use source | Overwrite the destination (backup first) |
| Use this version | Copy the destination into the source (writable sources without transform only) |
| Edit merged version | Write your edited content to both sides (writable sources without transform only) |
| Keep local, stop syncing this file | Adds the file to the destination's `ignore` list |

## Folder sources

- Dot-folders such as `.claude/` or `.agents/` are included. Only `.git`, OS junk files
  (`.DS_Store`, `Thumbs.db`, `desktop.ini`) and temporary `*.agentsync.tmp` files are skipped.
- Secret-looking files are always excluded (see [ARCHITECTURE.md](ARCHITECTURE.md#4-safety-guarantees)).
- Symlinks to folders and symlinks pointing outside the source are not followed.
- Files deleted from the source are deleted from destinations only if the destination copy is
  unchanged since the last sync (backup first); otherwise you get a conflict.

## Importing existing files

Picking a file from "This PC" in the rule editor creates a `local` source. Use **Import into
repository** to copy it into the repo instead, so it is versioned and available on every PC.
