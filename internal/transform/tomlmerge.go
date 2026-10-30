package transform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"agentsync/internal/apperr"

	toml "github.com/pelletier/go-toml/v2"
)

// Managed block markers. The Key is appended after the prefix and before the suffix.
const (
	managedBeginPrefix = "# >>> agentsync managed: "
	managedBeginSuffix = " >>>"
	managedEndPrefix   = "# <<< agentsync managed: "
	managedEndSuffix   = " <<<"
)

const (
	lineFeed       = "\n"
	crlf           = "\r\n"
	tomlKeySep     = "."
	tomlListSep    = ", "
	tomlFloatPoint = ".0"
	// floatFormatShortest asks strconv for the shortest representation that round-trips.
	floatFormatShortest = -1
	unicodeEscapeFormat = `\u%04X`
	// asciiDelete is U+007F, a control character TOML requires to be escaped.
	asciiDelete = 0x7f
	// lastC0Control is the highest code point of the C0 control range (U+0000..U+001F).
	lastC0Control = 0x1f
)

const detailUnterminatedBlock = "unterminated_managed_block"

var (
	errKeyNotTable  = errors.New("value under key is not a table")
	errEntryNoTable = errors.New("entry is not an object")
	errNullValue    = errors.New("null values cannot be represented in TOML")
	errFloatRange   = errors.New("number is out of range")
)

func managedBeginMarker(key string) string {
	return managedBeginPrefix + key + managedBeginSuffix
}

func managedEndMarker(key string) string {
	return managedEndPrefix + key + managedEndSuffix
}

// managedBlock locates the managed block inside a document as a byte range
// [start, end) covering both marker lines (end includes the closing line break).
type managedBlock struct {
	found bool
	start int
	end   int
}

func findManagedBlock(key string, doc []byte) (managedBlock, error) {
	begin, end := managedBeginMarker(key), managedEndMarker(key)
	block := managedBlock{}
	offset := 0
	for offset < len(doc) {
		lineEnd := len(doc)
		next := len(doc)
		if idx := bytes.IndexByte(doc[offset:], '\n'); idx >= 0 {
			lineEnd = offset + idx
			next = lineEnd + 1
		}
		line := strings.TrimSpace(string(doc[offset:lineEnd]))
		switch {
		case !block.found && line == begin:
			block.found = true
			block.start = offset
		case block.found && line == end:
			block.end = next
			return block, nil
		}
		offset = next
	}
	if block.found {
		return managedBlock{}, newError(ReasonInvalidTargetTOML, nil).
			WithParam(ParamKey, key).WithParam(ParamDetail, detailUnterminatedBlock)
	}
	return block, nil
}

// renderTOML replaces (or appends) the managed block holding the source entries as
// [Key.<name>] tables. Everything outside the block is preserved byte-for-byte.
func renderTOML(spec Spec, entries []sourceEntry, existing []byte) ([]byte, error) {
	block, err := findManagedBlock(spec.Key, existing)
	if err != nil {
		return nil, err
	}
	var before, after []byte
	if block.found {
		before, after = existing[:block.start], existing[block.end:]
	} else {
		before = existing
	}

	outside := append(append([]byte(nil), before...), after...)
	unowned, err := tomlEntriesUnderKey(spec.Key, outside)
	if err != nil {
		return nil, err
	}
	if name, conflict := firstConflict(entries, func(name string) bool {
		_, exists := unowned[name]
		return exists
	}); conflict {
		return nil, newError(ReasonEntryConflict, nil).WithParam(ParamEntry, name)
	}

	newline := detectNewline(existing)
	var out bytes.Buffer
	out.Write(before)
	if len(entries) > 0 {
		if !block.found {
			writeAppendSeparator(&out, newline)
		}
		content, err := renderTOMLBlock(spec.Key, entries, newline)
		if err != nil {
			return nil, err
		}
		out.Write(content)
	}
	out.Write(after)

	if err := toml.Unmarshal(out.Bytes(), &map[string]any{}); err != nil {
		return nil, newError(ReasonInvalidOutputTOML, err)
	}
	return out.Bytes(), nil
}

// writeAppendSeparator terminates the last line of a non-empty document and leaves one blank
// line before an appended managed block.
func writeAppendSeparator(out *bytes.Buffer, newline string) {
	if out.Len() == 0 {
		return
	}
	if !bytes.HasSuffix(out.Bytes(), []byte(lineFeed)) {
		out.WriteString(newline)
	}
	if !bytes.HasSuffix(out.Bytes(), []byte(newline+newline)) {
		out.WriteString(newline)
	}
}

func detectNewline(doc []byte) string {
	if bytes.Contains(doc, []byte(crlf)) {
		return crlf
	}
	return lineFeed
}

// tomlEntriesUnderKey parses doc and returns the table stored under key (nil map when absent).
func tomlEntriesUnderKey(key string, doc []byte) (map[string]any, error) {
	parsed := map[string]any{}
	if err := toml.Unmarshal(doc, &parsed); err != nil {
		return nil, newError(ReasonInvalidTargetTOML, err)
	}
	value, ok := parsed[key]
	if !ok {
		return map[string]any{}, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, newError(ReasonInvalidTargetTOML, errKeyNotTable).WithParam(ParamKey, key)
	}
	return table, nil
}

