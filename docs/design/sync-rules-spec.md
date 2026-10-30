# Generic Sync Rules — Implementation Spec (Round 4)

This is the binding contract for the parallel implementation of generic sync rules,
recognizers, transforms, subscriptions and remote connection. Each package owner
implements exactly the API below. Field names, JSON tags and function signatures
are the contract; internal helpers are free.

Conventions (whole repo):

- Go module `agentsync`. Errors returned to the UI are `*apperr.Error`
  (`internal/apperr`). New codes already exist: `rule_not_found`, `invalid_rule`,
  `source_not_found`, `source_read_only`, `location_unmapped`, `location_invalid`,
  `collision_detected`, `transform_failed`, `invalid_remote_url`,
  `subscription_not_found`, `remote_branch_not_found`, `recognizers_invalid`.
- File writes go through `fsops.AtomicWrite` / `fsops.AtomicCopy`. Hashes via
  `fsops.HashFile` (sha256 hex). Ignore list via `fsops.ShouldIgnore`.
- Paths in JSON files use forward slashes; convert with `filepath.FromSlash` at use.
- Tests use `t.TempDir()`, `t.Setenv(config.ConfigEnvVar, ...)`, no network.
  Git tests use local bare repos (`git init --bare`) and `file://` URLs.
- No magic numbers: named constants. Comments in English. `gofmt` clean.
- Never add co-author or AI attribution anywhere.

---

## 1. Concepts

| Concept | Where it lives | Shared across PCs |
|---|---|---|
| **Location** — alias → absolute folder on this PC | `config.json` → `locations` (`config.Location`) | No (each PC maps its own paths) |
| **Rule** — one source → N destinations | repo rules: `<repoRoot>/agentsync.rules.json`; local + subscription rules: `<configDir>/rules.local.json` | Repo rules yes; local rules no |
| **Subscription** — read-only remote repo used as a source | `config.json` → `subscriptions` (`config.Subscription`); clone at `<configDir>/subscriptions/<id>/repo` | No |
| **Recognizers** — data describing known content kinds and known folder profiles | embedded defaults + `<configDir>/recognizers.json` override | No (user file is per PC) |
| **Baselines** — last synced hash per (rule, destination, file) | `state.json` via `state.State` with key `rule:<ruleID>:<destID>` | No |

Built-in location alias: `home` always resolves to the user home directory
(`AGENTSYNC_HOME_OVERRIDE` env first — used by tests/sandbox —, then `os.UserHomeDir()`).
User aliases cannot be named `home`.

---

## 2. Package `internal/rules` (model, storage, resolution, collisions)

```go
package rules

const (
    RulesFileName      = "agentsync.rules.json" // in repo root
    LocalRulesFileName = "rules.local.json"     // in config.GetConfigDir()
    CurrentVersion     = 1
    BuiltinHomeAlias   = "home"
)

// Source origins.
const (
    OriginRepo         = "repo"         // Path relative to repo root
    OriginLocal        = "local"        // Path absolute (only in local rules file)
    OriginSubscription = "subscription" // Path relative to subscription clone root
)

// Destination modes.
const (
    ModeTwoWay     = "two_way"     // default for plain copies; destination edits flow back to source
    ModeOneWay     = "one_way"     // source -> destination only; destination edits reported as drift
    ModeCreateOnly = "create_only" // write only when destination file is missing
)

// Content kinds (detected or forced).
const (
    KindAuto         = "auto"
    KindFile         = "file"
    KindFolder       = "folder"
    KindSkill        = "skill"
    KindMCP          = "mcp"
    KindInstructions = "instructions"
)

type Source struct {
    Origin       string `json:"origin"`
    Subscription string `json:"subscription,omitempty"` // subscription ID when Origin == subscription
    Path         string `json:"path"`
}

type Transform struct {
    Type string `json:"type"`          // "json_merge" | "toml_merge"
    Key  string `json:"key"`           // e.g. "mcpServers" or "mcp_servers"
}

type Destination struct {
    ID        string     `json:"id"`
    Location  string     `json:"location"`           // alias (user alias or "home")
    Path      string     `json:"path"`               // relative to location; file path or folder path
    Mode      string     `json:"mode"`               // ModeTwoWay | ModeOneWay | ModeCreateOnly
    Transform *Transform `json:"transform,omitempty"`// non-nil forces ModeOneWay semantics
    Enabled   bool       `json:"enabled"`
    Ignore    []string   `json:"ignore,omitempty"`   // relative file paths the user chose to keep local (stop syncing)
}

type Rule struct {
    ID           string        `json:"id"`
    Name         string        `json:"name"`
    Enabled      bool          `json:"enabled"`
    Source       Source        `json:"source"`
    Kind         string        `json:"kind"`             // KindAuto unless user forced
    Exclude      []string      `json:"exclude,omitempty"`// glob patterns (path.Match, forward slashes) relative to source folder
    Destinations []Destination `json:"destinations"`
    Scope        string        `json:"scope,omitempty"`  // "repo" | "local"; set on load, stripped before saving
}

type File struct {
    Version int    `json:"version"`
    Rules   []Rule `json:"rules"`
}
```

