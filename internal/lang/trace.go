package lang

import (
	"encoding/json"
	"fmt"
)

// Trace step events per docs/diagnostics-plan.md Part 4 Phase A: one JSON
// object per line, fields in declaration order.

type wordCallEvent struct {
	Event string   `json:"event"`
	Word  string   `json:"word"`
	File  string   `json:"file"`
	Line  int      `json:"line"`
	Args  []string `json:"args"`
}

type bindingEvent struct {
	Event string `json:"event"`
	Name  string `json:"name"`
	Value string `json:"value"`
	File  string `json:"file"`
	Line  int    `json:"line"`
}

// valueEvent is the shared shape of the flow-step and word-return events.
type valueEvent struct {
	Event string `json:"event"`
	Word  string `json:"word"`
	Value string `json:"value"`
	File  string `json:"file"`
	Line  int    `json:"line"`
}

// traceWordCall reports a program-word invocation after its arguments are
// evaluated. A flow step passes the flowing value without argument
// expressions, so its static type falls back to the call's input type.
func (r *runtime) traceWordCall(x *expression, args []value) {
	if r.trace == nil {
		return
	}
	rendered := make([]string, len(args))
	for i, a := range args {
		typ := x.inputType
		if i < len(x.args) {
			typ = x.args[i].typ
		}
		rendered[i] = formatValue(a, typ)
	}
	r.emit(wordCallEvent{Event: "word-call", Word: qualifiedName(x), File: x.pos.file, Line: x.pos.line, Args: rendered})
}

func (r *runtime) traceBinding(pos position, b *binding, v value) {
	if r.trace == nil {
		return
	}
	r.emit(bindingEvent{Event: "binding", Name: b.name, Value: traceValue(v, b.typ), File: pos.file, Line: pos.line})
}

// traceValue renders a value for the event stream. A binding without postfix
// try holds a Result; those render as ok(inner) or err(message) — message
// only, single line — per the agent-facing event vocabulary.
func traceValue(v value, typ valueType) string {
	inner, ok := resultInner(typ)
	if !ok {
		return formatValue(v, typ)
	}
	if v.result.err != nil {
		message := v.result.err.Error()
		if d, ok := v.result.err.(*diagnostic); ok {
			message = d.message
		}
		return fmt.Sprintf("err(%q)", message)
	}
	return fmt.Sprintf("ok(%q)", formatValue(v.result.value, inner))
}

func (r *runtime) traceFlowStep(x *expression, v value) {
	if r.trace == nil {
		return
	}
	r.emit(valueEvent{Event: "flow-step", Word: qualifiedName(x), Value: formatValue(v, x.inputType), File: x.pos.file, Line: x.pos.line})
}

func (r *runtime) traceWordReturn(w *word, v value) {
	if r.trace == nil {
		return
	}
	name := w.name
	if w.module != "" {
		name = w.module + "." + w.name
	}
	r.emit(valueEvent{Event: "word-return", Word: name, Value: traceValue(v, w.result), File: w.pos.file, Line: w.pos.line})
}

// qualifiedName names a call for the event stream: a module's word carries
// its module prefix; everything else keeps the name as written.
func qualifiedName(x *expression) string {
	if x.word != nil && x.word.module != "" {
		return x.word.module + "." + x.word.name
	}
	return x.text
}

// shortcut: trace write errors are dropped — tracing must never fail a run.
func (r *runtime) emit(event any) {
	line, err := json.Marshal(event)
	if err != nil {
		return
	}
	r.trace.Write(append(line, '\n'))
}
