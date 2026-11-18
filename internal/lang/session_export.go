package lang

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// exportFormat is the header of every Session.Export file.
const exportFormat = "# zhex session v1"

// exportSkipReason names the binding classes that cannot cross a process
// boundary: open files, function values, Errors, and Unit values.
const exportSkipReason = "not serializable (File, function, Error, or Unit value)"

// Export reconstructs the session's adopted state as one Zhex source file:
// use lines, type declarations and word definitions by their original source
// slices, annotated bindings as literals, and the installed reporter. It is
// engine-side serialization for the #65 prototype's repl.save/repl.load: a
// fresh session Enters the file and resumes with the same words and values.
// Buffers reconstruct losslessly through buffer.new plus one buffer.add-line
// per line, arrays through array.new plus indexed writes; a Buffer or Array
// nested inside another value skips the whole binding. In-flight entry scopes
// and per-run host words are not adopted state and never export.
func (s *Session) Export() (string, error) {
	var out strings.Builder
	out.WriteString(exportFormat + "\n")
	for _, use := range s.uses {
		out.WriteString("use " + use + "\n")
	}
	for _, name := range s.recordOrder {
		if record := s.records[name]; record != nil && !record.builtin && record.source != "" {
			out.WriteString(record.source)
		}
	}
	for _, name := range s.wordOrder {
		if w := s.words[name]; w != nil && w.host == nil && w.source != "" {
			out.WriteString(w.source)
		}
	}
	names := make([]string, 0, len(s.scope.bindings))
	for name := range s.scope.bindings {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		binding := s.scope.bindings[name]
		v, retained := s.frame[binding]
		if !retained {
			out.WriteString("# note: skipped " + name + ": no retained value\n")
			continue
		}
		statements, ok := s.exportBinding(name, binding, v)
		if !ok {
			out.WriteString("# note: skipped " + name + ": " + exportSkipReason + "\n")
			continue
		}
		for _, statement := range statements {
			out.WriteString(statement + "\n")
		}
	}
	if s.reporter != nil {
		out.WriteString("error.set-reporter(" + s.reporter.text + ")\n")
	}
	return out.String(), nil
}

// exportBinding renders one binding's reconstruction: a let/var statement
// plus any follow-up statements, in order. Buffers and arrays rebuild through
// follow-up statements, so they reconstruct only as named top-level bindings.
func (s *Session) exportBinding(name string, b *binding, v value) ([]string, bool) {
	keyword := "let"
	if b.mutable {
		keyword = "var"
	}
	switch b.typ {
	case bufferType:
		if v.buffer == nil {
			return nil, false
		}
		statements := []string{keyword + " " + name + ": Buffer = buffer.new()"}
		for _, line := range v.buffer.lines {
			statements = append(statements, "buffer.add-line("+name+", "+strconv.Quote(line)+")")
		}
		return statements, true
	}
	if kind, inner, ok := collectionInner(b.typ); ok && kind == "Array" {
		return s.exportArray(name, keyword, b.typ, inner, v)
	}
	literal, ok := s.literal(v, b.typ)
	if !ok {
		return nil, false
	}
	return []string{keyword + " " + name + ": " + string(b.typ) + " = " + literal}, true
}

// exportArray renders an array binding as array.new with the first element as
// fill, then one indexed write per differing element, reproducing the flat
// row-major contents exactly.
func (s *Session) exportArray(name, keyword string, typ, inner valueType, v value) ([]string, bool) {
	if v.array == nil {
		return nil, false
	}
	var fill string
	if len(v.array.items) == 0 {
		switch inner {
		case intType:
			fill = "0"
		case boolType:
			fill = "false"
		case stringType:
			fill = `""`
		default:
			return nil, false
		}
	} else {
		literal, ok := s.literal(v.array.items[0], inner)
		if !ok {
			return nil, false
		}
		fill = literal
	}
	dims := make([]string, len(v.array.shape))
	for i, n := range v.array.shape {
		dims[i] = strconv.FormatInt(n, 10)
	}
	statements := []string{keyword + " " + name + ": " + string(typ) + " = array.new([" + strings.Join(dims, ", ") + "], " + fill + ") try"}
	for i, item := range v.array.items {
		literal, ok := s.literal(item, inner)
		if !ok {
			return nil, false
		}
		if literal == fill {
			continue
		}
		coords := make([]string, len(v.array.shape))
		n := int64(i)
		for axis := len(coords) - 1; axis >= 0; axis-- {
			coords[axis] = strconv.FormatInt(n%v.array.shape[axis], 10)
			n /= v.array.shape[axis]
		}
		statements = append(statements, name+"["+strings.Join(coords, ", ")+"] = "+literal)
	}
	return statements, true
}

