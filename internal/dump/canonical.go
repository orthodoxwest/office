package dump

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Record is one JSON object in the dump. Values are limited to the types the
// canonical encoding admits: nil, bool, int, int64, non-empty string,
// []any, and map[string]any.
type Record = map[string]any

// errEmptyString enforces the "absent is null" rule: an empty string would let
// one implementation write "" where the other writes null for the same state.
var errEmptyString = errors.New("empty string (absent values are null)")

// Marshal returns the canonical compact encoding of v (RUST-PORT.md, "Dump
// format"). For this value domain it is byte-identical to RFC 8785 and to
// serde_json's to_string over a serde_json::Value, whose default Map is sorted.
func Marshal(v any) ([]byte, error) {
	return appendValue(nil, v, false, 0)
}

// MarshalPretty returns the canonical indented encoding of v followed by a
// newline: serde_json's to_string_pretty output, used for checked-in files
// that people read in diffs.
func MarshalPretty(v any) ([]byte, error) {
	b, err := appendValue(nil, v, true, 0)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// pathError locates an encoding failure with a JSON Pointer.
type pathError struct {
	path []string
	err  error
}

func (e *pathError) Error() string {
	return "/" + strings.Join(e.path, "/") + ": " + e.err.Error()
}

func (e *pathError) Unwrap() error { return e.err }

func within(segment string, err error) error {
	var pe *pathError
	if errors.As(err, &pe) {
		pe.path = append([]string{segment}, pe.path...)
		return pe
	}
	return &pathError{path: []string{segment}, err: err}
}

func appendValue(buf []byte, v any, pretty bool, depth int) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(buf, "null"...), nil
	case bool:
		return strconv.AppendBool(buf, x), nil
	case int:
		return strconv.AppendInt(buf, int64(x), 10), nil
	case int64:
		return strconv.AppendInt(buf, x, 10), nil
	case string:
		if x == "" {
			return nil, errEmptyString
		}
		if !utf8.ValidString(x) {
			return nil, fmt.Errorf("invalid UTF-8 in %q", x)
		}
		return appendString(buf, x), nil
	case []any:
		if len(x) == 0 {
			return append(buf, "[]"...), nil
		}
		buf = append(buf, '[')
		for i, elem := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = newline(buf, pretty, depth+1)
			var err error
			if buf, err = appendValue(buf, elem, pretty, depth+1); err != nil {
				return nil, within(strconv.Itoa(i), err)
			}
		}
		buf = newline(buf, pretty, depth)
		return append(buf, ']'), nil
	case map[string]any:
		if len(x) == 0 {
			return append(buf, "{}"...), nil
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		buf = append(buf, '{')
		for i, k := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = newline(buf, pretty, depth+1)
			buf = appendString(buf, k)
			buf = append(buf, ':')
			if pretty {
				buf = append(buf, ' ')
			}
			var err error
			if buf, err = appendValue(buf, x[k], pretty, depth+1); err != nil {
				return nil, within(pointerToken(k), err)
			}
		}
		buf = newline(buf, pretty, depth)
		return append(buf, '}'), nil
	default:
		return nil, fmt.Errorf("unsupported value type %T", v)
	}
}

func newline(buf []byte, pretty bool, depth int) []byte {
	if !pretty {
		return buf
	}
	buf = append(buf, '\n')
	for range depth {
		buf = append(buf, ' ', ' ')
	}
	return buf
}

const hexDigits = "0123456789abcdef"

// appendString writes s with RFC 8785 escaping: the two-character forms for
// quote, backslash, \b, \t, \n, \f, and \r; \u00xx (lowercase) for any other
// control character; every other code point literally.
func appendString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' {
			continue
		}
		buf = append(buf, s[start:i]...)
		switch c {
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '\b':
			buf = append(buf, '\\', 'b')
		case '\t':
			buf = append(buf, '\\', 't')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\f':
			buf = append(buf, '\\', 'f')
		case '\r':
			buf = append(buf, '\\', 'r')
		default:
			buf = append(buf, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
		}
		start = i + 1
	}
	buf = append(buf, s[start:]...)
	return append(buf, '"')
}

// pointerToken escapes one JSON Pointer reference token (RFC 6901).
func pointerToken(key string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(key)
}
