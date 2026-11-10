package ops

import "agentsync/internal/rules"

// SaveCategoryInput carries the complete category and an explicit storage scope.
type SaveCategoryInput struct {
	Category rules.Category `json:"category" jsonschema:"full category; omit id to create, preserve id to update"`
	Scope    string         `json:"scope" jsonschema:"storage scope: repo or local; required explicitly"`
}

// SaveRuleCategory delegates validation and persistence to the service.
func (a *API) SaveRuleCategory(category rules.Category, scope string) (rules.Category, error) {
	return a.svc.SaveRuleCategory(category, scope)
}

// DeleteRuleCategory removes a category and its assignments using service behavior.
func (a *API) DeleteRuleCategory(id, scope string) error {
	return a.svc.DeleteRuleCategory(id, scope)
}

// MigrateRuleCategories explicitly assigns categories to legacy local rules.
func (a *API) MigrateRuleCategories() error {
	return a.svc.MigrateRuleCategories()
}

// SuggestRuleCategory suggests a category without modifying assignments.
func (a *API) SuggestRuleCategory(source rules.Source, kind string) (string, error) {
	return a.svc.SuggestRuleCategory(withDefaultOrigin(source), kind)
}

// GetRuleCategories lists categories without migrating rule assignments.
func (a *API) GetRuleCategories() ([]rules.Category, error) {
	return a.svc.GetRuleCategories()
}
