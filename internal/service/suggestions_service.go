package service

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/recognize"
	"agentsync/internal/rules"
)

// Reasons added by the service on top of the rule validation reasons.
const (
	reasonSourceOutsideRepo = "source_outside_repository"
	reasonSourceMissing     = "source_not_found"
	reasonInvalidLocation   = "invalid_location"
	reasonUnknownSuggestion = "unknown_suggestion"
	paramReason             = "reason"
	paramAlias              = "alias"
)

// Per-suggestion outcomes of ApplySubscriptionSuggestions.
const (
	SuggestionApplied = "applied"
	SuggestionSkipped = "skipped"
	SuggestionFailed  = "failed"
)

// SuggestionDestinationInfo is one destination of a suggestion as seen on this PC.
type SuggestionDestinationInfo struct {
	Location string `json:"location"`
	Path     string `json:"path"`
	Mode     string `json:"mode"`
	// Mapped is true when the location alias points to a folder on this PC.
	Mapped bool `json:"mapped"`
	// TargetPath is the absolute path the content would be written to; empty while unmapped.
	TargetPath   string `json:"targetPath"`
	TargetExists bool   `json:"targetExists"`
	// DetectedPath is a local folder recognized as this location, offered while it is unmapped.
	DetectedPath string `json:"detectedPath"`
	// Applied is true when a rule already writes this source to this location and path.
	Applied bool `json:"applied"`
}

// SuggestionInfo describes one suggested rule of a subscription.
type SuggestionInfo struct {
	ID           string                      `json:"id"`
	Name         string                      `json:"name"`
	Description  string                      `json:"description"`
	Source       string                      `json:"source"`
	Include      []string                    `json:"include"`
	Kind         string                      `json:"kind"`
	SourceExists bool                        `json:"sourceExists"`
	Destinations []SuggestionDestinationInfo `json:"destinations"`
	// Applied is true when every destination is already covered by an existing rule.
	Applied bool `json:"applied"`
	// Valid is false when the suggestion cannot be applied; InvalidReason says why.
	Valid         bool   `json:"valid"`
	InvalidReason string `json:"invalidReason"`
}

// SuggestionApplyResult reports what happened to one selected suggestion.
type SuggestionApplyResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	RuleID string `json:"ruleId"`
	// ErrorCode and ErrorReason describe a failure (apperr code and its "reason" parameter).
	ErrorCode   string `json:"errorCode"`
	ErrorReason string `json:"errorReason"`
	// ErrorAlias names the location that needs a folder when ErrorCode is location_unmapped.
	ErrorAlias string `json:"errorAlias"`
	Collisions int    `json:"collisions"`
}

// SuggestionsApplyOutcome lists one result per requested suggestion.
type SuggestionsApplyOutcome struct {
	Results []SuggestionApplyResult `json:"results"`
	Applied int                     `json:"applied"`
}

func errorReason(err error) string {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr.Params[paramReason]
	}
	return ""
}

func errorCode(err error) string {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return apperr.Unknown
}

// loadSuggestionsLocked reads the suggestions file of a subscription clone. A missing file is an
// empty list; anything that is not a small regular file is refused.
func loadSuggestionsLocked(subscriptionID string) (*rules.SuggestionsFile, error) {
	path := filepath.Join(subscriptionsRoot(subscriptionID), rules.SuggestionsFileName)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return &rules.SuggestionsFile{Version: rules.SuggestionsVersion, Suggestions: []rules.Suggestion{}}, nil
	}
	if err != nil {
		return nil, apperr.Wrap(apperr.InvalidSuggestions, err).WithParam(paramReason, rules.ReasonMalformedFile)
	}
	if !info.Mode().IsRegular() {
		return nil, apperr.New(apperr.InvalidSuggestions).WithParam(paramReason, rules.ReasonMalformedFile)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, apperr.Wrap(apperr.InvalidSuggestions, err).WithParam(paramReason, rules.ReasonMalformedFile)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, rules.MaxSuggestionsFileBytes+1))
	if err != nil {
		return nil, apperr.Wrap(apperr.InvalidSuggestions, err).WithParam(paramReason, rules.ReasonMalformedFile)
	}
	return rules.ParseSuggestions(data)
}

