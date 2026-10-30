package rules

import (
	"path"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
)

const (
	currentDir = "."
	parentDir  = ".."
)

// EffectiveMode returns the mode actually applied to a destination: a transform forces
// one-way semantics and an empty mode defaults to two-way.
func (d Destination) EffectiveMode() string {
	if d.Transform != nil {
		return ModeOneWay
	}
	if d.Mode == "" {
		return ModeTwoWay
	}
	return d.Mode
}

// EffectiveKind returns the forced kind or KindAuto when empty.
func (r Rule) EffectiveKind() string {
	if r.Kind == "" {
		return KindAuto
	}
	return r.Kind
}

// HasWritableSource reports whether captures may write back into the rule source.
func (r Rule) HasWritableSource() bool {
	return r.Source.Origin == OriginRepo || r.Source.Origin == OriginLocal
}

// NormalizeRelPath cleans a relative path to forward slashes and rejects absolute paths and
// paths escaping their root with "..". The empty string and "." normalize to ".".
func NormalizeRelPath(p string) (string, error) {
	slashed := strings.ReplaceAll(p, "\\", "/")
	if filepath.IsAbs(p) || filepath.VolumeName(p) != "" || strings.HasPrefix(slashed, "/") {
		return "", apperr.New(apperr.InvalidRule).WithParam("reason", ReasonInvalidRelPath).WithParam("path", p)
	}
	cleaned := path.Clean(slashed)
	if cleaned == parentDir || strings.HasPrefix(cleaned, parentDir+"/") {
		return "", apperr.New(apperr.InvalidRule).WithParam("reason", ReasonInvalidRelPath).WithParam("path", p)
	}
	return cleaned, nil
}

// hasParentSegment reports whether any path segment is "..".
func hasParentSegment(p string) bool {
	for _, seg := range strings.Split(strings.ReplaceAll(p, "\\", "/"), "/") {
		if seg == parentDir {
			return true
		}
	}
	return false
}

func invalid(reason string) *apperr.Error {
	return apperr.New(apperr.InvalidRule).WithParam("reason", reason)
}

func isKnownKind(kind string) bool {
	switch kind {
	case "", KindAuto, KindFile, KindFolder, KindSkill, KindMCP, KindInstructions:
		return true
	}
	return false
}

func isKnownMode(mode string) bool {
	switch mode {
	case "", ModeTwoWay, ModeOneWay, ModeCreateOnly:
		return true
	}
	return false
}

func isKnownTransform(t string) bool {
	return t == TransformJSONMerge || t == TransformTOMLMerge
}

// isFolderKind reports whether a forced kind can only describe a folder source.
func isFolderKind(kind string) bool {
	return kind == KindFolder || kind == KindSkill
}

// Validate checks a single rule. It returns apperr InvalidRule with param "reason"
// (and "destId" for destination-level problems). An empty Kind is treated as KindAuto
// and an empty Mode as ModeTwoWay.
func Validate(r Rule, scope string) error {
	if strings.TrimSpace(r.ID) == "" {
		return invalid(ReasonMissingID)
	}
	if strings.TrimSpace(r.Name) == "" {
		return invalid(ReasonMissingName)
	}
	if scope != ScopeRepo && scope != ScopeLocal {
		return invalid(ReasonInvalidScope).WithParam("scope", scope)
	}
	if err := validateSource(r.Source, scope); err != nil {
		return err
	}
	if !isKnownKind(r.Kind) {
		return invalid(ReasonInvalidKind).WithParam("kind", r.Kind)
	}
	for _, pattern := range r.Exclude {
		if pattern == "" {
			return invalid(ReasonInvalidExclude).WithParam("pattern", pattern)
		}
		if _, err := path.Match(pattern, ""); err != nil {
			return invalid(ReasonInvalidExclude).WithParam("pattern", pattern)
		}
	}

	seen := make(map[string]bool, len(r.Destinations))
	for _, d := range r.Destinations {
		if strings.TrimSpace(d.ID) == "" {
			return invalid(ReasonMissingDestinationID)
		}
		if seen[d.ID] {
			return invalid(ReasonDuplicateDestination).WithParam("destId", d.ID)
		}
		seen[d.ID] = true
		if err := validateDestination(r, d); err != nil {
			return err
		}
	}
	return nil
}

func validateSource(src Source, scope string) error {
	switch src.Origin {
	case OriginRepo, OriginSubscription:
		if src.Origin == OriginSubscription && strings.TrimSpace(src.Subscription) == "" {
			return invalid(ReasonMissingSubscription)
		}
		if !isSafeSourceRelPath(src.Path) {
			return invalid(ReasonInvalidSourcePath).WithParam("path", src.Path)
		}
	case OriginLocal:
		if scope != ScopeLocal {
			return invalid(ReasonLocalOriginInRepo)
		}
		if src.Path == "" || !filepath.IsAbs(src.Path) {
			return invalid(ReasonInvalidSourcePath).WithParam("path", src.Path)
		}
	default:
		return invalid(ReasonInvalidOrigin).WithParam("origin", src.Origin)
	}
	return nil
}

// isSafeSourceRelPath accepts non-empty relative paths without ".." that do not point at the root itself.
func isSafeSourceRelPath(p string) bool {
	if strings.TrimSpace(p) == "" || hasParentSegment(p) {
		return false
	}
	cleaned, err := NormalizeRelPath(p)
	if err != nil {
		return false
	}
	return cleaned != currentDir
}

func validateDestination(r Rule, d Destination) error {
	if strings.TrimSpace(d.Location) == "" {
		return invalid(ReasonMissingLocation).WithParam("destId", d.ID)
	}
	if _, err := NormalizeRelPath(d.Path); err != nil {
		return invalid(ReasonInvalidDestPath).WithParam("destId", d.ID).WithParam("path", d.Path)
	}
	if !isKnownMode(d.Mode) {
		return invalid(ReasonInvalidMode).WithParam("destId", d.ID).WithParam("mode", d.Mode)
	}
	if d.Transform != nil {
		if !isKnownTransform(d.Transform.Type) || strings.TrimSpace(d.Transform.Key) == "" {
			return invalid(ReasonInvalidTransform).WithParam("destId", d.ID)
		}
		if isFolderKind(r.Kind) {
			return invalid(ReasonTransformNeedsFile).WithParam("destId", d.ID)
		}
	}
	for _, ig := range d.Ignore {
		cleaned, err := NormalizeRelPath(ig)
		if err != nil || cleaned == currentDir {
			return invalid(ReasonInvalidIgnore).WithParam("destId", d.ID).WithParam("path", ig)
		}
	}
	return nil
}
