package transform

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"agentsync/internal/apperr"

	toml "github.com/pelletier/go-toml/v2"
)

const (
	jsonTestKey = "mcpServers"
	tomlTestKey = "mcp_servers"
)

var (
	jsonSpec = Spec{Type: TypeJSONMerge, Key: jsonTestKey}
	tomlSpec = Spec{Type: TypeTOMLMerge, Key: tomlTestKey}
)

// mcpSource wraps the given entries object JSON under key.
func mcpSource(key, entries string) []byte {
	return []byte(`{"` + key + `": ` + entries + `}`)
}

// assertTransformError checks that err is a TransformFailed apperr carrying the given params.
func assertTransformError(t *testing.T, err error, reason string, params map[string]string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with reason %q, got nil", reason)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperr.Error, got %T: %v", err, err)
	}
	if appErr.Code != apperr.TransformFailed {
		t.Fatalf("code = %q, want %q", appErr.Code, apperr.TransformFailed)
	}
	if got := appErr.Params[ParamReason]; got != reason {
		t.Fatalf("reason = %q, want %q (err: %v)", got, reason, err)
	}
	for name, want := range params {
		if got := appErr.Params[name]; got != want {
			t.Fatalf("param %q = %q, want %q", name, got, want)
		}
	}
}

func TestSpecValidation(t *testing.T) {
	source := mcpSource(jsonTestKey, `{}`)
	tests := []struct {
		name   string
		spec   Spec
		reason string
	}{
		{"unknown type", Spec{Type: "yaml_merge", Key: jsonTestKey}, ReasonUnsupportedType},
		{"empty type", Spec{Key: jsonTestKey}, ReasonUnsupportedType},
		{"empty key json", Spec{Type: TypeJSONMerge}, ReasonInvalidSpec},
		{"empty key toml", Spec{Type: TypeTOMLMerge}, ReasonInvalidSpec},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(tt.spec, source, nil, nil)
			assertTransformError(t, err, tt.reason, nil)
			_, err = SourceView(tt.spec, source)
			assertTransformError(t, err, tt.reason, nil)
			_, err = TargetView(tt.spec, nil, nil)
			assertTransformError(t, err, tt.reason, nil)
		})
	}
}

func TestSourceErrors(t *testing.T) {
	tests := []struct {
		name   string
		source string
		reason string
	}{
		{"not json", `{"mcpServers": `, ReasonInvalidSourceJSON},
		{"empty", ``, ReasonInvalidSourceJSON},
		{"top-level array", `[1, 2]`, ReasonInvalidSourceJSON},
		{"trailing data", `{"mcpServers": {}} {}`, ReasonInvalidSourceJSON},
		{"missing key", `{"other": {}}`, ReasonMissingKey},
		{"key is array", `{"mcpServers": []}`, ReasonInvalidSourceJSON},
		{"key is null", `{"mcpServers": null}`, ReasonInvalidSourceJSON},
		{"key is string", `{"mcpServers": "x"}`, ReasonInvalidSourceJSON},
	}
	for _, spec := range []Spec{jsonSpec, {Type: TypeTOMLMerge, Key: jsonTestKey}} {
		for _, tt := range tests {
			t.Run(spec.Type+"/"+tt.name, func(t *testing.T) {
				_, err := Render(spec, []byte(tt.source), nil, nil)
				assertTransformError(t, err, tt.reason, nil)
				_, err = SourceView(spec, []byte(tt.source))
				assertTransformError(t, err, tt.reason, nil)
			})
		}
	}
}