### Storage

```go
// LoadRepoRules reads <repoRoot>/agentsync.rules.json. Missing file => empty File, nil error.
func LoadRepoRules(repoRoot string) (*File, error)
func SaveRepoRules(repoRoot string, f *File) error
// LoadLocalRules reads <configDir>/rules.local.json (config.GetConfigDir()).
func LoadLocalRules() (*File, error)
func SaveLocalRules(f *File) error
// Validate checks a single rule: non-empty name/ID, known origin/mode/kind/transform type,
// repo/subscription paths relative & without "..", local origin only allowed in local scope,
// destination paths relative and not escaping with ".." beyond the location root,
// transform only allowed on a file source, unique destination IDs. Returns apperr InvalidRule with param "reason".
func Validate(r Rule, scope string) error
func NewID(prefix string) string // e.g. "r_" + 10 random hex chars
```

JSON writes are pretty-printed with 2-space indent and a trailing newline (diff-friendly in git).

### Resolution

```go
// Env carries everything needed to turn a rule into absolute paths on this machine.
type Env struct {
    RepoRoot          string
    Locations         map[string]string // alias -> absolute path (from config.Locations)
    HomeDir           string            // resolved builtin "home"
    SubscriptionRoots map[string]string // subscription ID -> clone root
}

type Pair struct {
    RelPath    string // path relative to the destination root, forward slashes ("" not allowed)
    SourceAbs  string
    TargetAbs  string
}

type ResolvedDestination struct {
    RuleID       string
    DestID       string
    TargetRoot   string // folder that contains every TargetAbs of this destination
    SourceIsDir  bool
    Pairs        []Pair
    Unmapped     bool   // location alias not mapped on this PC -> skipped, not an error
    SourceMissing bool
}

// Resolve expands one destination of one rule. For a file source there is one Pair with
// RelPath = base name of the destination path and TargetRoot = its parent folder.
// For a folder source, every managed file (fsops.WalkManaged semantics + rule Exclude
// + DefaultSecretExcludes) becomes a Pair under TargetAbs = <location>/<dest.Path>/<rel>,
// TargetRoot = <location>/<dest.Path>. Files in dest.Ignore are skipped.
func Resolve(env Env, r Rule, d Destination) (ResolvedDestination, error)

// DefaultSecretExcludes are always excluded from folder sources (safety net).
var DefaultSecretExcludes = []string{".credentials.json", "auth.json", "*.pem", "*.key", ".env", ".env.*", "id_rsa*", "*.p12"}
```

### Collisions

```go
const (
    CollisionSameTarget    = "same_target"    // two destinations write the same file
    CollisionFolderOverlap = "folder_overlap" // a folder destination contains another destination's file/folder, or a legacy connection target folder contains it
    CollisionCycle         = "cycle"          // a destination target is (or is inside) a rule source
    CollisionInsideRepo    = "inside_repo"    // destination inside the repo root
    CollisionCaseConflict  = "case_conflict"  // same path differing only by case (always checked; Windows/macOS are case-insensitive)
)

type DestRef struct {
    RuleID       string `json:"ruleId,omitempty"`
    DestID       string `json:"destId,omitempty"`
    ConnectionID string `json:"connectionId,omitempty"` // legacy folder connection
}

type Collision struct {
    Type string    `json:"type"`
    Path string    `json:"path"`   // the colliding absolute path
    Refs []DestRef `json:"refs"`
}

// DetectCollisions checks all resolved enabled destinations plus legacy connection target folders.
// Every DestRef involved in any collision must be treated as blocked by callers.
func DetectCollisions(env Env, resolved []ResolvedDestination, sources []string /*absolute rule sources*/, legacy []config.Connection) []Collision
```

