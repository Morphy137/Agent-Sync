// Package rules defines generic sync rules (one source -> many destinations), their storage,
// resolution to absolute paths and collision detection. See docs/design/sync-rules-spec.md.
package rules

const (
	RulesFileName      = "agentsync.rules.json"
	LocalRulesFileName = "rules.local.json"
	CurrentVersion     = 1
	BuiltinHomeAlias   = "home"
)

// Rule scopes (where the rule is stored).
const (
	ScopeRepo  = "repo"
	ScopeLocal = "local"
)

// Source origins.
const (
	OriginRepo         = "repo"
	OriginLocal        = "local"
	OriginSubscription = "subscription"
)

// Destination modes.
const (
	ModeTwoWay     = "two_way"
	ModeOneWay     = "one_way"
	ModeCreateOnly = "create_only"
)

// Content kinds (detected or forced by the user).
const (
	KindAuto         = "auto"
	KindFile         = "file"
	KindFolder       = "folder"
	KindSkill        = "skill"
	KindMCP          = "mcp"
	KindInstructions = "instructions"
)

// Transform types.
const (
	TransformJSONMerge = "json_merge"
	TransformTOMLMerge = "toml_merge"
)

// Source identifies where a rule reads its content from.
type Source struct {
	Origin       string `json:"origin"`
	Subscription string `json:"subscription,omitempty"`
	Path         string `json:"path"`
	// Include, when set, turns a folder source into a selection: only these files (relative to
	// Path, forward slashes) are synced, each keeping its relative path in the destination.
	Include []string `json:"include,omitempty"`
}

// HasSelection reports whether the source syncs a chosen set of files instead of a whole path.
func (s Source) HasSelection() bool {
	return len(s.Include) > 0
}

// Transform converts the source before writing it into the destination file.
type Transform struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// Destination is one place a rule writes to, expressed relative to a location alias.
type Destination struct {
	ID        string     `json:"id"`
	Location  string     `json:"location"`
	Path      string     `json:"path"`
	Mode      string     `json:"mode"`
	Transform *Transform `json:"transform,omitempty"`
	Enabled   bool       `json:"enabled"`
	Ignore    []string   `json:"ignore,omitempty"`
}

// Rule maps one source to many destinations.
type Rule struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Enabled      bool          `json:"enabled"`
	Source       Source        `json:"source"`
	Kind         string        `json:"kind"`
	Exclude      []string      `json:"exclude,omitempty"`
	Destinations []Destination `json:"destinations"`
	Scope        string        `json:"scope,omitempty"`
}

// File is the on-disk format of both the repo and the local rules files.
type File struct {
	Version int    `json:"version"`
	Rules   []Rule `json:"rules"`
}

// Env carries everything needed to turn a rule into absolute paths on this machine.
type Env struct {
	RepoRoot          string
	Locations         map[string]string
	HomeDir           string
	SubscriptionRoots map[string]string
}

// Pair is one source file mapped to one destination file.
type Pair struct {
	RelPath   string
	SourceAbs string
	TargetAbs string
}

// ResolvedDestination is a destination expanded to absolute file pairs.
type ResolvedDestination struct {
	RuleID        string
	DestID        string
	TargetRoot    string
	SourceIsDir   bool
	Pairs         []Pair
	Unmapped      bool
	SourceMissing bool
}

// Collision types.
const (
	CollisionSameTarget    = "same_target"
	CollisionFolderOverlap = "folder_overlap"
	CollisionCycle         = "cycle"
	CollisionInsideRepo    = "inside_repo"
	CollisionCaseConflict  = "case_conflict"
)

// DestRef points at a rule destination or a legacy folder connection.
type DestRef struct {
	RuleID       string `json:"ruleId,omitempty"`
	DestID       string `json:"destId,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"`
}

// Collision reports destinations that would write to the same or overlapping places.
type Collision struct {
	Type string    `json:"type"`
	Path string    `json:"path"`
	Refs []DestRef `json:"refs"`
}

// DefaultSecretExcludes are always excluded from folder sources as a safety net.
var DefaultSecretExcludes = []string{".credentials.json", "auth.json", "*.pem", "*.key", ".env", ".env.*", "id_rsa*", "*.p12"}