// literal renders a value as a Zhex expression of the given type; a Buffer
// or Array nested inside another value has no reconstruction path and rejects.
func (s *Session) literal(v value, typ valueType) (string, bool) {
	if v.function != nil {
		return "", false
	}
	switch typ {
	case intType:
		return i128string(v.number), true
	case boolType:
		return strconv.FormatBool(v.boolean), true
	case stringType:
		return strconv.Quote(v.text), true
	case unitType, errorType, fileType, bufferType:
		return "", false
	case dateType:
		return `date.parse("` + v.moment.Format("2006-01-02") + `") try`, true
	case timeType:
		return `time.parse("` + v.moment.Format("15:04:05.999999999") + `") try`, true
	case durationType:
		return "duration.new(" + i128string(v.number) + ")", true
	case instantType:
		return `instant.parse("` + v.moment.UTC().Format(time.RFC3339Nano) + `") try`, true
	case timezoneType:
		if v.zone == nil || v.zone.String() == "" || v.zone.String() == "Local" {
			return "", false
		}
		return `timezone.load(` + strconv.Quote(v.zone.String()) + `) try`, true
	case zonedType:
		civil := v.moment
		return `zoned.new(date.parse("` + civil.Format("2006-01-02") + `") try, time.parse("` +
			civil.Format("15:04:05.999999999") + `") try, timezone.load(` + strconv.Quote(civil.Location().String()) + `) try)`, true
	}
	if isResult(typ) {
		return "", false
	}
	base, args, applied := typeApplication(typ)
	name := base
	if !applied {
		name = string(typ)
	}
	if kind, inner, ok := collectionInner(typ); ok && kind == "List" {
		if v.list == nil {
			return "", false
		}
		items := make([]string, len(v.list.items))
		for i, item := range v.list.items {
			literal, ok := s.literal(item, inner)
			if !ok {
				return "", false
			}
			items[i] = literal
		}
		return "[" + strings.Join(items, ", ") + "]", true
	}
	if keyType, elementType, ok := mapTypes(typ); ok {
		if v.mapping == nil {
			return "", false
		}
		entries := make([]string, 0, len(v.mapping.entries))
		for _, entry := range v.mapping.entries {
			key, okKey := s.literal(v.mapping.keyValue(entry.key), keyType)
			element, okValue := s.literal(entry.value, elementType)
			if !okKey || !okValue {
				return "", false
			}
			entries = append(entries, key+": "+element)
		}
		return "{" + strings.Join(entries, ", ") + "}", true
	}
	record := s.choiceOrRecord(name)
	if record == nil {
		return "", false
	}
	if record.alias {
		return s.literal(v, record.aliasTarget)
	}
	if record.choice {
		return s.literalChoice(v, record, args, applied)
	}
	if v.record == nil || len(v.record.fields) != len(record.fields) {
		return "", false
	}
	fields := make([]string, len(record.fields))
	for i, field := range record.fields {
		literal, ok := s.literal(v.record.fields[i], field.typ)
		if !ok {
			return "", false
		}
		fields[i] = field.name + ": " + literal
	}
	return record.id() + "(" + strings.Join(fields, ", ") + ")", true
}

// choiceOrRecord resolves a nominal type name against the session's visible
// types, then the host-predeclared Option and Json choices.
func (s *Session) choiceOrRecord(name string) *recordType {
	if record := s.records[name]; record != nil {
		return record
	}
	if name == "Option" {
		return builtinOption
	}
	if name == "Json" {
		return builtinJSON
	}
	return nil
}

// literalChoice renders a choice value as its variant constructor, applying
// the type argument to generic variant payloads (Option; Json is concrete).
func (s *Session) literalChoice(v value, record *recordType, args []valueType, applied bool) (string, bool) {
	if v.choice == nil {
		return "", false
	}
	tail := v.choice.name[strings.LastIndex(v.choice.name, ".")+1:]
	variant := -1
	for i, candidate := range record.variants {
		if candidate.name == tail {
			variant = i
			break
		}
	}
	if variant < 0 || len(v.choice.fields) != len(record.variants[variant].fields) {
		return "", false
	}
	fields := make([]string, len(v.choice.fields))
	for i, field := range v.choice.fields {
		fieldType := record.variants[variant].fields[i].typ
		if record.typeParam != "" && applied && len(args) == 1 {
			fieldType = substituteType(fieldType, record.typeParam, args[0])
		}
		literal, ok := s.literal(field, fieldType)
		if !ok {
			return "", false
		}
		fields[i] = literal
	}
	if len(fields) == 0 {
		return record.id() + "." + record.variants[variant].name, true
	}
	return record.id() + "." + record.variants[variant].name + "(" + strings.Join(fields, ", ") + ")", true
}
