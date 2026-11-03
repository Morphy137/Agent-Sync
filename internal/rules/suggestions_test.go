package rules_test

import (
	"strings"
	"testing"

	"agentsync/internal/apperr"
	"agentsync/internal/rules"
)

const validSuggestionsJSON = `{ "version": 1, "suggestions": [
  { "name": "Claude instructions", "description": "Main file", "source": "AGENTS/Claude/CLAUDE-AGENTS.md",
    "include": [], "kind": "auto",
    "destinations": [ { "location": "claude", "path": "CLAUDE.md", "mode": "one_way" } ] }
] }`

func validSuggestion() rules.Suggestion {
	return rules.Suggestion{
		Name:         "Claude instructions",
		Source:       "AGENTS/Claude/CLAUDE-AGENTS.md",
		Destinations: []rules.SuggestedDestination{{Location: "claude", Path: "CLAUDE.md", Mode: rules.ModeOneWay}},
	}
}

func TestParseSuggestionsReadsDocumentedFormat(t *testing.T) {
	file, err := rules.ParseSuggestions([]byte(validSuggestionsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Suggestions) != 1 {
		t.Fatalf("suggestions = %d", len(file.Suggestions))
	}
	got := file.Suggestions[0]
	if got.Name != "Claude instructions" || got.Source != "AGENTS/Claude/CLAUDE-AGENTS.md" || got.Destinations[0].Location != "claude" {
		t.Fatalf("unexpected suggestion: %+v", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("valid suggestion rejected: %v", err)
	}
}

func TestParseSuggestionsRejectsBadFiles(t *testing.T) {
	tooMany := `{"version":1,"suggestions":[` + strings.Repeat(`{"name":"a","source":"a","destinations":[]},`, rules.MaxSuggestions) + `{"name":"a","source":"a","destinations":[]}]}`
	cases := []struct {
		name, data, reason string
	}{
		{"malformed", `{"version":`, rules.ReasonMalformedFile},
		{"not an object", `[]`, rules.ReasonMalformedFile},
		{"wrong version", `{"version":2,"suggestions":[]}`, rules.ReasonUnsupportedVersion},
		{"missing version", `{"suggestions":[]}`, rules.ReasonUnsupportedVersion},
		{"too many", tooMany, rules.ReasonTooManySuggestions},
		{"too large", `{"version":1,"suggestions":[],"pad":"` + strings.Repeat("x", rules.MaxSuggestionsFileBytes) + `"}`, rules.ReasonSuggestionsTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := rules.ParseSuggestions([]byte(tc.data))
			if !apperr.HasCode(err, apperr.InvalidSuggestions) {
				t.Fatalf("want invalid_suggestions, got %v", err)
			}
			var appErr *apperr.Error
			if !asAppErr(err, &appErr) || appErr.Params["reason"] != tc.reason {
				t.Fatalf("reason = %v, want %s", err, tc.reason)
			}
		})
	}
}

func asAppErr(err error, target **apperr.Error) bool {
	appErr, ok := err.(*apperr.Error)
	*target = appErr
	return ok
}

func TestParseSuggestionsMissingListIsEmpty(t *testing.T) {
	file, err := rules.ParseSuggestions([]byte(`{"version":1}`))
	if err != nil || file.Suggestions == nil || len(file.Suggestions) != 0 {
		t.Fatalf("got %+v, %v", file, err)
	}
}

func TestSuggestionValidateRejectsMaliciousInput(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*rules.Suggestion)
	}{
		{"source dotdot", func(s *rules.Suggestion) { s.Source = "../secrets.md" }},
		{"source nested dotdot", func(s *rules.Suggestion) { s.Source = "a/../../b.md" }},
		{"source backslash dotdot", func(s *rules.Suggestion) { s.Source = `a\..\..\b.md` }},
		{"source absolute posix", func(s *rules.Suggestion) { s.Source = "/etc/passwd" }},
		{"source absolute windows", func(s *rules.Suggestion) { s.Source = `C:\Windows\win.ini` }},
		{"source drive relative", func(s *rules.Suggestion) { s.Source = "C:win.ini" }},
		{"source backslash root", func(s *rules.Suggestion) { s.Source = `\\server\share\x` }},
		{"source git folder", func(s *rules.Suggestion) { s.Source = ".git/config" }},
		{"source root without include", func(s *rules.Suggestion) { s.Source = "." }},
		{"source empty", func(s *rules.Suggestion) { s.Source = "" }},
		{"include dotdot", func(s *rules.Suggestion) { s.Source = "."; s.Include = []string{"../x.md"} }},
		{"include absolute", func(s *rules.Suggestion) { s.Source = "."; s.Include = []string{"/x.md"} }},
		{"dest dotdot", func(s *rules.Suggestion) { s.Destinations[0].Path = "../../.bashrc" }},
		{"dest backslash dotdot", func(s *rules.Suggestion) { s.Destinations[0].Path = `..\..\.bashrc` }},
		{"dest absolute posix", func(s *rules.Suggestion) { s.Destinations[0].Path = "/home/me/.bashrc" }},
		{"dest absolute windows", func(s *rules.Suggestion) { s.Destinations[0].Path = `C:\Users\me\x` }},
		{"dest backslash root", func(s *rules.Suggestion) { s.Destinations[0].Path = `\Users\me\x` }},
		{"dest git folder", func(s *rules.Suggestion) { s.Destinations[0].Path = "repo/.git/hooks/pre-commit" }},
		{"dest nul", func(s *rules.Suggestion) { s.Destinations[0].Path = "a\x00b" }},
		{"mode two way", func(s *rules.Suggestion) { s.Destinations[0].Mode = rules.ModeTwoWay }},
		{"mode unknown", func(s *rules.Suggestion) { s.Destinations[0].Mode = "sideways" }},
		{"no destinations", func(s *rules.Suggestion) { s.Destinations = nil }},
		{"too many destinations", func(s *rules.Suggestion) {
			s.Destinations = make([]rules.SuggestedDestination, rules.MaxSuggestionDests+1)
			for i := range s.Destinations {
				s.Destinations[i] = rules.SuggestedDestination{Location: "claude", Path: "f" + string(rune('a'+i%26)) + string(rune('a'+i/26))}
			}
		}},
		{"empty location", func(s *rules.Suggestion) { s.Destinations[0].Location = "" }},
		{"empty name", func(s *rules.Suggestion) { s.Name = "  " }},
		{"name too long", func(s *rules.Suggestion) { s.Name = strings.Repeat("n", rules.MaxSuggestionNameLength+1) }},
		{"description too long", func(s *rules.Suggestion) { s.Description = strings.Repeat("d", rules.MaxSuggestionDescLength+1) }},
		{"unknown kind", func(s *rules.Suggestion) { s.Kind = "zip" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSuggestion()
			tc.mutate(&s)
			if err := s.Validate(); err == nil {
				t.Fatalf("accepted %+v", s)
			}
		})
	}
}