// tomlTargetEntries returns the entries currently stored under spec.Key anywhere in the document.
func tomlTargetEntries(spec Spec, existing []byte) (map[string]any, error) {
	return tomlEntriesUnderKey(spec.Key, existing)
}

// renderTOMLBlock writes the complete managed block, markers included.
func renderTOMLBlock(key string, entries []sourceEntry, newline string) ([]byte, error) {
	w := &tomlWriter{newline: newline}
	w.line(managedBeginMarker(key))
	for _, entry := range entries {
		value, err := decodeOrderedValue(entry.raw)
		if err != nil {
			return nil, newError(ReasonInvalidSourceJSON, err).WithParam(ParamEntry, entry.name)
		}
		object, ok := value.(*orderedObject)
		if !ok {
			return nil, newError(ReasonUnsupportedValue, errEntryNoTable).WithParam(ParamEntry, entry.name)
		}
		if err := w.table([]string{key, entry.name}, object); err != nil {
			return nil, err.WithParam(ParamEntry, entry.name)
		}
	}
	w.line(managedEndMarker(key))
	return w.buf.Bytes(), nil
}

type tomlWriter struct {
	buf        bytes.Buffer
	newline    string
	wroteTable bool
}

func (w *tomlWriter) line(text string) {
	w.buf.WriteString(text)
	w.buf.WriteString(w.newline)
}

// table writes a [path] header, the non-table members as key/value pairs and then every
// nested object as its own sub-table.
func (w *tomlWriter) table(path []string, object *orderedObject) *apperr.Error {
	if w.wroteTable {
		w.line("")
	}
	w.wroteTable = true
	w.line("[" + tomlKeyPath(path) + "]")

	var subTables []string
	for _, name := range object.names {
		if _, isObject := object.values[name].(*orderedObject); isObject {
			subTables = append(subTables, name)
			continue
		}
		memberPath := append(append([]string(nil), path...), name)
		literal, err := tomlValue(object.values[name], memberPath)
		if err != nil {
			return err
		}
		w.line(tomlKey(name) + " = " + literal)
	}
	for _, name := range subTables {
		memberPath := append(append([]string(nil), path...), name)
		if err := w.table(memberPath, object.values[name].(*orderedObject)); err != nil {
			return err
		}
	}
	return nil
}

// tomlValue renders a decoded JSON value as an inline TOML value.
func tomlValue(value any, path []string) (string, *apperr.Error) {
	switch v := value.(type) {
	case string:
		return tomlString(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case json.Number:
		literal, err := tomlNumber(v)
		if err != nil {
			return "", newError(ReasonUnsupportedValue, err).WithParam(ParamPath, strings.Join(path, tomlKeySep))
		}
		return literal, nil
	case []any:
		items := make([]string, 0, len(v))
		for i, item := range v {
			literal, err := tomlValue(item, append(append([]string(nil), path...), strconv.Itoa(i)))
			if err != nil {
				return "", err
			}
			items = append(items, literal)
		}
		return "[" + strings.Join(items, tomlListSep) + "]", nil
	case *orderedObject:
		if len(v.names) == 0 {
			return "{}", nil
		}
		members := make([]string, 0, len(v.names))
		for _, name := range v.names {
			literal, err := tomlValue(v.values[name], append(append([]string(nil), path...), name))
			if err != nil {
				return "", err
			}
			members = append(members, tomlKey(name)+" = "+literal)
		}
		return "{ " + strings.Join(members, tomlListSep) + " }", nil
	case nil:
		return "", newError(ReasonUnsupportedValue, errNullValue).WithParam(ParamPath, strings.Join(path, tomlKeySep))
	default:
		return "", newError(ReasonUnsupportedValue, fmt.Errorf("unexpected value type %T", v)).
			WithParam(ParamPath, strings.Join(path, tomlKeySep))
	}
}

// tomlNumber converts a JSON number literal into a TOML integer or float literal.
func tomlNumber(number json.Number) (string, error) {
	literal := number.String()
	isInteger := !strings.ContainsAny(literal, ".eE")
	if isInteger {
		if _, err := strconv.ParseInt(literal, decimalBase, int64BitSize); err == nil {
			return literal, nil
		}
	}
	float, err := strconv.ParseFloat(literal, int64BitSize)
	if err != nil || math.IsInf(float, 0) {
		return "", errFloatRange
	}
	if !isInteger {
		// Every JSON float literal is also a valid TOML float literal.
		return literal, nil
	}
	formatted := strconv.FormatFloat(float, 'g', floatFormatShortest, int64BitSize)
	if !strings.ContainsAny(formatted, ".eE") {
		formatted += tomlFloatPoint
	}
	return formatted, nil
}

// tomlString renders s as a TOML basic string, escaping quotes, backslashes and control characters.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\f':
			b.WriteString(`\f`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r <= lastC0Control || r == asciiDelete {
				fmt.Fprintf(&b, unicodeEscapeFormat, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// tomlKey renders a single key, bare when possible and quoted otherwise.
func tomlKey(key string) string {
	if isBareKey(key) {
		return key
	}
	return tomlString(key)
}

func tomlKeyPath(path []string) string {
	keys := make([]string, len(path))
	for i, key := range path {
		keys[i] = tomlKey(key)
	}
	return strings.Join(keys, tomlKeySep)
}

func isBareKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		isAlnum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if !isAlnum && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
