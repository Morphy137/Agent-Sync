package service_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"agentsync/internal/rules"
)

func TestCategoryMigrationAndDeletion(t *testing.T) {
	f := newRulesFixture(t)
	writeFile(t, filepath.Join(f.root, "skills", "example", "SKILL.md"), "# Example")
	skill := instructionsRule()
	skill.Source.Path = "skills"
	saved, err := f.svc.SaveRule(skill, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	manual := instructionsRule()
	manual.CategoryID = "uncategorized"
	manualSaved, err := f.svc.SaveRule(manual, rules.ScopeLocal)
	if err != nil {
		t.Fatal(err)
	}
	missing := instructionsRule()
	missing.Source.Path = "missing"
	missingSaved, err := f.svc.SaveRule(missing, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateRuleCategories(); err != nil {
		t.Fatal(err)
	}
	repo, err := rules.LoadRepoRules(f.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range repo.Rules {
		if rule.ID == saved.Rule.ID && rule.CategoryID != "skill" {
			t.Fatalf("skills folder category: %q", rule.CategoryID)
		}
		if rule.ID == missingSaved.Rule.ID && rule.CategoryID != "" {
			t.Fatalf("missing source category: %q", rule.CategoryID)
		}
	}
	local, err := rules.LoadLocalRules()
	if err != nil {
		t.Fatal(err)
	}
	if local.Rules[0].ID != manualSaved.Rule.ID || local.Rules[0].CategoryID != "uncategorized" {
		t.Fatal("manual choice changed")
	}
	before, err := os.ReadFile(filepath.Join(f.root, rules.RulesFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateRuleCategories(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(f.root, rules.RulesFileName))
	if err != nil || string(before) != string(after) {
		t.Fatal("migration was not idempotent")
	}
	category, err := f.svc.SaveRuleCategory(rules.Category{Name: "Custom", Color: "blue"}, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	skill = saved.Rule
	skill.CategoryID = category.ID
	if _, err := f.svc.SaveRule(skill, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteRuleCategory(category.ID, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	repo, err = rules.LoadRepoRules(f.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range repo.Rules {
		if rule.ID == skill.ID && rule.CategoryID != "uncategorized" {
			t.Fatalf("deleted category assignment: %q", rule.CategoryID)
		}
	}
}

func TestCategoryOverridesAndReadDoesNotMigrate(t *testing.T) {
	f := newRulesFixture(t)
	saved, err := f.svc.SaveRule(instructionsRule(), rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(f.root, rules.RulesFileName))
	if err != nil {
		t.Fatal(err)
	}
	report, err := f.svc.GetRulesReport()
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Rules) != 1 || report.Rules[0].Rule.CategoryID != "instructions" {
		t.Fatalf("inference: %+v", report.Rules)
	}
	after, err := os.ReadFile(filepath.Join(f.root, rules.RulesFileName))
	if err != nil || string(before) != string(after) {
		t.Fatal("report modified rules file")
	}
	if _, err := f.svc.SaveRuleCategory(rules.Category{ID: "skill", Name: "Tools", Color: "rose"}, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	categories, err := f.svc.GetRuleCategories()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, category := range categories {
		if category.ID == "skill" {
			count++
			if category.Name != "Tools" || category.Color != "rose" {
				t.Fatalf("override: %+v", category)
			}
		}
	}
	if count != 1 {
		t.Fatalf("duplicate built-in: %d", count)
	}
	// Updating an older caller that omits category must retain an existing assignment.
	if err := f.svc.MigrateRuleCategories(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SaveRule(saved.Rule, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	file, err := rules.LoadRepoRules(f.root)
	if err != nil || file.Rules[0].CategoryID != "instructions" {
		t.Fatalf("assignment lost: %+v, %v", file, err)
	}
}

func TestCategoryMigrationPreservesExtensionsAndRejectsLocalCategoryInRepo(t *testing.T) {
	f := newRulesFixture(t)
	path := filepath.Join(f.root, rules.RulesFileName)
	writeFile(t, path, `{"version":1,"custom":{"nested":[1,2]},"rules":[{"id":"r_test","name":"Test","enabled":true,"source":{"origin":"repo","path":"shared/AGENTS.md","customSource":true},"kind":"auto","destinations":[],"customRule":{"x":1}}]}`)
	if err := f.svc.MigrateRuleCategories(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	item := document["rules"].([]any)[0].(map[string]any)
	if document["custom"] == nil || item["customRule"] == nil || item["source"].(map[string]any)["customSource"] != true || item["categoryId"] != "instructions" {
		t.Fatalf("extensions lost: %s", data)
	}
	category, err := f.svc.SaveRuleCategory(rules.Category{Name: "Local", Color: "blue"}, rules.ScopeLocal)
	if err != nil {
		t.Fatal(err)
	}
	rule := instructionsRule()
	rule.CategoryID = category.ID
	if _, err := f.svc.SaveRule(rule, rules.ScopeRepo); err == nil {
		t.Fatal("repo rule accepted local-only category")
	}
	if _, err := f.svc.SaveRule(rule, rules.ScopeLocal); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteBuiltinCategoryStaysDeleted(t *testing.T) {
	f := newRulesFixture(t)
	category := rules.Category{ID: "instructions", Name: "Local override", Color: "rose"}
	if _, err := f.svc.SaveRuleCategory(category, rules.ScopeLocal); err != nil {
		t.Fatal(err)
	}
	rule := instructionsRule()
	rule.CategoryID = "instructions"
	saved, err := f.svc.SaveRule(rule, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteRuleCategory("instructions", "builtin"); err != nil {
		t.Fatal(err)
	}
	categories, err := f.svc.GetRuleCategories()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range categories {
		if c.ID == "instructions" {
			t.Fatal("local override restored deleted built-in")
		}
	}
	repo, err := rules.LoadRepoRules(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Rules[0].CategoryID != "uncategorized" || len(repo.Categories) != 1 || !repo.Categories[0].Deleted {
		t.Fatalf("deletion not persisted: %+v", repo)
	}
	fresh := instructionsRule()
	fresh.ID = "new-rule"
	if _, err := f.svc.SaveRule(fresh, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.MigrateRuleCategories(); err != nil {
		t.Fatal(err)
	}
	repo, err = rules.LoadRepoRules(f.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range repo.Rules {
		if r.CategoryID != "uncategorized" {
			t.Fatalf("rule %s reclassified: %q", r.ID, r.CategoryID)
		}
	}
	rule = saved.Rule
	rule.CategoryID = "instructions"
	result, err := f.svc.SaveRule(rule, rules.ScopeRepo)
	if err != nil || result.Rule.CategoryID != "uncategorized" {
		t.Fatalf("deleted category accepted: %+v %v", result, err)
	}
}

func TestSuggestRuleCategoryUsesMigrationDetection(t *testing.T) {
	f := newRulesFixture(t)
	writeFile(t, filepath.Join(f.root, "skills", "example", "SKILL.md"), "# Example")
	source := rules.Source{Origin: rules.OriginRepo, Path: "skills"}
	category, err := f.svc.SuggestRuleCategory(source, rules.KindAuto)
	if err != nil || category != "skill" {
		t.Fatalf("skills container: %q, %v", category, err)
	}
	if err := f.svc.DeleteRuleCategory("skill", "builtin"); err != nil {
		t.Fatal(err)
	}
	category, err = f.svc.SuggestRuleCategory(source, rules.KindAuto)
	if err != nil || category != "uncategorized" {
		t.Fatalf("deleted category: %q, %v", category, err)
	}
}

func TestCategoryInferenceRecognizesAgentInstructionPairs(t *testing.T) {
	f := newRulesFixture(t)
	writeFile(t, filepath.Join(f.root, "shared", "CODEX-AGENTS.md"), "# Instructions")
	rule := instructionsRule()
	rule.Source.Path = "shared/CODEX-AGENTS.md"
	rule.Kind = rules.KindAuto
	rule.Destinations = []rules.Destination{{Location: "claude", Path: "AGENTS.md", Enabled: true}}
	saved, err := f.svc.SaveRule(rule, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	report, err := f.svc.GetRulesReport()
	if err != nil || report.Rules[0].Rule.CategoryID != "instructions" {
		t.Fatalf("recognized pair: %+v, %v", report, err)
	}
	rule = saved.Rule
	rule.Destinations[0].Path = "other.md"
	if _, err := f.svc.SaveRule(rule, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	report, err = f.svc.GetRulesReport()
	if err != nil || report.Rules[0].Rule.CategoryID != "uncategorized" {
		t.Fatalf("unrecognized destination: %+v, %v", report, err)
	}
}

func TestBuiltinCategoryDefaultColors(t *testing.T) {
	f := newRulesFixture(t)
	categories, err := f.svc.GetRuleCategories()
	if err != nil {
		t.Fatal(err)
	}
	colors := map[string]string{"skill": "amber", "mcp": "blue", "instructions": "green"}
	if len(categories) != len(colors) {
		t.Fatalf("unexpected builtin categories: %+v", categories)
	}
	for _, category := range categories {
		if category.Color != colors[category.ID] {
			t.Fatalf("unexpected default color: %+v", category)
		}
	}
}

func TestCategoryIconsPersistAndDefault(t *testing.T) {
	f := newRulesFixture(t)
	categories, err := f.svc.GetRuleCategories()
	if err != nil {
		t.Fatal(err)
	}
	defaults := map[string]string{"skill": "puzzle", "mcp": "terminal", "instructions": "book"}
	for _, category := range categories {
		if category.Icon != defaults[category.ID] {
			t.Fatalf("default icon: %+v", category)
		}
	}
	category, err := f.svc.SaveRuleCategory(rules.Category{Name: "Custom", Color: "blue", Icon: "shield"}, rules.ScopeRepo)
	if err != nil {
		t.Fatal(err)
	}
	category.Name = "Renamed"
	category.Icon = ""
	category, err = f.svc.SaveRuleCategory(category, rules.ScopeRepo)
	if err != nil || category.Icon != "shield" {
		t.Fatalf("icon lost on edit: %+v, %v", category, err)
	}
	category.Icon = "invalid"
	if _, err := f.svc.SaveRuleCategory(category, rules.ScopeRepo); err == nil {
		t.Fatal("invalid icon accepted")
	}
	if _, err := f.svc.SaveRuleCategory(rules.Category{ID: "skill", Color: "purple", Icon: "star"}, rules.ScopeRepo); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteRuleCategory("skill", "builtin"); err != nil {
		t.Fatal(err)
	}
	repo, err := rules.LoadRepoRules(f.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, stored := range repo.Categories {
		if stored.ID == "skill" && (!stored.Deleted || stored.Icon != "star") {
			t.Fatalf("icon lost on deletion: %+v", stored)
		}
	}
	// Files written before icons existed get a stable fallback on read.
	repo.Categories = append(repo.Categories, rules.Category{ID: "legacy", Name: "Legacy", Color: "gray"})
	if err := rules.SaveRepoRules(f.root, repo); err != nil {
		t.Fatal(err)
	}
	categories, err = f.svc.GetRuleCategories()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range categories {
		if c.ID == "legacy" {
			found = true
			if c.Icon != "folder" {
				t.Fatalf("legacy icon: %q", c.Icon)
			}
		}
	}
	if !found {
		t.Fatal("legacy category missing")
	}
}