// suggestionSourceLocked resolves a validated suggestion source inside the clone. A source that is a
// symlink (or sits behind one) leading outside the clone is refused; a missing one is reported.
func suggestionSourceLocked(subscriptionID, source string) (string, error) {
	root, err := filepath.EvalSymlinks(subscriptionsRoot(subscriptionID))
	if err != nil {
		return "", apperr.Wrap(apperr.SourceNotFound, err)
	}
	abs := filepath.Join(root, filepath.FromSlash(source))
	resolved, err := filepath.EvalSymlinks(abs)
	if os.IsNotExist(err) {
		return "", apperr.New(apperr.SourceNotFound).WithParam(paramReason, reasonSourceMissing)
	}
	if err != nil {
		return "", apperr.Wrap(apperr.SourceNotFound, err).WithParam(paramReason, reasonSourceMissing)
	}
	if !config.IsSubdirectory(resolved, root) {
		return "", apperr.New(apperr.InvalidSuggestions).WithParam(paramReason, reasonSourceOutsideRepo)
	}
	return resolved, nil
}

// validateSuggestionShape checks everything that can be known without touching the disk.
func validateSuggestionShape(raw rules.Suggestion) error {
	if err := raw.Validate(); err != nil {
		return err
	}
	for _, d := range raw.Destinations {
		if len(d.Location) > maxAliasLength || !aliasPattern.MatchString(d.Location) {
			return apperr.New(apperr.InvalidSuggestions).WithParam(paramReason, reasonInvalidLocation)
		}
	}
	return nil
}

// validateSuggestionLocked adds the source check (it must exist and stay inside the clone).
func validateSuggestionLocked(subscriptionID string, raw rules.Suggestion) error {
	if err := validateSuggestionShape(raw); err != nil {
		return err
	}
	_, err := suggestionSourceLocked(subscriptionID, raw.Normalized().Source)
	return err
}

// isSuggestedDestApplied reports whether an existing rule of the subscription already writes the
// suggested source to the same location and path.
func isSuggestedDestApplied(existing []rules.Rule, subscriptionID, source string, dest rules.SuggestedDestination) bool {
	for _, rule := range existing {
		if rule.Source.Origin != rules.OriginSubscription || rule.Source.Subscription != subscriptionID {
			continue
		}
		if !sameRelPath(rule.Source.Path, source) {
			continue
		}
		for _, d := range rule.Destinations {
			if d.Location == dest.Location && sameRelPath(d.Path, dest.Path) {
				return true
			}
		}
	}
	return false
}

func sameRelPath(a, b string) bool {
	left, errLeft := rules.NormalizeRelPath(a)
	right, errRight := rules.NormalizeRelPath(b)
	return errLeft == nil && errRight == nil && left == right
}

// detectLocationFolder offers a folder of the home directory that the recognizers identify as the
// given location (for example ~/.claude for "claude"). The user still has to confirm it.
func detectLocationFolder(set *recognize.Set, homeDir, alias string) string {
	if homeDir == "" {
		return ""
	}
	for _, name := range []string{"." + alias, alias} {
		candidate := filepath.Join(homeDir, name)
		if _, ok := set.DetectFolder(candidate); ok {
			return candidate
		}
	}
	return ""
}

// describeSuggestionLocked resolves one suggestion of the file against this PC.
func (s *Service) describeSuggestionLocked(subscriptionID string, raw rules.Suggestion, env rules.Env, existing []rules.Rule, set *recognize.Set) SuggestionInfo {
	info := SuggestionInfo{
		ID:           raw.ID(),
		Name:         strings.TrimSpace(raw.Name),
		Description:  raw.Description,
		Source:       raw.Source,
		Include:      append([]string{}, raw.Include...),
		Kind:         raw.Kind,
		Destinations: make([]SuggestionDestinationInfo, 0, len(raw.Destinations)),
	}
	err := validateSuggestionShape(raw)
	if err == nil {
		_, err = suggestionSourceLocked(subscriptionID, raw.Normalized().Source)
		info.SourceExists = err == nil
	}
	info.Valid = err == nil
	if err != nil {
		info.InvalidReason = errorReason(err)
		for _, d := range raw.Destinations {
			info.Destinations = append(info.Destinations, SuggestionDestinationInfo{Location: d.Location, Path: d.Path, Mode: d.EffectiveMode()})
		}
		return info
	}

	suggestion := raw.Normalized()
	info.Source = suggestion.Source
	info.Include = suggestion.Include
	info.SourceExists = true
	info.Applied = true
	for _, d := range suggestion.Destinations {
		dest := SuggestionDestinationInfo{
			Location: d.Location,
			Path:     d.Path,
			Mode:     d.Mode,
			Applied:  isSuggestedDestApplied(existing, subscriptionID, suggestion.Source, d),
		}
		if root, ok := rules.LocationRoot(env, d.Location); ok {
			dest.Mapped = true
			dest.TargetPath = filepath.Clean(filepath.Join(root, filepath.FromSlash(d.Path)))
			_, statErr := os.Lstat(dest.TargetPath)
			dest.TargetExists = statErr == nil
		} else {
			dest.DetectedPath = detectLocationFolder(set, env.HomeDir, d.Location)
		}
		info.Applied = info.Applied && dest.Applied
		info.Destinations = append(info.Destinations, dest)
	}
	return info
}

