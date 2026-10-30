// Package rulesync computes status for generic sync rules and applies, captures, resolves
// and diffs their files. See docs/design/sync-rules-spec.md section 5.
package rulesync

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"

	"agentsync/internal/config"
	"agentsync/internal/rules"
	"agentsync/internal/state"
	"agentsync/internal/syncer"
)

// BaselinePrefix is the state key prefix: "rule:<ruleID>:<destID>". Managed transform entries are
// stored under key + ManagedKeySuffix with relPath ManagedEntriesRel as comma-joined names.
const BaselinePrefix = "rule:"

const (
	// ManagedKeySuffix is appended to the destination key to store transform bookkeeping.
	ManagedKeySuffix = "#managed"
	// ManagedEntriesRel stores the comma-joined entry names owned by AgentSync.
	ManagedEntriesRel = "__managed__"
	// TargetViewRel stores the hash of the destination's managed view right after the last write.
	TargetViewRel = "__target__"
	// RulesBackupDir is the folder under the backup root holding rule snapshots.
	RulesBackupDir = "rules"
	// SourceBackupID names the backup slot used before a capture overwrites a source file.
	SourceBackupID = "_source"

	managedSeparator = ","
	keySeparator     = ":"
	backupSeparator  = "__"
)

// File statuses (identical values to the syncer constants where they overlap).
const (
	StatusInSync        = syncer.FileInSync
	StatusNewInSource   = syncer.FileNewInSource
	StatusSourceChanged = syncer.FileSourceChanged
	StatusTargetChanged = syncer.FileTargetChanged
	StatusTargetMissing = syncer.FileTargetMissing
	StatusConflict      = syncer.FileConflict
	StatusUntrackedDiff = syncer.FileUntrackedDiff
	StatusUnreadable    = syncer.FileUnreadable
	StatusSourceDeleted = "source_deleted"
	StatusDrift         = "drift"
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

// Resolution choices.
const (
	UseSource = "use_source"
	UseTarget = "use_target"
	UseCustom = "custom"
	KeepLocal = "keep_local"
)

// MaxDiffBytes is the largest file shown in a diff; larger files are reported as binary.
const MaxDiffBytes = 1 << 20

// Transformer is implemented by an adapter over internal/transform (wired in service).
type Transformer interface {
	Render(spec rules.Transform, source, existing []byte, prevManaged []string) (out []byte, managed []string, managedView []byte, err error)
	SourceView(spec rules.Transform, source []byte) ([]byte, error)
	TargetView(spec rules.Transform, existing []byte, managed []string) ([]byte, error)
}

// FileStatus is the status of one file of one destination.
type FileStatus struct {
	RelPath   string `json:"relPath"`
	SourceAbs string `json:"sourceAbs"`
	TargetAbs string `json:"targetAbs"`
	Status    string `json:"status"`
}

// DestinationStatus is the status of one destination of one rule.
type DestinationStatus struct {
	RuleID     string       `json:"ruleId"`
	DestID     string       `json:"destId"`
	State      string       `json:"state"`
	TargetRoot string       `json:"targetRoot"`
	Files      []FileStatus `json:"files"`
}

// RuleStatus groups the destinations of one rule.
type RuleStatus struct {
	Rule         rules.Rule          `json:"rule"`
	Scope        string              `json:"scope"`
	DetectedKind string              `json:"detectedKind"`
	Destinations []DestinationStatus `json:"destinations"`
}

// Report is the full rules status.
type Report struct {
	Rules      []RuleStatus      `json:"rules"`
	Collisions []rules.Collision `json:"collisions"`
	Unmapped   []string          `json:"unmapped"`
	Pending    int               `json:"pending"`
	Conflicts  int               `json:"conflicts"`
}

// ApplyResult summarizes a Sync run.
type ApplyResult struct {
	Written           int      `json:"written"`
	Captured          int      `json:"captured"`
	Deleted           int      `json:"deleted"`
	Backups           []string `json:"backups"`
	Skipped           int      `json:"skipped"`
	CapturedRepoPaths []string `json:"capturedRepoPaths"`
}

// Resolution is one user decision about one file.
type Resolution struct {
	RuleID  string `json:"ruleId"`
	DestID  string `json:"destId"`
	RelPath string `json:"relPath"`
	Choice  string `json:"choice"`
	Content string `json:"content,omitempty"`
}

// Diff holds both sides of a file for the diff view.
type Diff struct {
	LeftLabel  string `json:"leftLabel"`
	RightLabel string `json:"rightLabel"`
	Left       string `json:"left"`
	Right      string `json:"right"`
	Binary     bool   `json:"binary"`
	Transform  bool   `json:"transform"`
}

// Engine evaluates and applies rules against the local file system.
type Engine struct {
	Env         rules.Env
	State       *state.State
	BackupRoot  string
	Retention   int
	Transformer Transformer
	Legacy      []config.Connection
	DetectKind  func(absPath string) string
}

// BaselineKey returns the state key of a rule destination.
func BaselineKey(ruleID, destID string) string {
	return BaselinePrefix + ruleID + keySeparator + destID
}

// ManagedKey returns the state key holding transform bookkeeping for a destination.
func ManagedKey(ruleID, destID string) string {
	return BaselineKey(ruleID, destID) + ManagedKeySuffix
}

var unsafeFileNameChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// BackupRootFor returns the backup root used for one rule destination (or SourceBackupID).
// Rule backups live under <backupRoot>/rules/<ruleID>__<destID> so they never mix with
// legacy connection snapshots, and IDs are sanitized to be valid folder names.
func BackupRootFor(backupRoot, ruleID, destID string) string {
	name := unsafeFileNameChars.ReplaceAllString(ruleID, "_") + backupSeparator +
		unsafeFileNameChars.ReplaceAllString(destID, "_")
	return filepath.Join(backupRoot, RulesBackupDir, name)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (e *Engine) baseline(key, rel string) string {
	if e.State == nil {
		return ""
	}
	return e.State.GetBaseline(key, rel)
}

func (e *Engine) setBaseline(key, rel, hash string) {
	if e.State != nil {
		e.State.SetBaseline(key, rel, hash)
	}
}

func (e *Engine) removeBaseline(key, rel string) {
	if e.State != nil {
		e.State.RemoveBaseline(key, rel)
	}
}

func (e *Engine) baselines(key string) map[string]string {
	if e.State == nil {
		return map[string]string{}
	}
	return e.State.GetConnectionBaselines(key)
}

func (e *Engine) saveState() error {
	if e.State == nil {
		return nil
	}
	return e.State.Save()
}

// managedEntries returns the entry names owned by AgentSync in a transformed destination.
func (e *Engine) managedEntries(ruleID, destID string) []string {
	raw := e.baseline(ManagedKey(ruleID, destID), ManagedEntriesRel)
	if raw == "" {
		return nil
	}
	var out []string
	for _, name := range strings.Split(raw, managedSeparator) {
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func (e *Engine) recordTransformBaselines(ruleID, destID, rel, sourceHash string, managed []string, targetViewHash string) {
	e.setBaseline(BaselineKey(ruleID, destID), rel, sourceHash)
	mk := ManagedKey(ruleID, destID)
	e.setBaseline(mk, ManagedEntriesRel, strings.Join(managed, managedSeparator))
	e.setBaseline(mk, TargetViewRel, targetViewHash)
}