func TestSourceViewCanonical(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"empty entries", `{"mcpServers": {}}`, `{}`},
		{
			"keys sorted recursively and compact",
			`{"zzz": 1, "mcpServers": {"b": {"z": 1, "a": {"y": true, "x": [3, {"q": 1, "p": 2}]}}, "a": {"command": "npx"}}}`,
			`{"a":{"command":"npx"},"b":{"a":{"x":[3,{"p":2,"q":1}],"y":true},"z":1}}`,
		},
		{
			"numbers normalized",
			`{"mcpServers": {"n": {"i": 10, "f": 1.50, "e": 1e3, "z": 1.0, "neg": -0, "big": 12345678901234567890}}}`,
			`{"n":{"big":12345678901234567000,"e":1000,"f":1.5,"i":10,"neg":0,"z":1}}`,
		},
		{
			"no html escaping",
			`{"mcpServers": {"h": {"url": "http://x/?a=1&b=<2>"}}}`,
			`{"h":{"url":"http://x/?a=1&b=<2>"}}`,
		},
		{"non-object entries allowed", `{"mcpServers": {"s": "text", "n": null}}`, `{"n":null,"s":"text"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SourceView(jsonSpec, []byte(tt.source))
			if err != nil {
				t.Fatalf("SourceView: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("SourceView =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestSourceViewIgnoresFormattingAndOrder(t *testing.T) {
	a := []byte(`{"mcpServers": {"x": {"command": "npx", "args": ["-y", "pkg"]}, "y": {"url": "u"}}}`)
	b := []byte("{\n  \"mcpServers\": {\n    \"y\": {\"url\": \"u\"},\n    \"x\": {\n      \"args\": [\"-y\", \"pkg\"],\n      \"command\": \"npx\"\n    }\n  },\n  \"extra\": 1\n}\n")
	viewA, err := SourceView(jsonSpec, a)
	if err != nil {
		t.Fatal(err)
	}
	viewB, err := SourceView(jsonSpec, b)
	if err != nil {
		t.Fatal(err)
	}
	if string(viewA) != string(viewB) {
		t.Fatalf("views differ:\n%s\n%s", viewA, viewB)
	}
}

func TestRenderJSON(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		existing    string
		prevManaged []string
		want        string
		wantManaged []string
	}{
		{
			name:        "missing file",
			source:      `{"mcpServers": {"b": {"command": "b"}, "a": {"command": "a", "args": ["-y"]}}}`,
			existing:    "",
			want:        "{\n  \"mcpServers\": {\n    \"b\": {\n      \"command\": \"b\"\n    },\n    \"a\": {\n      \"command\": \"a\",\n      \"args\": [\n        \"-y\"\n      ]\n    }\n  }\n}\n",
			wantManaged: []string{"a", "b"},
		},
		{
			name:        "whitespace-only file is empty",
			source:      `{"mcpServers": {"a": {"command": "a"}}}`,
			existing:    " \n\t",
			want:        "{\n  \"mcpServers\": {\n    \"a\": {\n      \"command\": \"a\"\n    }\n  }\n}\n",
			wantManaged: []string{"a"},
		},
		{
			name:        "preserves other keys, their order and unmanaged entries",
			source:      `{"mcpServers": {"new": {"command": "n"}}}`,
			existing:    `{"zeta": 1, "numStartups": 12345678901234567890, "mcpServers": {"user": {"command": "u", "env": {"B": "2", "A": "1"}}}, "alpha": {"nested": [1, {"y": 2, "x": 1}]}}`,
			want:        "{\n  \"zeta\": 1,\n  \"numStartups\": 12345678901234567890,\n  \"mcpServers\": {\n    \"user\": {\n      \"command\": \"u\",\n      \"env\": {\n        \"B\": \"2\",\n        \"A\": \"1\"\n      }\n    },\n    \"new\": {\n      \"command\": \"n\"\n    }\n  },\n  \"alpha\": {\n    \"nested\": [\n      1,\n      {\n        \"y\": 2,\n        \"x\": 1\n      }\n    ]\n  }\n}\n",
			wantManaged: []string{"new"},
		},
		{
			name:        "managed entry replaced in place, stale managed entry removed",
			source:      `{"mcpServers": {"keep": {"command": "v2"}}}`,
			existing:    `{"mcpServers": {"stale": {"command": "s"}, "user": {"command": "u"}, "keep": {"command": "v1"}, "last": {}}}`,
			prevManaged: []string{"keep", "stale"},
			want:        "{\n  \"mcpServers\": {\n    \"user\": {\n      \"command\": \"u\"\n    },\n    \"keep\": {\n      \"command\": \"v2\"\n    },\n    \"last\": {}\n  }\n}\n",
			wantManaged: []string{"keep"},
		},
		{
			name:        "previously managed entry already gone",
			source:      `{"mcpServers": {"a": {"command": "a"}}}`,
			existing:    `{"other": true}`,
			prevManaged: []string{"a", "ghost"},
			want:        "{\n  \"other\": true,\n  \"mcpServers\": {\n    \"a\": {\n      \"command\": \"a\"\n    }\n  }\n}\n",
			wantManaged: []string{"a"},
		},
		{
			name:        "empty source does not add key",
			source:      `{"mcpServers": {}}`,
			existing:    `{"other": [true, false, null]}`,
			want:        "{\n  \"other\": [\n    true,\n    false,\n    null\n  ]\n}\n",
			wantManaged: []string{},
		},
		{
			name:        "empty source removes managed entries but keeps key",
			source:      `{"mcpServers": {}}`,
			existing:    `{"mcpServers": {"a": {"command": "a"}}}`,
			prevManaged: []string{"a"},
			want:        "{\n  \"mcpServers\": {}\n}\n",
			wantManaged: []string{},
		},
		{
			name:        "empty source and empty file",
			source:      `{"mcpServers": {}}`,
			existing:    "",
			want:        "{}\n",
			wantManaged: []string{},
		},
		{
			name:        "null key replaced",
			source:      `{"mcpServers": {"a": {"command": "a"}}}`,
			existing:    `{"x": 1, "mcpServers": null, "y": 2}`,
			want:        "{\n  \"x\": 1,\n  \"mcpServers\": {\n    \"a\": {\n      \"command\": \"a\"\n    }\n  },\n  \"y\": 2\n}\n",
			wantManaged: []string{"a"},
		},
		{
			name:        "html characters and unicode not escaped, number literals kept",
			source:      `{"mcpServers": {"h": {"url": "http://x/?a=1&b=<2>", "name": "caf\u00e9", "n": 1.50, "e": 1E+2}}}`,
			existing:    `{"t": "<&>"}`,
			want:        "{\n  \"t\": \"<&>\",\n  \"mcpServers\": {\n    \"h\": {\n      \"url\": \"http://x/?a=1&b=<2>\",\n      \"name\": \"caf\\u00e9\",\n      \"n\": 1.50,\n      \"e\": 1E+2\n    }\n  }\n}\n",
			wantManaged: []string{"h"},
		},
		{
			name:        "duplicate top-level key keeps first position and last value",
			source:      `{"mcpServers": {}}`,
			existing:    `{"a": 1, "b": 2, "a": 3}`,
			want:        "{\n  \"a\": 3,\n  \"b\": 2\n}\n",
			wantManaged: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Render(jsonSpec, []byte(tt.source), []byte(tt.existing), tt.prevManaged)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if string(result.Output) != tt.want {
				t.Fatalf("output =\n%s\nwant\n%s", result.Output, tt.want)
			}
			if !reflect.DeepEqual(result.Managed, tt.wantManaged) {
				t.Fatalf("managed = %#v, want %#v", result.Managed, tt.wantManaged)
			}
			again, err := Render(jsonSpec, []byte(tt.source), result.Output, result.Managed)
			if err != nil {
				t.Fatalf("second Render: %v", err)
			}
			if string(again.Output) != string(result.Output) {
				t.Fatalf("render is not idempotent:\n%s\nvs\n%s", again.Output, result.Output)
			}
		})
	}
}

func TestRenderJSONErrors(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		existing    string
		prevManaged []string
		reason      string
		params      map[string]string
	}{
		{
			name:     "unowned entry conflict",
			source:   `{"mcpServers": {"b": {}, "a": {}}}`,
			existing: `{"mcpServers": {"a": {"command": "mine"}, "b": {}}}`,
			reason:   ReasonEntryConflict,
			params:   map[string]string{ParamEntry: "a"},
		},
		{
			name:        "conflict only for unowned names",
			source:      `{"mcpServers": {"a": {}, "b": {}}}`,
			existing:    `{"mcpServers": {"a": {}, "b": {}}}`,
			prevManaged: []string{"a"},
			reason:      ReasonEntryConflict,
			params:      map[string]string{ParamEntry: "b"},
		},
		{name: "invalid target", source: `{"mcpServers": {}}`, existing: `{"a": `, reason: ReasonInvalidTargetJSON},
		{name: "target is array", source: `{"mcpServers": {}}`, existing: `[]`, reason: ReasonInvalidTargetJSON},
		{name: "target trailing data", source: `{"mcpServers": {}}`, existing: `{} x`, reason: ReasonInvalidTargetJSON},
		{
			name:     "target key not object",
			source:   `{"mcpServers": {}}`,
			existing: `{"mcpServers": [1]}`,
			reason:   ReasonInvalidTargetJSON,
			params:   map[string]string{ParamKey: jsonTestKey},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(jsonSpec, []byte(tt.source), []byte(tt.existing), tt.prevManaged)
			assertTransformError(t, err, tt.reason, tt.params)
		})
	}
}

func TestTargetViewJSON(t *testing.T) {
	existing := []byte(`{"numStartups": 7, "mcpServers": {"a": {"z": 1, "a": [2.0, "x"]}, "user": {"command": "u"}, "b": "s"}}`)
	tests := []struct {
		name     string
		existing []byte
		managed  []string
		want     string
	}{
		{"selected entries only, canonical", existing, []string{"a", "b"}, `{"a":{"a":[2,"x"],"z":1},"b":"s"}`},
		{"missing entries omitted", existing, []string{"a", "gone"}, `{"a":{"a":[2,"x"],"z":1}}`},
		{"no managed entries", existing, nil, `{}`},
		{"empty file", nil, []string{"a"}, `{}`},
		{"key absent", []byte(`{"x": 1}`), []string{"a"}, `{}`},
		{"key null", []byte(`{"mcpServers": null}`), []string{"a"}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TargetView(jsonSpec, tt.existing, tt.managed)
			if err != nil {
				t.Fatalf("TargetView: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("TargetView = %s, want %s", got, tt.want)
			}
		})
	}

	_, err := TargetView(jsonSpec, []byte(`{"mcpServers": 3}`), []string{"a"})
	assertTransformError(t, err, ReasonInvalidTargetJSON, nil)
	_, err = TargetView(jsonSpec, []byte(`not json`), []string{"a"})
	assertTransformError(t, err, ReasonInvalidTargetJSON, nil)
}

func TestRenderTOMLExactOutput(t *testing.T) {
	source := mcpSource(tomlTestKey, `{
		"zeta": {"command": "npx", "args": ["-y", "@scope/pkg"], "env": {"API_URL": "http://x", "DEBUG": "1"}, "enabled": true, "timeout": 30, "ratio": 0.5},
		"my server": {"url": "https://example.com/mcp", "headers": {}, "tools": [{"name": "a", "opts": {"deep": [1, 2]}}, []]}
	}`)
	existing := "model = \"o3\"\n# user comment\n\n[profiles.fast]\nmodel = \"o4-mini\"\n"
	want := existing + "\n" +
		"# >>> agentsync managed: mcp_servers >>>\n" +
		"[mcp_servers.zeta]\n" +
		"command = \"npx\"\n" +
		"args = [\"-y\", \"@scope/pkg\"]\n" +
		"enabled = true\n" +
		"timeout = 30\n" +
		"ratio = 0.5\n" +
		"\n" +
		"[mcp_servers.zeta.env]\n" +
		"API_URL = \"http://x\"\n" +
		"DEBUG = \"1\"\n" +
		"\n" +
		"[mcp_servers.\"my server\"]\n" +
		"url = \"https://example.com/mcp\"\n" +
		"tools = [{ name = \"a\", opts = { deep = [1, 2] } }, []]\n" +
		"\n" +
		"[mcp_servers.\"my server\".headers]\n" +
		"# <<< agentsync managed: mcp_servers <<<\n"

	result, err := Render(tomlSpec, source, []byte(existing), nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(result.Output) != want {
		t.Fatalf("output =\n%s\nwant\n%s", result.Output, want)
	}
	if !reflect.DeepEqual(result.Managed, []string{"my server", "zeta"}) {
		t.Fatalf("managed = %#v", result.Managed)
	}
}

func TestRenderTOMLPreservesOutsideContent(t *testing.T) {
	head := "# top comment\r\nmodel = \"o3\"   # trailing comment\r\n\r\n[mcp_servers.user]\r\ncommand = 'literal'\r\n\r\n"
	tail := "\r\n[profiles.slow]\r\nmodel = \"o1\"\r\n"
	oldBlock := "# >>> agentsync managed: mcp_servers >>>\r\n[mcp_servers.stale]\r\ncommand = \"old\"\r\n# <<< agentsync managed: mcp_servers <<<\r\n"
	existing := head + oldBlock + tail

	source := mcpSource(tomlTestKey, `{"fresh": {"command": "new"}}`)
	result, err := Render(tomlSpec, source, []byte(existing), []string{"stale"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	wantBlock := "# >>> agentsync managed: mcp_servers >>>\r\n[mcp_servers.fresh]\r\ncommand = \"new\"\r\n# <<< agentsync managed: mcp_servers <<<\r\n"
	if want := head + wantBlock + tail; string(result.Output) != want {
		t.Fatalf("output =\n%q\nwant\n%q", result.Output, want)
	}

	view, err := TargetView(tomlSpec, result.Output, []string{"fresh", "stale", "user"})
	if err != nil {
		t.Fatalf("TargetView: %v", err)
	}
	if want := `{"fresh":{"command":"new"},"user":{"command":"literal"}}`; string(view) != want {
		t.Fatalf("TargetView = %s, want %s", view, want)
	}
}

func TestRenderTOML(t *testing.T) {
	const (
		begin = "# >>> agentsync managed: mcp_servers >>>\n"
		end   = "# <<< agentsync managed: mcp_servers <<<\n"
	)
	tests := []struct {
		name     string
		source   string
		existing string
		want     string
	}{
		{
			name:     "missing file",
			source:   `{"a": {"command": "a"}}`,
			existing: "",
			want:     begin + "[mcp_servers.a]\ncommand = \"a\"\n" + end,
		},
		{
			name:     "file without trailing newline",
			source:   `{"a": {"command": "a"}}`,
			existing: `model = "x"`,
			want:     "model = \"x\"\n\n" + begin + "[mcp_servers.a]\ncommand = \"a\"\n" + end,
		},
		{
			name:     "file already ending with a blank line",
			source:   `{"a": {"command": "a"}}`,
			existing: "model = \"x\"\n\n",
			want:     "model = \"x\"\n\n" + begin + "[mcp_servers.a]\ncommand = \"a\"\n" + end,
		},
		{
			name:     "empty source without block leaves file untouched",
			source:   `{}`,
			existing: "model = \"x\"",
			want:     "model = \"x\"",
		},
		{
			name:     "empty source removes block",
			source:   `{}`,
			existing: "a = 1\n" + begin + "[mcp_servers.old]\nx = 1\n" + end + "b = 2\n",
			want:     "a = 1\nb = 2\n",
		},
		{
			name:     "invalid TOML inside the old block is discarded",
			source:   `{"a": {"command": "a"}}`,
			existing: "a = 1\n" + begin + "[[[ broken\n" + end,
			want:     "a = 1\n" + begin + "[mcp_servers.a]\ncommand = \"a\"\n" + end,
		},
		{
			name:     "markers with surrounding whitespace are recognised",
			source:   `{"a": {"command": "a"}}`,
			existing: "  # >>> agentsync managed: mcp_servers >>>  \n[mcp_servers.old]\n\t# <<< agentsync managed: mcp_servers <<<",
			want:     begin + "[mcp_servers.a]\ncommand = \"a\"\n" + end,
		},
		{
			name:     "unowned entries with other names coexist",
			source:   `{"a": {"command": "a"}}`,
			existing: "[mcp_servers]\ninline = { command = \"i\" }\n\n[mcp_servers.table]\ncommand = \"t\"\n",
			want:     "[mcp_servers]\ninline = { command = \"i\" }\n\n[mcp_servers.table]\ncommand = \"t\"\n\n" + begin + "[mcp_servers.a]\ncommand = \"a\"\n" + end,
		},
		{
			name:     "marker of another key is ignored",
			source:   `{"a": {"command": "a"}}`,
			existing: "# >>> agentsync managed: other >>>\n",
			want:     "# >>> agentsync managed: other >>>\n\n" + begin + "[mcp_servers.a]\ncommand = \"a\"\n" + end,
		},
		{
			name:     "special keys are quoted",
			source:   `{"a.b": {"": 1, "k\"q": 2, "ok-key_1": 3, "ünï": 4}}`,
			existing: "",
			want:     begin + "[mcp_servers.\"a.b\"]\n\"\" = 1\n\"k\\\"q\" = 2\nok-key_1 = 3\n\"ünï\" = 4\n" + end,
		},
		{
			name:     "string escaping",
			source:   `{"s": {"v": "q\"b\\n\nt\tc\u0001d\u007fe\b\f\r é😀"}}`,
			existing: "",
			want:     begin + "[mcp_servers.s]\nv = \"q\\\"b\\\\n\\nt\\tc\\u0001d\\u007Fe\\b\\f\\r é😀\"\n" + end,
		},
		{
			name:     "numbers",
			source:   `{"n": {"i": 42, "neg": -7, "zero": -0, "f": 1.50, "one": 1.0, "exp": 1e3, "big": 12345678901234567890, "small": 2.5E-7}}`,
			existing: "",
			want:     begin + "[mcp_servers.n]\ni = 42\nneg = -7\nzero = -0\nf = 1.50\none = 1.0\nexp = 1e3\nbig = 1.2345678901234567e+19\nsmall = 2.5E-7\n" + end,
		},
		{
			name:     "deeply nested objects become sub-tables",
			source:   `{"d": {"a": {"b": {"c": {"v": true}}, "x": false}}}`,
			existing: "",
			want:     begin + "[mcp_servers.d]\n\n[mcp_servers.d.a]\nx = false\n\n[mcp_servers.d.a.b]\n\n[mcp_servers.d.a.b.c]\nv = true\n" + end,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := mcpSource(tomlTestKey, tt.source)
			result, err := Render(tomlSpec, source, []byte(tt.existing), nil)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if string(result.Output) != tt.want {
				t.Fatalf("output =\n%q\nwant\n%q", result.Output, tt.want)
			}
			if err := toml.Unmarshal(result.Output, &map[string]any{}); err != nil {
				t.Fatalf("output is not valid TOML: %v", err)
			}
			again, err := Render(tomlSpec, source, result.Output, result.Managed)
			if err != nil {
				t.Fatalf("second Render: %v", err)
			}
			if string(again.Output) != string(result.Output) {
				t.Fatalf("render is not idempotent:\n%q\nvs\n%q", again.Output, result.Output)
			}
			assertRoundTrip(t, tomlSpec, source, result)
		})
	}
}

func TestRenderTOMLErrors(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		existing string
		prev     []string
		reason   string
		params   map[string]string
	}{
		{
			name:     "unowned table conflicts",
			source:   `{"b": {}, "a": {}}`,
			existing: "[mcp_servers.b]\ncommand = \"mine\"\n",
			reason:   ReasonEntryConflict,
			params:   map[string]string{ParamEntry: "b"},
		},
		{
			name:     "unowned inline entry conflicts",
			source:   `{"a": {}}`,
			existing: "[mcp_servers]\na = { command = \"mine\" }\n",
			reason:   ReasonEntryConflict,
			params:   map[string]string{ParamEntry: "a"},
		},
		{
			name:     "unowned dotted entry conflicts even if previously managed",
			source:   `{"a": {}}`,
			existing: "mcp_servers.a.command = \"mine\"\n",
			prev:     []string{"a"},
			reason:   ReasonEntryConflict,
			params:   map[string]string{ParamEntry: "a"},
		},
		{
			name:     "unterminated block",
			source:   `{"a": {}}`,
			existing: "# >>> agentsync managed: mcp_servers >>>\n[mcp_servers.a]\n",
			reason:   ReasonInvalidTargetTOML,
			params:   map[string]string{ParamDetail: detailUnterminatedBlock},
		},
		{name: "invalid outside TOML", source: `{"a": {}}`, existing: "a = \n", reason: ReasonInvalidTargetTOML},
		{
			name:     "key is not a table",
			source:   `{"a": {}}`,
			existing: "mcp_servers = 1\n",
			reason:   ReasonInvalidTargetTOML,
			params:   map[string]string{ParamKey: tomlTestKey},
		},
		{
			name:     "inline key table cannot be extended",
			source:   `{"a": {}}`,
			existing: "mcp_servers = { other = { command = \"x\" } }\n",
			reason:   ReasonInvalidOutputTOML,
		},
		{
			name:   "null value",
			source: `{"a": {"x": {"y": null}}}`,
			reason: ReasonUnsupportedValue,
			params: map[string]string{ParamEntry: "a", ParamPath: "mcp_servers.a.x.y"},
		},
		{
			name:   "null inside array",
			source: `{"a": {"x": [1, null]}}`,
			reason: ReasonUnsupportedValue,
			params: map[string]string{ParamPath: "mcp_servers.a.x.1"},
		},
		{name: "entry is not an object", source: `{"a": "text"}`, reason: ReasonUnsupportedValue, params: map[string]string{ParamEntry: "a"}},
		{name: "float out of range", source: `{"a": {"x": 1e400}}`, reason: ReasonUnsupportedValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Render(tomlSpec, mcpSource(tomlTestKey, tt.source), []byte(tt.existing), tt.prev)
			assertTransformError(t, err, tt.reason, tt.params)
		})
	}
}

func TestTargetViewTOML(t *testing.T) {
	doc := []byte("title = \"x\"\n" +
		"[mcp_servers.a]\ncommand = \"a\"\nn = 3\nf = 2.0\nwhen = 1979-05-27T07:32:00Z\nday = 1979-05-27\nnan = nan\ninf = -inf\n" +
		"[mcp_servers.a.env]\nK = \"v\"\n" +
		"[mcp_servers.user]\ncommand = \"u\"\n")
	tests := []struct {
		name     string
		existing []byte
		managed  []string
		want     string
	}{
		{
			"types normalized",
			doc,
			[]string{"a"},
			`{"a":{"command":"a","day":"1979-05-27","env":{"K":"v"},"f":2,"inf":"-inf","n":3,"nan":"nan","when":"1979-05-27T07:32:00Z"}}`,
		},
		{"missing entries omitted", doc, []string{"user", "gone"}, `{"user":{"command":"u"}}`},
		{"empty file", nil, []string{"a"}, `{}`},
		{"key absent", []byte("x = 1\n"), []string{"a"}, `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TargetView(tomlSpec, tt.existing, tt.managed)
			if err != nil {
				t.Fatalf("TargetView: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("TargetView = %s, want %s", got, tt.want)
			}
		})
	}

	_, err := TargetView(tomlSpec, []byte("= broken"), []string{"a"})
	assertTransformError(t, err, ReasonInvalidTargetTOML, nil)
	_, err = TargetView(tomlSpec, []byte("mcp_servers = [1]"), []string{"a"})
	assertTransformError(t, err, ReasonInvalidTargetTOML, nil)
}

