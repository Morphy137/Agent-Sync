package rules

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"agentsync/internal/apperr"
)

// Suggested rules: a repository may ship a file listing rules its subscribers can apply in one step.
// The content comes from a third party, so everything is validated before it becomes a rule.
const (
	SuggestionsFileName     = "agentsync.suggestions.json"
	SuggestionsVersion      = 1
	MaxSuggestionsFileBytes = 256 * 1024
	MaxSuggestions          = 100
	MaxSuggestionDests      = 20
	MaxSuggestionNameLength = 120
	MaxSuggestionDescLength = 500
	suggestionIDLength      = 12
	gitDirName              = ".git"
)

// Reasons reported by suggestion validation (in addition to the rule validation reasons).
const (
	ReasonUnsupportedSuggestionMode = "unsupported_suggestion_mode"
	ReasonMissingSuggestionDest     = "missing_suggestion_destination"
	ReasonTooManySuggestionDests    = "too_many_suggestion_destinations"
	ReasonSuggestionTextTooLong     = "suggestion_text_too_long"
	ReasonSuggestionsTooLarge       = "suggestions_file_too_large"
	ReasonTooManySuggestions        = "too_many_suggestions"
)

// SuggestedDestination is one place a suggested rule writes to.
type SuggestedDestination struct {
	Location string `json:"location"`
	Path     string `json:"path"`
	Mode     string `json:"mode,omitempty"`
}

// Suggestion is one rule proposed by the owner of a repository. Its source lives inside that repository.
type Suggestion struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description,omitempty"`
	Source       string                 `json:"source"`
	Include      []string               `json:"include,omitempty"`
	Kind         string                 `json:"kind,omitempty"`
	Destinations []SuggestedDestination `json:"destinations"`
}

// SuggestionsFile is the on-disk format of agentsync.suggestions.json.
type SuggestionsFile struct {
	Version     int          `json:"version"`
	Suggestions []Suggestion `json:"suggestions"`
}

// ParseSuggestions decodes the suggestions file of a repository. Size and count are capped; the
// suggestions themselves are checked one by one with Suggestion.Validate so a single bad entry
// does not hide the valid ones.
func ParseSuggestions(data []byte) (*SuggestionsFile, error) {
	if len(data) > MaxSuggestionsFileBytes {
		return nil, apperr.New(apperr.InvalidSuggestions).WithParam("reason", ReasonSuggestionsTooLarge)
	}
	var file SuggestionsFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&file); err != nil {
		return nil, apperr.Wrap(apperr.InvalidSuggestions, err).WithParam("reason", ReasonMalformedFile)
	}
	if file.Version != SuggestionsVersion {
		return nil, apperr.New(apperr.InvalidSuggestions).WithParam("reason", ReasonUnsupportedVersion)
	}
	if len(file.Suggestions) > MaxSuggestions {
		return nil, apperr.New(apperr.InvalidSuggestions).WithParam("reason", ReasonTooManySuggestions)
	}
	if file.Suggestions == nil {
		file.Suggestions = []Suggestion{}
	}
	return &file, nil
}

// ID identifies a suggestion by its content, so a selection made before the file changed cannot
// apply a different suggestion.
func (s Suggestion) ID() string {
	encoded, _ := json.Marshal(s)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])[:suggestionIDLength]
}

// EffectiveMode is the mode a suggested destination gets: one_way unless the owner chose create_only.
func (d SuggestedDestination) EffectiveMode() string {
	if d.Mode == "" {
		return ModeOneWay
	}
	return d.Mode
}

// ToRule builds the local rule that applies the suggestion to a subscription. IDs are left empty:
// saving the rule generates them.
func (s Suggestion) ToRule(subscriptionID string) Rule {
	rule := Rule{
		Name:    strings.TrimSpace(s.Name),
		Enabled: true,
		Source:  Source{Origin: OriginSubscription, Subscription: subscriptionID, Path: s.Source, Include: s.Include},
		Kind:    s.Kind,
	}
	for _, d := range s.Destinations {
		rule.Destinations = append(rule.Destinations, Destination{
			Location: d.Location,
			Path:     d.Path,
			Mode:     d.EffectiveMode(),
			Enabled:  true,
		})
	}
	return rule
}

// Validate checks a suggestion coming from an untrusted repository. On top of rule validation it
// rejects paths that are not plain relative paths on every platform (drive letters, backslash
// roots, ".git" folders), two-way mode (subscription sources are read-only) and transforms
// (the format has none).
func (s Suggestion) Validate() error {
	if len(s.Name) > MaxSuggestionNameLength || len(s.Description) > MaxSuggestionDescLength {
		return invalid(ReasonSuggestionTextTooLong)
	}
	if len(s.Destinations) == 0 {
		return invalid(ReasonMissingSuggestionDest)
	}
	if len(s.Destinations) > MaxSuggestionDests {
		return invalid(ReasonTooManySuggestionDests)
	}
	if !isPlainRelPath(s.Source) {
		return invalid(ReasonInvalidSourcePath).WithParam("path", s.Source)
	}
	for _, entry := range s.Include {
		if !isPlainRelPath(entry) {
			return invalid(ReasonInvalidInclude).WithParam("path", entry)
		}
	}
	for _, d := range s.Destinations {
		if d.Mode != "" && d.Mode != ModeOneWay && d.Mode != ModeCreateOnly {
			return invalid(ReasonUnsupportedSuggestionMode).WithParam("mode", d.Mode)
		}
		if !isPlainRelPath(d.Path) {
			return invalid(ReasonInvalidDestPath).WithParam("path", d.Path)
		}
	}

	rule := s.ToRule("suggestion")
	rule.ID = "suggestion"
	for i := range rule.Destinations {
		rule.Destinations[i].ID = string(rune('a' + i))
	}
	return Validate(rule, ScopeLocal)
}

// isPlainRelPath rejects paths that are only relative on some platforms or point into git metadata:
// drive letters or other ":" forms, a leading slash or backslash, control characters and ".git" segments.
// Empty and "." are accepted here: rule validation decides where they make sense.
func isPlainRelPath(p string) bool {
	if strings.ContainsAny(p, ":\x00") || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
		return false
	}
	for _, r := range p {
		if r < ' ' {
			return false
		}
	}
	for _, segment := range strings.Split(strings.ReplaceAll(p, "\\", "/"), "/") {
		if strings.EqualFold(segment, gitDirName) {
			return false
		}
	}
	return true
}

// Normalized returns the suggestion with its paths cleaned (forward slashes, no "./" or doubled
// separators) and the name trimmed. Call it only after Validate succeeded.
func (s Suggestion) Normalized() Suggestion {
	clean := func(p string) string {
		cleaned, err := NormalizeRelPath(p)
		if err != nil {
			return p
		}
		return cleaned
	}
	out := s
	out.Name = strings.TrimSpace(s.Name)
	out.Source = clean(s.Source)
	out.Include = make([]string, 0, len(s.Include))
	for _, entry := range s.Include {
		out.Include = append(out.Include, clean(entry))
	}
	out.Destinations = make([]SuggestedDestination, 0, len(s.Destinations))
	for _, d := range s.Destinations {
		out.Destinations = append(out.Destinations, SuggestedDestination{Location: d.Location, Path: clean(d.Path), Mode: d.EffectiveMode()})
	}
	return out
}