Collisions are **never** resolved by priority. Callers block every involved destination
until the user changes the rules.

---

## 3. Package `internal/transform`

Pure functions, no filesystem access.

```go
package transform

const (
    TypeJSONMerge = "json_merge"
    TypeTOMLMerge = "toml_merge"
)

type Spec struct {
    Type string
    Key  string
}

type Result struct {
    Output      []byte   // full new destination file content
    Managed     []string // sorted entry names under Key now owned by AgentSync
    ManagedView []byte   // canonical JSON of the managed entries (stable key order) — hashed for drift detection
}

// Render merges the source entries into the existing destination content.
// Source is always JSON whose top-level object contains Key (e.g. {"mcpServers": {...}}).
// existing may be nil/empty (file missing). prevManaged lists entries owned in the previous run:
// entries in prevManaged but no longer in source are removed; entries not owned are preserved untouched.
// If an unowned entry has the same name as a source entry, return apperr TransformFailed with
// param reason "entry_conflict" and param "entry".
// json_merge: preserve all other top-level keys, 2-space indent output, trailing newline.
// toml_merge: write source entries as [Key.<name>] tables. The rest of the TOML document must be
// preserved byte-for-byte except the managed block, which is delimited by
// "# >>> agentsync managed: <Key> >>>" / "# <<< agentsync managed: <Key> <<<" comment lines.
// JSON values map to TOML (strings, numbers, bools, arrays, nested objects as inline tables or
// sub-tables). Validate the final document parses as TOML.
func Render(spec Spec, source []byte, existing []byte, prevManaged []string) (Result, error)

// SourceView returns the canonical JSON of the source entries under Key (used as source hash input).
func SourceView(spec Spec, source []byte) ([]byte, error)

// TargetView returns the canonical JSON of the entries named in managed as currently present in the destination.
// Missing entries are omitted. Used to detect drift without reacting to unrelated edits in the file
// (e.g. ~/.claude.json changes constantly).
func TargetView(spec Spec, existing []byte, managed []string) ([]byte, error)
```

Canonical JSON: object keys sorted recursively, no insignificant whitespace.
TOML library: `github.com/pelletier/go-toml/v2` (parse/validate). Add it with `go get`.

---

## 4. Package `internal/recognize`

Detection only suggests; nothing is applied without the user confirming in the preview.

### Data file (`recognizers.json`)

Defaults are embedded (`//go:embed defaults/recognizers.json`). A user file at
`<configDir>/recognizers.json` (same schema) is merged over the defaults: `sourceKinds`
and `profiles` entries with the same `id` replace the default; new ids are appended;
an entry with `"disabled": true` removes the default with that id.

```json
{
  "version": 1,
  "sourceKinds": [
    { "id": "skill", "kind": "skill", "match": { "type": "dir", "containsFile": "SKILL.md", "frontmatterKeys": ["name", "description"] } },
    { "id": "mcp-json", "kind": "mcp", "match": { "type": "file", "extensions": [".json"], "jsonKey": "mcpServers" } },
    { "id": "instructions", "kind": "instructions", "match": { "type": "file", "fileNames": ["AGENTS.md", "CLAUDE.md", "GEMINI.md"] } }
  ],
  "profiles": [
    {
      "id": "claude-code",
      "label": "Claude Code",
      "detect": { "folderNames": [".claude"], "anyOf": [ { "file": "settings.json" }, { "dir": "skills" }, { "file": "CLAUDE.md" } ] },
      "placements": {
        "instructions": { "path": "CLAUDE.md" },
        "skill": { "path": "skills/{name}" },
        "mcp": { "base": "home", "path": ".claude.json", "transform": { "type": "json_merge", "key": "mcpServers" } }
      }
    }
  ]
}
```

Matching rules: `match.type` is `"dir"` or `"file"`; all present conditions must hold.
`detect`: profile matches a folder when its base name is in `folderNames` (case-insensitive)
OR any `anyOf` condition holds (file/dir exists directly inside the folder). Score = 2 for
folder name match + 1 per `anyOf` hit; highest score wins; ties → first in list.
`placements.<kind>.path` supports `{name}` (source base name without extension for files,
folder name for dirs; for skills the frontmatter `name` if present). `base: "home"` means the path
is relative to the home dir instead of the detected folder.

### Go API

