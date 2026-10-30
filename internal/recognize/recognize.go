// Package recognize detects what a sync source is (skill, MCP config, agent instructions or a
// plain file/folder) and what kind of agent folder a destination is, so the UI can propose a
// placement. All agent knowledge is data: embedded defaults merged with a per-PC user file.
// See docs/design/sync-rules-spec.md, section 4.
package recognize

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
	"agentsync/internal/rules"
)

// UserFileName is the name of the per-PC override file inside config.GetConfigDir().
const UserFileName = "recognizers.json"

// SchemaVersion is the only recognizers.json version understood by this package.
const SchemaVersion = 1

// Match types for source kinds.
const (
	MatchTypeDir  = "dir"
	MatchTypeFile = "file"
)

// BaseHome makes a placement path relative to the user home directory.
const BaseHome = "home"

// Error parameter names used with apperr.RecognizersInvalid.
const (
	paramReason = "reason"
	paramID     = "id"
	paramPath   = "path"
)

//go:embed defaults/recognizers.json
var defaultsJSON []byte

// Match describes the conditions a source must satisfy; every present condition must hold.
type Match struct {
	Type            string   `json:"type"`
	ContainsFile    string   `json:"containsFile,omitempty"`
	FrontmatterKeys []string `json:"frontmatterKeys,omitempty"`
	Extensions      []string `json:"extensions,omitempty"`
	FileNames       []string `json:"fileNames,omitempty"`
	JSONKey         string   `json:"jsonKey,omitempty"`
}

// SourceKind maps a match to a content kind (rules.Kind*).
type SourceKind struct {
	ID       string `json:"id"`
	Kind     string `json:"kind,omitempty"`
	Match    Match  `json:"match"`
	Disabled bool   `json:"disabled,omitempty"`
}

// DetectCondition is satisfied when the named file or directory exists directly inside a folder.
// Exactly one of File or Dir is set.
type DetectCondition struct {
	File string `json:"file,omitempty"`
	Dir  string `json:"dir,omitempty"`
}

// Detect describes how a folder is recognized as belonging to a profile.
type Detect struct {
	FolderNames []string          `json:"folderNames,omitempty"`
	AnyOf       []DetectCondition `json:"anyOf,omitempty"`
}

// PlacementSpec tells where a content kind goes inside a detected folder (or the home dir).
type PlacementSpec struct {
	Base      string           `json:"base,omitempty"`
	Path      string           `json:"path"`
	Transform *rules.Transform `json:"transform,omitempty"`
}

// Profile is a known kind of destination folder (e.g. a Claude Code config folder).
type Profile struct {
	ID         string                   `json:"id"`
	Label      string                   `json:"label,omitempty"`
	Detect     Detect                   `json:"detect"`
	Placements map[string]PlacementSpec `json:"placements,omitempty"`
	Disabled   bool                     `json:"disabled,omitempty"`
}

// Set is the parsed content of a recognizers.json file.
type Set struct {
	Version     int          `json:"version"`
	SourceKinds []SourceKind `json:"sourceKinds"`
	Profiles    []Profile    `json:"profiles"`
}

// Defaults returns a freshly parsed copy of the embedded default recognizers.
func Defaults() (*Set, error) {
	set, err := parseSet(defaultsJSON)
	if err != nil {
		return nil, err
	}
	if err := validateSet(set, false); err != nil {
		return nil, err
	}
	return set, nil
}

// UserFilePath returns the absolute path of the per-PC override file.
func UserFilePath() string {
	return filepath.Join(config.GetConfigDir(), UserFileName)
}

// Load returns the defaults merged with the user file. A missing user file is not an error;
// an unreadable, malformed or invalid one returns apperr.RecognizersInvalid.
func Load() (*Set, error) {
	defaults, err := Defaults()
	if err != nil {
		return nil, err
	}

	userPath := UserFilePath()
	data, err := os.ReadFile(userPath)
	if errors.Is(err, fs.ErrNotExist) {
		return defaults, nil
	}
	if err != nil {
		return nil, apperr.Wrap(apperr.RecognizersInvalid, err).WithParam(paramPath, userPath)
	}

	user, err := parseSet(data)
	if err != nil {
		return nil, withPath(err, userPath)
	}
	if err := validateSet(user, true); err != nil {
		return nil, withPath(err, userPath)
	}

	merged := merge(defaults, user)
	if err := validateSet(merged, false); err != nil {
		return nil, withPath(err, userPath)
	}
	return merged, nil
}

// WriteUserTemplate creates the user file with the default recognizers when it does not exist
// yet and returns its path. An existing file is never overwritten.
func WriteUserTemplate() (string, error) {
	userPath := UserFilePath()
	_, err := os.Stat(userPath)
	if err == nil {
		return userPath, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", apperr.Wrap(apperr.WriteFailed, err).WithParam(paramPath, userPath)
	}
	if err := fsops.AtomicWrite(userPath, defaultsJSON, fsops.DefaultFilePerms); err != nil {
		return "", err
	}
	return userPath, nil
}

// parseSet decodes a recognizers document, rejecting unknown fields so typos surface early.
func parseSet(data []byte) (*Set, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var set Set
	if err := decoder.Decode(&set); err != nil {
		return nil, apperr.Wrap(apperr.RecognizersInvalid, err).WithParam(paramReason, "malformed_json")
	}
	if decoder.More() {
		return nil, invalid("trailing_data", "")
	}
	return &set, nil
}

// merge applies the user entries over the defaults: same id replaces, new ids are appended,
// "disabled": true removes the default with that id.
func merge(defaults, user *Set) *Set {
	merged := &Set{
		Version:     defaults.Version,
		SourceKinds: append([]SourceKind(nil), defaults.SourceKinds...),
		Profiles:    append([]Profile(nil), defaults.Profiles...),
	}

	for _, override := range user.SourceKinds {
		idx := indexOf(merged.SourceKinds, func(k SourceKind) bool { return k.ID == override.ID })
		merged.SourceKinds = applyOverride(merged.SourceKinds, idx, override, override.Disabled)
	}
	for _, override := range user.Profiles {
		idx := indexOf(merged.Profiles, func(p Profile) bool { return p.ID == override.ID })
		merged.Profiles = applyOverride(merged.Profiles, idx, override, override.Disabled)
	}
	return merged
}

func indexOf[T any](items []T, match func(T) bool) int {
	for i, item := range items {
		if match(item) {
			return i
		}
	}
	return -1
}

func applyOverride[T any](items []T, idx int, override T, disabled bool) []T {
	if idx < 0 {
		if disabled {
			return items
		}
		return append(items, override)
	}
	if disabled {
		return append(items[:idx:idx], items[idx+1:]...)
	}
	items[idx] = override
	return items
}

func invalid(reason, id string) *apperr.Error {
	err := apperr.New(apperr.RecognizersInvalid).WithParam(paramReason, reason)
	if id != "" {
		err.WithParam(paramID, id)
	}
	return err
}

func withPath(err error, path string) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		appErr.WithParam(paramPath, path)
		return appErr
	}
	return apperr.Wrap(apperr.RecognizersInvalid, err).WithParam(paramPath, path)
}
