package cli

import (
	"agentsync/internal/rules"
	"path/filepath"
	"strings"
	"testing"
)

// Register explicitly until the parent wires the shared command registry.
func categoryParityCLI(t *testing.T) {
	t.Helper()
	old := commands
	commands = append(append([]command(nil), commands...), categoryCommands()...)
	t.Cleanup(func() { commands = old })
	newFixture(t)
}

func TestCategoryParitySave(t *testing.T) {
	categoryParityCLI(t)
	oldStdin := stdin
	t.Cleanup(func() { stdin = oldStdin })
	stdin = strings.NewReader(`{"category":{"name":"Work","color":"purple","icon":"star"},"scope":"local"}`)
	var saved rules.Category
	mustJSON(t, &saved, "categories", "save", "--input", "-")
	if saved.ID == "" || saved.Name != "Work" || saved.Scope != "local" || saved.Icon != "star" {
		t.Fatalf("saved: %+v", saved)
	}
	file := filepath.Join(t.TempDir(), "category.json")
	mustWrite(t, file, `{"category":{"id":"`+saved.ID+`","name":"Updated","color":"blue"},"scope":"local"}`)
	mustJSON(t, &saved, "categories", "save", "--input", file)
	var all []rules.Category
	mustJSON(t, &all, "categories", "list")
	if len(all) != 4 || all[3].Name != "Updated" || all[3].Icon != "star" {
		t.Fatalf("updated: %+v", all)
	}
	for _, invalid := range []string{
		`{"category":{"name":"X","color":"blue","typo":true},"scope":"local"}`,
		`{"category":{"name":"X","color":"blue"},"scope":"local","typo":true}`,
		`{"category":{"name":"X","color":"blue"},"scope":"local"} {}`,
		`null`,
		`{"category":{"name":"X","color":"blue"}}`,
		`{"category":{"name":"X","color":"invalid"},"scope":"local"}`,
	} {
		stdin = strings.NewReader(invalid)
		code, envelope := runJSON(t, nil, "categories", "save", "--input", "-")
		if code == ExitOK || envelope.OK {
			t.Fatalf("accepted %s", invalid)
		}
	}
}

func TestCategoryParityRemoveClearsAssignments(t *testing.T) {
	categoryParityCLI(t)
	if err := rules.SaveLocalRules(&rules.File{Version: rules.CurrentVersion, Categories: []rules.Category{{ID: "work", Name: "Work", Color: "blue"}}, Rules: []rules.Rule{{ID: "assigned", CategoryID: "work"}}}); err != nil {
		t.Fatal(err)
	}
	code, env := runJSON(t, nil, "categories", "remove", "work", "--scope", "local")
	if code != ExitError || env.Error == nil || env.Error.Code != "confirmation_required" {
		t.Fatalf("confirmation: %d %+v", code, env)
	}
	var all []rules.Category
	mustJSON(t, &all, "categories", "list")
	if len(all) != 4 {
		t.Fatalf("unconfirmed delete changed categories: %+v", all)
	}
	mustJSON(t, nil, "categories", "remove", "work", "--scope", "local", "--yes")
	file, err := rules.LoadLocalRules()
	if err != nil || len(file.Categories) != 0 || file.Rules[0].CategoryID != "uncategorized" {
		t.Fatalf("assignments: %+v %v", file, err)
	}
	code, _ = runJSON(t, nil, "categories", "remove", "skill", "--yes")
	if code == ExitOK {
		t.Fatal("scope must be explicit")
	}
}

func TestCategoryParityMigrateIsExplicit(t *testing.T) {
	categoryParityCLI(t)
	source := filepath.Join(t.TempDir(), "AGENTS.md")
	mustWrite(t, source, "# Instructions")
	if err := rules.SaveLocalRules(&rules.File{Version: rules.CurrentVersion, Rules: []rules.Rule{{ID: "legacy", Source: rules.Source{Origin: "local", Path: source}}}}); err != nil {
		t.Fatal(err)
	}
	mustJSON(t, nil, "categories", "list")
	before, err := rules.LoadLocalRules()
	if err != nil || before.Rules[0].CategoryID != "" {
		t.Fatalf("list migrated: %+v %v", before, err)
	}
	mustJSON(t, nil, "categories", "migrate")
	after, err := rules.LoadLocalRules()
	if err != nil || after.Rules[0].CategoryID != rules.KindInstructions {
		t.Fatalf("migration: %+v %v", after, err)
	}
	mustJSON(t, nil, "categories", "migrate")
}

