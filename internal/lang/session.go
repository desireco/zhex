package lang

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// IsIncomplete reports a missing block or a still-open delimiter.
func IsIncomplete(err error) bool {
	d, ok := err.(*diagnostic)
	return ok && (d.code == CodeExpectedIndent || d.code == CodeUnclosedDelimiter)
}

// DelimiterIndent reports the editable continuation indentation for an open delimiter.
func DelimiterIndent(source string, err error) (int, bool) {
	d, ok := err.(*diagnostic)
	if !ok || d.code != CodeUnclosedDelimiter {
		return 0, false
	}
	lines := strings.Split(source, "\n")
	if d.pos.line < 1 || d.pos.line > len(lines) {
		return 0, false
	}
	line := lines[d.pos.line-1]
	return len(line) - len(strings.TrimLeft(line, " ")) + 4, true
}

// LastStatementIndent reports the leading indentation of the last logical
// statement, ignoring physical line breaks inside delimiters.
func LastStatementIndent(source string) (int, bool) {
	tokens, err := lex("", source)
	if err != nil {
		return 0, false
	}
	first, last := 0, 0
	for _, token := range tokens {
		switch token.kind {
		case "indent", "dedent", "eof":
		case "newline":
			if first != 0 {
				last = first
				first = 0
			}
		default:
			if first == 0 {
				first = token.pos.line
			}
		}
	}
	lines := strings.Split(source, "\n")
	if last < 1 || last > len(lines) {
		return 0, false
	}
	line := lines[last-1]
	return len(line) - len(strings.TrimLeft(line, " ")), true
}

// Session is a REPL: an interactive run whose word definitions, top-level
// names, values, and adopted modules survive from one entry to the next.
type Session struct {
	input       *bufio.Reader
	output      io.Writer
	diagnostics io.Writer
	trace       io.Writer // nil disables the JSONL step-event stream
	scope       *scope
	records     map[string]*recordType
	words       map[string]*word   // own words plus adopted words under qualified names
	modules     map[string]*module // adopted modules, keyed by use name
	uses        []string           // the names use directives named directly; a module's own imports do not count
	sources     map[string]string  // every adopted module's text, for annotated errors
	frame       frame
	reporter    *expression
	resolve     func(name string) (ModuleSource, bool)
}

// NewSession shares input's buffer when it is already buffered, mirroring
// Program.RunWithTrace, so prompts and entries read one stream in order.
// resolve supplies each use entry's module, following the sibling
// <name>.zhex convention against the REPL's working directory; nil rejects
// every use with use-missing.
func NewSession(input io.Reader, output, trace io.Writer, resolve func(name string) (ModuleSource, bool)) *Session {
	reader, ok := input.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(input)
	}
	return &Session{
		input:       reader,
		output:      output,
		diagnostics: io.Discard,
		trace:       trace,
		records:     make(map[string]*recordType),
		words:       make(map[string]*word),
		modules:     make(map[string]*module),
		sources:     make(map[string]string),
		frame:       make(frame),
		resolve:     resolve,
	}
}

// SetDiagnostics routes console.fail output; nil discards it.
func (s *Session) SetDiagnostics(output io.Writer) {
	if output == nil {
		output = io.Discard
	}
	s.diagnostics = output
}

// usedDirectly reports whether a use directive already named this module.
func (s *Session) usedDirectly(name string) bool {
	for _, use := range s.uses {
		if use == name {
			return true
		}
	}
	return false
}

// Enter checks and runs one source entry against the session state. A check
// error adopts nothing; a runtime error stops the entry but keeps effects
// already performed. On success the entry's words, modules, and names are
// adopted and the entry's last value is echoed (empty for Unit).
func (s *Session) Enter(source string) (string, error) {
	p, err := parse("repl", source)
	if err != nil {
		return "", annotate(err, source)
	}
	sources := make(map[string]string, len(s.sources)+1)
	for path, text := range s.sources {
		sources[path] = text
	}
	sources["repl"] = source
	// A shallow copy keeps a failed entry from defining words; dropping the
	// names this entry redefines lets a redefinition replace the old word.
	words := make(map[string]*word, len(s.words)+len(p.words))
	for name, w := range s.words {
		words[name] = w
	}
	var redefined []string
	// Only this entry's new direct uses adopt; a repeat use is a no-op, and a
	// module another module imported stays invisible until named directly.
	var fresh []use
	seen := make(map[string]bool)
	for _, u := range p.uses {
		if !s.usedDirectly(u.name) && !seen[u.name] {
			fresh = append(fresh, u)
			seen[u.name] = true
		}
	}
	modules, err := s.adopt(fresh, words, &redefined, sources, p)
	if err != nil {
		return "", err
	}
	for _, w := range p.words {
		if words[w.name] != nil {
			delete(words, w.name)
			redefined = append(redefined, w.name)
		}
	}
	uses := make([]string, 0, len(s.uses)+len(fresh))
	uses = append(uses, s.uses...)
	for _, u := range fresh {
		uses = append(uses, u.name)
	}
	records := make(map[string]*recordType, len(s.records)+len(p.records))
	for name, record := range s.records {
		records[name] = record
	}
	for _, u := range uses {
		if m := modules[u]; m != nil {
			for _, record := range m.recordOrder {
				if !strings.HasPrefix(record.name, "_") {
					records[record.id()] = record
				}
			}
		}
	}
	all := collectRecords(modules)
	for _, record := range s.records {
		all[record.id()] = record
	}
	c := checker{words: words, types: namedTypeRegistry{visible: records, all: all}, modules: modules, uses: uses}
	env := newScope(s.scope)
	typ, err := c.checkProgram(p, env)
	if err != nil {
		return "", annotate(err, source)
	}
	// Existing compiled callers retain word pointers. Reject incompatible
	// redefinitions before executing or adopting anything in this entry.
	for _, w := range p.words {
		if previous := s.words[w.name]; previous != nil {
			same := previous.result == w.result && len(previous.params) == len(w.params)
			if same {
				for i := range w.params {
					if previous.params[i].typ != w.params[i].typ {
						same = false
						break
					}
				}
			}
			if !same {
				return "", annotate(errorAt(w.pos, CodeWordResult, "cannot redefine %q with a different signature in the REPL", w.name), source)
			}
		}
	}
	// A runtime per entry resets the recursion depth but keeps the session frame.
	r := runtime{input: s.input, output: s.output, diagnostics: s.diagnostics, trace: s.trace, reporter: s.reporter}
	v, err := r.block(p.body, s.frame)
	s.reporter = r.reporter
	if err != nil {
		err = unwrapPropagation(err)
		r.report(err, "unhandled", position{})
		return "", annotateSource(err, sources)
	}
	s.words, s.records, s.modules, s.uses, s.sources = words, c.types.visible, modules, uses, sources
	s.scope = env
	for _, name := range redefined {
		fmt.Fprintf(s.output, "redefined %s\n", name)
	}
	return traceValue(v, typ), nil
}

