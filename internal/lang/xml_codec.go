package lang

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The XML codec reuses the json schema and clone machinery: both decodes are
// schema-directed over the same supported types, so only the element-tree
// mapping and its diagnostics are XML-specific.

type xmlFailure struct {
	path, message    string
	code             Code
	reason           string
	expected, actual string
	offset           int64
}

func (e *xmlFailure) Error() string { return e.message }

func xmlFieldPath(path, name string) string { return path + "/" + name }

type xmlAttr struct {
	name  string
	value string
}
type xmlElement struct {
	name     string
	attrs    []xmlAttr
	text     string
	children []*xmlElement
}

const maxXMLDepth = 256

func xmlStackPath(names []string) string {
	return "/" + strings.Join(names, "/")
}

// parseXMLTree streams tokens with the strict stdlib decoder: predefined and
// numeric entities decode, undefined ones error, DTD directives are rejected,
// and nothing outside the input string is ever consulted.
func parseXMLTree(raw string) (*xmlElement, error) {
	dec := xml.NewDecoder(strings.NewReader(raw))
	var root *xmlElement
	var stack []*xmlElement
	elements := 0
	seenRoot := false
	for {
		tok, err := dec.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, &xmlFailure{message: "invalid XML: " + err.Error(), code: CodeXMLSyntax, offset: dec.InputOffset()}
		}
		switch v := tok.(type) {
		case xml.ProcInst:
			if seenRoot || len(stack) > 0 {
				return nil, &xmlFailure{message: "processing instruction outside the prolog", code: CodeXMLSyntax, offset: dec.InputOffset()}
			}
		case xml.Directive:
			return nil, &xmlFailure{message: "DTD and entity declarations are rejected", code: CodeXMLSyntax, offset: dec.InputOffset()}
		case xml.Comment:
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(v)
			}
		case xml.StartElement:
			if v.Name.Space != "" {
				return nil, &xmlFailure{path: xmlStackPath(namesOf(stack)), message: fmt.Sprintf("namespace-qualified name %q is not supported", v.Name.Local), code: CodeXMLField, reason: "prefixed", offset: dec.InputOffset()}
			}
			if seenRoot && len(stack) == 0 {
				return nil, &xmlFailure{path: xmlStackPath(namesOf(stack)), message: "multiple root elements", code: CodeXMLSyntax, offset: dec.InputOffset()}
			}
			elements++
			if elements > maxCollectionElements {
				return nil, &xmlFailure{path: xmlStackPath(namesOf(stack)), message: "document exceeds element limit", code: CodeXMLRange, reason: "element-limit"}
			}
			if len(stack) >= maxXMLDepth {
				return nil, &xmlFailure{path: xmlStackPath(namesOf(stack)), message: "XML nesting limit exceeded", code: CodeXMLDepth, offset: dec.InputOffset()}
			}
			node := &xmlElement{name: v.Name.Local}
			for _, a := range v.Attr {
				if a.Name.Space != "" {
					return nil, &xmlFailure{path: xmlFieldPath(xmlStackPath(namesOf(stack)), a.Name.Local), message: fmt.Sprintf("namespace-qualified name %q is not supported", a.Name.Local), code: CodeXMLField, reason: "prefixed", offset: dec.InputOffset()}
				}
				node.attrs = append(node.attrs, xmlAttr{a.Name.Local, a.Value})
			}
			if len(stack) == 0 {
				root = node
				seenRoot = true
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	if root == nil {
		return nil, &xmlFailure{message: "no root element", code: CodeXMLSyntax, offset: dec.InputOffset()}
	}
	return root, nil
}

func namesOf(stack []*xmlElement) []string {
	names := make([]string, len(stack))
	for i, node := range stack {
		names[i] = node.name
	}
	return names
}

// xmlIntegerForm is the integer-form shape: optional sign, digits only.
var xmlIntegerForm = regexp.MustCompile(`^[+-]?[0-9]+$`)

// exactXMLInt parses integer-form element or attribute text in the signed
// 128-bit range; integer-form text beyond the width is a range failure,
// anything else a type failure.
func exactXMLInt(text, path string) (i128, error) {
	trimmed := strings.TrimSpace(text)
	n, ok := i128parse(trimmed)
	if !ok {
		if xmlIntegerForm.MatchString(trimmed) {
			return i128{}, &xmlFailure{path: path, message: "expected an integer-form signed 128-bit number", code: CodeXMLRange, expected: "Int", actual: trimmed}
		}
		return i128{}, &xmlFailure{path: path, message: "expected Int", code: CodeXMLType, expected: "Int", actual: "text"}
	}
	return n, nil
}

// decodeXMLText decodes attribute or element text against a scalar or Option
// schema; structure-bearing schemas cannot come from text.
func decodeXMLText(text string, s *jsonSchema, def *value, memo *jsonClone, path string) (value, error) {
	if s.kind == "option" {
		var inner *value
		if def != nil && def.choice != nil && def.choice.variant == 1 {
			inner = &def.choice.fields[0]
		}
		v, err := decodeXMLText(text, s.element, inner, memo, path)
		if err != nil {
			return value{}, err
		}
		return value{choice: &choiceValue{name: "Option.Some", variant: 1, fields: []value{v}}}, nil
	}
	switch s.kind {
	case "int":
		n, err := exactXMLInt(text, path)
		if err != nil {
			return value{}, err
		}
		return value{number: n}, nil
	case "bool":
		switch strings.TrimSpace(text) {
		case "true":
			return value{boolean: true}, nil
		case "false":
			return value{boolean: false}, nil
		}
		return value{}, &xmlFailure{path: path, message: "expected Bool", code: CodeXMLType, expected: "Bool", actual: "text"}
	case "string":
		return value{text: text}, nil
	}
	return value{}, &xmlFailure{path: path, message: "expected " + string(s.typ), code: CodeXMLType, expected: string(s.typ), actual: "text"}
}

func decodeXMLValue(n *xmlElement, s *jsonSchema, def *value, memo *jsonClone, path string) (value, error) {
	fail := func(msg, actual string) (value, error) {
		return value{}, &xmlFailure{path: path, message: msg, code: CodeXMLType, expected: string(s.typ), actual: actual}
	}
	if s.kind == "option" {
		var inner *value
		if def != nil && def.choice != nil && def.choice.variant == 1 {
			inner = &def.choice.fields[0]
		}
		v, err := decodeXMLValue(n, s.element, inner, memo, path)
		if err != nil {
			return value{}, err
		}
		return value{choice: &choiceValue{name: "Option.Some", variant: 1, fields: []value{v}}}, nil
	}
	switch s.kind {
	case "int", "bool", "string":
		if len(n.attrs) > 0 {
			return fail("expected "+string(s.typ), "attribute")
		}
		if len(n.children) > 0 {
			return fail("expected "+string(s.typ), "element")
		}
		return decodeXMLText(n.text, s, nil, memo, path)
	case "list", "map":
		if len(n.attrs) > 0 {
			return fail("expected "+string(s.typ), "attribute")
		}
		if strings.TrimSpace(n.text) != "" {
			return fail("mixed content is not supported", "text")
		}
		if len(n.children) > maxCollectionElements {
			return value{}, &xmlFailure{path: path, message: "list exceeds element limit", code: CodeXMLRange, reason: "list-limit"}
		}
	}
	if s.kind == "list" {
		items := make([]value, len(n.children))
		for i, child := range n.children {
			v, err := decodeXMLValue(child, s.element, nil, memo, fmt.Sprintf("%s/%d", path, i))
			if err != nil {
				return value{}, err
			}
			items[i] = v
		}
		return value{list: &listValue{items: items}}, nil
	}
	if s.kind == "map" {
		m := &mapValue{keyType: stringType, index: map[mapKey]int{}}
		seen := map[string]bool{}
		for _, child := range n.children {
			if seen[child.name] {
				return value{}, &xmlFailure{path: xmlFieldPath(path, child.name), message: "duplicate map key", code: CodeXMLField, reason: "duplicate"}
			}
			seen[child.name] = true
			v, err := decodeXMLValue(child, s.element, nil, memo, xmlFieldPath(path, child.name))
			if err != nil {
				return value{}, err
			}
			m.put(value{text: child.name}, v)
		}
		return value{mapping: m}, nil
	}
	// record: attributes and child elements map onto fields verbatim, by name
	failField := func(name, msg, reason string) (value, error) {
		return value{}, &xmlFailure{path: xmlFieldPath(path, name), message: msg, code: CodeXMLField, reason: reason}
	}
	attrText := map[string]string{}
	childElements := map[string][]*xmlElement{}
	for _, a := range n.attrs {
		if _, ok := s.fieldIdx(a.name); !ok {
			return failField(a.name, fmt.Sprintf("unknown attribute %q", a.name), "unknown")
		}
		if _, ok := childElements[a.name]; ok {
			return failField(a.name, fmt.Sprintf("attribute %q collides with a child element", a.name), "collision")
		}
		attrText[a.name] = a.value
	}
	for _, child := range n.children {
		idx, ok := s.fieldIdx(child.name)
		if !ok {
			return failField(child.name, fmt.Sprintf("unknown element %q", child.name), "unknown")
		}
		if _, ok := attrText[child.name]; ok {
			return failField(child.name, fmt.Sprintf("element %q collides with an attribute", child.name), "collision")
		}
		if s.fields[idx].schema.kind != "list" && len(childElements[child.name]) > 0 {
			return failField(child.name, fmt.Sprintf("duplicate element %q", child.name), "duplicate")
		}
		childElements[child.name] = append(childElements[child.name], child)
	}
	if strings.TrimSpace(n.text) != "" {
		return fail("mixed content is not supported", "text")
	}
	out := &recordValue{name: string(s.typ), fields: make([]value, len(s.fields))}
	for i, f := range s.fields {
		var fallback *value
		if def != nil && def.record != nil {
			fallback = &def.record.fields[i]
		}
		p := xmlFieldPath(path, f.name)
		if text, ok := attrText[f.name]; ok {
			v, err := decodeXMLText(text, f.schema, fallback, memo, p)
			if err != nil {
				return value{}, err
			}
			out.fields[i] = v
			continue
		}
		if children := childElements[f.name]; len(children) > 0 {
			if f.schema.kind == "list" {
				items := make([]value, len(children))
				for j, child := range children {
					v, err := decodeXMLValue(child, f.schema.element, nil, memo, fmt.Sprintf("%s/%d", p, j))
					if err != nil {
						return value{}, err
					}
					items[j] = v
				}
				out.fields[i] = value{list: &listValue{items: items}}
				continue
			}
			v, err := decodeXMLValue(children[0], f.schema, fallback, memo, p)
			if err != nil {
				return value{}, err
			}
			out.fields[i] = v
			continue
		}
		if fallback == nil {
			return failField(f.name, fmt.Sprintf("missing required field %q", f.name), "missing")
		}
		out.fields[i] = memo.clone(*fallback)
	}
	return value{record: out}, nil
}

func xmlError(pos position, op string, err error, raw string) value {
	f, ok := err.(*xmlFailure)
	if !ok {
		f = &xmlFailure{message: err.Error(), code: CodeXMLSyntax}
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
	if f.code == CodeXMLSyntax {
		details["offset"] = strconv.FormatInt(f.offset, 10)
		if f.offset >= 0 && f.offset <= int64(len(raw)) {
			head := raw[:f.offset]
			line := strings.Count(head, "\n") + 1
			if i := strings.LastIndexByte(head, '\n'); i >= 0 {
				head = head[i+1:]
			}
			details["line"] = strconv.Itoa(line)
			details["column"] = strconv.Itoa(utf8.RuneCountInString(head) + 1)
		}
	}
	return failedCollection(pos, f.code, op, fmt.Sprintf("%s at %q", f.message, f.path), details)
}

func decodeXML(pos position, raw string, defaults value, s *jsonSchema) value {
	if !utf8.ValidString(raw) {
		return xmlError(pos, "xml.decode", &xmlFailure{message: "XML is not valid UTF-8", code: CodeXMLSyntax}, raw)
	}
	root, err := parseXMLTree(raw)
	if err != nil {
		return xmlError(pos, "xml.decode", err, raw)
	}
	v, err := decodeXMLValue(root, s, &defaults, newJSONClone(), "")
	if err != nil {
		return xmlError(pos, "xml.decode", err, raw)
	}
	return value{result: &inputResult{value: v}}
}