```go
package recognize

type Match struct { /* mirrors JSON */ }
type SourceKind struct { ID, Kind string; Match Match; Disabled bool }
type PlacementSpec struct { Base, Path string; Transform *rules.Transform }
type Profile struct { ID, Label string; Detect Detect; Placements map[string]PlacementSpec; Disabled bool }
type Set struct { Version int; SourceKinds []SourceKind; Profiles []Profile }

const UserFileName = "recognizers.json"

func Defaults() (*Set, error)                 // parsed embedded file
func Load() (*Set, error)                     // defaults merged with <configDir>/recognizers.json; invalid user file => apperr RecognizersInvalid
func UserFilePath() string
func WriteUserTemplate() (string, error)      // creates the user file with the defaults if missing; returns its path

type SourceInfo struct {
    Kind     string `json:"kind"`     // rules.Kind* (never "auto")
    KindID   string `json:"kindId"`   // matching sourceKinds id, "" for plain file/folder
    Name     string `json:"name"`     // used for {name}
    IsDir    bool   `json:"isDir"`
}
func (s *Set) DetectSource(absPath string) (SourceInfo, error)

type FolderProfile struct {
    ProfileID string `json:"profileId"`
    Label     string `json:"label"`
    Score     int    `json:"score"`
}
func (s *Set) DetectFolder(absFolder string) (FolderProfile, bool)

type Suggestion struct {
    ProfileID  string           `json:"profileId"`  // "" when generic
    Label      string           `json:"label"`
    Location   string           `json:"location"`   // alias to use ("home" when base=home, otherwise the given alias)
    Path       string           `json:"path"`       // relative to Location
    Mode       string           `json:"mode"`
    Transform  *rules.Transform `json:"transform,omitempty"`
    Detected   bool             `json:"detected"`   // false => generic fallback
}
// Suggest proposes placement for a source into a location (alias + its absolute folder).
// Generic fallback: Path = source base name, Mode = two_way, no transform.
func (s *Set) Suggest(src SourceInfo, alias, locationAbs string) Suggestion
```

The default `recognizers.json` must cover Claude Code, Codex CLI, Gemini CLI, OpenCode and
Antigravity using **verified** current paths/file names (instructions file name, skills folder,
MCP config file + key + format). Verify with Context7 (`npx ctx7@latest ...`) or official docs.
Anything that cannot be verified is left out of `placements` and listed in the docs as unverified.

---

## 5. Package `internal/rulesync` (status, apply, capture, resolve, diff)

