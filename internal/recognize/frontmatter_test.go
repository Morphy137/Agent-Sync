package recognize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFrontmatter(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "plain and quoted scalars",
			content: "---\nname: my-skill\ndescription: 'Does things: well'\nlicense: \"MIT\"\n---\nbody\n",
			want:    map[string]string{"name": "my-skill", "description": "Does things: well", "license": "MIT"},
		},
		{
			name:    "crlf and bom",
			content: "\xef\xbb\xbf---\r\nname: win\r\ndescription: crlf # trailing comment\r\n---\r\n",
			want:    map[string]string{"name": "win", "description": "crlf"},
		},
		{
			name:    "multi-line plain description",
			content: "---\nname: code-reviewer\ndescription:\n  Expertise in reviewing code.\n  Use when asked.\n---\n",
			want:    map[string]string{"name": "code-reviewer", "description": "Expertise in reviewing code. Use when asked."},
		},
		{
			name:    "folded and literal block scalars",
			content: "---\ndescription: >-\n  one\n  two\nnotes: |\n  a\n  b\n---\n",
			want:    map[string]string{"description": "one two", "notes": "a\nb"},
		},
		{
			name:    "nested map and comments are tolerated",
			content: "---\n# comment\nname: x\nmetadata:\n  author: me\n\ndescription: d\n...\n",
			want:    map[string]string{"name": "x", "metadata": "author: me", "description": "d"},
		},
		{
			name:    "empty value",
			content: "---\nname:\ndescription: d\n---\n",
			want:    map[string]string{"name": "", "description": "d"},
		},
		{name: "no frontmatter", content: "# Title\nname: x\n", wantErr: true},
		{name: "unterminated", content: "---\nname: x\n", wantErr: true},
		{name: "empty file", content: "", wantErr: true},
	}

	dir := t.TempDir()
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, strings.Repeat("f", i+1)+".md")
			if err := os.WriteFile(path, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := readFrontmatter(path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("readFrontmatter: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			for key, value := range tc.want {
				if got[key] != value {
					t.Errorf("%s = %q, want %q", key, got[key], value)
				}
			}
		})
	}
}

func TestReadLimitedRejectsLargeFiles(t *testing.T) {
	const limit = 4
	path := filepath.Join(t.TempDir(), "big.json")
	if err := os.WriteFile(path, []byte("123456"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readLimited(path, limit); err == nil {
		t.Fatal("expected error for file over the limit")
	}
}

func TestIsSafeRelativePath(t *testing.T) {
	cases := map[string]bool{
		"CLAUDE.md":         true,
		"skills/{name}":     true,
		".agents/skills/x":  true,
		"":                  false,
		".":                 false,
		"../x":              false,
		"a/../../x":         false,
		"/abs":              false,
		`C:\x`:              false,
		`skills\x`:          false,
		"C:/x":              false,
		"skills/./{name}/.": true,
	}
	for input, want := range cases {
		if got := isSafeRelativePath(input); got != want {
			t.Errorf("isSafeRelativePath(%q) = %v, want %v", input, got, want)
		}
	}
}
