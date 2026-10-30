package recognize

import (
	"path"
	"strings"

	"agentsync/internal/rules"
)

// Validation failure reasons reported in the "reason" parameter of apperr.RecognizersInvalid.
const (
	reasonUnsupportedVersion = "unsupported_version"
	reasonMissingID          = "missing_id"
	reasonDuplicateID        = "duplicate_id"
	reasonUnknownKind        = "unknown_kind"
	reasonUnknownMatchType   = "unknown_match_type"
	reasonInvalidMatch       = "invalid_match"
	reasonMissingLabel       = "missing_label"
	reasonEmptyDetect        = "empty_detect"
	reasonInvalidCondition   = "invalid_condition"
	reasonInvalidPlacement   = "invalid_placement"
	reasonUnknownBase        = "unknown_base"
	reasonInvalidTransform   = "invalid_transform"
	reasonUnexpectedDisabled = "unexpected_disabled"
)

// knownKinds are the content kinds a source kind may produce and a placement may target.
var knownKinds = map[string]bool{
	rules.KindFile:         true,
	rules.KindFolder:       true,
	rules.KindSkill:        true,
	rules.KindMCP:          true,
	rules.KindInstructions: true,
}

var knownTransforms = map[string]bool{
	rules.TransformJSONMerge: true,
	rules.TransformTOMLMerge: true,
}

// validateSet checks a whole document. allowDisabled is true for the user file, where an entry
// may be just {"id": "...", "disabled": true}; disabled entries are not validated further.
func validateSet(set *Set, allowDisabled bool) error {
	if set.Version != SchemaVersion {
		return invalid(reasonUnsupportedVersion, "")
	}

	seenKinds := make(map[string]bool, len(set.SourceKinds))
	for _, kind := range set.SourceKinds {
		if err := checkID(kind.ID, seenKinds); err != nil {
			return err
		}
		if kind.Disabled && allowDisabled {
			continue
		}
		if err := validateSourceKind(kind); err != nil {
			return err
		}
	}

	seenProfiles := make(map[string]bool, len(set.Profiles))
	for _, profile := range set.Profiles {
		if err := checkID(profile.ID, seenProfiles); err != nil {
			return err
		}
		if profile.Disabled && allowDisabled {
			continue
		}
		if err := validateProfile(profile); err != nil {
			return err
		}
	}
	return nil
}

func checkID(id string, seen map[string]bool) error {
	if strings.TrimSpace(id) == "" {
		return invalid(reasonMissingID, "")
	}
	if seen[id] {
		return invalid(reasonDuplicateID, id)
	}
	seen[id] = true
	return nil
}

func validateSourceKind(kind SourceKind) error {
	if kind.Disabled {
		return invalid(reasonUnexpectedDisabled, kind.ID)
	}
	if !knownKinds[kind.Kind] {
		return invalid(reasonUnknownKind, kind.ID)
	}

	match := kind.Match
	switch match.Type {
	case MatchTypeDir:
		if match.JSONKey != "" || len(match.Extensions) > 0 {
			return invalid(reasonInvalidMatch, kind.ID)
		}
	case MatchTypeFile:
		if match.ContainsFile != "" {
			return invalid(reasonInvalidMatch, kind.ID)
		}
	default:
		return invalid(reasonUnknownMatchType, kind.ID)
	}

	if len(match.FrontmatterKeys) > 0 && match.ContainsFile == "" {
		return invalid(reasonInvalidMatch, kind.ID)
	}
	if match.ContainsFile != "" && !isPlainName(match.ContainsFile) {
		return invalid(reasonInvalidMatch, kind.ID)
	}
	for _, ext := range match.Extensions {
		if !strings.HasPrefix(ext, ".") || len(ext) < len(".x") {
			return invalid(reasonInvalidMatch, kind.ID)
		}
	}
	for _, name := range match.FileNames {
		if !isPlainName(name) {
			return invalid(reasonInvalidMatch, kind.ID)
		}
	}
	for _, key := range match.FrontmatterKeys {
		if strings.TrimSpace(key) == "" {
			return invalid(reasonInvalidMatch, kind.ID)
		}
	}
	return nil
}

func validateProfile(profile Profile) error {
	if profile.Disabled {
		return invalid(reasonUnexpectedDisabled, profile.ID)
	}
	if strings.TrimSpace(profile.Label) == "" {
		return invalid(reasonMissingLabel, profile.ID)
	}
	if len(profile.Detect.FolderNames) == 0 && len(profile.Detect.AnyOf) == 0 {
		return invalid(reasonEmptyDetect, profile.ID)
	}
	for _, name := range profile.Detect.FolderNames {
		if !isPlainName(name) {
			return invalid(reasonInvalidCondition, profile.ID)
		}
	}
	for _, cond := range profile.Detect.AnyOf {
		if err := validateCondition(profile.ID, cond); err != nil {
			return err
		}
	}
	for kind, placement := range profile.Placements {
		if !knownKinds[kind] {
			return invalid(reasonUnknownKind, profile.ID)
		}
		if err := validatePlacement(profile.ID, placement); err != nil {
			return err
		}
	}
	return nil
}

func validateCondition(profileID string, cond DetectCondition) error {
	hasFile := cond.File != ""
	hasDir := cond.Dir != ""
	if hasFile == hasDir {
		return invalid(reasonInvalidCondition, profileID)
	}
	name := cond.File + cond.Dir
	if !isPlainName(name) {
		return invalid(reasonInvalidCondition, profileID)
	}
	return nil
}

func validatePlacement(profileID string, placement PlacementSpec) error {
	if placement.Base != "" && placement.Base != BaseHome {
		return invalid(reasonUnknownBase, profileID)
	}
	if !isSafeRelativePath(placement.Path) {
		return invalid(reasonInvalidPlacement, profileID)
	}
	if placement.Transform == nil {
		return nil
	}
	if !knownTransforms[placement.Transform.Type] || strings.TrimSpace(placement.Transform.Key) == "" {
		return invalid(reasonInvalidTransform, profileID)
	}
	return nil
}

// isPlainName reports whether name is a single path segment (no separators, not "." or "..").
func isPlainName(name string) bool {
	if strings.TrimSpace(name) == "" || name == "." || name == ".." {
		return false
	}
	return !strings.ContainsAny(name, `/\:`)
}

// isSafeRelativePath reports whether p is a non-empty forward-slash relative path that does not
// escape its base with "..". Backslashes and drive letters are rejected.
func isSafeRelativePath(p string) bool {
	if strings.TrimSpace(p) == "" || strings.ContainsAny(p, `\:`) || strings.HasPrefix(p, "/") {
		return false
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == ".." {
			return false
		}
	}
	return path.Clean(p) != "."
}
