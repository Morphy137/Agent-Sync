package recognize_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/config"
	"agentsync/internal/recognize"
	"agentsync/internal/rules"
)

const testFilePerm = 0o644
const testDirPerm = 0o755

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), testDirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), testFilePerm); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, testDirPerm); err != nil {
		t.Fatal(err)
	}
}

func useConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(config.ConfigEnvVar, dir)
	return dir
}

func mustDefaults(t *testing.T) *recognize.Set {
	t.Helper()
	set, err := recognize.Defaults()
	if err != nil {
		t.Fatalf("Defaults: %v", err)
	}
	return set
}

func findProfile(set *recognize.Set, id string) (recognize.Profile, bool) {
	for _, p := range set.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return recognize.Profile{}, false
}

func findKind(set *recognize.Set, id string) (recognize.SourceKind, bool) {
	for _, k := range set.SourceKinds {
		if k.ID == id {
			return k, true
		}
	}
	return recognize.SourceKind{}, false
}

func requireCode(t *testing.T, err error, code string) *apperr.Error {
	t.Helper()
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperr.Error with code %s, got %v", code, err)
	}
	if appErr.Code != code {
		t.Fatalf("expected code %s, got %s (%v)", code, appErr.Code, appErr)
	}
	return appErr
}

func TestDefaultsAreWellFormed(t *testing.T) {
	set := mustDefaults(t)

	if set.Version != recognize.SchemaVersion {
		t.Fatalf("version = %d, want %d", set.Version, recognize.SchemaVersion)
	}

	wantKinds := map[string]string{
		"skill":        rules.KindSkill,
		"mcp-json":     rules.KindMCP,
		"instructions": rules.KindInstructions,
	}
	for id, kind := range wantKinds {
		got, ok := findKind(set, id)
		if !ok {
			t.Fatalf("missing source kind %q", id)
		}
		if got.Kind != kind || got.Disabled {
			t.Errorf("source kind %q = %+v, want kind %q enabled", id, got, kind)
		}
		if got.Match.Type != recognize.MatchTypeDir && got.Match.Type != recognize.MatchTypeFile {
			t.Errorf("source kind %q has match type %q", id, got.Match.Type)
		}
	}

	wantProfiles := []string{"claude-code", "codex-cli", "gemini-cli", "opencode", "antigravity"}
	for _, id := range wantProfiles {
		if _, ok := findProfile(set, id); !ok {
			t.Errorf("missing profile %q", id)
		}
	}

	knownKinds := map[string]bool{
		rules.KindFile: true, rules.KindFolder: true, rules.KindSkill: true,
		rules.KindMCP: true, rules.KindInstructions: true,
	}
	for _, profile := range set.Profiles {
		if profile.Label == "" || profile.Disabled {
			t.Errorf("profile %q: empty label or disabled", profile.ID)
		}
		if len(profile.Detect.FolderNames) == 0 && len(profile.Detect.AnyOf) == 0 {
			t.Errorf("profile %q: empty detect", profile.ID)
		}
		for _, cond := range profile.Detect.AnyOf {
			if (cond.File == "") == (cond.Dir == "") {
				t.Errorf("profile %q: condition %+v must set exactly one of file/dir", profile.ID, cond)
			}
		}
		if len(profile.Placements) == 0 {
			t.Errorf("profile %q: no placements", profile.ID)
		}
		for kind, placement := range profile.Placements {
			if !knownKinds[kind] {
				t.Errorf("profile %q: unknown placement kind %q", profile.ID, kind)
			}
			if placement.Path == "" || strings.Contains(placement.Path, "..") || strings.Contains(placement.Path, `\`) || filepath.IsAbs(placement.Path) {
				t.Errorf("profile %q: bad placement path %q", profile.ID, placement.Path)
			}
			if placement.Base != "" && placement.Base != recognize.BaseHome {
				t.Errorf("profile %q: bad base %q", profile.ID, placement.Base)
			}
			if kind == rules.KindSkill && !strings.HasSuffix(placement.Path, "/{name}") {
				t.Errorf("profile %q: skill placement %q should end with /{name}", profile.ID, placement.Path)
			}
			if placement.Transform != nil {
				if placement.Transform.Type != rules.TransformJSONMerge && placement.Transform.Type != rules.TransformTOMLMerge {
					t.Errorf("profile %q: bad transform %+v", profile.ID, placement.Transform)
				}
				if placement.Transform.Key == "" {
					t.Errorf("profile %q: transform without key", profile.ID)
				}
			}
		}
	}

	// MCP formats that are not Claude-compatible must not get an mcp placement.
	for _, id := range []string{"opencode", "antigravity"} {
		profile, _ := findProfile(set, id)
		if _, ok := profile.Placements[rules.KindMCP]; ok {
			t.Errorf("profile %q must not define an mcp placement", id)
		}
	}
}

func TestDefaultsReturnsIndependentCopies(t *testing.T) {
	first := mustDefaults(t)
	first.Profiles[0].Label = "mutated"
	second := mustDefaults(t)
	if second.Profiles[0].Label == "mutated" {
		t.Fatal("Defaults must return a fresh copy on each call")
	}
}

func TestLoadWithoutUserFileReturnsDefaults(t *testing.T) {
	useConfigDir(t)
	set, err := recognize.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defaults := mustDefaults(t)
	if len(set.Profiles) != len(defaults.Profiles) || len(set.SourceKinds) != len(defaults.SourceKinds) {
		t.Fatalf("Load without user file should equal defaults")
	}
}

func TestLoadMergesUserFile(t *testing.T) {
	dir := useConfigDir(t)
	writeFile(t, filepath.Join(dir, recognize.UserFileName), `{
  "version": 1,
  "sourceKinds": [
    { "id": "instructions", "disabled": true },
    { "id": "prompt", "kind": "instructions", "match": { "type": "file", "fileNames": ["PROMPT.md"] } },
    { "id": "does-not-exist", "disabled": true }
  ],
  "profiles": [
    { "id": "claude-code", "label": "My Claude", "detect": { "folderNames": ["claude-work"] },
      "placements": { "instructions": { "path": "MINE.md" } } },
    { "id": "opencode", "disabled": true },
    { "id": "my-agent", "label": "My Agent", "detect": { "anyOf": [ { "file": "agent.yaml" } ] },
      "placements": { "skill": { "path": "skills/{name}" } } }
  ]
}`)

	set, err := recognize.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	defaults := mustDefaults(t)

	if _, ok := findKind(set, "instructions"); ok {
		t.Error("disabled source kind 'instructions' should be removed")
	}
	if _, ok := findKind(set, "prompt"); !ok {
		t.Error("new source kind 'prompt' should be appended")
	}
	if len(set.SourceKinds) != len(defaults.SourceKinds) {
		t.Errorf("source kinds = %d, want %d (one removed, one added)", len(set.SourceKinds), len(defaults.SourceKinds))
	}

	claude, ok := findProfile(set, "claude-code")
	if !ok || claude.Label != "My Claude" || claude.Placements[rules.KindInstructions].Path != "MINE.md" {
		t.Errorf("claude-code should be replaced, got %+v", claude)
	}
	if _, ok := claude.Placements[rules.KindSkill]; ok {
		t.Error("replacement is whole-entry: default skill placement must be gone")
	}
	if set.Profiles[0].ID != "claude-code" {
		t.Error("replaced entry should keep its position")
	}
	if _, ok := findProfile(set, "opencode"); ok {
		t.Error("disabled profile 'opencode' should be removed")
	}
	last := set.Profiles[len(set.Profiles)-1]
	if last.ID != "my-agent" {
		t.Errorf("new profile should be appended last, got %q", last.ID)
	}

	// Defaults are untouched by the merge.
	if again := mustDefaults(t); len(again.Profiles) != len(defaults.Profiles) {
		t.Error("merge must not mutate defaults")
	}
}

func TestLoadInvalidUserFile(t *testing.T) {
	cases := map[string]string{
		"malformed json":    `{"version": 1, "profiles": [`,
		"unknown field":     `{"version": 1, "profilez": []}`,
		"wrong version":     `{"version": 2}`,
		"missing version":   `{"profiles": []}`,
		"missing id":        `{"version": 1, "profiles": [{"label": "x", "detect": {"folderNames": ["x"]}}]}`,
		"duplicate id":      `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {"folderNames": ["a"]}}, {"id": "a", "label": "a", "detect": {"folderNames": ["a"]}}]}`,
		"missing label":     `{"version": 1, "profiles": [{"id": "a", "detect": {"folderNames": ["a"]}}]}`,
		"empty detect":      `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {}}]}`,
		"bad condition":     `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {"anyOf": [{"file": "a", "dir": "b"}]}}]}`,
		"unknown base":      `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {"folderNames": ["a"]}, "placements": {"skill": {"base": "root", "path": "x"}}}]}`,
		"escaping path":     `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {"folderNames": ["a"]}, "placements": {"skill": {"path": "../x"}}}]}`,
		"absolute path":     `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {"folderNames": ["a"]}, "placements": {"skill": {"path": "/x"}}}]}`,
		"unknown placement": `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {"folderNames": ["a"]}, "placements": {"widget": {"path": "x"}}}]}`,
		"bad transform":     `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {"folderNames": ["a"]}, "placements": {"mcp": {"path": "x", "transform": {"type": "yaml_merge", "key": "k"}}}}]}`,
		"transform no key":  `{"version": 1, "profiles": [{"id": "a", "label": "a", "detect": {"folderNames": ["a"]}, "placements": {"mcp": {"path": "x", "transform": {"type": "json_merge"}}}}]}`,
		"unknown kind":      `{"version": 1, "sourceKinds": [{"id": "a", "kind": "widget", "match": {"type": "file"}}]}`,
		"unknown match":     `{"version": 1, "sourceKinds": [{"id": "a", "kind": "mcp", "match": {"type": "symlink"}}]}`,
		"jsonKey on dir":    `{"version": 1, "sourceKinds": [{"id": "a", "kind": "mcp", "match": {"type": "dir", "jsonKey": "k"}}]}`,
		"bad extension":     `{"version": 1, "sourceKinds": [{"id": "a", "kind": "mcp", "match": {"type": "file", "extensions": ["json"]}}]}`,
		"frontmatter alone": `{"version": 1, "sourceKinds": [{"id": "a", "kind": "skill", "match": {"type": "dir", "frontmatterKeys": ["name"]}}]}`,
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			dir := useConfigDir(t)
			userPath := filepath.Join(dir, recognize.UserFileName)
			writeFile(t, userPath, content)

			set, err := recognize.Load()
			if set != nil {
				t.Errorf("expected nil set on error")
			}
			appErr := requireCode(t, err, apperr.RecognizersInvalid)
			if appErr.Params["path"] != userPath {
				t.Errorf("path param = %q, want %q", appErr.Params["path"], userPath)
			}
		})
	}
}

func TestUserFilePath(t *testing.T) {
	dir := useConfigDir(t)
	if got, want := recognize.UserFilePath(), filepath.Join(dir, recognize.UserFileName); got != want {
		t.Fatalf("UserFilePath = %q, want %q", got, want)
	}
}

func TestWriteUserTemplate(t *testing.T) {
	dir := useConfigDir(t)
	nested := filepath.Join(dir, "nested")
	t.Setenv(config.ConfigEnvVar, nested)

	path, err := recognize.WriteUserTemplate()
	if err != nil {
		t.Fatalf("WriteUserTemplate: %v", err)
	}
	if path != filepath.Join(nested, recognize.UserFileName) {
		t.Fatalf("unexpected path %q", path)
	}
	set, err := recognize.Load()
	if err != nil {
		t.Fatalf("Load after template: %v", err)
	}
	if len(set.Profiles) != len(mustDefaults(t).Profiles) {
		t.Error("template should contain the defaults")
	}

	const custom = `{"version": 1}`
	writeFile(t, path, custom)
	if _, err := recognize.WriteUserTemplate(); err != nil {
		t.Fatalf("second WriteUserTemplate: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != custom {
		t.Error("WriteUserTemplate must not overwrite an existing file")
	}
}

func TestDetectSource(t *testing.T) {
	set := mustDefaults(t)
	root := t.TempDir()

	skillDir := filepath.Join(root, "pdf-tools")
	writeFile(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: pdf-helper\ndescription: \"Work with PDFs\"\n---\n# Body\n")

	unnamedSkill := filepath.Join(root, "reviewer")
	writeFile(t, filepath.Join(unnamedSkill, "SKILL.md"), "---\nname: ../escape\ndescription: Reviews code\n---\n")

	noDescription := filepath.Join(root, "half-skill")
	writeFile(t, filepath.Join(noDescription, "SKILL.md"), "---\nname: half\n---\n")

	mcpFile := filepath.Join(root, "servers.json")
	writeFile(t, mcpFile, `{"mcpServers": {"fs": {"command": "npx", "args": ["x"]}}}`)

	nestedKey := filepath.Join(root, "nested.json")
	writeFile(t, nestedKey, `{"other": {"mcpServers": {}}}`)

	brokenJSON := filepath.Join(root, "broken.json")
	writeFile(t, brokenJSON, `{"mcpServers": `)

	wrongExt := filepath.Join(root, "servers.txt")
	writeFile(t, wrongExt, `{"mcpServers": {}}`)

	instructions := filepath.Join(root, "CLAUDE.md")
	writeFile(t, instructions, "# rules")

	lowerInstructions := filepath.Join(root, "sub", "agents.md")
	writeFile(t, lowerInstructions, "# rules")

	plainFile := filepath.Join(root, "notes.txt")
	writeFile(t, plainFile, "hello")

	plainFolder := filepath.Join(root, "docs")
	mkdir(t, plainFolder)

	cases := []struct {
		name string
		path string
		want recognize.SourceInfo
	}{
		{"skill with frontmatter name", skillDir, recognize.SourceInfo{Kind: rules.KindSkill, KindID: "skill", Name: "pdf-helper", IsDir: true, BaseName: "pdf-tools"}},
		{"skill with unsafe name falls back to folder", unnamedSkill, recognize.SourceInfo{Kind: rules.KindSkill, KindID: "skill", Name: "reviewer", IsDir: true, BaseName: "reviewer"}},
		{"skill missing description is a folder", noDescription, recognize.SourceInfo{Kind: rules.KindFolder, Name: "half-skill", IsDir: true, BaseName: "half-skill"}},
		{"mcp json", mcpFile, recognize.SourceInfo{Kind: rules.KindMCP, KindID: "mcp-json", Name: "servers", BaseName: "servers.json"}},
		{"nested key is not top-level", nestedKey, recognize.SourceInfo{Kind: rules.KindFile, Name: "nested", BaseName: "nested.json"}},
		{"broken json", brokenJSON, recognize.SourceInfo{Kind: rules.KindFile, Name: "broken", BaseName: "broken.json"}},
		{"wrong extension", wrongExt, recognize.SourceInfo{Kind: rules.KindFile, Name: "servers", BaseName: "servers.txt"}},
		{"instructions", instructions, recognize.SourceInfo{Kind: rules.KindInstructions, KindID: "instructions", Name: "CLAUDE", BaseName: "CLAUDE.md"}},
		{"instructions case-insensitive", lowerInstructions, recognize.SourceInfo{Kind: rules.KindInstructions, KindID: "instructions", Name: "agents", BaseName: "agents.md"}},
		{"generic file", plainFile, recognize.SourceInfo{Kind: rules.KindFile, Name: "notes", BaseName: "notes.txt"}},
		{"generic folder", plainFolder, recognize.SourceInfo{Kind: rules.KindFolder, Name: "docs", IsDir: true, BaseName: "docs"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := set.DetectSource(tc.path)
			if err != nil {
				t.Fatalf("DetectSource: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}

	_, err := set.DetectSource(filepath.Join(root, "missing"))
	requireCode(t, err, apperr.SourceNotFound)
}

func TestDetectSourceSkipsDisabledKinds(t *testing.T) {
	set := mustDefaults(t)
	for i := range set.SourceKinds {
		set.SourceKinds[i].Disabled = true
	}
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	writeFile(t, path, "x")
	got, err := set.DetectSource(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != rules.KindFile || got.KindID != "" {
		t.Errorf("disabled kinds must not match, got %+v", got)
	}
}

func TestDetectFolder(t *testing.T) {
	set := mustDefaults(t)
	root := t.TempDir()

	claude := filepath.Join(root, ".claude")
	writeFile(t, filepath.Join(claude, "settings.json"), "{}")
	mkdir(t, filepath.Join(claude, "skills"))

	upperClaude := filepath.Join(root, "upper", ".CLAUDE")
	mkdir(t, upperClaude)

	geminiLike := filepath.Join(root, "gem")
	writeFile(t, filepath.Join(geminiLike, "GEMINI.md"), "x")
	writeFile(t, filepath.Join(geminiLike, "settings.json"), "{}")

	tie := filepath.Join(root, "tie")
	writeFile(t, filepath.Join(tie, "settings.json"), "{}")

	nameBeatsMarker := filepath.Join(root, "x", ".codex")
	writeFile(t, filepath.Join(nameBeatsMarker, "settings.json"), "{}")

	fileNotDir := filepath.Join(root, "filecase")
	writeFile(t, filepath.Join(fileNotDir, "skills"), "not a dir")

	empty := filepath.Join(root, "empty")
	mkdir(t, empty)

	cases := []struct {
		name   string
		folder string
		want   recognize.FolderProfile
		ok     bool
	}{
		{"folder name plus markers", claude, recognize.FolderProfile{ProfileID: "claude-code", Label: "Claude Code", Score: 4}, true},
		{"folder name case-insensitive", upperClaude, recognize.FolderProfile{ProfileID: "claude-code", Label: "Claude Code", Score: 2}, true},
		{"more markers win", geminiLike, recognize.FolderProfile{ProfileID: "gemini-cli", Label: "Gemini CLI", Score: 2}, true},
		{"tie keeps first profile", tie, recognize.FolderProfile{ProfileID: "claude-code", Label: "Claude Code", Score: 1}, true},
		{"folder name outweighs one marker", nameBeatsMarker, recognize.FolderProfile{ProfileID: "codex-cli", Label: "Codex CLI", Score: 2}, true},
		{"dir condition needs a directory", fileNotDir, recognize.FolderProfile{}, false},
		{"no match", empty, recognize.FolderProfile{}, false},
		{"missing folder", filepath.Join(root, "missing"), recognize.FolderProfile{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := set.DetectFolder(tc.folder)
			if ok != tc.ok || got != tc.want {
				t.Errorf("got %+v/%v, want %+v/%v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestSuggest(t *testing.T) {
	set := mustDefaults(t)
	root := t.TempDir()

	claude := filepath.Join(root, ".claude")
	mkdir(t, claude)
	codex := filepath.Join(root, ".codex")
	mkdir(t, codex)
	opencode := filepath.Join(root, "opencode")
	mkdir(t, opencode)
	plain := filepath.Join(root, "plain")
	mkdir(t, plain)

	skill := recognize.SourceInfo{Kind: rules.KindSkill, KindID: "skill", Name: "pdf-helper", IsDir: true, BaseName: "pdf-tools"}
	mcp := recognize.SourceInfo{Kind: rules.KindMCP, KindID: "mcp-json", Name: "servers", BaseName: "servers.json"}
	instructions := recognize.SourceInfo{Kind: rules.KindInstructions, KindID: "instructions", Name: "AGENTS", BaseName: "AGENTS.md"}
	folder := recognize.SourceInfo{Kind: rules.KindFolder, Name: "docs", IsDir: true, BaseName: "docs"}

	cases := []struct {
		name     string
		src      recognize.SourceInfo
		location string
		want     recognize.Suggestion
	}{
		{"skill into claude", skill, claude, recognize.Suggestion{ProfileID: "claude-code", Label: "Claude Code", Location: "work", Path: "skills/pdf-helper", Mode: rules.ModeTwoWay, Detected: true}},
		{"instructions into claude", instructions, claude, recognize.Suggestion{ProfileID: "claude-code", Label: "Claude Code", Location: "work", Path: "CLAUDE.md", Mode: rules.ModeTwoWay, Detected: true}},
		{"mcp into claude uses home", mcp, claude, recognize.Suggestion{ProfileID: "claude-code", Label: "Claude Code", Location: rules.BuiltinHomeAlias, Path: ".claude.json", Mode: rules.ModeOneWay, Transform: &rules.Transform{Type: rules.TransformJSONMerge, Key: "mcpServers"}, Detected: true}},
		{"mcp into codex", mcp, codex, recognize.Suggestion{ProfileID: "codex-cli", Label: "Codex CLI", Location: "work", Path: "config.toml", Mode: rules.ModeOneWay, Transform: &rules.Transform{Type: rules.TransformTOMLMerge, Key: "mcp_servers"}, Detected: true}},
		{"skill into codex uses home", skill, codex, recognize.Suggestion{ProfileID: "codex-cli", Label: "Codex CLI", Location: rules.BuiltinHomeAlias, Path: ".agents/skills/pdf-helper", Mode: rules.ModeTwoWay, Detected: true}},
		{"profile without placement falls back", mcp, opencode, recognize.Suggestion{Location: "work", Path: "servers.json", Mode: rules.ModeTwoWay}},
		{"unknown folder falls back", folder, plain, recognize.Suggestion{Location: "work", Path: "docs", Mode: rules.ModeTwoWay}},
		{"missing folder falls back", skill, filepath.Join(root, "missing"), recognize.Suggestion{Location: "work", Path: "pdf-tools", Mode: rules.ModeTwoWay}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := set.Suggest(tc.src, "work", tc.location)
			if !suggestionEqual(got, tc.want) {
				t.Errorf("got %+v (transform %+v), want %+v (transform %+v)", got, got.Transform, tc.want, tc.want.Transform)
			}
		})
	}
}

func TestSuggestDoesNotAliasSetTransform(t *testing.T) {
	set := mustDefaults(t)
	claude := filepath.Join(t.TempDir(), ".claude")
	mkdir(t, claude)
	mcp := recognize.SourceInfo{Kind: rules.KindMCP, Name: "servers", BaseName: "servers.json"}

	first := set.Suggest(mcp, "work", claude)
	first.Transform.Key = "mutated"
	second := set.Suggest(mcp, "work", claude)
	if second.Transform.Key != "mcpServers" {
		t.Fatal("Suggest must copy the placement transform")
	}
}

func TestSuggestUnsafeNameUsesBaseName(t *testing.T) {
	set := mustDefaults(t)
	claude := filepath.Join(t.TempDir(), ".claude")
	mkdir(t, claude)
	src := recognize.SourceInfo{Kind: rules.KindSkill, Name: "../evil", IsDir: true, BaseName: "good"}
	got := set.Suggest(src, "work", claude)
	if got.Path != "skills/good" {
		t.Fatalf("path = %q, want skills/good", got.Path)
	}
}

func suggestionEqual(a, b recognize.Suggestion) bool {
	if (a.Transform == nil) != (b.Transform == nil) {
		return false
	}
	if a.Transform != nil && *a.Transform != *b.Transform {
		return false
	}
	a.Transform, b.Transform = nil, nil
	return a == b
}
