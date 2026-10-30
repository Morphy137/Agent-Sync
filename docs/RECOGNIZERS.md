# Recognizers (`recognizers.json`)

Recognizers let AgentSync **suggest** where content should go without hardcoding any agent in
the code. They answer two questions:

1. **What is the source?** — a skill folder, an MCP config, an instructions file, or a generic file/folder.
2. **What is the destination folder?** — e.g. "this looks like a Claude Code config folder".

With both answers the rule editor pre-fills the destination path, mode and transform, and shows a
**Detected: <label>** chip. You can always change the suggestion; nothing is applied until you save
the rule and review the preview.

## Where the data lives

| File | Purpose |
|---|---|
| `internal/recognize/defaults/recognizers.json` | Defaults embedded in the binary |
| `<configDir>/recognizers.json` | Your overrides (Locations view → "Open recognizers file" creates it from the defaults) |

`<configDir>` is `%APPDATA%\AgentSync` on Windows (or `AGENTSYNC_CONFIG_DIR`).

Merge rules (user file over defaults):

- Entries with the **same `id`** replace the default entry.
- **New ids** are appended.
- `{ "id": "<id>", "disabled": true }` removes a default entry.
- The user file must have `"version": 1`; unknown fields are rejected. An invalid file shows the
  error `recognizers_invalid` and the defaults are used.

> Note: a user file created from the template contains a copy of every default. Entries you keep
> unchanged in your file will hide future improvements of the same id in new app versions —
> delete entries you did not customize.

## Schema

```json
{
  "version": 1,
  "sourceKinds": [
    { "id": "skill", "kind": "skill",
      "match": { "type": "dir", "containsFile": "SKILL.md", "frontmatterKeys": ["name", "description"] } },
    { "id": "mcp-json", "kind": "mcp",
      "match": { "type": "file", "extensions": [".json"], "jsonKey": "mcpServers" } },
    { "id": "instructions", "kind": "instructions",
      "match": { "type": "file", "fileNames": ["AGENTS.md", "CLAUDE.md", "GEMINI.md"] } }
  ],
  "profiles": [
    {
      "id": "claude-code",
      "label": "Claude Code",
      "detect": { "folderNames": [".claude"], "anyOf": [{ "file": "CLAUDE.md" }, { "file": "settings.json" }, { "dir": "skills" }] },
      "placements": {
        "instructions": { "path": "CLAUDE.md" },
        "skill": { "path": "skills/{name}" },
        "mcp": { "base": "home", "path": ".claude.json", "transform": { "type": "json_merge", "key": "mcpServers" } }
      }
    }
  ]
}
```

### `sourceKinds[].match`

All present conditions must hold.

| Field | Applies to | Meaning |
|---|---|---|
| `type` | both | `"dir"` or `"file"` |
| `containsFile` | dir | File that must exist directly inside the folder |
| `frontmatterKeys` | dir | Keys that must exist in the YAML frontmatter of `containsFile` (values may be empty) |
| `extensions` | file | Allowed extensions (case-insensitive) |
| `fileNames` | file | Allowed file names (case-insensitive) |
| `jsonKey` | file | Top-level key that must exist in the JSON |

Detected kinds: `skill`, `mcp`, `instructions`; anything else is `file` or `folder`.

### `profiles[]`

| Field | Meaning |
|---|---|
| `detect.folderNames` | Folder base names that identify the profile (case-insensitive) — score +2 |
| `detect.anyOf` | `{ "file": "…" }` / `{ "dir": "…" }` that may exist directly inside the folder — score +1 each |
| `placements.<kind>` | Where a source of that kind goes: `path` relative to the detected folder, or to the user home when `"base": "home"` |
| `placements.<kind>.transform` | Optional transform (forces one-way) |

The highest score wins; ties go to the first profile in the list. `{name}` in a path is replaced by
the source name (file name without extension, folder name, or the skill's frontmatter `name` when it
is a single safe path segment). Placement paths must be relative, use `/`, and may not contain `..`.

If no profile matches, or the matched profile has no placement for that kind, the suggestion is a
**generic copy**: same name as the source, two-way, no transform.

## Defaults (verified against official docs, October 2026)

| Profile | Detected folder | Instructions | Skills | MCP |
|---|---|---|---|---|
| Claude Code | `.claude` (+ `CLAUDE.md`, `settings.json`, `skills/`) | `CLAUDE.md` | `skills/{name}` | `~/.claude.json` · `json_merge` `mcpServers` |
| Codex CLI | `.codex` (+ `config.toml`, `AGENTS.md`) | `AGENTS.md` | `~/.agents/skills/{name}` | `config.toml` · `toml_merge` `mcp_servers` |
| Gemini CLI | `.gemini` (+ `GEMINI.md`, `settings.json`) | `GEMINI.md` | `skills/{name}` | `settings.json` · `json_merge` `mcpServers` |
| OpenCode | `opencode` (+ `opencode.json[c]`) | `AGENTS.md` | `skills/{name}` | — (different schema: `mcp`, command arrays) |
| Antigravity | `antigravity`, `antigravity-cli` (+ `mcp_config.json`) | `~/.gemini/AGENTS.md` | `skills/{name}` | — (remote servers need `serverUrl`) |
| Shared agent skills | `.agents` | — | `skills/{name}` | — |

Sources: code.claude.com docs (MCP, skills, memory), github.com/openai/codex (config, skills
roots, instructions), github.com/google-gemini/gemini-cli docs (configuration, MCP, skills,
GEMINI.md), opencode.ai/docs (config, MCP, rules, skills), antigravity.google/docs (MCP, rules,
skills).

### Known caveats

- Gemini CLI treats a bare `url` as SSE (`httpUrl` is streamable HTTP); stdio entries are fully
  compatible. A Claude-style `{ "type": "http", "url": … }` entry may connect as SSE.
- Codex expects `http_headers`; a Claude `headers` field is copied as-is and ignored by Codex.
- `~/.gemini` is shared by Gemini CLI and Antigravity; a folder named `.gemini` is detected as Gemini CLI.
- Antigravity's desktop/IDE config folder is `~/.gemini/config`; it is detected only through
  `mcp_config.json` because `config` is too generic a name.

## Adding your own tool

Add a profile to your user file, for example for a tool that reads `~/.mytool/RULES.md` and
`~/.mytool/skills/<name>`:

```json
{
  "version": 1,
  "sourceKinds": [],
  "profiles": [
    {
      "id": "mytool",
      "label": "My Tool",
      "detect": { "folderNames": [".mytool"], "anyOf": [{ "file": "RULES.md" }] },
      "placements": {
        "instructions": { "path": "RULES.md" },
        "skill": { "path": "skills/{name}" }
      }
    }
  ]
}
```

Restart is not required: detection reads the file every time a suggestion is requested.
