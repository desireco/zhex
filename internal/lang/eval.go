package lang

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"time"
)

type value struct {
	number  int64
	boolean bool
	text    string
	result  *inputResult
	fault   error
	buffer  *bufferValue
	file    *fileValue
	record  *recordValue
	list    *listValue
	array   *arrayValue
	choice  *choiceValue
}

const maxCollectionElements = 1000000

type listValue struct {
	items  []value
	active int
}

type choiceValue struct {
	name    string
	variant int
	fields  []value
}

type arrayValue struct {
	shape  []int64
	items  []value
	active int
}

type recordValue struct {
	name   string
	fields []value
}

// bufferValue is a pointer in value, so names alias one buffer.
type bufferValue struct{ lines []string }

// fileValue keeps the handle so close can release it; closed makes close
// idempotent and read-after-close a file-read failure.
type fileValue struct {
	path   string
	handle *os.File
	reader *bufio.Reader
	closed bool
}

type errorOccurrence struct {
	reported bool
	id       string
}

func ensureOccurrence(d *diagnostic) *errorOccurrence {
	if d.occurrence == nil {
		d.occurrence = newOccurrence()
	}
	return d.occurrence
}

type inputResult struct {
	value value
	err   error
}

// ExitError requests a deliberate program exit; embedders choose how to apply it.
type ExitError struct{ Status int }

func (e *ExitError) Error() string { return fmt.Sprintf("program requested exit status %d", e.Status) }

type returnValue struct{ result value }

func (r *returnValue) Error() string { return "return" }

// propagation crosses nested blocks but is caught by the current word or runner.
type propagation struct {
	cause    error
	boundary position
}

func (p *propagation) Error() string { return p.cause.Error() }
func unwrapPropagation(err error) error {
	if p, ok := err.(*propagation); ok {
		return p.cause
	}
	return err
}

type frame map[*binding]value

type runtime struct {
	input            *bufio.Reader
	output           io.Writer
	diagnostics      io.Writer
	trace            io.Writer // nil disables the JSONL step-event stream
	depth            int
	args             []string
	reporter         *expression
	reporting        bool
	hostReporter     func(ErrorReport) error
	includeSensitive bool
	correlation      string
}

// Run checks a file before executing it against the supplied console streams.
func Run(file, source string, input io.Reader, output io.Writer) error {
	return RunWithTrace(file, source, input, output, nil)
}

// RunWithTrace is Run with JSONL step events written to trace; nil trace
// leaves the run identical to Run.
func RunWithTrace(file, source string, input io.Reader, output, trace io.Writer) error {
	return RunWithArgs(file, source, input, output, trace, nil)
}

// RunWithArgs is RunWithTrace with the program's command-line arguments,
// readable through console.arg and console.args-count.
func RunWithArgs(file, source string, input io.Reader, output, trace io.Writer, args []string) error {
	return RunWithModules(file, source, input, output, trace, args, nil)
}

// RunWithModules is RunWithArgs for multi-file programs: resolve supplies
// each use directive's module, following the sibling <name>.zhex convention.
func RunWithModules(file, source string, input io.Reader, output, trace io.Writer, args []string, resolve func(name string) (ModuleSource, bool)) error {
	return RunWithModulesOptions(file, source, input, output, RunOptions{Trace: trace, Args: args}, resolve)
}

// RunWithModulesOptions compiles and runs modules with optional reporting and diagnostic output.
func RunWithModulesOptions(file, source string, input io.Reader, output io.Writer, options RunOptions, resolve func(name string) (ModuleSource, bool)) error {
	p, err := CompileModules(file, source, resolve)
	if err != nil {
		return err
	}
	return p.RunWithOptions(input, output, options)
}

// Run executes an already checked program with fresh local state.
func (p *Program) Run(input io.Reader, output io.Writer) error {
	return p.RunWithTrace(input, output, nil)
}

// RunWithTrace executes like Run and writes step events to trace when non-nil.
func (p *Program) RunWithTrace(input io.Reader, output, trace io.Writer) error {
	return p.RunWithArgs(input, output, trace, nil)
}

// RunWithArgs executes like RunWithTrace with the program's arguments.
func (p *Program) RunWithArgs(input io.Reader, output, trace io.Writer, args []string) error {
	return p.RunWithOptions(input, output, RunOptions{Trace: trace, Args: args})
}

