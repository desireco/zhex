package lang

import (
	"encoding/json"
	"fmt"
	"strings"
)

type position struct {
	file         string
	line, column int
}

type diagnostic struct {
	pos        position
	code       Code
	severity   string
	message    string
	hint       string
	line       string
	spanStart  position
	spanEnd    position
	operation  string
	details    map[string]string
	cause      *diagnostic
	occurrence *errorOccurrence
}

// Error keeps the first line stable — file:line:column: message — so
// scripts and agents parsing it keep working; the source excerpt with a
// caret and the hint are additive lines.
func (d *diagnostic) Error() string {
	out := fmt.Sprintf("%s:%d:%d: %s", d.pos.file, d.pos.line, d.pos.column, d.message)
	if d.line != "" {
		out += "\n    " + d.line + "\n    "
		indent, width := d.pos.column-1, 1
		if d.spanStart.column > 0 && d.spanStart.line == d.spanEnd.line && d.spanEnd.column-d.spanStart.column > 1 {
			indent, width = d.spanStart.column-1, d.spanEnd.column-d.spanStart.column
		}
		if indent < 0 {
			indent = 0
		}
		if width < 1 {
			width = 1
		}
		out += strings.Repeat(" ", indent) + strings.Repeat("^", width)
	}
	if d.hint != "" {
		out += "\nhint: " + d.hint
	}
	return out
}

func errorAt(pos position, code Code, format string, args ...any) error {
	return &diagnostic{pos: pos, code: code, severity: "error", message: fmt.Sprintf(format, args...)}
}

// Report renders err as the machine-readable diagnostic report — one JSON
// object per run, per the contract in docs/diagnostics-plan.md. ok is false
// when err did not come from this package.
func Report(err error) (string, bool) {
	d, ok := err.(*diagnostic)
	if !ok {
		return "", false
	}
	report := struct {
		Code            string `json:"code"`
		Severity        string `json:"severity"`
		File            string `json:"file"`
		Line            int    `json:"line"`
		Column          int    `json:"column"`
		Message         string `json:"message"`
		Hint            string `json:"hint"`
		SpanStartColumn int    `json:"span_start_column,omitempty"`
		SpanEndColumn   int    `json:"span_end_column,omitempty"`
	}{string(d.code), d.severity, d.pos.file, d.pos.line, d.pos.column, d.message, d.hint, d.spanStart.column, d.spanEnd.column}
	out, err := json.Marshal(report)
	if err != nil {
		return "", false
	}
	return string(out), true
}

// annotate attaches the source line so Error can point at the mistake.
func annotate(err error, source string) error {
	d, ok := err.(*diagnostic)
	if !ok || d.line != "" {
		return err
	}
	lines := strings.Split(source, "\n")
	if d.pos.line >= 1 && d.pos.line <= len(lines) {
		d.line = strings.TrimSuffix(lines[d.pos.line-1], "\r")
	}
	return d
}

func withHint(err error, hint string) error {
	if d, ok := err.(*diagnostic); ok && hint != "" {
		d.hint = hint
	}
	return err
}

// withSpanRange attaches a same-line caret span wider than one token; the
// first line keeps reporting its own position regardless.
func withSpanRange(err error, start, end position) error {
	if d, ok := err.(*diagnostic); ok && start.line == end.line && end.column > start.column {
		d.spanStart, d.spanEnd = start, end
	}
	return err
}

// withSpan spans the whole expression: from the leftmost operand (or the
// leading operator, for unary) to its end.
func withSpan(err error, x *expression) error {
	start := x.pos
	for left := x.left; left != nil; left = left.left {
		if left.pos.line < start.line || left.pos.line == start.line && left.pos.column < start.column {
			start = left.pos
		}
	}
	return withSpanRange(err, start, x.end)
}

func isResult(t valueType) bool { return strings.HasPrefix(string(t), "Result<") }

// resultInner maps a Result type to its success type; ok is false for
// non-Result types. The one mapping behind try, traceValue, and the hint.
func resultParts(t valueType) (valueType, string, bool) {
	text := string(t)
	if !strings.HasPrefix(text, "Result<") || !strings.HasSuffix(text, ">") {
		return "", "", false
	}
	body := strings.TrimSuffix(strings.TrimPrefix(text, "Result<"), ">")
	depth := 0
	for i, r := range body {
		switch r {
		case '<':
			depth++
		case '>':
			depth--
			if depth < 0 {
				return "", "", false
			}
		case ',':
			if depth == 0 {
				inner := strings.TrimSpace(body[:i])
				family := strings.TrimSpace(body[i+1:])
				if inner == "" || family == "" {
					return "", "", false
				}
				return valueType(inner), family, true
			}
		}
	}
	return "", "", false
}
func resultInner(t valueType) (valueType, bool) { inner, _, ok := resultParts(t); return inner, ok }
func resultFamily(t valueType) string           { _, family, _ := resultParts(t); return family }