```go
package rulesync

const BaselinePrefix = "rule:" // state key: "rule:<ruleID>:<destID>"; managed transform entries stored under key + "#managed" relPath "__managed__" as comma-joined names

// File statuses (reuse syncer constants where identical).
const (
    StatusInSync        = "in_sync"
    StatusNewInSource   = "new_in_source"
    StatusSourceChanged = "source_changed"
    StatusTargetChanged = "target_changed"
    StatusTargetMissing = "target_missing"
    StatusConflict      = "conflict"
    StatusUntrackedDiff = "untracked_diff"
    StatusUnreadable    = "unreadable"
    StatusSourceDeleted = "source_deleted" // source file gone, destination unchanged since baseline -> delete on apply
    StatusDrift         = "drift"          // one_way/transform destination edited locally
    StatusIgnored       = "ignored"
)

// Destination states.
const (
    DestReady         = "ready"
    DestUnmapped      = "location_unmapped"
    DestBlocked       = "blocked_collision"
    DestDisabled      = "disabled"
    DestSourceMissing = "source_missing"
)

// Transformer is implemented by an adapter over internal/transform (wired in service).
type Transformer interface {
    Render(spec rules.Transform, source, existing []byte, prevManaged []string) (out []byte, managed []string, managedView []byte, err error)
    SourceView(spec rules.Transform, source []byte) ([]byte, error)
    TargetView(spec rules.Transform, existing []byte, managed []string) ([]byte, error)
}

type FileStatus struct {
    RelPath   string `json:"relPath"`
    SourceAbs string `json:"sourceAbs"`
    TargetAbs string `json:"targetAbs"`
    Status    string `json:"status"`
}

type DestinationStatus struct {
    RuleID     string       `json:"ruleId"`
    DestID     string       `json:"destId"`
    State      string       `json:"state"`
    TargetRoot string       `json:"targetRoot"`
    Files      []FileStatus `json:"files"`
}

type RuleStatus struct {
    Rule         rules.Rule          `json:"rule"`
    Scope        string              `json:"scope"`
    DetectedKind string              `json:"detectedKind"`
    Destinations []DestinationStatus `json:"destinations"`
}

type Report struct {
    Rules      []RuleStatus      `json:"rules"`
    Collisions []rules.Collision `json:"collisions"`
    Unmapped   []string          `json:"unmapped"` // aliases referenced but not mapped on this PC
    Pending    int               `json:"pending"`  // files that Apply/Capture would act on
    Conflicts  int               `json:"conflicts"`
}

type Engine struct {
    Env         rules.Env
    State       *state.State
    BackupRoot  string
    Retention   int
    Transformer Transformer
    Legacy      []config.Connection
    DetectKind  func(absPath string) string // optional; returns rules.Kind*
}

// Status computes the full report (no writes). Uses the same 3-way matrix as syncer.EvaluateFileStatus
// with source/target/baseline hashes. For transformed destinations: sourceHash = hash(SourceView),
// targetHash = hash(TargetView(managed)), and a target-side difference is StatusDrift (never upload).
// For ModeOneWay plain copies a target-side edit is StatusDrift. For ModeCreateOnly an existing
// target is always in_sync.
func (e *Engine) Status(all []rules.Rule) Report

type ApplyResult struct {
    Written  int      `json:"written"`
    Captured int      `json:"captured"`
    Deleted  int      `json:"deleted"`
    Backups  []string `json:"backups"`
    Skipped  int      `json:"skipped"`  // conflicts/drift/blocked left for the user
    CapturedRepoPaths []string `json:"capturedRepoPaths"` // repo-relative paths changed by capture (to commit)
}

// Sync applies every non-conflicting change: source->target writes (new_in_source, source_changed,
// target_missing, source_deleted), and for ModeTwoWay with a writable source (repo/local) captures
// target_changed back to the source. Subscription sources are read-only: target edits are reported
// as drift. Every overwrite/delete of a destination file is preceded by a backup
// (backup.CreateBackup with a synthetic config.Connection{ID: "rule:<ruleID>:<destID>", Profile: rule.Name,
// TargetPath: TargetRoot}). Blocked/unmapped/disabled destinations are skipped. Baselines are updated
// after each successful write and state saved at the end.
func (e *Engine) Sync(all []rules.Rule) (ApplyResult, error)

// Resolution choices.
const (
    UseSource = "use_source" // overwrite destination with source (backup first)
    UseTarget = "use_target" // copy destination into source (only writable sources, no transform)
    UseCustom = "custom"     // write Content to both sides (for transforms: only allowed as edited source)
    KeepLocal = "keep_local" // add RelPath to Destination.Ignore (caller persists the rule)
)

type Resolution struct {
    RuleID  string `json:"ruleId"`
    DestID  string `json:"destId"`
    RelPath string `json:"relPath"`
    Choice  string `json:"choice"`
    Content string `json:"content,omitempty"`
}

// Resolve applies one resolution. Returns the possibly-modified rule when Choice == KeepLocal
// (caller saves it), plus the repo-relative path touched if the source is in the repo.
func (e *Engine) Resolve(all []rules.Rule, res Resolution) (updated *rules.Rule, repoPath string, err error)

type Diff struct {
    LeftLabel  string `json:"leftLabel"`   // "source"
    RightLabel string `json:"rightLabel"`  // "destination"
    Left       string `json:"left"`
    Right      string `json:"right"`
    Binary     bool   `json:"binary"`
    Transform  bool   `json:"transform"`   // Left/Right are rendered output vs current file
}
const MaxDiffBytes = 1 << 20 // larger files are reported as Binary=true with empty contents
func (e *Engine) Diff(all []rules.Rule, ruleID, destID, relPath string) (Diff, error)
```

Binary detection: NUL byte in the first 8000 bytes or invalid UTF-8.

---

## 6. Package `internal/gitops` additions + `internal/subscriptions`

### gitops additions (existing file `internal/gitops/gitops.go`)

