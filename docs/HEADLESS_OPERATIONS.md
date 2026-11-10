# Editing rules, categories and files without the window

These operations are available through `agentsync-cli` and its MCP server. They reuse the same
service validation, locking and persistence as the desktop app. They do not run a sync or upload
files unless the operation explicitly says so. Desktop dialogs, tray controls, startup integration
and theme remain GUI-only.

For installation, connection, operational settings and repository setup, see [CLI_AND_MCP.md](CLI_AND_MCP.md).

On Windows network shares, use a repository subfolder such as `\\server\share\agent-configs`.
A pre-existing path-helper issue with bare UNC share roots (without a trailing separator) is not
addressed by this change; avoid selecting the share root itself.

## Update rules without recreating them

- CLI: `rules update <ruleId> --input FILE|-`
- MCP: `update_rule` with `{"ruleId":"r_...","patch":{...}}`

The CLI input is the **patch itself**, not the MCP wrapper. Supported fields:

| Field | Meaning |
|---|---|
| `name`, `categoryId`, `enabled` | Rule display name, category and enablement |
| `scope` | `repo` or `local`; move the existing rule to that scope |
| `origin`, `subscription`, `sourcePath` | Source origin, subscription ID and path |
| `include`, `kind`, `exclude` | Selected source files, content kind and excluded globs |

Omitted or `null` fields preserve the stored value. `false` disables; `[]` clears a list.
Source fields are independent: changing origin does not automatically clear the old subscription
ID or translate the source path. Supply all source fields that must change together.
IDs, destinations and sync baselines are retained. An unknown rule ID fails instead of creating a
rule. Moving scopes uses the same persistence behavior as the GUI; it is not a transactional write
across both rules files if the filesystem fails midway.

Example CLI patch file:

```json
{"name":"Review skill","categoryId":"skill","enabled":false,"exclude":[]}
```

Creation also supports `rules add --category ID`, or `categoryId` in JSON/MCP `add_rule`.
JSON inputs reject unknown fields, including nested destination/transform fields, and trailing JSON
values. Existing creation defaults are unchanged; pass `--scope local` / `scope:"local"` explicitly
for rules that should stay on this PC.

## Update one destination

- CLI: `destinations update <ruleId> <destinationId> --input FILE|-`
- MCP: `update_destination` with `{"ruleId":"r_...","destinationId":"d_...","patch":{...}}`

The patch accepts `location`, `path`, `mode`, `enabled`, `ignore`, `transform`, `clearTransform`.
The destination ID stays unchanged. Omitted or `null` fields preserve values; `ignore:[]` clears
ignored patterns. `transform` replaces the whole `{type,key}` object. Use `clearTransform:true` to
remove a transform; combining it with `transform` is invalid. Clearing a transform preserves the
current mode, so it never silently enables two-way capture. To change that mode, supply `mode` too.

These edits change rule configuration, not destination files. Review collisions and run
`sync_rules` / `sync --rules-only` when ready to apply.

## Categories

| CLI | MCP | Result |
|---|---|---|
| `categories list` | `list_categories` | Available built-in, repository and local categories |
| `categories save --input FILE|-` | `save_category` | Create/update a category in an explicit scope |
| `categories remove <id> --scope repo\|local\|builtin --yes` | `delete_category` with `id`, `scope`, `confirm:true` | Remove a category and clear affected assignments |
| `categories migrate` | `migrate_categories` with `confirm:true` | Explicitly assign categories to legacy local rules |
| `categories suggest <path> [--origin O] [--subscription ID] [--kind K]` | `suggest_category` with `source:{path,origin?,subscription?}`, optional `kind` | Suggested category ID; no assignments changed |

Save input for both interfaces:

```json
{"category":{"name":"Work","color":"blue","icon":"folder"},"scope":"local"}
```

Omit category `id` to create; include the existing ID to update. Save scope is `repo` or `local`.
Category save is a full category operation, not a partial patch; name and color must satisfy the
same rules as the GUI. Icons default/preserve according to the service. Use `categoryId` in a rule
patch to assign an existing category. CLI list returns an array; MCP list wraps it in `categories`.
CLI suggest returns a string; MCP suggest wraps it in `categoryId`.

## Read and edit configured files

The browser operates on configured roots, not arbitrary absolute paths:

| CLI | MCP |
|---|---|
| `files roots` | `list_file_roots` |
| `files list --root ID [--path P]` | `list_file_directory` with `rootId`, optional `path` |
| `files read --root ID --path P` | `read_file` with `rootId`, `path` |
| `files save --root ID --path P --content-file FILE\|- --expected-hash H --expected-path ABS` | `save_file` with `rootId`, `path`, `content`, `expectedHash`, `expectedPath` |

Read first. Use the returned `hash` and `absolutePath` as the save's expected hash and path. Both
are mandatory. A changed file, root or resolved path is refused (`plan_changed`); reread and review
instead of blindly retrying. Complete replacement content can be empty. Only supported text files
inside the configured roots can be read/saved; traversal, escaping symlinks and protected paths are
rejected by the service. Subscription browser edits stay local and can block subsequent updates.

MCP lists wrap arrays in `roots` / `entries`; save returns `{hash}`. CLI uses its standard JSON
envelope around the underlying array or returned hash string.

### Selected source files of a rule

| CLI | MCP |
|---|---|
| `rules files <ruleId>` | `list_rule_source_files` with `ruleId` |
| `rules read-file <ruleId> <path>` | `read_rule_source_file` with `ruleId`, `path` |
| `rules save-file <ruleId> <path> --content-file FILE\|- --expected-hash H` | `save_rule_source_file` with `ruleId`, `path`, `content`, `expectedHash` |

Use the paths returned by the listing (`.` for a single-file rule). These operations respect rule
selection and exclusions. Subscription **rule sources remain read-only**; the browser is the explicit
way to edit their local clone. MCP listing wraps files in `files`; save returns `{hash}`.

Direct editor saves use concurrency checks and atomic replacement, **not backup snapshots**.
Sync writes/deletions, discards and backup restores retain their existing backup behavior.

### Import and preview subscription updates

- `repo import <source> [--dir DIR]` / MCP `import_into_repo` with `sourcePath`, optional `repoRelDir`:
  copy a local file/folder into the repository, respecting import filters. Existing targets are not
  overwritten. No rule is created and nothing is pushed. CLI returns the imported relative path;
  MCP wraps it in `path`. Destination links must stay inside the repository; Windows junctions in
  the destination are rejected, including inside-root junctions, rather than followed implicitly.
- `subscriptions diff <id> <path>` / MCP `subscription_file_diff` with `id`, `path`:
  compare a file in the local clone with the **last fetched** remote version. First run
  `subscriptions check <id>` / `check_subscription` for current incoming changes. The diff itself
  performs no fetch or write.
