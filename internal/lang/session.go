package lang

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// IsIncomplete reports whether err says the source stopped right where an
// indented block should follow — the REPL's cue to keep reading lines.
// Parentheses and dangling expressions never continue a line, so only the
// missing-block diagnostic counts.
func IsIncomplete(err error) bool {
	d, ok := err.(*diagnostic)
	return ok && d.code == CodeExpectedIndent
}

// Session is a REPL: an interactive run whose word definitions, top-level
// names, values, and adopted modules survive from one entry to the next.
type Session struct {
	input   *bufio.Reader
	output  io.Writer
	trace   io.Writer // nil disables the JSONL step-event stream
	scope   *scope
	words   map[string]*word   // own words plus adopted words under qualified names
	modules map[string]*module // adopted modules, keyed by use name
	uses    []string           // the names use directives named directly; a module's own imports do not count
	sources map[string]string  // every adopted module's text, for annotated errors
	frame   frame
	resolve func(name string) (ModuleSource, bool)
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
		input:   reader,
		output:  output,
		trace:   trace,
		words:   make(map[string]*word),
		modules: make(map[string]*module),
		sources: make(map[string]string),
		frame:   make(frame),
		resolve: resolve,
	}
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
	modules, err := s.adopt(fresh, words, &redefined, sources)
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
	c := checker{words: words, modules: modules, uses: uses}
	env := newScope(s.scope)
	typ, err := c.checkProgram(p, env)
	if err != nil {
		return "", annotate(err, source)
	}
	// A runtime per entry resets the recursion depth but keeps the session frame.
	r := runtime{input: s.input, output: s.output, trace: s.trace}
	v, err := r.block(p.body, s.frame)
	if err != nil {
		return "", annotateSource(unwrapPropagation(err), sources)
	}
	s.words, s.modules, s.uses, s.sources = words, modules, uses, sources
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
func (s *Session) adopt(fresh []use, words map[string]*word, redefined *[]string, sources map[string]string) (map[string]*module, error) {
	known := make(map[string]*module, len(s.modules))
	for name, m := range s.modules {
		known[name] = m
	}
	modules, order, err := loadModules(fresh, known, "", s.resolve, missingInWorkingDirectory, sources)
	if err != nil {
		return nil, annotateSource(err, sources)
	}
	c := &checker{modules: modules}
	for _, m := range order {
		c.module, c.words, c.uses = m.name, m.words, m.uses
		if err := c.checkAdopted(m.uses); err != nil {
			return nil, annotateSource(err, sources)
		}
		if err := c.checkWords(m.order); err != nil {
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
		}
		sigs = append(sigs, fmt.Sprintf("%s(%s) -> %s", name, strings.Join(params, ", "), w.result))
	}
	sort.Strings(sigs)
	return sigs
}
