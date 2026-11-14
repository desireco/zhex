package lang

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

type jsonField struct {
	name   string
	schema *jsonSchema
}
type jsonSchema struct {
	typ     valueType
	kind    string
	fields  []jsonField
	element *jsonSchema
}

func (c *checker) makeJSONSchema(typ valueType, active map[valueType]bool) (*jsonSchema, error) {
	return c.buildJSONSchema(typ, active, make(map[valueType]*jsonSchema))
}
func (c *checker) buildJSONSchema(typ valueType, active map[valueType]bool, cache map[valueType]*jsonSchema) (*jsonSchema, error) {
	if active[typ] {
		return nil, fmt.Errorf("recursive type")
	}
	if schema := cache[typ]; schema != nil {
		return schema, nil
	}
	active[typ] = true
	defer delete(active, typ)
	s := &jsonSchema{typ: typ}
	cache[typ] = s
	switch typ {
	case intType:
		s.kind = "int"
		return s, nil
	case boolType:
		s.kind = "bool"
		return s, nil
	case stringType:
		s.kind = "string"
		return s, nil
	}
	if kind, inner, ok := collectionInner(typ); ok {
		if kind != "List" {
			return nil, fmt.Errorf("unsupported collection")
		}
		s.kind = "list"
		v, err := c.buildJSONSchema(inner, active, cache)
		s.element = v
		return s, err
	}
	if key, val, ok := mapTypes(typ); ok {
		if key != stringType {
			return nil, fmt.Errorf("non-string map key")
		}
		s.kind = "map"
		v, err := c.buildJSONSchema(val, active, cache)
		s.element = v
		return s, err
	}
	if base, args, ok := typeApplication(typ); ok && base == "Option" && len(args) == 1 {
		s.kind = "option"
		v, err := c.buildJSONSchema(args[0], active, cache)
		s.element = v
		if err == nil && v.kind == "option" {
			return nil, fmt.Errorf("nested Option cannot round-trip through JSON null")
		}
		return s, err
	}
	if record := c.types.all[string(typ)]; record != nil && !record.choice {
		s.kind = "record"
		for _, field := range record.fields {
			f, err := c.buildJSONSchema(field.typ, active, cache)
			if err != nil {
				return nil, err
			}
			s.fields = append(s.fields, jsonField{field.name, f})
		}
		return s, nil
	}
	return nil, fmt.Errorf("unsupported type")
}

type jsonMember struct {
	key string
	val *jsonNode
}
type jsonNode struct {
	kind    string
	text    string
	boolean bool
	array   []*jsonNode
	object  []jsonMember
}
type jsonFailure struct {
	path, message    string
	code             Code
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
	return &jsonFailure{path: path, message: "invalid JSON: " + err.Error(), code: CodeJSONSyntax, offset: offset}
}
func parseJSON(dec *json.Decoder, path string, depth int) (*jsonNode, error) {
	if depth > 256 {
		return nil, &jsonFailure{path: path, message: "JSON nesting limit exceeded", code: CodeJSONDepth}
	}
	token, err := dec.Token()
	if err != nil {
		return nil, jsonSyntaxError(dec, path, err)
	}
	switch v := token.(type) {
	case nil:
		return &jsonNode{kind: "null"}, nil
	case bool:
		return &jsonNode{kind: "bool", boolean: v}, nil
	case string:
		return &jsonNode{kind: "string", text: v}, nil
	case json.Number:
		return &jsonNode{kind: "number", text: string(v)}, nil
	case json.Delim:
		if v == '[' {
			n := &jsonNode{kind: "array"}
			for dec.More() {
				child, err := parseJSON(dec, fmt.Sprintf("%s/%d", path, len(n.array)), depth+1)
				if err != nil {
					return nil, err
				}
				n.array = append(n.array, child)
			}
			if _, err := dec.Token(); err != nil {
				return nil, jsonSyntaxError(dec, path, err)
			}
			return n, nil
		}
		if v == '{' {
			n := &jsonNode{kind: "object"}
			seen := map[string]bool{}
			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, jsonSyntaxError(dec, path, err)
				}
				name, ok := key.(string)
				if !ok {
					return nil, &jsonFailure{path: path, message: "invalid object key", code: CodeJSONSyntax, offset: dec.InputOffset()}
				}
				p := jsonPath(path, name)
				if seen[name] {
					return nil, &jsonFailure{path: p, message: "duplicate object key", code: CodeJSONField, reason: "duplicate"}
				}
				seen[name] = true
				child, err := parseJSON(dec, p, depth+1)
				if err != nil {
					return nil, err
				}
				n.object = append(n.object, jsonMember{name, child})
			}
			if _, err := dec.Token(); err != nil {
				return nil, jsonSyntaxError(dec, path, err)
			}
			return n, nil
		}
	}
	return nil, &jsonFailure{path: path, message: "invalid JSON value", code: CodeJSONSyntax, offset: dec.InputOffset()}
}