// adopt loads the entry's fresh use directives through the session's
// resolver and checks each newly loaded module's bodies, then merges the
// named modules' importable words, qualified, into words. The merge walks
// the modules map, not the freshly loaded set — a module first loaded
// transitively is already in the map when a later use names it directly. A
// qualified name that replaces an existing session word is noted in
// redefined — the REPL replaces instead of rejecting.
func (s *Session) adopt(fresh []use, words map[string]*word, redefined *[]string, sources map[string]string, p *Program) (map[string]*module, error) {
	known := make(map[string]*module, len(s.modules))
	for name, m := range s.modules {
		known[name] = m
	}
	modules, order, err := loadModules(fresh, known, "", s.resolve, missingInWorkingDirectory, sources)
	if err != nil {
		return nil, annotateSource(err, sources)
	}
	uses := append([]string{}, s.uses...)
	for _, use := range fresh {
		uses = append(uses, use.name)
	}
	context := &inferenceSession{words: words, records: s.records, scope: s.scope, uses: uses}
	if err := inferModuleParameters(p, modules, order, context); err != nil {
		return nil, annotateSource(err, sources)
	}
	allRecords := collectRecords(modules)
	for _, record := range s.records {
		allRecords[record.id()] = record
	}
	for _, record := range p.records {
		allRecords[record.id()] = record
	}
	c := &checker{modules: modules, types: namedTypeRegistry{all: allRecords}}
	for _, m := range order {
		c.module, c.words, c.uses = m.name, m.words, m.uses
		c.types.visible = c.importedRecords(m.uses)
		if err := c.registerRecords(m.recordOrder); err != nil {
			return nil, annotateSource(err, sources)
		}
		if err := c.checkAdopted(m.uses); err != nil {
			return nil, annotateSource(err, sources)
		}
		if err := c.checkWords(m.order); err != nil {
			return nil, annotateSource(err, sources)
		}
		if err := c.checkRecordDefaults(m.recordOrder); err != nil {
			return nil, annotateSource(err, sources)
		}
	}
	for _, u := range fresh {
		m := modules[u.name]
		for _, w := range m.order {
			if strings.HasPrefix(w.name, "_") {
				continue
			}
			name := m.name + "." + w.name
			if words[name] != nil {
				*redefined = append(*redefined, name)
			}
			words[name] = w
		}
	}
	return modules, nil
}

// missingInWorkingDirectory is the REPL's use-missing: its resolver reads
// <name>.zhex from the working directory.
func missingInWorkingDirectory(u use, _ string) error {
	return withSpanRange(withHint(errorAt(u.pos, CodeUseMissing, "module %q not found; searched %s", u.name, u.name+".zhex"),
		fmt.Sprintf("create %s in the working directory, or fix the module name", u.name+".zhex")), u.pos, u.end)
}

// Words lists the session's word signatures, sorted by name; a module's
// word carries its module prefix.
func (s *Session) Words() []string {
	sigs := make([]string, 0, len(s.words))
	for _, w := range s.words {
		name := w.name
		if w.module != "" {
			name = w.module + "." + w.name
		}
		params := make([]string, len(w.params))
		for i, param := range w.params {
			params[i] = fmt.Sprintf("%s: %s", param.name, param.typ)
			if param.defaultExpr != nil {
				params[i] += " = ..."
			}
		}
		sigs = append(sigs, fmt.Sprintf("%s(%s) -> %s", name, strings.Join(params, ", "), w.result))
	}
	sort.Strings(sigs)
	return sigs
}
