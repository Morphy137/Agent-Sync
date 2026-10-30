package recognize

import (
	"strings"

	"agentsync/internal/rules"
)

// namePlaceholder is replaced by SourceInfo.Name in placement paths.
const namePlaceholder = "{name}"

// Suggestion is a proposed destination for a source inside a location.
type Suggestion struct {
	ProfileID string           `json:"profileId"` // "" when generic
	Label     string           `json:"label"`
	Location  string           `json:"location"` // alias to use ("home" when base=home, otherwise the given alias)
	Path      string           `json:"path"`     // relative to Location, forward slashes
	Mode      string           `json:"mode"`
	Transform *rules.Transform `json:"transform,omitempty"`
	Detected  bool             `json:"detected"` // false => generic fallback
}

// Suggest proposes the placement of src into the location alias whose folder is locationAbs.
// When the folder matches a profile with a placement for src.Kind, that placement is used;
// otherwise the generic fallback copies the source under its own base name (two-way).
func (s *Set) Suggest(src SourceInfo, alias, locationAbs string) Suggestion {
	fallback := genericSuggestion(src, alias)

	folder, ok := s.DetectFolder(locationAbs)
	if !ok {
		return fallback
	}
	profile, ok := s.profileByID(folder.ProfileID)
	if !ok {
		return fallback
	}
	placement, ok := profile.Placements[src.Kind]
	if !ok {
		return fallback
	}

	name := src.Name
	if !isPlainName(name) {
		name = fallback.Path
	}
	suggestion := Suggestion{
		ProfileID: profile.ID,
		Label:     profile.Label,
		Location:  alias,
		Path:      strings.ReplaceAll(placement.Path, namePlaceholder, name),
		Mode:      rules.ModeTwoWay,
		Detected:  true,
	}
	if placement.Base == BaseHome {
		suggestion.Location = rules.BuiltinHomeAlias
	}
	if placement.Transform != nil {
		transform := *placement.Transform
		suggestion.Transform = &transform
		suggestion.Mode = rules.ModeOneWay
	}
	return suggestion
}

func genericSuggestion(src SourceInfo, alias string) Suggestion {
	path := src.BaseName
	if !isPlainName(path) {
		path = src.Name
	}
	return Suggestion{
		Location: alias,
		Path:     path,
		Mode:     rules.ModeTwoWay,
	}
}

func (s *Set) profileByID(id string) (Profile, bool) {
	for _, profile := range s.Profiles {
		if profile.ID == id && !profile.Disabled {
			return profile, true
		}
	}
	return Profile{}, false
}