func TestSuggestionValidateAcceptsLegitimateVariants(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*rules.Suggestion)
	}{
		{"create only", func(s *rules.Suggestion) { s.Destinations[0].Mode = rules.ModeCreateOnly }},
		{"default mode", func(s *rules.Suggestion) { s.Destinations[0].Mode = "" }},
		{"folder skill", func(s *rules.Suggestion) {
			s.Source = "skills/review"
			s.Kind = rules.KindSkill
			s.Destinations[0].Path = "skills/review"
		}},
		{"root selection", func(s *rules.Suggestion) { s.Source = "."; s.Include = []string{"AGENTS.md"} }},
		{"destination root", func(s *rules.Suggestion) {
			s.Source = "skills"
			s.Kind = rules.KindFolder
			s.Destinations[0].Path = "."
		}},
		{"backslash separators", func(s *rules.Suggestion) { s.Destinations[0].Path = `nested\CLAUDE.md` }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := validSuggestion()
			tc.mutate(&s)
			if err := s.Validate(); err != nil {
				t.Fatalf("rejected %+v: %v", s, err)
			}
		})
	}
}

func TestSuggestionToRuleIsLocalSubscriptionRule(t *testing.T) {
	rule := validSuggestion().Normalized().ToRule("sub1")
	if rule.Source.Origin != rules.OriginSubscription || rule.Source.Subscription != "sub1" || !rule.Enabled {
		t.Fatalf("unexpected source: %+v", rule)
	}
	if len(rule.Destinations) != 1 || rule.Destinations[0].Mode != rules.ModeOneWay || !rule.Destinations[0].Enabled || rule.Destinations[0].Transform != nil {
		t.Fatalf("unexpected destinations: %+v", rule.Destinations)
	}
}

func TestSuggestionIDTracksContent(t *testing.T) {
	a, b := validSuggestion(), validSuggestion()
	if a.ID() != b.ID() {
		t.Fatal("same content must give the same id")
	}
	b.Destinations[0].Path = "OTHER.md"
	if a.ID() == b.ID() {
		t.Fatal("different content must give a different id")
	}
}
