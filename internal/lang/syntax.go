package lang

import "fmt"

type position struct {
	file         string
	line, column int
}

type diagnostic struct {
	pos     position
	message string
}

func (d *diagnostic) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s", d.pos.file, d.pos.line, d.pos.column, d.message)
}

func errorAt(pos position, format string, args ...any) error {
	return &diagnostic{pos, fmt.Sprintf(format, args...)}
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
	words []*word
	body  []*statement
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
