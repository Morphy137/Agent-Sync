package transform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"
)

// Labels used in canonical JSON for floating point values JSON cannot represent.
const (
	nanLabel         = "nan"
	positiveInfLabel = "+inf"
	negativeInfLabel = "-inf"
)

const (
	decimalBase  = 10
	int64BitSize = 64
)

var (
	errNotObject     = errors.New("value is not a JSON object")
	errTrailingData  = errors.New("unexpected data after the top-level value")
	errUnexpectedEnd = errors.New("unexpected token")
)

// rawObject is a JSON object whose member values are kept as raw bytes, in document order.
type rawObject struct {
	names  []string
	values map[string][]byte
}

func (o *rawObject) get(name string) ([]byte, bool) {
	value, ok := o.values[name]
	return value, ok
}

// set stores a member; a duplicate name keeps its first position and takes the last value.
func (o *rawObject) set(name string, value []byte) {
	if _, exists := o.values[name]; !exists {
		o.names = append(o.names, name)
	}
	o.values[name] = value
}

func (o *rawObject) remove(name string) {
	if _, exists := o.values[name]; !exists {
		return
	}
	delete(o.values, name)
	for i, existing := range o.names {
		if existing == name {
			o.names = append(o.names[:i], o.names[i+1:]...)
			return
		}
	}
}

func newRawObject() *rawObject {
	return &rawObject{values: make(map[string][]byte)}
}

// readRawObject decodes a JSON object into its members, preserving their order and raw values.
func readRawObject(data []byte) (*rawObject, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, errNotObject
	}
	object := newRawObject()
	for dec.More() {
		nameToken, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, ok := nameToken.(string)
		if !ok {
			return nil, errUnexpectedEnd
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		object.set(name, bytes.TrimSpace(raw))
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if err := expectEOF(dec); err != nil {
		return nil, err
	}
	return object, nil
}

func expectEOF(dec *json.Decoder) error {
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errTrailingData
	}
	return nil
}

// isJSONObject reports whether raw holds a JSON object (as opposed to any other value).
func isJSONObject(raw []byte) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func isJSONNull(raw []byte) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// decodeJSONValue decodes a JSON value into plain Go values, keeping numbers as json.Number.
func decodeJSONValue(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if err := expectEOF(dec); err != nil {
		return nil, err
	}
	return value, nil
}

// orderedObject is a decoded JSON object that remembers member order.
type orderedObject struct {
	names  []string
	values map[string]any
}

// decodeOrderedValue decodes a JSON value into Go values where objects are *orderedObject,
// arrays are []any, numbers are json.Number, and strings/bools/null map to their Go forms.
func decodeOrderedValue(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	value, err := decodeOrderedToken(dec)
	if err != nil {
		return nil, err
	}
	if err := expectEOF(dec); err != nil {
		return nil, err
	}
	return value, nil
}

func decodeOrderedToken(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, isDelim := token.(json.Delim)
	if !isDelim {
		return token, nil
	}
	switch delim {
	case '{':
		object := &orderedObject{values: make(map[string]any)}
		for dec.More() {
			nameToken, err := dec.Token()
			if err != nil {
				return nil, err
			}
			name, ok := nameToken.(string)
			if !ok {
				return nil, errUnexpectedEnd
			}
			value, err := decodeOrderedToken(dec)
			if err != nil {
				return nil, err
			}
			if _, exists := object.values[name]; !exists {
				object.names = append(object.names, name)
			}
			object.values[name] = value
		}
		_, err := dec.Token()
		return object, err
	case '[':
		items := []any{}
		for dec.More() {
			item, err := decodeOrderedToken(dec)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		_, err := dec.Token()
		return items, err
	default:
		return nil, errUnexpectedEnd
	}
}

// canonicalJSON encodes value with object keys sorted recursively and no insignificant
// whitespace. Numbers are normalized (integers that fit int64 stay integers, everything else
// becomes float64) so that equivalent JSON and TOML documents produce identical output.
func canonicalJSON(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(normalizeValue(value)); err != nil {
		return nil, newError(ReasonUnsupportedValue, err)
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// normalizeValue converts decoded JSON or TOML values into a common set of Go types
// (map[string]any, []any, string, bool, nil, int64, float64) suitable for canonical encoding.
func normalizeValue(value any) any {
	switch v := value.(type) {
	case nil, string, bool, int64:
		return v
	case map[string]any:
		normalized := make(map[string]any, len(v))
		for name, item := range v {
			normalized[name] = normalizeValue(item)
		}
		return normalized
	case *orderedObject:
		normalized := make(map[string]any, len(v.values))
		for name, item := range v.values {
			normalized[name] = normalizeValue(item)
		}
		return normalized
	case []any:
		normalized := make([]any, len(v))
		for i, item := range v {
			normalized[i] = normalizeValue(item)
		}
		return normalized
	case json.Number:
		return normalizeNumber(v.String())
	case float64:
		return normalizeFloat(v)
	case int:
		return int64(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func normalizeNumber(literal string) any {
	if integer, err := strconv.ParseInt(literal, decimalBase, int64BitSize); err == nil {
		return integer
	}
	float, _ := strconv.ParseFloat(literal, int64BitSize)
	return normalizeFloat(float)
}

func normalizeFloat(value float64) any {
	switch {
	case math.IsNaN(value):
		return nanLabel
	case math.IsInf(value, 1):
		return positiveInfLabel
	case math.IsInf(value, -1):
		return negativeInfLabel
	default:
		return value
	}
}

// encodeJSONString encodes s as a JSON string literal without HTML escaping.
func encodeJSONString(s string) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // encoding a string cannot fail
	return bytes.TrimRight(buf.Bytes(), "\n")
}
