# Remotes, subscriptions and reviewing changes

## 1. Your repository and its remote

The repository root (Settings → Agent Configurations Root) is normally a git repo with a remote
(GitHub, GitLab, any git server). The status bar shows the branch and `↑ahead ↓behind`.

- **Download changes**: the periodic check (or the refresh button) fetches the remote. When the remote
  is ahead, a banner appears; **Review changes** lists the incoming files with diffs, and **Sync** pulls
  them and distributes them through your rules.
- **Upload changes**: edits inside managed paths (repo-origin rule sources, `agentsync.rules.json`)
  and edits captured from two-way destinations are committed (`chore(sync): …`) and pushed by **Sync**.
- **Sync preview** (enabled with "Confirm before syncing") lists conflicts (a file changed both locally
  and on the remote), where you pick the repository or this PC version for each.

### Connecting a folder that is not connected yet

**Connect to remote repository** works whether the folder is not a git repo, is a repo without a
remote, or has its own unrelated history. It never deletes local files:

1. Initializes git if needed (`main` branch), sets `origin`, fetches.
2. **Remote is empty** → commits the local files and pushes (`git push -u`). If no git identity is
   configured the app asks for one; retry afterwards.
3. **Remote has content** →
   - local commits not present in the remote are kept in a branch named
     `agentsync/local-backup-<yyyymmdd-hhmmss>`;
   - the branch is pointed at the remote (`reset --mixed`, working files untouched);
   - files that exist only in the remote are restored locally;
   - every remaining local difference stays as an **uncommitted change** to review.

### Reviewing local changes

The changes panel lists each local change with a diff (left = repository, right = this PC):

| Choice | Effect |
|---|---|
| Keep mine | Nothing now; it will be committed and pushed on the next Sync |
| Take repository version | Backup of your file, then it is reverted to the committed version (or removed if it does not exist in the repository) |

## 2. Subscriptions (repositories shared by others)

Someone publishes a repo (e.g. `skills/`, `agents/`, `AGENTS.md`) and shares the URL. In
**Subscriptions → Add subscription** you paste the URL; AgentSync makes a local clone in
`<configDir>/subscriptions/<id>/repo`.

1. **Browse & pick**: the tree shows the repo with detected kinds (skill, MCP, instructions…). Pick
   only what you want (one skill, the whole `skills/` folder, a single file) and choose destinations —
   this creates a **local-scope rule** with `source.origin = "subscription"`.
2. **Check for updates**: fetches and lists incoming files; each has a diff (local clone vs remote).
3. **Update now**: moves the clone to the latest remote commit and applies the subscription's rules
   (backups first; conflicts and local edits are skipped and shown in the Rules view).
4. **Auto-update**: when enabled, the periodic check updates and applies automatically.

The Files view can edit text files in the local clone. These edits are not sent to the publisher.
Updates (including automatic updates) are blocked while the clone has local changes, including
untracked or ignored files. Save those changes elsewhere and clean the clone before updating.

Your local edits to subscribed files are never overwritten silently: they show as **drift** and you
choose to keep yours (stop syncing that file) or take the publisher's version. Sync rules never
write destination edits back into subscription sources.

Removing a subscription deletes the clone and its rules; files already copied to your folders stay.
Removal is blocked when the clone contains local files, commits or stashes that could be lost.

### Suggested rules

A repository can list rules for its subscribers in a file at its root, `agentsync.suggestions.json`.
Right after you add a subscription that has suggestions, a dialog lists them; a small counter on the
subscription card reopens it while some are not applied. The file never appears in the subscription's
file list, like `agentsync.rules.json`.

```json
{ "version": 1, "suggestions": [
  { "name": "Claude instructions", "description": "optional",
    "source": "AGENTS/Claude/CLAUDE-AGENTS.md", "include": [], "kind": "auto",
    "destinations": [ { "location": "claude", "path": "CLAUDE.md", "mode": "one_way" } ] }
] }
```

| Field | Meaning |
|---|---|
| `source` | Path inside the repository (a file or folder). `.` is allowed only together with `include` |
| `include` | Optional list of files of the source folder to sync (same meaning as in a rule) |
| `kind` | `auto` (default), `file`, `folder`, `skill`, `mcp` or `instructions` |
| `destinations[].location` | Location alias (`claude`, `codex`, `home`…), lowercase letters, numbers, `-` or `_` |
| `destinations[].path` | Relative path inside that location, including the final name |
| `destinations[].mode` | `one_way` (default: the repository wins) or `create_only` (written once, never updated) |

For each suggestion the dialog shows the source and, for every destination, the location, the
relative path and the absolute path it resolves to on this PC, whether a file is already there, and
whether the suggestion is already applied (a rule of the same subscription with the same source,
location and path exists). Valid, not yet applied suggestions are ticked; invalid or applied ones
cannot be ticked. A location that is not mapped on this PC needs a folder: if a known agent folder is
found (for example `~/.claude` for `claude`) it is offered, and you confirm or change it; the folder
is saved as that location like any other. **Apply** creates **local** rules (origin `subscription`)
through the same validation and collision checks as any rule. It only creates rules: files are
copied when you sync, so the pending state and the Sync button appear as for any new rule. Applying
again skips what exists, and a suggestion where only some destinations exist adds a rule for the
missing ones.

Authors can generate the file from rules they set up by hand on a subscription with
`agentsync-cli suggestions export --subscription ID --output agentsync.suggestions.json` (the MCP
tool `export_subscription_suggestions` returns the same JSON). Two-way destinations are exported as
one-way and destinations with a transform are left out, because the format has neither.

**Security.** The repository is untrusted, so the file is treated as data and nothing is applied
automatically:

- The file is read only if it is a regular file of at most 256 KiB, with at most 100 suggestions and
  20 destinations each. A malformed file gives the `invalid_suggestions` error; a bad entry is shown
  as not valid while the others stay usable.
- `source` and `include` must be relative, free of `..`, drive letters (`C:`), a leading `/` or `\`
  and `.git` segments, and the source must exist inside the clone (a symlink leading out of the clone
  is refused).
- `destinations[].path` follows the same rules, so it cannot leave its location folder. The `mode`
  is limited to `one_way` and `create_only`: `two_way` is refused because subscription sources are
  read-only. Transforms, scripts and absolute paths do not exist in the format.
- Suggestions are identified by their content, so a selection made before the file changed cannot
  apply a different suggestion.
- The dialog shows the exact resolved target before you accept; `one_way` replaces an existing file
  on sync (a backup is taken first) and `create_only` keeps it.

### Accepted URLs

`https://…`, `http://…`, `ssh://…`, `git@host:owner/repo.git`, `file:///…` and absolute local paths.
Rejected: `ext::`/transport helpers, `git://`, URLs starting with `-`, control characters, URLs over
2048 characters. Git runs non-interactively (`GIT_TERMINAL_PROMPT=0`), so private repos need
credentials already configured (credential manager or SSH agent).

## 3. Diff view

All diffs use the same view: unified lines with numbers for both sides, added/removed counts and
translated side labels. Binary files or files over 1 MiB show a notice instead. When a merged edit is
allowed, **Edit merged version** opens an editor prefilled with the left side.
