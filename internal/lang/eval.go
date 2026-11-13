package lang

import (
	"bufio"
	"io"
	"math"
)

type value struct {
	number  int64
	boolean bool
	text    string
	result  *inputResult
}

type inputResult struct {
	value value
	err   error
}

type frame map[*binding]value

type runtime struct {
	input  *bufio.Reader
	output io.Writer
	trace  io.Writer // nil disables the JSONL step-event stream
	depth  int
}

// Run checks a file before executing it against the supplied console streams.
func Run(file, source string, input io.Reader, output io.Writer) error {
	return RunWithTrace(file, source, input, output, nil)
}

// RunWithTrace is Run with JSONL step events written to trace; nil trace
// leaves the run identical to Run.
func RunWithTrace(file, source string, input io.Reader, output, trace io.Writer) error {
	p, err := Compile(file, source)
	if err != nil {
		return annotate(err, source)
	}
	return p.RunWithTrace(input, output, trace)
}

// Run executes an already checked program with fresh local state.
func (p *Program) Run(input io.Reader, output io.Writer) error {
	return p.RunWithTrace(input, output, nil)
}

// RunWithTrace executes like Run and writes step events to trace when non-nil.
func (p *Program) RunWithTrace(input io.Reader, output, trace io.Writer) error {
	reader, ok := input.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(input)
	}
	r := runtime{input: reader, output: output, trace: trace}
	_, err := r.block(p.body, make(frame))
	return annotate(err, p.source)
}

func (r *runtime) block(body []*statement, env frame) (value, error) {
	var result value
	for _, s := range body {
		var err error
		result, err = r.statement(s, env)
		if err != nil {
			return value{}, err
		}
	}
	return result, nil
}

func (r *runtime) statement(s *statement, env frame) (value, error) {
	v, err := r.expression(s.expr, env)
	if err != nil {
		return value{}, err
	}
	switch s.kind {
	case "expression":
		return v, nil
	case "let", "var", "assign":
		env[s.binding] = v
		r.traceBinding(s.pos, s.binding, v)
	case "if":
		if v.boolean {
			return r.block(s.body, env)
		}
		return r.block(s.otherwise, env)
	case "for":
		end, err := r.expression(s.end, env)
		if err != nil {
			return value{}, err
		}
		for n := v.number; n < end.number; n++ {
			env[s.binding] = value{number: n}
			r.traceBinding(s.pos, s.binding, value{number: n})
			if _, err := r.block(s.body, env); err != nil {
				return value{}, err
			}
		}
	case "flow":
		for _, step := range s.steps {
			r.traceFlowStep(step, v)
			v, err = r.call(step, []value{v})
			if err != nil {
				return value{}, err
			}
		}
		return v, nil
	}
	return value{}, nil
}

func (r *runtime) expression(x *expression, env frame) (value, error) {
	switch x.kind {
	case "int":
		return value{number: x.number}, nil
	case "bool":
		return value{boolean: x.text == "true"}, nil
	case "string":
		return value{text: x.text}, nil
	case "name":
		return env[x.binding], nil
	case "call":
		args := make([]value, len(x.args))
		for i, arg := range x.args {
			var err error
			args[i], err = r.expression(arg, env)
			if err != nil {
				return value{}, err
			}
		}
		return r.call(x, args)
	case "try":
		v, err := r.expression(x.left, env)
		if err != nil {
			return value{}, err
		}
		if v.result.err != nil {
			return value{}, v.result.err
		}
		return v.result.value, nil
	case "unary":
		v, err := r.expression(x.left, env)
		if err != nil {
			return value{}, err
		}
		if x.text == "not" {
			return value{boolean: !v.boolean}, nil
		}
		if v.number == math.MinInt64 {
			return value{}, errorAt(x.pos, CodeIntegerOverflow, "integer overflow")
		}
		return value{number: -v.number}, nil
	case "binary":
		left, err := r.expression(x.left, env)
		if err != nil {
			return value{}, err
		}
		if x.text == "and" && !left.boolean {
			return left, nil
		}
		if x.text == "or" && left.boolean {
			return left, nil
		}
		right, err := r.expression(x.right, env)
		if err != nil {
			return value{}, err
		}
		return binary(x, left, right)
	}
	return value{}, errorAt(x.pos, CodeUnsupportedCode, "unsupported expression")
}

func (r *runtime) call(x *expression, args []value) (value, error) {
	if x.word == nil {
		return r.builtin(x, args)
	}
	if r.depth >= 256 {
		return value{}, errorAt(x.pos, CodeDepthLimit, "word call depth limit of 256 exceeded")
	}
	r.depth++
	defer func() { r.depth-- }()
	r.traceWordCall(x, args)
	env := make(frame, len(x.word.params))
	for i, p := range x.word.params {
		env[p.binding] = args[i]
		r.traceBinding(p.pos, p.binding, args[i])
	}
	result, err := r.block(x.word.body, env)
	if err != nil {
		return value{}, err
	}
	r.traceWordReturn(x.word, result)
	return result, nil
}

func binary(x *expression, a, b value) (value, error) {
	switch x.text {
	case "and", "or":
		return value{boolean: b.boolean}, nil
	case "==", "!=":
		equal := false
		switch x.left.typ {
		case intType:
			equal = a.number == b.number
		case boolType:
			equal = a.boolean == b.boolean
		case stringType:
			equal = a.text == b.text
		}
		if x.text == "!=" {
			equal = !equal
		}
		return value{boolean: equal}, nil
	case "<":
		return value{boolean: a.number < b.number}, nil
	case "<=":
		return value{boolean: a.number <= b.number}, nil
	case ">":
		return value{boolean: a.number > b.number}, nil
	case ">=":
		return value{boolean: a.number >= b.number}, nil
	case "+":
		if x.typ == stringType {
			return value{text: a.text + b.text}, nil
		}
	}
	var n int64
	overflow := false
	switch x.text {
	case "+":
		n = a.number + b.number
		overflow = b.number > 0 && a.number > math.MaxInt64-b.number || b.number < 0 && a.number < math.MinInt64-b.number
	case "-":
		n = a.number - b.number
		overflow = b.number < 0 && a.number > math.MaxInt64+b.number || b.number > 0 && a.number < math.MinInt64+b.number
	case "*":
		n = a.number * b.number
		overflow = a.number != 0 && (a.number == -1 && b.number == math.MinInt64 || n/a.number != b.number)
	case "/", "%":
		if b.number == 0 {
			return value{}, errorAt(x.pos, CodeDivisionZero, "division or remainder by zero")
		}
		if a.number == math.MinInt64 && b.number == -1 {
			if x.text == "%" {
				return value{number: 0}, nil
			}
			return value{}, errorAt(x.pos, CodeIntegerOverflow, "integer overflow")
		}
		if x.text == "/" {
			n = a.number / b.number
		} else {
			n = a.number % b.number
		}
	}
	if overflow {
		return value{}, errorAt(x.pos, CodeIntegerOverflow, "integer overflow")
	}
	return value{number: n}, nil
}