// RunWithOptions executes a checked program with optional host reporting.
func (p *Program) RunWithOptions(input io.Reader, output io.Writer, options RunOptions) error {
	reader, ok := input.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(input)
	}
	diagnostics := options.Diagnostics
	if diagnostics == nil {
		diagnostics = io.Discard
	}
	r := runtime{input: reader, output: output, diagnostics: diagnostics, trace: options.Trace, args: options.Args, hostReporter: options.Reporter, includeSensitive: options.IncludeSensitive, correlation: options.Correlation}
	_, err := r.block(p.body, make(frame))
	boundary := position{}
	if propagated, ok := err.(*propagation); ok {
		boundary = propagated.boundary
	}
	err = unwrapPropagation(err)
	if err != nil {
		r.report(err, "unhandled", boundary)
	}
	return annotateSource(err, p.sources)
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
	case "return":
		return value{}, &returnValue{result: v}
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
	case "for-collection":
		var items []value
		if v.list != nil {
			v.list.active++
			defer func() { v.list.active-- }()
			items = v.list.items
		} else {
			v.array.active++
			defer func() { v.array.active-- }()
			items = v.array.items
		}
		for index, item := range items {
			first := item
			if s.secondBinding != nil {
				if v.list != nil {
					first = value{number: int64(index)}
				} else {
					coords := make([]value, len(v.array.shape))
					n := int64(index)
					for i := len(coords) - 1; i >= 0; i-- {
						coords[i] = value{number: n % v.array.shape[i]}
						n /= v.array.shape[i]
					}
					first = value{list: &listValue{items: coords}}
				}
				env[s.secondBinding] = item
				r.traceBinding(s.pos, s.secondBinding, item)
			}
			env[s.binding] = first
			r.traceBinding(s.pos, s.binding, first)
			if _, err := r.block(s.body, env); err != nil {
				return value{}, err
			}
		}
	case "for-lines":
		f, err := openFile(s.pos, v.text)
		if err != nil {
			if d, ok := err.(*diagnostic); ok {
				d.operation = "file.lines"
			}
			return value{}, err
		}
		defer f.close()
		for {
			line, ok, err := f.nextLine(s.pos)
			if err != nil {
				if d, ok := err.(*diagnostic); ok {
					d.operation = "file.lines"
				}
				return value{}, err
			}
			if !ok {
				return value{}, nil
			}
			env[s.binding] = value{text: line}
			r.traceBinding(s.pos, s.binding, value{text: line})
			if _, err := r.block(s.body, env); err != nil {
				return value{}, err
			}
		}
	case "flow":
		for _, step := range s.steps {
			r.traceFlowStep(step, v)
			args := []value{v}
			for _, arg := range step.args[1:] {
				item, err := r.expression(arg, env)
				if err != nil {
					return value{}, err
				}
				args = append(args, item)
			}
			v, err = r.call(step, args)
			if err != nil {
				return value{}, err
			}
			if step.propagate {
				if v.result.err != nil {
					return value{}, &propagation{cause: v.result.err, boundary: step.pos}
				}
				v = v.result.value
			}
		}
		return v, nil
	}
	return value{}, nil
}

func (r *runtime) expression(x *expression, env frame) (value, error) {
	switch x.kind {
	case "list":
		items := make([]value, len(x.args))
		for i, item := range x.args {
			var err error
			items[i], err = r.expression(item, env)
			if err != nil {
				return value{}, err
			}
		}
		return value{list: &listValue{items: items}}, nil
	case "index", "index-write":
		idx := x
		if x.kind == "index-write" {
			idx = x.left
		}
		receiver, err := r.expression(idx.left, env)
		if err != nil {
			return value{}, err
		}
		indices := make([]int64, len(idx.args))
		for i, arg := range idx.args {
			v, err := r.expression(arg, env)
			if err != nil {
				return value{}, err
			}
			indices[i] = v.number
		}
		var item value
		if x.kind == "index-write" {
			item, err = r.expression(x.right, env)
			if err != nil {
				return value{}, err
			}
		}
		if receiver.list != nil {
			if x.kind == "index-write" {
				return listWrite(x.pos, receiver.list, indices[0], item), nil
			}
			return listRead(x.pos, receiver.list, indices[0]), nil
		}
		return arrayIndex(x.pos, receiver.array, indices, item, x.kind == "index-write"), nil
	case "int":
		return value{number: x.number}, nil
	case "bool":
		return value{boolean: x.text == "true"}, nil
	case "string":
		return value{text: x.text}, nil
	case "name":
		if x.record != nil && x.record.choice {
			return value{choice: &choiceValue{name: x.record.id() + "." + x.record.variants[x.variant].name, variant: x.variant}}, nil
		}
		v := env[x.binding]
		for _, index := range x.fields {
			v = v.record.fields[index]
		}
		return v, nil
	case "match":
		subject, err := r.expression(x.left, env)
		if err != nil {
			return value{}, err
		}
		for _, arm := range x.arms {
			if arm.variant < 0 || arm.variant == subject.choice.variant {
				for i, b := range arm.bindings {
					if b != nil {
						env[b] = subject.choice.fields[i]
						r.traceBinding(arm.pos, b, subject.choice.fields[i])
					}
				}
				return r.block(arm.body, env)
			}
		}
		return value{}, errorAt(x.pos, CodeUnsupportedCode, "unmatched choice")
	case "call":
		if x.text == "error.set-reporter" {
			r.reporter = &expression{word: x.word, text: x.args[0].text, pos: x.pos, inputType: errorType}
			return value{}, nil
		}
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
			return value{}, &propagation{cause: v.result.err, boundary: x.pos}
		}
		return v.result.value, nil
	case "catch":
		v, err := r.expression(x.left, env)
		if err != nil {
			return value{}, err
		}
		if v.result.err == nil {
			return v.result.value, nil
		}
		if x.binding != nil {
			env[x.binding] = value{fault: v.result.err}
		}
		return r.block(x.handler, env)
	case "unary":
		v, err := r.expression(x.left, env)
		if err != nil {
			return value{}, err
		}
		if x.text == "not" {
			return value{boolean: !v.boolean}, nil
		}
		if v.number == math.MinInt64 {
			d := errorAt(x.pos, CodeIntegerOverflow, "integer overflow").(*diagnostic)
			d.operation = x.text
			return value{}, d
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
		v, err := binary(x, left, right)
		if d, ok := err.(*diagnostic); ok && d.operation == "" {
			d.operation = x.text
		}
		return v, err
	}
	return value{}, errorAt(x.pos, CodeUnsupportedCode, "unsupported expression")
}