// exactJSONInt accepts only integer-form JSON tokens in the signed 64-bit range.
func exactJSONInt(text string) (int64, bool) {
	if strings.ContainsAny(text, ".eE") {
		return 0, false
	}
	n, err := strconv.ParseInt(text, 10, 64)
	return n, err == nil
}

type jsonClone struct {
	lists   map[*listValue]*listValue
	maps    map[*mapValue]*mapValue
	records map[*recordValue]*recordValue
	choices map[*choiceValue]*choiceValue
}

func newJSONClone() *jsonClone {
	return &jsonClone{lists: map[*listValue]*listValue{}, maps: map[*mapValue]*mapValue{}, records: map[*recordValue]*recordValue{}, choices: map[*choiceValue]*choiceValue{}}
}
func (m *jsonClone) clone(v value) value {
	if v.list != nil {
		if old := m.lists[v.list]; old != nil {
			v.list = old
			return v
		}
		src := v.list
		dst := &listValue{items: make([]value, len(src.items))}
		m.lists[src] = dst
		v.list = dst
		for i, item := range src.items {
			dst.items[i] = m.clone(item)
		}
		return v
	}
	if v.mapping != nil {
		if old := m.maps[v.mapping]; old != nil {
			v.mapping = old
			return v
		}
		src := v.mapping
		dst := &mapValue{keyType: src.keyType, index: make(map[mapKey]int), entries: make([]mapEntry, len(src.entries))}
		m.maps[src] = dst
		v.mapping = dst
		for i, e := range src.entries {
			dst.entries[i] = mapEntry{key: e.key, value: m.clone(e.value)}
			dst.index[e.key] = i
		}
		return v
	}
	if v.record != nil {
		if old := m.records[v.record]; old != nil {
			v.record = old
			return v
		}
		src := v.record
		dst := &recordValue{name: src.name, fields: make([]value, len(src.fields))}
		m.records[src] = dst
		v.record = dst
		for i, item := range src.fields {
			dst.fields[i] = m.clone(item)
		}
		return v
	}
	if v.choice != nil {
		if old := m.choices[v.choice]; old != nil {
			v.choice = old
			return v
		}
		src := v.choice
		dst := &choiceValue{name: src.name, variant: src.variant, fields: make([]value, len(src.fields))}
		m.choices[src] = dst
		v.choice = dst
		for i, item := range src.fields {
			dst.fields[i] = m.clone(item)
		}
		return v
	}
	return v
}

