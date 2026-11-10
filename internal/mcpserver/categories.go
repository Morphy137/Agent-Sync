package mcpserver

import (
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"agentsync/internal/ops"
	"agentsync/internal/rules"
)

// CategoriesOutput lists categories without changing assignments.
type CategoriesOutput struct {
	Categories []rules.Category `json:"categories"`
}

// DeleteCategoryInput selects a category and explicitly confirms assignment removal.
type DeleteCategoryInput struct {
	ID      string `json:"id" jsonschema:"category id from list_categories"`
	Scope   string `json:"scope" jsonschema:"category storage scope: repo or local; builtin accepted for built-in deletion"`
	Confirm bool   `json:"confirm,omitempty" jsonschema:"must be true; ask the user before deleting the category and clearing assignments"`
}

// MigrateCategoriesInput confirms an explicit migration of legacy assignments.
type MigrateCategoriesInput struct {
	Confirm bool `json:"confirm,omitempty" jsonschema:"must be true; ask the user before assigning categories to legacy local rules"`
}

// SuggestCategoryInput identifies a source and optional explicit recognition kind.
type SuggestCategoryInput struct {
	Source SourceInput `json:"source" jsonschema:"source whose category should be suggested"`
	Kind   string      `json:"kind,omitempty" jsonschema:"optional explicit source kind; auto or omitted uses recognition"`
}

// SuggestCategoryOutput reports the suggested category, or uncategorized.
type SuggestCategoryOutput struct {
	CategoryID string `json:"categoryId"`
}

func (s *Server) registerCategoryTools() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "suggest_category",
		Description: "Suggest the available category for a repo, local or subscription source, optionally using an explicit kind. Does not modify rules or migrate assignments.",
		Annotations: readOnly,
	}, handler(func(in SuggestCategoryInput) (SuggestCategoryOutput, error) {
		id, err := s.api.SuggestRuleCategory(in.Source.source(), in.Kind)
		return SuggestCategoryOutput{CategoryID: id}, err
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "migrate_categories",
		Description: "Explicitly assign inferred categories to legacy local rules with no category. Does not change existing assignments. Requires confirm=true after asking the user.",
		Annotations: destructive,
	}, handler(func(in MigrateCategoriesInput) (Done, error) {
		if err := requireConfirm(in.Confirm); err != nil {
			return Done{}, err
		}
		if err := s.api.MigrateRuleCategories(); err != nil {
			return Done{}, err
		}
		return done("Rule categories migrated."), nil
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "delete_category",
		Description: "Delete a rule category and clear its assignments using the service's scope rules. Requires confirm=true after asking the user.",
		Annotations: destructive,
	}, handler(func(in DeleteCategoryInput) (Done, error) {
		if err := requireConfirm(in.Confirm); err != nil {
			return Done{}, err
		}
		if err := s.api.DeleteRuleCategory(in.ID, in.Scope); err != nil {
			return Done{}, err
		}
		return done("Category removed."), nil
	}))
	saveSchema, err := jsonschema.For[ops.SaveCategoryInput](nil)
	if err != nil {
		panic(err)
	} // Static, JSON-compatible input; mirrors SDK registration failures.
	// The service generates IDs and permits unnamed built-in overrides.
	saveSchema.Properties["category"].Required = []string{"color"}
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "save_category",
		InputSchema: saveSchema,
		Description: "Create or update a rule category from its full category object and explicit repo or local scope. Service validation and icon preservation apply; omit id to create.",
		Annotations: additive,
	}, handler(func(in ops.SaveCategoryInput) (rules.Category, error) {
		return s.api.SaveRuleCategory(in.Category, in.Scope)
	}))
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_categories",
		Description: "List built-in, repository and local rule categories. Does not migrate rule assignments.",
		Annotations: readOnly,
	}, handler(func(NoInput) (CategoriesOutput, error) {
		categories, err := s.api.GetRuleCategories()
		return CategoriesOutput{Categories: categories}, err
	}))
}