func collectionInner(typ valueType) (string, valueType, bool) {
	text := string(typ)
	for _, kind := range []string{"List", "Array"} {
		prefix := kind + "<"
		if strings.HasPrefix(text, prefix) && strings.HasSuffix(text, ">") {
			return kind, valueType(text[len(prefix) : len(text)-1]), true
		}
	}
	return "", "", false
}

// originOf reports the word a value's binding was defined from, for
// provenance narration.
func originOf(x *expression) string {
	switch x.kind {
	case "call":
		return x.text
	case "try":
		return originOf(x.left)
	case "name":
		if x.binding != nil {
			return x.binding.origin
		}
	}
	return ""
}

// narrate explains where a mismatched value came from, for the first named
// operand whose binding has a known origin.
func narrate(xs ...*expression) string {
	for _, x := range xs {
		if x.kind == "name" && x.binding != nil && x.binding.origin != "" {
			return fmt.Sprintf("%s comes from %s", x.text, x.binding.origin)
		}
	}
	return ""
}

// narrated appends the provenance clause to a diagnostic message.
func narrated(msg, narration string) string {
	if narration != "" {
		return msg + "; " + narration
	}
	return msg
}

type valueType string

const (
	intType                  valueType = "Int"
	boolType                 valueType = "Bool"
	stringType               valueType = "String"
	unitType                 valueType = "Unit"
	valueTypeResultUnitError valueType = "Result<Unit, Error>"
	bufferType               valueType = "Buffer"
	fileType                 valueType = "File"
	errorType                valueType = "Error"
	neverType                valueType = "<never>"
	intResultType            valueType = "Result<Int, InputError>"
	stringResultType         valueType = "Result<String, InputError>"
	intParseResultType       valueType = "Result<Int, ParseError>"
	// inputResult is the runtime repr for every Result, whatever its error type.
	bufferFileResultType   valueType = "Result<Buffer, FileError>"
	stringBufferResultType valueType = "Result<String, BufferError>"
	unitBufferResultType   valueType = "Result<Unit, BufferError>"
	stringFileResultType   valueType = "Result<String, FileError>"
	fileResultType         valueType = "Result<File, FileError>"
)

type binding struct {
	name        string
	typ         valueType
	mutable     bool
	consumed    bool   // a stored Result can be handled only once
	errorFamily string // concrete Result error family for a catch binding
	origin      string // defining word for provenance narration, e.g. read-line
}

type parameter struct {
	name        string
	typ         valueType
	pos         position
	binding     *binding
	defaultExpr *expression
}

type word struct {
	name     string
	module   string // owning module's use name; "" in the entry file and the REPL
	pos      position
	params   []parameter
	result   valueType
	success  valueType // declared success type of an inferred fallible word
	inferred bool
	body     []*statement
}

type recordType struct {
	module        string
	name          string
	pos           position
	fields        []parameter
	choice        bool
	typeParam     string
	alias         bool
	aliasTarget   valueType
	aliasSource   valueType
	aliasResolved bool
	variants      []choiceVariant
}

type choiceVariant struct {
	name   string
	pos    position
	fields []parameter
}
type matchArm struct {
	pos      position
	name     string
	names    []string
	bindings []*binding
	variant  int
	body     []*statement
}

func (r *recordType) id() string {
	if r.module == "" {
		return r.name
	}
	return r.module + "." + r.name
}

// A use directive imports a module: use math.
type use struct {
	name string
	pos  position // the use token
	end  position // end of the module name, for the caret span
}

// Program is a checked source file. Syntax, type checking, and execution are separate stages.
type Program struct {
	words   []*word
	records []*recordType
	uses    []use
	body    []*statement
	sources map[string]string // every file in the program, keyed by path, for annotated errors
}

type expression struct {
	kind         string
	pos          position
	end          position // exclusive, one past the last rune
	text         string
	number       int64
	depth        int
	left, right  *expression
	args         []*expression
	typ          valueType
	inputType    valueType
	binding      *binding
	word         *word
	record       *recordType
	fields       []int
	arms         []matchArm
	variant      int
	typeArg      valueType
	handler      []*statement
	propagate    bool
	plainWrite   bool
	checkedIndex bool
	jsonSchema   *jsonSchema
}

type statement struct {
	kind            string
	pos             position
	name            string
	secondName      string
	secondBinding   *binding
	annotation      valueType
	expr, end       *expression
	body, otherwise []*statement
	steps           []*expression
	binding         *binding
}
