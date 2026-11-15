package lang

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// doctestPrefix marks a doc line as a doctest input: the text after it is the
// expression to evaluate.
const doctestPrefix = "> "

// doctest is one ## > example: the input expression, the expected echo lines
// that follow it in the doc, and the doc line it starts on (1-based, in the
// owning file).
type doctest struct {
	input    string
	expected []string
	line     int
}

// doctests splits a word's doc into examples. The doc lines after a ## >
// line are its expected output until a blank doc line or the next ## >
// line ends them — after a blank line the doc is prose again, while prose
// directly after an example reads as output and mismatches.
func doctests(w *word) []doctest {
	if w.doc == "" {
		return nil
	}
	lines := strings.Split(w.doc, "\n")
	first := w.pos.line - len(lines)
	var tests []doctest
	for i, line := range lines {
		if !strings.HasPrefix(line, doctestPrefix) {
			continue
		}
		t := doctest{input: line[len(doctestPrefix):], line: first + i}
		for _, next := range lines[i+1:] {
			if next == "" || strings.HasPrefix(next, doctestPrefix) {
				break
			}
			t.expected = append(t.expected, next)
		}
		tests = append(tests, t)
	}
	return tests
}

// doctestName renders the word a doctest belongs to the way words does.
func doctestName(w *word) string {
	if w.module != "" {
		return w.module + "." + w.name
	}
	return w.name
}

// doctestColumn returns the column the ## > line's expression starts at, so
// a remapped diagnostic's caret lands inside the doc line.
func doctestColumn(w *word, t doctest) int {
	if w.pos.source == nil {
		return 1
	}
	lines := strings.Split(*w.pos.source, "\n")
	if t.line < 1 || t.line > len(lines) {
		return 1
	}
	raw := strings.TrimSuffix(lines[t.line-1], "\r")
	// Only ## and one optional space precede the doc text, so the first "> "
	// in the raw line is the doctest marker.
	marker := strings.Index(raw, "> ")
	if marker < 0 {
		return 1
	}
	return marker + len(doctestPrefix) + 1
}

// RunDoctests compiles the file like Run — the checker validates the whole
// program first and the body never executes — then runs every ## > example
// in each documented word's doc, the entry file's words and every adopted
// module's, checking and evaluating the expression in the owning module's
// scope so bare calls resolve inside it. The echo renders as the REPL echoes
// a bare expression. Each failing doctest returns one diagnostic:
// doctest-mismatch when the echo differs from the expected output lines,
// doctest-check when the expression itself fails to check or evaluate.
// Passing doctests return nothing.
func RunDoctests(file, source string, input io.Reader, output io.Writer, resolve func(name string) (ModuleSource, bool)) []error {
	p, err := CompileModules(file, source, resolve)
	if err != nil {
		return []error{err}
	}
	reader, ok := input.(*bufio.Reader)
	if !ok {
		reader = bufio.NewReader(input)
	}
	allRecords := collectRecords(p.modules)
	for _, record := range p.records {
		allRecords[record.id()] = record
	}
	names := make([]string, 0, len(p.modules))
	for name := range p.modules {
		names = append(names, name)
	}
	sort.Strings(names)
	var failures []error
	check := func(c *checker, words []*word) {
		for _, w := range words {
			for _, t := range doctests(w) {
				if err := runDoctest(c, w, t, reader, output); err != nil {
					failures = append(failures, err)
				}
			}
		}
	}
	for _, name := range names {
		m := p.modules[name]
		// The module's own view, as CompileModules checked it in: its words
		// bare (private ones included), its uses qualified. registerRecords
		// re-validates what compilation already accepted, so it cannot fail.
		c := &checker{modules: p.modules, types: namedTypeRegistry{all: allRecords}, module: m.name, words: m.words, uses: m.uses}
		c.types.visible = c.importedRecords(m.uses)
		_ = c.registerRecords(m.recordOrder)
		check(c, m.order)
	}
	entry := &checker{modules: p.modules, types: namedTypeRegistry{all: allRecords}, words: make(map[string]*word), uses: useNames(p)}
	entry.types.visible = entry.importedRecords(entry.uses)
	_ = entry.registerRecords(p.records)
	for _, w := range p.words {
		entry.words[w.name] = w
	}
	check(entry, p.words)
	return failures
}

// runDoctest checks and evaluates one example through the ordinary Enter
// path: the same checkProgram a REPL entry takes and a fresh runtime per
// example, so depth limits and diagnostics apply unchanged.
func runDoctest(c *checker, w *word, t doctest, input *bufio.Reader, output io.Writer) error {
	dp, err := parse(w.pos.file, t.input+"\n")
	if err != nil {
		return doctestFailure(w, t, err, "failed to check")
	}
	typ, err := c.checkProgram(dp, newScope(nil))
	if err != nil {
		return doctestFailure(w, t, err, "failed to check")
	}
	r := runtime{input: input, output: output}
	v, err := r.block(dp.body, make(frame))
	if err != nil {
		if _, exiting := err.(*ExitError); exiting {
			return err
		}
		return doctestFailure(w, t, unwrapPropagation(err), "failed to evaluate")
	}
	echo := traceValue(v, typ)
	if echo == strings.Join(t.expected, "\n") {
		return nil
	}
	column := doctestColumn(w, t)
	pos := position{file: w.pos.file, line: t.line, column: 1, source: w.pos.source}
	end := pos
	// Columns count runes, as the lexer's positions do.
	end.column = column + utf8.RuneCountInString(t.input)
	return annotate(withHint(withSpanRange(
		errorAt(pos, CodeDoctestMismatch, "doctest mismatch in %s: expected %s, found %s", doctestName(w), doctestValue(strings.Join(t.expected, "\n")), doctestValue(echo)),
		pos, end), `update the expected output after "## > ", or fix the word`), "")
}

// doctestFailure wraps an example's own failed check or evaluation as a
// doctest-check diagnostic: the position and caret move into the doc line,
// the original message follows the phase, and an original hint naming the
// real fix survives.
func doctestFailure(w *word, t doctest, err error, phase string) error {
	d, ok := err.(*diagnostic)
	if !ok {
		return err
	}
	shift := doctestColumn(w, t)
	remapPosition(&d.pos, t.line, shift)
	remapPosition(&d.spanStart, t.line, shift)
	remapPosition(&d.spanEnd, t.line, shift)
	d.pos.file, d.pos.source = w.pos.file, w.pos.source
	d.line = ""
	d.code = CodeDoctestCheck
	d.message = fmt.Sprintf("doctest in %s %s: %s", doctestName(w), phase, d.message)
	if d.hint == "" {
		d.hint = `fix the expression after "## > " so it checks and evaluates`
	}
	return annotate(d, "")
}

// remapPosition moves a snippet position onto the doc line it came from; a
// column shifts only when the position sat on the expression's own line.
func remapPosition(pos *position, line, shift int) {
	onInput := pos.line == 1
	pos.line = line
	if onInput {
		pos.column += shift - 1
	}
}

// doctestValue renders an expected or actual echo in a diagnostic message;
// multi-line values quote, keeping the first line stable for scripts.
func doctestValue(echo string) string {
	if strings.Contains(echo, "\n") {
		return fmt.Sprintf("%q", echo)
	}
	return echo
}
