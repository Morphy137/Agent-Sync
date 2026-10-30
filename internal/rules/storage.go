package rules

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/fsops"
)

const (
	// HomeOverrideEnvVar overrides the builtin "home" location (used by tests and sandboxes).
	HomeOverrideEnvVar = "AGENTSYNC_HOME_OVERRIDE"
	// RuleIDPrefix and DestinationIDPrefix are the conventional prefixes passed to NewID.
	RuleIDPrefix        = "r_"
	DestinationIDPrefix = "d_"

	idRandomBytes = 5 // 10 hex characters
	jsonIndent    = "  "
)

// Error reasons reported in the "reason" param of apperr.InvalidRule.
const (
	ReasonMalformedFile        = "malformed_file"
	ReasonUnsupportedVersion   = "unsupported_version"
	ReasonMissingID            = "missing_id"
	ReasonMissingName          = "missing_name"
	ReasonInvalidScope         = "invalid_scope"
	ReasonInvalidOrigin        = "invalid_origin"
	ReasonLocalOriginInRepo    = "local_origin_in_repo_scope"
	ReasonMissingSubscription  = "missing_subscription"
	ReasonInvalidSourcePath    = "invalid_source_path"
	ReasonInvalidKind          = "invalid_kind"
	ReasonInvalidExclude       = "invalid_exclude"
	ReasonMissingDestinationID = "missing_destination_id"
	ReasonDuplicateDestination = "duplicate_destination_id"
	ReasonMissingLocation      = "missing_location"
	ReasonInvalidDestPath      = "invalid_destination_path"
	ReasonInvalidMode          = "invalid_mode"
	ReasonInvalidTransform     = "invalid_transform"
	ReasonTransformNeedsFile   = "transform_requires_file_source"
	ReasonInvalidIgnore        = "invalid_ignore"
	ReasonFileDestNeedsPath    = "file_destination_requires_path"
	ReasonInvalidRelPath       = "invalid_rel_path"
)

// LoadRepoRules reads <repoRoot>/agentsync.rules.json. Missing file => empty File, nil error.
func LoadRepoRules(repoRoot string) (*File, error) {
	if repoRoot == "" {
		return nil, apperr.New(apperr.RootNotFound)
	}
	return loadFile(filepath.Join(repoRoot, RulesFileName), ScopeRepo)
}

// SaveRepoRules writes <repoRoot>/agentsync.rules.json.
func SaveRepoRules(repoRoot string, f *File) error {
	if repoRoot == "" {
		return apperr.New(apperr.RootNotFound)
	}
	return saveFile(filepath.Join(repoRoot, RulesFileName), f)
}

// LocalRulesPath returns the absolute path of the per-PC rules file.
func LocalRulesPath() string {
	return filepath.Join(config.GetConfigDir(), LocalRulesFileName)
}

// LoadLocalRules reads <configDir>/rules.local.json. Missing file => empty File, nil error.
func LoadLocalRules() (*File, error) {
	return loadFile(LocalRulesPath(), ScopeLocal)
}

// SaveLocalRules writes <configDir>/rules.local.json.
func SaveLocalRules(f *File) error {
	return saveFile(LocalRulesPath(), f)
}

// NewID returns prefix followed by 10 random hex characters.
func NewID(prefix string) string {
	buf := make([]byte, idRandomBytes)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand never fails on supported platforms; panic keeps IDs unique by contract.
		panic(fmt.Sprintf("rules: random source unavailable: %v", err))
	}
	return prefix + hex.EncodeToString(buf)
}

// DefaultHomeDir resolves the builtin "home" location: AGENTSYNC_HOME_OVERRIDE first, then the OS home.
func DefaultHomeDir() string {
	if override := os.Getenv(HomeOverrideEnvVar); override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func emptyFile() *File {
	return &File{Version: CurrentVersion, Rules: []Rule{}}
}

func loadFile(path, scope string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyFile(), nil
		}
		return nil, apperr.Wrap(apperr.WriteFailed, err).WithParam("path", path)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return emptyFile(), nil
	}

	f := emptyFile()
	if err := json.Unmarshal(data, f); err != nil {
		return nil, apperr.Wrap(apperr.InvalidRule, err).
			WithParam("reason", ReasonMalformedFile).
			WithParam("path", path)
	}
	if f.Version == 0 {
		f.Version = CurrentVersion
	}
	if f.Version > CurrentVersion {
		return nil, apperr.New(apperr.InvalidRule).
			WithParam("reason", ReasonUnsupportedVersion).
			WithParam("path", path)
	}
	if f.Rules == nil {
		f.Rules = []Rule{}
	}
	for i := range f.Rules {
		f.Rules[i].Scope = scope
		if f.Rules[i].Destinations == nil {
			f.Rules[i].Destinations = []Destination{}
		}
	}
	return f, nil
}

func saveFile(path string, f *File) error {
	if f == nil {
		f = emptyFile()
	}
	out := File{Version: CurrentVersion, Rules: make([]Rule, 0, len(f.Rules))}
	for _, r := range f.Rules {
		r.Scope = ""
		if r.Destinations == nil {
			r.Destinations = []Destination{}
		}
		out.Rules = append(out.Rules, r)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", jsonIndent)
	// Encode appends the trailing newline required for diff-friendly files.
	if err := enc.Encode(out); err != nil {
		return apperr.Wrap(apperr.WriteFailed, err)
	}
	return fsops.AtomicWrite(path, buf.Bytes(), fsops.DefaultFilePerms)
}