// ListSubscriptionSuggestions returns the rules the repository owner suggests, resolved for this PC.
// Nothing is created: applying is a separate, explicit step.
func (s *Service) ListSubscriptionSuggestions(subscriptionID string) ([]SuggestionInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hasSubscriptionLocked(subscriptionID) {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", subscriptionID)
	}
	file, err := loadSuggestionsLocked(subscriptionID)
	if err != nil {
		return nil, err
	}
	existing, err := s.allRulesLocked()
	if err != nil {
		return nil, err
	}
	env := s.rulesEnvLocked()
	set := s.recognizersLocked()
	result := make([]SuggestionInfo, 0, len(file.Suggestions))
	for _, raw := range file.Suggestions {
		result = append(result, s.describeSuggestionLocked(subscriptionID, raw, env, existing, set))
	}
	return result, nil
}

// pendingSuggestionsLocked counts the suggestions that could be applied now. Problems reading the
// file count as none: the list is shown by ListSubscriptionSuggestions, which reports them.
func (s *Service) pendingSuggestionsLocked(subscriptionID string, existing []rules.Rule) int {
	file, err := loadSuggestionsLocked(subscriptionID)
	if err != nil {
		return 0
	}
	pending := 0
	for _, raw := range file.Suggestions {
		if validateSuggestionLocked(subscriptionID, raw) != nil {
			continue
		}
		suggestion := raw.Normalized()
		for _, d := range suggestion.Destinations {
			if !isSuggestedDestApplied(existing, subscriptionID, suggestion.Source, d) {
				pending++
				break
			}
		}
	}
	return pending
}

// ApplySubscriptionSuggestions creates a local rule for each selected suggestion, with the same
// validation and collision handling as any rule saved by hand. Destinations already covered by a
// rule are skipped. A location that is not mapped on this PC needs an entry in locationMappings
// (alias -> folder), which is saved like any other location; otherwise the suggestion fails.
// Rules are only created: files are written when the user runs Sync.
func (s *Service) ApplySubscriptionSuggestions(subscriptionID string, ids []string, locationMappings map[string]string) (*SuggestionsApplyOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hasSubscriptionLocked(subscriptionID) {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", subscriptionID)
	}
	file, err := loadSuggestionsLocked(subscriptionID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]rules.Suggestion, len(file.Suggestions))
	for _, raw := range file.Suggestions {
		if _, seen := byID[raw.ID()]; !seen {
			byID[raw.ID()] = raw
		}
	}

	outcome := &SuggestionsApplyOutcome{Results: make([]SuggestionApplyResult, 0, len(ids))}
	handled := make(map[string]bool, len(ids))
	for _, id := range ids {
		if handled[id] {
			continue
		}
		handled[id] = true
		result := SuggestionApplyResult{ID: id}
		raw, ok := byID[id]
		if !ok {
			result.Status, result.ErrorCode, result.ErrorReason = SuggestionFailed, apperr.InvalidSuggestions, reasonUnknownSuggestion
		} else {
			result = s.applySuggestionLocked(subscriptionID, id, raw, locationMappings)
		}
		if result.Status == SuggestionApplied {
			outcome.Applied++
		}
		outcome.Results = append(outcome.Results, result)
	}
	s.emitRulesChangedLocked()
	return outcome, nil
}

func failedSuggestion(id string, err error) SuggestionApplyResult {
	result := SuggestionApplyResult{ID: id, Status: SuggestionFailed, ErrorCode: errorCode(err), ErrorReason: errorReason(err)}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		result.ErrorAlias = appErr.Params[paramAlias]
	}
	return result
}