func TestCategoryParitySuggest(t *testing.T) {
	categoryParityCLI(t)
	source := filepath.Join(t.TempDir(), "AGENTS.md")
	mustWrite(t, source, "# Instructions")
	var category string
	mustJSON(t, &category, "categories", "suggest", source, "--origin", "local")
	if category != rules.KindInstructions {
		t.Fatalf("suggestion: %q", category)
	}
	mustJSON(t, &category, "categories", "suggest", source, "--origin", "local", "--kind", "file")
	if category != "uncategorized" {
		t.Fatalf("explicit kind ignored: %q", category)
	}
	mustJSON(t, &category, "categories", "suggest", "AGENTS.md", "--origin", "subscription", "--subscription", "unavailable")
	if category != "uncategorized" {
		t.Fatalf("missing subscription: %q", category)
	}
	mustJSON(t, nil, "categories", "remove", "instructions", "--scope", "builtin", "--yes")
	mustJSON(t, &category, "categories", "suggest", source, "--origin", "local")
	if category != "uncategorized" {
		t.Fatalf("suggested deleted category: %q", category)
	}
}

func TestCategoryParityLocalizedHelp(t *testing.T) {
	categoryParityCLI(t)
	for _, lang := range []string{LangEN, LangES} {
		res := run("--lang", lang, "help", "categories")
		if res.code != ExitOK {
			t.Fatalf("help %s: %+v", lang, res)
		}
		for _, cmd := range categoryCommands() {
			description := messages[lang][cmd.desc]
			if description == "" || !strings.Contains(res.stdout, description) {
				t.Fatalf("missing localized description %s %s: %s", lang, cmd.path, res.stdout)
			}
		}
	}
}

func TestCategoryParityRepositoryLifecycle(t *testing.T) {
	categoryParityCLI(t)
	root := t.TempDir()
	mustJSON(t, nil, "root", "set", root)
	oldStdin := stdin
	t.Cleanup(func() { stdin = oldStdin })
	stdin = strings.NewReader(`{"category":{"id":"shared","name":"Shared","color":"green"},"scope":"repo"}`)
	var saved rules.Category
	mustJSON(t, &saved, "categories", "save", "--input", "-")
	if saved.ID != "shared" || saved.Scope != "repo" {
		t.Fatalf("repository save: %+v", saved)
	}
	repo, err := rules.LoadRepoRules(root)
	if err != nil {
		t.Fatal(err)
	}
	repo.Rules = []rules.Rule{{ID: "repo-rule", CategoryID: saved.ID}}
	if err := rules.SaveRepoRules(root, repo); err != nil {
		t.Fatal(err)
	}
	if err := rules.SaveLocalRules(&rules.File{Version: rules.CurrentVersion, Rules: []rules.Rule{{ID: "local-rule", CategoryID: saved.ID}}}); err != nil {
		t.Fatal(err)
	}
	mustJSON(t, nil, "categories", "remove", saved.ID, "--scope", "repo", "--yes")
	repo, err = rules.LoadRepoRules(root)
	if err != nil || len(repo.Categories) != 0 || repo.Rules[0].CategoryID != "uncategorized" {
		t.Fatalf("repo assignment: %+v %v", repo, err)
	}
	local, err := rules.LoadLocalRules()
	if err != nil || local.Rules[0].CategoryID != "uncategorized" {
		t.Fatalf("local assignment: %+v %v", local, err)
	}
}

func TestCategoryParityList(t *testing.T) {
	categoryParityCLI(t)
	var categories []rules.Category
	mustJSON(t, &categories, "categories", "list")
	if len(categories) != 3 {
		t.Fatalf("builtins: %+v", categories)
	}
}
