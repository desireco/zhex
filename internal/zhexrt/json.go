package zhexrt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Json is the host-predeclared recursive tree type: Null, Bool(value),
// Text(value), Number(literal), List(values), Object(fields). Variants
// keep the declaration order (Null first), which match emission relies
// on.
type Json struct {
	Kind   int // 0 Null, 1 Bool, 2 Text, 3 Number, 4 List, 5 Object
	Bool   bool
	Text   string // Text's value, or Number's literal
	List   *List[*Json]
	Object *Map[string, *Json]
}

// jsonFailure mirrors the interpreter's internal parse failure, so tree
// mode reports the same messages, codes, and details.
type jsonFailure struct {
	path, message    string
	code             string
	reason           string
	expected, actual string
	offset           int64
}

func (e *jsonFailure) Error() string { return e.message }

func jsonPath(path, key string) string {
	return path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

func jsonSyntaxError(dec *json.Decoder, path string, err error) error {
	offset := dec.InputOffset()
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		offset = syntax.Offset
	}
	return &jsonFailure{path: path, message: "invalid JSON: " + err.Error(), code: "json-syntax", offset: offset}
}

func jsonErr(pos Pos, op string, err error) *Error {
	f, ok := err.(*jsonFailure)
	if !ok {
		f = &jsonFailure{message: err.Error(), code: "json-syntax"}
	}
	details := map[string]string{"path": f.path}
	if f.reason != "" {
		details["reason"] = f.reason
	}
	if f.expected != "" {
		details["expected"] = f.expected
	}
	if f.actual != "" {
		details["actual"] = f.actual
	}
	if f.code == "json-syntax" {
		details["offset"] = strconv.FormatInt(f.offset, 10)
	}
	return &Error{Pos: pos, Code: f.code, Operation: op, Message: fmt.Sprintf("%s at %q", f.message, f.path), Details: details}
}

// JsonParse is json.parse: one JSON value as a Json tree.
func JsonParse(pos Pos, raw string) Result[*Json] {
	if !utf8.ValidString(raw) {
		return Result[*Json]{Err: jsonErr(pos, "json.parse", &jsonFailure{message: "JSON is not valid UTF-8", code: "json-syntax"})}
	}
	if err := validateJSONSurrogates(raw); err != nil {
		return Result[*Json]{Err: jsonErr(pos, "json.parse", err)}
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	tree, err := parseJSONTree(dec, "", 0)
	if err != nil {
		return Result[*Json]{Err: jsonErr(pos, "json.parse", err)}
	}
	if _, err = dec.Token(); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return Result[*Json]{Err: jsonErr(pos, "json.parse", &jsonFailure{message: err.Error(), code: "json-syntax", offset: dec.InputOffset()})}
	}
	return Ok(tree)
}

func parseJSONTree(dec *json.Decoder, path string, depth int) (*Json, error) {
	if depth > 256 {
		return nil, &jsonFailure{path: path, message: "JSON nesting limit exceeded", code: "json-depth"}
	}
	token, err := dec.Token()
	if err != nil {
		return nil, jsonSyntaxError(dec, path, err)
	}
	switch v := token.(type) {
	case nil:
		return &Json{Kind: 0}, nil
	case bool:
		return &Json{Kind: 1, Bool: v}, nil
	case string:
		return &Json{Kind: 2, Text: v}, nil
	case json.Number:
		return &Json{Kind: 3, Text: string(v)}, nil
	case json.Delim:
		if v == '[' {
			n := &Json{Kind: 4, List: &List[*Json]{}}
			for dec.More() {
				if len(n.List.Items) >= MaxElements {
					return nil, &jsonFailure{path: path, message: "list exceeds element limit", code: "json-range", reason: "list-limit"}
				}
				child, err := parseJSONTree(dec, fmt.Sprintf("%s/%d", path, len(n.List.Items)), depth+1)
				if err != nil {
					return nil, err
				}
				n.List.Items = append(n.List.Items, child)
			}
			if _, err := dec.Token(); err != nil {
				return nil, jsonSyntaxError(dec, path, err)
			}
			return n, nil
		}
		if v == '{' {
			n := &Json{Kind: 5, Object: NewMap[string, *Json]()}
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, jsonSyntaxError(dec, path, err)
				}
				name, ok := key.(string)
				if !ok {
					return nil, &jsonFailure{path: path, message: "invalid object key", code: "json-syntax", offset: dec.InputOffset()}
				}
				p := jsonPath(path, name)
				if seen[name] {
					return nil, &jsonFailure{path: p, message: "duplicate object key", code: "json-field", reason: "duplicate"}
				}
				seen[name] = true
				child, err := parseJSONTree(dec, p, depth+1)
				if err != nil {
					return nil, err
				}
				n.Object.put(name, child)
			}
			if _, err := dec.Token(); err != nil {
				return nil, jsonSyntaxError(dec, path, err)
			}
			return n, nil
		}
	}
	return nil, &jsonFailure{path: path, message: "invalid JSON value", code: "json-syntax", offset: dec.InputOffset()}
}