// assertRoundTrip checks that the freshly rendered target reports the same view as its source.
func assertRoundTrip(t *testing.T, spec Spec, source []byte, result Result) {
	t.Helper()
	sourceView, err := SourceView(spec, source)
	if err != nil {
		t.Fatalf("SourceView: %v", err)
	}
	targetView, err := TargetView(spec, result.Output, result.Managed)
	if err != nil {
		t.Fatalf("TargetView: %v", err)
	}
	if string(sourceView) != string(result.ManagedView) {
		t.Fatalf("ManagedView = %s, SourceView = %s", result.ManagedView, sourceView)
	}
	if string(targetView) != string(sourceView) {
		t.Fatalf("round trip mismatch:\nTargetView = %s\nSourceView = %s", targetView, sourceView)
	}
}

func TestRoundTripProperty(t *testing.T) {
	entries := []string{
		`{}`,
		`{"a": {}}`,
		`{"fs": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "C:\\Users\\me"], "env": {"TOKEN": "x\"y", "EMPTY": ""}}}`,
		`{"remote": {"type": "http", "url": "https://x/mcp?a=1&b=<2>", "headers": {"Authorization": "Bearer t"}, "timeout": 60000, "trust": false}}`,
		`{"nums": {"i": 9223372036854775807, "min": -9223372036854775808, "f": 3.14159, "one": 1.0, "e": -2.5e-3, "big": 18446744073709551616, "z": -0.0}}`,
		`{"nested": {"a": {"b": {"c": [1, [2, [3]], {"d": {"e": "f"}}]}}, "list": [{"x": 1}, {"y": [true, false]}], "empty": {}, "emptyList": []}}`,
		`{"unicode name ü": {"v": "line1\nline2\ttab \u0000 \u001f \u007f 😀"}, "dotted.name": {"k": "v"}}`,
	}
	existingJSON := `{"numStartups": 3, "projects": {"C:/x": {"history": []}}, "mcpServers": {"user": {"command": "u"}}}`
	existingTOML := "model = \"o3\"\n[mcp_servers.user]\ncommand = \"u\"\n[profiles.p]\nmodel = \"x\"\n"

	for _, tc := range []struct {
		spec     Spec
		existing []string
	}{
		{jsonSpec, []string{"", existingJSON}},
		{tomlSpec, []string{"", existingTOML}},
	} {
		for i, entriesJSON := range entries {
			for j, existing := range tc.existing {
				source := mcpSource(tc.spec.Key, entriesJSON)
				t.Run(tc.spec.Type+"/"+string(rune('a'+i))+"/"+string(rune('0'+j)), func(t *testing.T) {
					result, err := Render(tc.spec, source, []byte(existing), nil)
					if err != nil {
						t.Fatalf("Render: %v", err)
					}
					assertRoundTrip(t, tc.spec, source, result)
					if existing != "" && !strings.Contains(string(result.Output), "user") {
						t.Fatalf("unmanaged entry lost:\n%s", result.Output)
					}
				})
			}
		}
	}
}