func (r *runtime) call(x *expression, args []value) (value, error) {
	if x.record != nil {
		if x.record.choice {
			return value{choice: &choiceValue{name: x.record.id() + "." + x.record.variants[x.variant].name, variant: x.variant, fields: args}}, nil
		}
		return value{record: &recordValue{name: x.record.id(), fields: args}}, nil
	}
	if x.word == nil {
		result, err := r.builtin(x, args)
		if d, ok := err.(*diagnostic); ok && d.operation == "" {
			d.operation = x.text
		}
		if d, ok := result.fault.(*diagnostic); ok {
			ensureOccurrence(d)
		}
		if result.result != nil && result.result.err != nil {
			if d, ok := result.result.err.(*diagnostic); ok && d.operation == "" {
				d.operation = x.text
				ensureOccurrence(d)
				if d.details == nil {
					d.details = make(map[string]string)
				}
				switch x.text {
				case "buffer.line", "buffer.set-line":
					d.details["index"] = strconv.FormatInt(args[1].number, 10)
					d.details["line-count"] = strconv.Itoa(len(args[0].buffer.lines))
				case "console.arg":
					d.details["index"] = strconv.FormatInt(args[0].number, 10)
					d.details["argument-count"] = strconv.Itoa(len(r.args))
				case "file.read-all", "file.read-lines", "file.open":
					d.details["path"] = args[0].text
				case "file.read-line":
					d.details["path"] = args[0].file.path
				}
			}
		}
		return result, err
	}
	if r.depth >= 256 {
		d := errorAt(x.pos, CodeDepthLimit, "word call depth limit of 256 exceeded").(*diagnostic)
		d.operation = qualifiedName(x)
		return value{}, d
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
	if returned, ok := err.(*returnValue); ok {
		result, err = returned.result, nil
	}
	if propagated, ok := err.(*propagation); ok {
		result = value{result: &inputResult{err: propagated.cause}}
		err = nil
	}
	if err != nil {
		return value{}, err
	}
	if x.word.inferred && result.result == nil {
		result = value{result: &inputResult{value: result}}
	}
	r.traceWordReturn(x.word, result)
	return result, nil
}

// report runs only at an explicit handled report or the terminal boundary.
func (r *runtime) report(err error, outcome string, boundary position) {
	d, ok := err.(*diagnostic)
	if !ok || r.reporting || (r.reporter == nil && r.hostReporter == nil) {
		return
	}
	occurrence := ensureOccurrence(d)
	if occurrence.reported {
		return
	}
	occurrence.reported = true
	r.reporting = true
	defer func() { r.reporting = false }()
	if boundary.line == 0 {
		boundary = d.pos
	}
	if r.reporter != nil {
		func() { defer func() { _ = recover() }(); _, _ = r.call(r.reporter, []value{{fault: d}}) }()
	}
	if r.hostReporter != nil {
		func() {
			defer func() { _ = recover() }()
			event := ErrorReport{SchemaVersion: 1, OccurrenceID: occurrence.id, Timestamp: time.Now().UTC(), Outcome: outcome, Error: snapshot(d, r.includeSensitive, 1), Boundary: location(boundary, r.includeSensitive), Correlation: r.correlation}
			_ = r.hostReporter(event)
		}()
	}
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
