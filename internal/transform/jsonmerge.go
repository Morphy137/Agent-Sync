package transform

import (
	"bytes"
	"encoding/json"
)

// jsonIndent is the indentation unit used for json_merge output.
const jsonIndent = "  "

const jsonOutputTerminator = "\n"

// renderJSON merges the source entries under spec.Key of the existing JSON document.
// Other top-level keys keep their values and order; unmanaged entries under Key are untouched.
func renderJSON(spec Spec, entries []sourceEntry, existing []byte, prevManaged []string) ([]byte, error) {
	top, err := readTargetObject(existing)
	if err != nil {
		return nil, err
	}
	current, err := targetKeyObject(spec, top)
	if err != nil {
		return nil, err
	}

	owned := toSet(prevManaged)
	if name, conflict := firstConflict(entries, func(name string) bool {
		_, exists := current.get(name)
		return exists && !owned[name]
	}); conflict {
		return nil, newError(ReasonEntryConflict, nil).WithParam(ParamEntry, name)
	}

	sourceNames := make(map[string]bool, len(entries))
	for _, entry := range entries {
		sourceNames[entry.name] = true
	}
	for _, name := range append([]string(nil), current.names...) {
		if owned[name] && !sourceNames[name] {
			current.remove(name)
		}
	}
	for _, entry := range entries {
		current.set(entry.name, entry.raw)
	}

	_, keyPresent := top.get(spec.Key)
	if keyPresent || len(current.names) > 0 {
		top.set(spec.Key, encodeRawObject(current))
	}
	return indentJSON(encodeRawObject(top))
}

// jsonTargetEntries returns the decoded entries currently stored under spec.Key.
func jsonTargetEntries(spec Spec, existing []byte) (map[string]any, error) {
	top, err := readTargetObject(existing)
	if err != nil {
		return nil, err
	}
	current, err := targetKeyObject(spec, top)
	if err != nil {
		return nil, err
	}
	entries := make(map[string]any, len(current.names))
	for _, name := range current.names {
		value, err := decodeJSONValue(current.values[name])
		if err != nil {
			return nil, newError(ReasonInvalidTargetJSON, err).WithParam(ParamEntry, name)
		}
		entries[name] = value
	}
	return entries, nil
}

// readTargetObject parses the destination JSON document; empty content is an empty object.
func readTargetObject(existing []byte) (*rawObject, error) {
	if isBlank(existing) {
		return newRawObject(), nil
	}
	top, err := readRawObject(existing)
	if err != nil {
		return nil, newError(ReasonInvalidTargetJSON, err)
	}
	return top, nil
}

// targetKeyObject returns the object stored under spec.Key; a missing or null key is empty.
func targetKeyObject(spec Spec, top *rawObject) (*rawObject, error) {
	raw, ok := top.get(spec.Key)
	if !ok || isJSONNull(raw) {
		return newRawObject(), nil
	}
	if !isJSONObject(raw) {
		return nil, newError(ReasonInvalidTargetJSON, errNotObject).WithParam(ParamKey, spec.Key)
	}
	object, err := readRawObject(raw)
	if err != nil {
		return nil, newError(ReasonInvalidTargetJSON, err).WithParam(ParamKey, spec.Key)
	}
	return object, nil
}

// encodeRawObject writes the object as compact JSON, keeping member order and raw values.
func encodeRawObject(object *rawObject) []byte {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, name := range object.names {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(encodeJSONString(name))
		buf.WriteByte(':')
		buf.Write(object.values[name])
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

func indentJSON(compact []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, compact, "", jsonIndent); err != nil {
		return nil, newError(ReasonInvalidTargetJSON, err)
	}
	buf.WriteString(jsonOutputTerminator)
	return buf.Bytes(), nil
}

func toSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}
