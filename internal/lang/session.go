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
// names, and values survive from one entry to the next.
type Session struct {
	input  *bufio.Reader
	output io.Writer
	trace  io.Writer // nil disables the JSONL step-event stream
	scope  *scope
	words  map[string]*word
	frame  frame
}

// NewSession shares input's buffer when it is already buffered, mirroring
// Program.RunWithTrace, so prompts and entries read one stream in order.
func NewSession(input io.Reader, output, trace io.Writer) *Session {
	reader, ok := input.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(input)
	}
	return &Session{input: reader, output: output, trace: trace, words: make(map[string]*word), frame: make(frame)}
}

// Enter checks and runs one source entry against the session state. A check
// error adopts nothing; a runtime error stops the entry but keeps effects
// already performed. On success the entry's words and names are adopted and
// the entry's last value is echoed (empty for Unit).
func (s *Session) Enter(source string) (string, error) {
	p, err := parse("repl", source)
	if err != nil {
		return "", annotate(err, source)
	}
	// A shallow copy keeps a failed entry from defining words; dropping the
	// names this entry redefines lets a redefinition replace the old word.
	words := make(map[string]*word, len(s.words)+len(p.words))
	for name, w := range s.words {
		words[name] = w
	}
	var redefined []string
	for _, w := range p.words {
		if words[w.name] != nil {
			delete(words, w.name)
			redefined = append(redefined, w.name)
		}
	}
	c := checker{words: words}
	env := newScope(s.scope)
	typ, err := c.checkProgram(p, env)
	if err != nil {
		return "", annotate(err, source)
	}
	// A runtime per entry resets the recursion depth but keeps the session frame.
	r := runtime{input: s.input, output: s.output, trace: s.trace}
	v, err := r.block(p.body, s.frame)
	if err != nil {
		return "", annotate(err, source)
	}
	s.words = words
	s.scope = env
	for _, name := range redefined {
		fmt.Fprintf(s.output, "redefined %s\n", name)
	}
	return traceValue(v, typ), nil
}

// Words lists the session's word signatures, sorted by name.
func (s *Session) Words() []string {
	sigs := make([]string, 0, len(s.words))
	for _, w := range s.words {
		params := make([]string, len(w.params))
		for i, param := range w.params {
			params[i] = fmt.Sprintf("%s: %s", param.name, param.typ)
		}
		sigs = append(sigs, fmt.Sprintf("%s(%s) -> %s", w.name, strings.Join(params, ", "), w.result))
	}
	sort.Strings(sigs)
	return sigs
}
