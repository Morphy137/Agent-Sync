package ops

import (
	"strings"

	"agentsync/internal/apperr"
	"agentsync/internal/rules"
	"agentsync/internal/service"
)

// ListSubscriptionSuggestions lists the rules a subscription's owner suggests, resolved for this PC.
func (a *API) ListSubscriptionSuggestions(subscriptionID string) ([]service.SuggestionInfo, error) {
	list, err := a.svc.ListSubscriptionSuggestions(subscriptionID)
	return nonNil(list), err
}

// ApplySubscriptionSuggestions creates local rules for the chosen suggestions. Nothing is applied
// unless the caller names suggestion ids or asks for all of them explicitly; "all" means every
// suggestion that is valid and not applied yet. locationMappings maps unmapped aliases to folders.
func (a *API) ApplySubscriptionSuggestions(subscriptionID string, suggestionIDs []string, all bool, locationMappings map[string]string) (*service.SuggestionsApplyOutcome, error) {
	ids := suggestionIDs
	if all {
		list, err := a.svc.ListSubscriptionSuggestions(subscriptionID)
		if err != nil {
			return nil, err
		}
		for _, info := range list {
			if info.Valid && !info.Applied {
				ids = append(ids, info.ID)
			}
		}
	}
	if len(ids) == 0 && !all {
		return nil, apperr.New(apperr.InvalidArgument).WithParam("field", "suggestionIds")
	}
	cleaned := make(map[string]string, len(locationMappings))
	for alias, folder := range locationMappings {
		cleaned[strings.TrimSpace(alias)] = folder
	}
	return a.svc.ApplySubscriptionSuggestions(subscriptionID, ids, cleaned)
}

// ExportSubscriptionSuggestions builds the contents of agentsync.suggestions.json from the rules
// that read from a subscription.
func (a *API) ExportSubscriptionSuggestions(subscriptionID string) (*rules.SuggestionsFile, error) {
	return a.svc.ExportSubscriptionSuggestions(subscriptionID)
}
