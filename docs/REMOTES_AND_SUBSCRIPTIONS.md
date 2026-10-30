# Remotes, subscriptions and reviewing changes

## 1. Your repository and its remote

The repository root (Settings → Agent Configurations Root) is normally a git repo with a remote
(GitHub, GitLab, any git server). The status bar shows the branch and `↑ahead ↓behind`.

- **Download changes**: the periodic check (or the refresh button) fetches the remote. When the remote
  is ahead, a banner appears; **Review changes** lists the incoming files with diffs, and **Sync** pulls
  them and distributes them through your connections and rules.
- **Upload changes**: edits inside managed paths (rule sources, profile folders, `agentsync.rules.json`)
  and edits captured from two-way destinations are committed (`chore(sync): …`) and pushed by **Sync**.
- **Sync preview** (enabled with "Confirm before syncing") lists downloads, uploads and conflicts, each
  with a diff.

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
**Subscriptions → Add subscription** you paste the URL; AgentSync makes a **read-only clone** in
`<configDir>/subscriptions/<id>/repo`.

1. **Browse & pick**: the tree shows the repo with detected kinds (skill, MCP, instructions…). Pick
   only what you want (one skill, the whole `skills/` folder, a single file) and choose destinations —
   this creates a **local-scope rule** with `source.origin = "subscription"`.
2. **Check for updates**: fetches and lists incoming files; each has a diff (local clone vs remote).
3. **Update now**: moves the clone to the latest remote commit and applies the subscription's rules
   (backups first; conflicts and local edits are skipped and shown in the Rules view).
4. **Auto-update**: when enabled, the periodic check updates and applies automatically.

Your local edits to subscribed files are never overwritten silently: they show as **drift** and you
choose to keep yours (stop syncing that file) or take the publisher's version. Subscription sources
are never written back.

Removing a subscription deletes the clone and its rules; files already copied to your folders stay.

### Accepted URLs

`https://…`, `http://…`, `ssh://…`, `git@host:owner/repo.git`, `file:///…` and absolute local paths.
Rejected: `ext::`/transport helpers, `git://`, URLs starting with `-`, control characters, URLs over
2048 characters. Git runs non-interactively (`GIT_TERMINAL_PROMPT=0`), so private repos need
credentials already configured (credential manager or SSH agent).

## 3. Diff view

All diffs use the same view: unified lines with numbers for both sides, added/removed counts and
translated side labels. Binary files or files over 1 MiB show a notice instead. When a merged edit is
allowed, **Edit merged version** opens an editor prefilled with the left side.
