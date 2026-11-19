package lang

import (
	"zhex/internal/zhexrt"
	"errors"
	"strings"
)

// RegisterReplSeam registers the #65 prototype's per-run repl.* seam on the
// session: the implicit words a Zhex-written REPL front end drives the engine
// through, plus the EnterResult choice their outcomes use. The words close
// over this one Session and exist only here — they never join hostWords, the
// diagnostic catalog, or the shipped vocabulary, and a session that never
// registers them does not know them.
func (s *Session) RegisterReplSeam() {
	enterResult := &recordType{
		name:    "EnterResult",
		pos:     position{file: "repl", line: 1, column: 1},
		choice:  true,
		builtin: true, // host-predeclared: skipped by Session.Export and cycle visits
		variants: []choiceVariant{
			{name: "Ran", fields: []parameter{{name: "echo", typ: stringType}}},
			{name: "Failed", fields: []parameter{{name: "report", typ: stringType}}},
			{name: "Exited", fields: []parameter{{name: "status", typ: intType}}},
		},
	}
	s.records[enterResult.id()] = enterResult
	s.seam = true
	words := []*word{
		seamWord("repl.enter", []parameter{{name: "source", typ: stringType}}, "EnterResult", s.seamEnter),
		seamWord("repl.words", nil, "List<String>", s.seamWords),
		seamWord("repl.incomplete", []parameter{{name: "source", typ: stringType}}, boolType, s.seamIncomplete),
		seamWord("repl.open-delimiter", []parameter{{name: "source", typ: stringType}}, boolType, s.seamOpenDelimiter),
		seamWord("repl.diagnose", []parameter{{name: "source", typ: stringType}}, stringType, s.seamDiagnose),
		seamWord("repl.save", []parameter{{name: "path", typ: stringType}}, unitFileResultType, s.seamSave),
		seamWord("repl.load", []parameter{{name: "path", typ: stringType}}, unitFileResultType, s.seamLoad),
	}
	for _, w := range words {
		s.words[w.module+"."+w.name] = w
	}
}

// seamReject guards the seam's own vocabulary while it is registered: a
// module named repl would silently replace seam words through the qualified
// merge, and a user EnterResult would shadow the seam's outcome choice. The
// rejection is prototype-scoped — an unregistered session keeps accepting the
// module name and the type name.
func (s *Session) seamReject(p *Program) error {
	if !s.seam {
		return nil
	}
	for _, u := range p.uses {
		if u.name == "repl" {
			return withSpanRange(withHint(
				errorAt(u.pos, CodeUseReserved, "module name %q is reserved by the active repl seam", u.name),
				"rename the module file — the repl namespace belongs to the seam while it is registered"), u.pos, u.end)
		}
	}
	for _, record := range p.records {
		if record.name == "EnterResult" {
			return withHint(
				errorAt(record.pos, CodeDuplicateName, "type %q is reserved by the active repl seam", record.name),
				"choose another type name — EnterResult is the seam's outcome choice")
		}
	}
	return nil
}

// seamWord builds one host-evaluated session word under the repl namespace;
// name is the qualified call name, e.g. repl.enter.
func seamWord(name string, params []parameter, result valueType, fn hostFunc) *word {
	_, local, _ := strings.Cut(name, ".")
	return &word{
		name:   local,
		module: "repl",
		pos:    position{file: "repl", line: 1, column: 1},
		params: params,
		result: result,
		host:   fn,
	}
}

// outcome wraps a seam result as an EnterResult alternative.
func outcome(variant int, fields ...value) value {
	name := []string{"EnterResult.Ran", "EnterResult.Failed", "EnterResult.Exited"}[variant]
	return value{choice: &choiceValue{name: name, variant: variant, fields: fields}}
}

// seamEnter runs one entry against the session, reporting the three outcomes
// the front end dispatches on: a successful echo, the rendered diagnostic of
// a failed entry, or a requested exit status.
func (s *Session) seamEnter(r *runtime, x *expression, args []value) (value, error) {
	echo, err := s.Enter(args[0].text)
	if err != nil {
		var exit *ExitError
		if errors.As(err, &exit) {
			return outcome(2, value{number: zhexrt.I64(int64(exit.Status))}), nil
		}
		return outcome(1, value{text: err.Error()}), nil
	}
	return outcome(0, value{text: echo}), nil
}

// seamWords lists the session's word signatures exactly as the words command.
func (s *Session) seamWords(r *runtime, x *expression, args []value) (value, error) {
	sigs := s.Words()
	items := make([]value, len(sigs))
	for i, sig := range sigs {
		items[i] = value{text: sig}
	}
	return value{list: &listValue{items: items}}, nil
}

// seamIncomplete reports whether an entry needs more lines, using the same
// compile probe the production REPL loop uses.
func (s *Session) seamIncomplete(r *runtime, x *expression, args []value) (value, error) {
	_, err := Compile("repl", args[0].text)
	return value{boolean: IsIncomplete(err)}, nil
}

// seamOpenDelimiter reports whether an entry has a delimiter still open.
func (s *Session) seamOpenDelimiter(r *runtime, x *expression, args []value) (value, error) {
	_, err := Compile("repl", args[0].text)
	_, open := DelimiterIndent(args[0].text, err)
	return value{boolean: open}, nil
}

// seamDiagnose renders the entry's compile diagnostic, "" when it is clean —
// the piped EOF unclosed-delimiter path prints exactly this text.
func (s *Session) seamDiagnose(r *runtime, x *expression, args []value) (value, error) {
	_, err := Compile("repl", args[0].text)
	if err == nil {
		return value{text: ""}, nil
	}
	return value{text: err.Error()}, nil
}

// seamSave writes the engine-side session export to path.
func (s *Session) seamSave(r *runtime, x *expression, args []value) (value, error) {
	source, err := s.Export()
	if err != nil {
		return value{result: &inputResult{err: errorAt(x.pos, CodeFileWrite, "cannot export session: %v", err)}}, nil
	}
	return writeTextFile(x.pos, source, args[0].text, false), nil
}

// seamLoad reads a session export and Enters it transactionally: the whole
// file is one entry, so a failure adopts nothing. A read failure carries a
// FileError; an entry failure keeps its own diagnostic inside the Result. An
// exit requested by the loaded file propagates as an exit, not a failure.
func (s *Session) seamLoad(r *runtime, x *expression, args []value) (value, error) {
	source, err := readFile(x.pos, args[0].text)
	if err != nil {
		return value{result: &inputResult{err: err}}, nil
	}
	if _, err := s.Enter(source); err != nil {
		var exit *ExitError
		if errors.As(err, &exit) {
			return value{}, exit
		}
		return value{result: &inputResult{err: err}}, nil
	}
	return value{result: &inputResult{}}, nil
}