func decodeJSONValue(n *jsonNode, s *jsonSchema, def *value, memo *jsonClone, path string) (value, error) {
	fail := func(msg string) (value, error) {
		return value{}, &jsonFailure{path: path, message: msg, code: CodeJSONType, expected: string(s.typ), actual: n.kind}
	}
	if s.kind == "option" {
		if n.kind == "null" {
			return value{choice: &choiceValue{name: "Option.None", variant: 0}}, nil
		}
		var inner *value
		if def != nil && def.choice != nil && def.choice.variant == 1 {
			inner = &def.choice.fields[0]
		}
		v, err := decodeJSONValue(n, s.element, inner, memo, path)
		if err != nil {
			return value{}, err
		}
		return value{choice: &choiceValue{name: "Option.Some", variant: 1, fields: []value{v}}}, nil
	}
	switch s.kind {
	case "int":
		if n.kind != "number" {
			return fail("expected Int")
		}
		v, ok := exactJSONInt(n.text)
		if !ok {
			return value{}, &jsonFailure{path: path, message: "expected an integer-form signed 64-bit number", code: CodeJSONRange, expected: "Int", actual: "number"}
		}
		return value{number: v}, nil
	case "bool":
		if n.kind != "bool" {
			return fail("expected Bool")
		}
		return value{boolean: n.boolean}, nil
	case "string":
		if n.kind != "string" {
			return fail("expected String")
		}
		return value{text: n.text}, nil
	case "list":
		if n.kind != "array" {
			return fail("expected array")
		}
		if len(n.array) > maxCollectionElements {
			return value{}, &jsonFailure{path: path, message: "list exceeds element limit", code: CodeJSONRange, reason: "list-limit"}
		}
		items := make([]value, len(n.array))
		for i, item := range n.array {
			v, err := decodeJSONValue(item, s.element, nil, memo, fmt.Sprintf("%s/%d", path, i))
			if err != nil {
				return value{}, err
			}
			items[i] = v
		}
		return value{list: &listValue{items: items}}, nil
	case "map":
		if n.kind != "object" {
			return fail("expected object")
		}
		m := &mapValue{keyType: stringType, index: map[mapKey]int{}}
		for _, item := range n.object {
			v, err := decodeJSONValue(item.val, s.element, nil, memo, jsonPath(path, item.key))
			if err != nil {
				return value{}, err
			}
			m.put(value{text: item.key}, v)
		}
		return value{mapping: m}, nil
	case "record":
		if n.kind != "object" {
			return fail("expected object")
		}
		lookup := map[string]*jsonNode{}
		fields := map[string]bool{}
		for _, f := range s.fields {
			fields[f.name] = true
		}
		for _, item := range n.object {
			if !fields[item.key] {
				return value{}, &jsonFailure{path: jsonPath(path, item.key), message: "unknown field", code: CodeJSONField, reason: "unknown"}
			}
			lookup[item.key] = item.val
		}
		out := &recordValue{name: string(s.typ), fields: make([]value, len(s.fields))}
		for i, f := range s.fields {
			node, present := lookup[f.name]
			var fallback *value
			if def != nil && def.record != nil {
				fallback = &def.record.fields[i]
			}
			if !present {
				if fallback == nil {
					return value{}, &jsonFailure{path: jsonPath(path, f.name), message: "missing required field", code: CodeJSONField, reason: "missing"}
				}
				out.fields[i] = memo.clone(*fallback)
				continue
			}
			v, err := decodeJSONValue(node, f.schema, fallback, memo, jsonPath(path, f.name))
			if err != nil {
				return value{}, err
			}
			out.fields[i] = v
		}
		return value{record: out}, nil
	}
	return fail("unsupported JSON type")
}
func jsonError(pos position, op string, err error) value {
	f, ok := err.(*jsonFailure)
	if !ok {
		f = &jsonFailure{message: err.Error(), code: CodeJSONSyntax}
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
	if f.code == CodeJSONSyntax {
		details["offset"] = strconv.FormatInt(f.offset, 10)
	}
	return failedCollection(pos, f.code, op, fmt.Sprintf("%s at %q", f.message, f.path), details)
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
		if first >= 0xDC00 && first <= 0xDFFF {
			return &jsonFailure{message: "unpaired UTF-16 surrogate", code: CodeJSONSyntax, offset: int64(i + 1)}
		}
		if first >= 0xD800 && first <= 0xDBFF {
			if i+12 > len(raw) || raw[i+6] != '\\' || raw[i+7] != 'u' {
				return &jsonFailure{message: "unpaired UTF-16 surrogate", code: CodeJSONSyntax, offset: int64(i + 1)}
			}
			second, err := strconv.ParseUint(raw[i+8:i+12], 16, 16)
			if err != nil || second < 0xDC00 || second > 0xDFFF {
				return &jsonFailure{message: "unpaired UTF-16 surrogate", code: CodeJSONSyntax, offset: int64(i + 1)}
			}
			i += 11
			continue
		}
		i += 5
	}
	return nil
}
func decodeJSON(pos position, raw string, defaults value, s *jsonSchema) value {
	if !utf8.ValidString(raw) {
		return jsonError(pos, "json.decode", &jsonFailure{message: "JSON is not valid UTF-8", code: CodeJSONSyntax})
	}
	if err := validateJSONSurrogates(raw); err != nil {
		return jsonError(pos, "json.decode", err)
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	node, err := parseJSON(dec, "", 0)
	if err != nil {
		return jsonError(pos, "json.decode", err)
	}
	if _, err = dec.Token(); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return jsonError(pos, "json.decode", &jsonFailure{message: err.Error(), code: CodeJSONSyntax, offset: dec.InputOffset()})
	}
	v, err := decodeJSONValue(node, s, &defaults, newJSONClone(), "")
	if err != nil {
		return jsonError(pos, "json.decode", err)
	}
	return value{result: &inputResult{value: v}}
}
func appendJSON(out *bytes.Buffer, v value, s *jsonSchema, path string, depth int) error {
	if depth > 256 {
		return &jsonFailure{path: path, message: "JSON nesting limit exceeded", code: CodeJSONDepth}
	}
	switch s.kind {
	case "int":
		out.WriteString(strconv.FormatInt(v.number, 10))
	case "bool":
		out.WriteString(strconv.FormatBool(v.boolean))
	case "string":
		if !utf8.ValidString(v.text) {
			return &jsonFailure{path: path, message: "string is not valid UTF-8", code: CodeJSONType, expected: "String", actual: "invalid UTF-8"}
		}
		b, _ := json.Marshal(v.text)
		out.Write(b)
	case "option":
		if v.choice.variant == 0 {
			out.WriteString("null")
		} else {
			return appendJSON(out, v.choice.fields[0], s.element, path, depth)
		}
	case "list":
		out.WriteByte('[')
		for i, item := range v.list.items {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := appendJSON(out, item, s.element, fmt.Sprintf("%s/%d", path, i), depth+1); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case "map":
		out.WriteByte('{')
		for i, item := range v.mapping.entries {
			if i > 0 {
				out.WriteByte(',')
			}
			if !utf8.ValidString(item.key.text) {
				return &jsonFailure{path: path, message: "map key is not valid UTF-8", code: CodeJSONType, expected: "String", actual: "invalid UTF-8"}
			}
			key, _ := json.Marshal(item.key.text)
			out.Write(key)
			out.WriteByte(':')
			if err := appendJSON(out, item.value, s.element, jsonPath(path, item.key.text), depth+1); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	case "record":
		out.WriteByte('{')
		for i, f := range s.fields {
			if i > 0 {
				out.WriteByte(',')
			}
			key, _ := json.Marshal(f.name)
			out.Write(key)
			out.WriteByte(':')
			if err := appendJSON(out, v.record.fields[i], f.schema, jsonPath(path, f.name), depth+1); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	default:
		return &jsonFailure{path: path, message: "unsupported JSON type", code: CodeJSONType}
	}
	return nil
}
func encodeJSON(pos position, v value, s *jsonSchema) value {
	var out bytes.Buffer
	if err := appendJSON(&out, v, s, "", 0); err != nil {
		return jsonError(pos, "json.encode", err)
	}
	return value{result: &inputResult{value: value{text: out.String()}}}
}