func TestDriftDetection(t *testing.T) {
	for _, tc := range []struct {
		spec          Spec
		existing      string
		unrelatedEdit func(string) string
		managedEdit   func(string) string
	}{
		{
			spec:     jsonSpec,
			existing: `{"numStartups": 1, "mcpServers": {"user": {}}}`,
			unrelatedEdit: func(s string) string {
				return strings.Replace(s, `"numStartups": 1`, `"numStartups": 2, "tips": {"x": true}`, 1)
			},
			managedEdit: func(s string) string { return strings.Replace(s, `"npx"`, `"node"`, 1) },
		},
		{
			spec:     tomlSpec,
			existing: "model = \"o3\"\n",
			unrelatedEdit: func(s string) string {
				return strings.Replace(s, `model = "o3"`, "model = \"o4\"\n[mcp_servers.user]\ncommand = \"u\"", 1)
			},
			managedEdit: func(s string) string { return strings.Replace(s, `"npx"`, `"node"`, 1) },
		},
	} {
		t.Run(tc.spec.Type, func(t *testing.T) {
			source := mcpSource(tc.spec.Key, `{"fs": {"command": "npx", "args": ["a"]}}`)
			result, err := Render(tc.spec, source, []byte(tc.existing), nil)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			baseline, err := TargetView(tc.spec, result.Output, result.Managed)
			if err != nil {
				t.Fatal(err)
			}

			unrelated, err := TargetView(tc.spec, []byte(tc.unrelatedEdit(string(result.Output))), result.Managed)
			if err != nil {
				t.Fatal(err)
			}
			if string(unrelated) != string(baseline) {
				t.Fatalf("unrelated edit changed view: %s vs %s", unrelated, baseline)
			}

			drifted, err := TargetView(tc.spec, []byte(tc.managedEdit(string(result.Output))), result.Managed)
			if err != nil {
				t.Fatal(err)
			}
			if string(drifted) == string(baseline) {
				t.Fatalf("managed edit not detected: %s", drifted)
			}
		})
	}
}