```go
// ValidateRemoteURL accepts https://, http://, ssh://, git@host:path and file:// (tests).
// Rejects "ext::", leading "-", whitespace/control chars. Returns apperr InvalidRemoteURL.
func ValidateRemoteURL(url string) error
// All git invocations must set GIT_TERMINAL_PROMPT=0 and -c protocol.ext.allow=never;
// clone/fetch of untrusted URLs additionally pass "--" before the URL.

func (g *GitRunner) ShowFile(rev, relPath string) (content []byte, exists bool, err error) // git show rev:path
func (g *GitRunner) ResetHardToUpstream() error
func (g *GitRunner) HeadCommit() (string, error)
func (g *GitRunner) UpstreamCommit() (string, error)
func (g *GitRunner) ChangedFilesBetween(fromRev, toRev string) ([]string, error)
func (g *GitRunner) RemoteDefaultBranch() (string, bool, error) // ("", false, nil) when remote has no branches
func (g *GitRunner) CreateBranchAt(name, rev string) error

type ConnectRemoteResult struct {
    Branch           string        `json:"branch"`
    RemoteHadContent bool          `json:"remoteHadContent"`
    BackupBranch     string        `json:"backupBranch,omitempty"` // local history preserved here when replaced
    LocalChanges     []ChangedPath `json:"localChanges"`          // uncommitted differences vs remote for the user to review
}

// ConnectRemote attaches root (initializing git if needed) to url without losing local files:
//  1. init if not a repo; add/set origin; fetch.
//  2. remote empty  -> commit local files (if identity present; otherwise leave staged) and push -u.
//  3. remote has branch B:
//     - if local has commits: create backup branch "agentsync/local-backup-<yyyymmdd-hhmmss>" at HEAD.
//     - point HEAD to B, `reset --mixed origin/B` (working tree untouched), set upstream.
//     - restore files that exist in origin/B but are missing locally (`checkout -- <path>`).
//     - report remaining uncommitted changes as LocalChanges (user reviews with diffs).
func ConnectRemote(root, url string) (*ConnectRemoteResult, error)
```

### `internal/subscriptions`

```go
package subscriptions

const (
    DirName  = "subscriptions" // under config.GetConfigDir()
    CloneDir = "repo"
)

func Root(id string) string // <configDir>/subscriptions/<id>/repo

// Add validates URL, clones into a fresh dir (removing partial clone on failure), detects branch.
func Add(url, name string) (config.Subscription, error)
func Remove(id string) error

type UpdateInfo struct {
    SubscriptionID string   `json:"subscriptionId"`
    Behind         int      `json:"behind"`
    IncomingFiles  []string `json:"incomingFiles"`
    HeadCommit     string   `json:"headCommit"`
    RemoteCommit   string   `json:"remoteCommit"`
    Offline        bool     `json:"offline"`
}
func Check(sub config.Subscription) (UpdateInfo, error)        // fetch + compare, no working tree changes
func Update(sub config.Subscription) ([]string, error)         // reset --hard to upstream; returns changed files

type TreeEntry struct {
    Path  string `json:"path"`  // forward slashes, relative to clone root
    IsDir bool   `json:"isDir"`
    Size  int64  `json:"size"`
}
// Tree lists entries (skips .git and fsops.ShouldIgnore) up to maxDepth levels.
func Tree(id string, maxDepth int) ([]TreeEntry, error)
// IncomingDiff returns file content at HEAD and at upstream for one path (for the diff view).
func IncomingDiff(id, relPath string) (before, after []byte, err error)
```

---

## 7. Service / bindings (integration owner)

New `Service` methods exposed through `app.go` (Wails):

- Locations: `ListLocations`, `SaveLocation(alias, path)`, `RemoveLocation(alias)`.
- Rules: `GetRulesReport`, `SaveRule(rule, scope)`, `DeleteRule(id)`, `SyncRules`,
  `ResolveRuleFile(resolution)`, `GetRuleDiff(ruleID, destID, relPath)`.
- Detection: `DetectSource(origin, subscriptionID, path)`, `SuggestPlacement(source, alias)`,
  `GetRecognizersPath`, `OpenRecognizersFile`.
- Subscriptions: `ListSubscriptions`, `AddSubscription(url, name)`, `RemoveSubscription(id)`,
  `CheckSubscription(id)`, `UpdateSubscription(id)`, `GetSubscriptionTree(id)`,
  `GetSubscriptionIncomingDiff(id, path)`.
- Repo: `ConnectRemote(url)`, `GetRepoFileDiff(path, mode)` (mode `working` = HEAD vs working tree,
  `incoming` = HEAD vs upstream), `DiscardRepoFile(path)`.

Global `ExecuteSync`: after the orchestrator finishes (commit/pull/connections/push),
run `rulesync.Engine.Sync`; if it captured repo files, commit them and push.
The orchestrator treats rule source paths and `agentsync.rules.json` as managed
(not `DirtyOutside`). Scheduler checks also refresh subscriptions.