// JsonEncode is json.encode over a tree: compact, key order and number
// literals preserved byte-for-byte.
func JsonEncode(pos Pos, v *Json) Result[string] {
	var out bytes.Buffer
	if err := appendJSONTree(&out, v, "", 0); err != nil {
		return Result[string]{Err: jsonErr(pos, "json.encode", err)}
	}
	return Ok(out.String())
}

func appendJSONTree(out *bytes.Buffer, v *Json, path string, depth int) error {
	if depth > 256 {
		return &jsonFailure{path: path, message: "JSON nesting limit exceeded", code: "json-depth"}
	}
	switch v.Kind {
	case 0:
		out.WriteString("null")
	case 1:
		out.WriteString(strconv.FormatBool(v.Bool))
	case 2:
		if !utf8.ValidString(v.Text) {
			return &jsonFailure{path: path, message: "string is not valid UTF-8", code: "json-type", expected: "String", actual: "invalid UTF-8"}
		}
		b, _ := json.Marshal(v.Text)
		out.Write(b)
	case 3:
		literal := v.Text
		if !jsonNumberLiteral(literal) {
			return &jsonFailure{path: path, message: fmt.Sprintf("number literal %q is not valid JSON", literal), code: "json-type", expected: "number", actual: literal}
		}
		out.WriteString(literal)
	case 4:
		out.WriteByte('[')
		for i, item := range v.List.Items {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := appendJSONTree(out, item, fmt.Sprintf("%s/%d", path, i), depth+1); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	default:
		out.WriteByte('{')
		for i, item := range v.Object.Entries {
			if i > 0 {
				out.WriteByte(',')
			}
			if !utf8.ValidString(item.Key) {
				return &jsonFailure{path: path, message: "map key is not valid UTF-8", code: "json-type", expected: "String", actual: "invalid UTF-8"}
			}
			key, _ := json.Marshal(item.Key)
			out.Write(key)
			out.WriteByte(':')
			if err := appendJSONTree(out, item.Value, jsonPath(path, item.Key), depth+1); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	}
	return nil
}

// jsonNumberLiteral validates against the JSON number grammar, moved
// from the interpreter.
func jsonNumberLiteral(text string) bool {
	i := 0
	digits := func() bool {
		start := i
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			i++
		}
		return i > start
	}
	integer := func() bool {
		start := i
		if !digits() {
			return false
		}
		leadingZero := text[start] == '0' && i-start > 1
		return !leadingZero
	}
	if i < len(text) && text[i] == '-' {
		i++
	}
	if !integer() {
		return false
	}
	if i < len(text) && text[i] == '.' {
		i++
		if !digits() {
			return false
		}
	}
	if i < len(text) && (text[i] == 'e' || text[i] == 'E') {
		i++
		if i < len(text) && (text[i] == '+' || text[i] == '-') {
			i++
		}
		if !digits() {
			return false
		}
	}
	return i == len(text)
}

func validateJSONSurrogates(raw string) error {
	inString := false
	for i := 0; i < len(raw); i++ {
		if !inString {
			if raw[i] == '"' {
				inString = true
			}
			continue
		}
		if raw[i] == '"' {
			inString = false
			continue
		}
		if raw[i] != '\\' {
			continue
		}
		if i+1 >= len(raw) {
			break
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		if i+6 > len(raw) {
			break
		}
		first, err := strconv.ParseUint(raw[i+2:i+6], 16, 16)
		if err != nil {
			break
		}
		second, paired := uint64(0), false
		if i+12 <= len(raw) && raw[i+6:i+8] == "\\u" {
			second, err = strconv.ParseUint(raw[i+8:i+12], 16, 16)
			paired = err == nil
		}
		low := first >= 0xDC00 && first <= 0xDFFF
		high := first >= 0xD800 && first <= 0xDBFF
		if low || high && !(paired && second >= 0xDC00 && second <= 0xDFFF) {
			return &jsonFailure{message: fmt.Sprintf("unpaired surrogate \\u%04X in JSON string", first), code: "json-syntax"}
		}
		if high {
			i += 6
		} else {
			i += 5
		}
	}
	return nil
}
