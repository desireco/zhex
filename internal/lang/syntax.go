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
	pos      position
	code     Code
	severity string
	message  string
	hint     string
	line     string
}

// Error keeps the first line stable — file:line:column: message — so
// scripts and agents parsing it keep working; the source excerpt with a
// caret and the hint are additive lines.
func (d *diagnostic) Error() string {
	out := fmt.Sprintf("%s:%d:%d: %s", d.pos.file, d.pos.line, d.pos.column, d.message)
	if d.line != "" {
		out += "\n    " + d.line + "\n    " + strings.Repeat(" ", d.pos.column-1) + "^"
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
		Code     string `json:"code"`
		Severity string `json:"severity"`
		File     string `json:"file"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
		Message  string `json:"message"`
		Hint     string `json:"hint"`
	}{string(d.code), d.severity, d.pos.file, d.pos.line, d.pos.column, d.message, d.hint}
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

type valueType string

const (
	intType          valueType = "Int"
	boolType         valueType = "Bool"
	stringType       valueType = "String"
	unitType         valueType = "Unit"
	intResultType    valueType = "Result<Int, InputError>"
	stringResultType valueType = "Result<String, InputError>"
)

type binding struct {
	name    string
	typ     valueType
	mutable bool
}

type parameter struct {
	name    string
	typ     valueType
	pos     position
	binding *binding
}

type word struct {
	name   string
	pos    position
	params []parameter
	result valueType
	body   []*statement
}

// Program is a checked source file. Syntax, type checking, and execution are separate stages.
type Program struct {
	words  []*word
	body   []*statement
	source string
}

type expression struct {
	kind        string
	pos         position
	text        string
	number      int64
	depth       int
	left, right *expression
	args        []*expression
	typ         valueType
	inputType   valueType
	binding     *binding
	word        *word
}

type statement struct {
	kind            string
	pos             position
	name            string
	expr, end       *expression
	body, otherwise []*statement
	steps           []*expression
	binding         *binding
}
