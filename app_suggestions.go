package main

import "agentsync/internal/service"

// ---- Suggested rules ----

// ListSubscriptionSuggestions returns the rules a subscription's owner suggests, resolved for this PC.
func (a *App) ListSubscriptionSuggestions(subscriptionID string) ([]service.SuggestionInfo, error) {
	return a.svc.ListSubscriptionSuggestions(subscriptionID)
}

// ApplySubscriptionSuggestions creates local rules for the chosen suggestions. locationMappings gives a
// folder for each location alias not mapped on this PC yet.
func (a *App) ApplySubscriptionSuggestions(subscriptionID string, suggestionIDs []string, locationMappings map[string]string) (*service.SuggestionsApplyOutcome, error) {
	return a.svc.ApplySubscriptionSuggestions(subscriptionID, suggestionIDs, locationMappings)
}
