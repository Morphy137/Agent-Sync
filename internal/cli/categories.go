package cli

import (
	"encoding/json"
	"io"
	"os"

	"agentsync/internal/ops"
	"agentsync/internal/rules"
)

func init() {
	messages[LangEN]["cmd.categories_suggest"] = "Suggest a category for a source without changing rule assignments."
	messages[LangES]["cmd.categories_suggest"] = "Sugiere una categoría para un origen sin cambiar las asignaciones de reglas."
	messages[LangEN]["cmd.categories_migrate"] = "Assign categories to legacy local rules explicitly."
	messages[LangES]["cmd.categories_migrate"] = "Asigna categorías explícitamente a las reglas locales antiguas."
	messages[LangEN]["done.categories_migrated"] = "Rule categories migrated."
	messages[LangES]["done.categories_migrated"] = "Categorías de reglas migradas."
	messages[LangEN]["cmd.categories_list"] = "List built-in, repository and local rule categories."
	messages[LangES]["cmd.categories_list"] = "Lista las categorías de reglas integradas, del repositorio y locales."
	messages[LangEN]["cmd.categories_remove"] = "Remove a category and clear its rule assignments (requires --yes)."
	messages[LangES]["cmd.categories_remove"] = "Elimina una categoría y sus asignaciones de reglas (requiere --yes)."
	messages[LangEN]["done.category_removed"] = "Category removed."
	messages[LangES]["done.category_removed"] = "Categoría eliminada."
	messages[LangEN]["cmd.categories_save"] = "Create or update a category from JSON with an explicit scope."
	messages[LangES]["cmd.categories_save"] = "Crea o actualiza una categoría desde JSON con un ámbito explícito."
}

func categoryCommands() []command {
	return []command{
		{path: "categories suggest", usage: "<path> [--origin repo|local|subscription] [--subscription ID] [--kind K]", desc: "cmd.categories_suggest", run: cmdCategoriesSuggest},
		{path: "categories migrate", desc: "cmd.categories_migrate", run: cmdCategoriesMigrate},
		{path: "categories remove", usage: "<id> --scope repo|local|builtin --yes", desc: "cmd.categories_remove", run: cmdCategoriesRemove},
		{path: "categories list", desc: "cmd.categories_list", run: cmdCategoriesList},
		{path: "categories save", usage: "--input FILE|-", desc: "cmd.categories_save", run: cmdCategoriesSave},
	}
}

func cmdCategoriesSuggest(e *env, args []string) (any, error) {
	fs := newFlags("categories suggest")
	origin := fs.String("origin", "", "")
	subscription := fs.String("subscription", "", "")
	kind := fs.String("kind", "", "")
	pos, err := expectArgs(fs, args, "<path>")
	if err != nil {
		return nil, err
	}
	return e.API().SuggestRuleCategory(rules.Source{Origin: *origin, Subscription: *subscription, Path: pos[0]}, *kind)
}

func cmdCategoriesMigrate(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("categories migrate"), args); err != nil {
		return nil, err
	}
	if err := e.API().MigrateRuleCategories(); err != nil {
		return nil, err
	}
	return okMessage("done.categories_migrated")
}

func cmdCategoriesRemove(e *env, args []string) (any, error) {
	fs := newFlags("categories remove")
	scope := fs.String("scope", "", "")
	yes := fs.Bool("yes", false, "")
	pos, err := expectArgs(fs, args, "<id>")
	if err != nil {
		return nil, err
	}
	if err := requireYes("categories remove", *yes); err != nil {
		return nil, err
	}
	if err := e.API().DeleteRuleCategory(pos[0], *scope); err != nil {
		return nil, err
	}
	return okMessage("done.category_removed")
}

func cmdCategoriesSave(e *env, args []string) (any, error) {
	fs := newFlags("categories save")
	input := fs.String("input", "", "")
	if _, err := expectArgs(fs, args); err != nil {
		return nil, err
	}
	if *input == "" {
		return nil, usagef("--input is required")
	}
	reader := stdin
	if *input != stdinPath {
		file, err := os.Open(*input)
		if err != nil {
			return nil, usagef("--input: %v", err)
		}
		defer file.Close()
		reader = file
	}
	var in ops.SaveCategoryInput
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		return nil, usagef("--input: invalid JSON: %v", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, usagef("--input: expected exactly one JSON object")
	}
	return e.API().SaveRuleCategory(in.Category, in.Scope)
}

func cmdCategoriesList(e *env, args []string) (any, error) {
	if _, err := expectArgs(newFlags("categories list"), args); err != nil {
		return nil, err
	}
	return e.API().GetRuleCategories()
}