// applySuggestionLocked applies one suggestion; the file and rules are re-read each time so earlier
// suggestions of the same call count as existing rules.
func (s *Service) applySuggestionLocked(subscriptionID, id string, raw rules.Suggestion, mappings map[string]string) SuggestionApplyResult {
	if err := validateSuggestionLocked(subscriptionID, raw); err != nil {
		return failedSuggestion(id, err)
	}
	existing, err := s.allRulesLocked()
	if err != nil {
		return failedSuggestion(id, err)
	}
	suggestion := raw.Normalized()
	missing := make([]rules.SuggestedDestination, 0, len(suggestion.Destinations))
	for _, d := range suggestion.Destinations {
		if !isSuggestedDestApplied(existing, subscriptionID, suggestion.Source, d) {
			missing = append(missing, d)
		}
	}
	if len(missing) == 0 {
		return SuggestionApplyResult{ID: id, Status: SuggestionSkipped}
	}

	if err := s.mapSuggestionLocationsLocked(missing, mappings); err != nil {
		return failedSuggestion(id, err)
	}
	suggestion.Destinations = missing
	saved, err := s.saveRuleLocked(suggestion.ToRule(subscriptionID), rules.ScopeLocal)
	if err != nil {
		return failedSuggestion(id, err)
	}
	return SuggestionApplyResult{ID: id, Status: SuggestionApplied, RuleID: saved.Rule.ID, Collisions: len(saved.Collisions)}
}

// mapSuggestionLocationsLocked makes sure every destination location is mapped on this PC, saving the
// caller's mapping for the ones that are not. Existing mappings are never overwritten here.
func (s *Service) mapSuggestionLocationsLocked(dests []rules.SuggestedDestination, mappings map[string]string) error {
	env := s.rulesEnvLocked()
	for _, d := range dests {
		if _, mapped := rules.LocationRoot(env, d.Location); mapped {
			continue
		}
		folder := strings.TrimSpace(mappings[d.Location])
		if folder == "" {
			return apperr.New(apperr.LocationUnmapped).WithParam(paramAlias, d.Location)
		}
		if err := s.saveLocationLocked(d.Location, folder); err != nil {
			var appErr *apperr.Error
			if errors.As(err, &appErr) {
				appErr.WithParam(paramAlias, d.Location)
			}
			return err
		}
		env = s.rulesEnvLocked()
	}
	return nil
}

// ExportSubscriptionSuggestions builds an agentsync.suggestions.json from the rules that read from a
// subscription, so a repository owner can author the file from rules set up by hand. Destinations
// with a transform cannot be expressed and are left out, and two-way becomes one-way (subscription
// sources are read-only).
func (s *Service) ExportSubscriptionSuggestions(subscriptionID string) (*rules.SuggestionsFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hasSubscriptionLocked(subscriptionID) {
		return nil, apperr.New(apperr.SubscriptionNotFound).WithParam("subscription", subscriptionID)
	}
	all, err := s.allRulesLocked()
	if err != nil {
		return nil, err
	}
	file := &rules.SuggestionsFile{Version: rules.SuggestionsVersion, Suggestions: []rules.Suggestion{}}
	for _, rule := range all {
		if rule.Source.Origin != rules.OriginSubscription || rule.Source.Subscription != subscriptionID {
			continue
		}
		if suggestion, ok := suggestionFromRule(rule); ok {
			file.Suggestions = append(file.Suggestions, suggestion)
		}
	}
	return file, nil
}

func suggestionFromRule(rule rules.Rule) (rules.Suggestion, bool) {
	suggestion := rules.Suggestion{
		Name:    rule.Name,
		Source:  rule.Source.Path,
		Include: rule.Source.Include,
	}
	if kind := rule.EffectiveKind(); kind != rules.KindAuto {
		suggestion.Kind = kind
	}
	for _, d := range rule.Destinations {
		if d.Transform != nil {
			continue
		}
		mode := rules.ModeOneWay
		if d.EffectiveMode() == rules.ModeCreateOnly {
			mode = rules.ModeCreateOnly
		}
		suggestion.Destinations = append(suggestion.Destinations, rules.SuggestedDestination{Location: d.Location, Path: d.Path, Mode: mode})
	}
	return suggestion, len(suggestion.Destinations) > 0
}
