// Package transform merges a canonical set of entries (for example MCP server
// definitions) into agent configuration files without destroying unrelated
// content. All functions are pure: they operate on byte slices and never touch
// the filesystem.
package transform

import (
	"bytes"
	"sort"

	"agentsync/internal/apperr"
)

// Supported transform types.
const (
	TypeJSONMerge = "json_merge"
	TypeTOMLMerge = "toml_merge"
)

// Error parameter names attached to apperr.TransformFailed errors.
const (
	ParamReason = "reason"
	ParamEntry  = "entry"
	ParamType   = "type"
	ParamKey    = "key"
	ParamPath   = "path"
	ParamDetail = "detail"
)

// Values of the ParamReason error parameter.
const (
	ReasonInvalidSpec       = "invalid_spec"
	ReasonUnsupportedType   = "unsupported_type"
	ReasonInvalidSourceJSON = "invalid_source_json"
	ReasonMissingKey        = "missing_key"
	ReasonInvalidTargetJSON = "invalid_target_json"
	ReasonInvalidTargetTOML = "invalid_target_toml"
	ReasonInvalidOutputTOML = "invalid_output_toml"
	ReasonUnsupportedValue  = "unsupported_value"
	ReasonEntryConflict     = "entry_conflict"
)

// Spec describes how source entries are merged into a destination file.
type Spec struct {
	Type string
	Key  string
}

// Result is the outcome of a Render call.
type Result struct {
	Output      []byte   // full new destination file content
	Managed     []string // sorted entry names under Key now owned by AgentSync
	ManagedView []byte   // canonical JSON of the managed entries (stable key order) — hashed for drift detection
}

// Render merges the source entries into the existing destination content.
// Source is always JSON whose top-level object contains Key (e.g. {"mcpServers": {...}}).
// existing may be nil/empty (file missing). prevManaged lists entries owned in the previous run:
// entries in prevManaged but no longer in source are removed; entries not owned are preserved untouched.
// If an unowned entry has the same name as a source entry, an apperr TransformFailed error with
// param reason "entry_conflict" and param "entry" is returned.
func Render(spec Spec, source []byte, existing []byte, prevManaged []string) (Result, error) {
	if err := validateSpec(spec); err != nil {
		return Result{}, err
	}
	entries, err := parseSourceEntries(spec, source)
	if err != nil {
		return Result{}, err
	}
	view, err := canonicalEntries(entries)
	if err != nil {
		return Result{}, err
	}

	var output []byte
	switch spec.Type {
	case TypeJSONMerge:
		output, err = renderJSON(spec, entries, existing, prevManaged)
	case TypeTOMLMerge:
		output, err = renderTOML(spec, entries, existing)
	}
	if err != nil {
		return Result{}, err
	}
	return Result{
		Output:      output,
		Managed:     sortedEntryNames(entries),
		ManagedView: view,
	}, nil
}

// SourceView returns the canonical JSON of the source entries under Key (used as source hash input).
func SourceView(spec Spec, source []byte) ([]byte, error) {
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	entries, err := parseSourceEntries(spec, source)
	if err != nil {
		return nil, err
	}
	return canonicalEntries(entries)
}

// TargetView returns the canonical JSON of the entries named in managed as currently present in the destination.
// Missing entries are omitted. Used to detect drift without reacting to unrelated edits in the file
// (e.g. ~/.claude.json changes constantly).
func TargetView(spec Spec, existing []byte, managed []string) ([]byte, error) {
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	var present map[string]any
	var err error
	switch spec.Type {
	case TypeJSONMerge:
		present, err = jsonTargetEntries(spec, existing)
	case TypeTOMLMerge:
		present, err = tomlTargetEntries(spec, existing)
	}
	if err != nil {
		return nil, err
	}
	selected := make(map[string]any, len(managed))
	for _, name := range managed {
		if value, ok := present[name]; ok {
			selected[name] = value
		}
	}
	return canonicalJSON(selected)
}

// newError builds a TransformFailed error carrying the given reason.
func newError(reason string, cause error) *apperr.Error {
	var e *apperr.Error
	if cause != nil {
		e = apperr.Wrap(apperr.TransformFailed, cause)
	} else {
		e = apperr.New(apperr.TransformFailed)
	}
	return e.WithParam(ParamReason, reason)
}

func validateSpec(spec Spec) error {
	if spec.Type != TypeJSONMerge && spec.Type != TypeTOMLMerge {
		return newError(ReasonUnsupportedType, nil).WithParam(ParamType, spec.Type)
	}
	if spec.Key == "" {
		return newError(ReasonInvalidSpec, nil).WithParam(ParamKey, spec.Key)
	}
	return nil
}

// sourceEntry is one named entry under Key in the source document, in source order.
type sourceEntry struct {
	name string
	raw  []byte
}

// parseSourceEntries extracts the entries under spec.Key from the source JSON, preserving order.
func parseSourceEntries(spec Spec, source []byte) ([]sourceEntry, error) {
	top, err := readRawObject(source)
	if err != nil {
		return nil, newError(ReasonInvalidSourceJSON, err)
	}
	keyValue, ok := top.get(spec.Key)
	if !ok {
		return nil, newError(ReasonMissingKey, nil).WithParam(ParamKey, spec.Key)
	}
	members, err := readRawObject(keyValue)
	if err != nil {
		return nil, newError(ReasonInvalidSourceJSON, err).WithParam(ParamKey, spec.Key)
	}
	entries := make([]sourceEntry, 0, len(members.names))
	for _, name := range members.names {
		entries = append(entries, sourceEntry{name: name, raw: members.values[name]})
	}
	return entries, nil
}

// canonicalEntries returns the canonical JSON of the entries as a single object keyed by name.
func canonicalEntries(entries []sourceEntry) ([]byte, error) {
	values := make(map[string]any, len(entries))
	for _, entry := range entries {
		value, err := decodeJSONValue(entry.raw)
		if err != nil {
			return nil, newError(ReasonInvalidSourceJSON, err).WithParam(ParamEntry, entry.name)
		}
		values[entry.name] = value
	}
	return canonicalJSON(values)
}

func sortedEntryNames(entries []sourceEntry) []string {
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.name)
	}
	sort.Strings(names)
	return names
}

// firstConflict returns the first (sorted) source entry name that is held by an unowned entry.
func firstConflict(entries []sourceEntry, isUnowned func(name string) bool) (string, bool) {
	for _, name := range sortedEntryNames(entries) {
		if isUnowned(name) {
			return name, true
		}
	}
	return "", false
}

func isBlank(data []byte) bool {
	return len(bytes.TrimSpace(data)) == 0
}
