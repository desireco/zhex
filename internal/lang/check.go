package lang

import (
	"fmt"
	"path/filepath"
	"strings"
)

type scope struct {
	parent   *scope
	bindings map[string]*binding
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, bindings: make(map[string]*binding)}
}
func (s *scope) find(name string) *binding {
	for current := s; current != nil; current = current.parent {
		if b := current.bindings[name]; b != nil {
			return b
		}
	}
	return nil
}
func (s *scope) names() []string {
	var out []string
	for current := s; current != nil; current = current.parent {
		for name := range current.bindings {
			out = append(out, name)
		}
	}
	return out
}
func (s *scope) define(pos position, name string, typ valueType, mutable bool) (*binding, error) {
	if s.bindings[name] != nil {
		return nil, errorAt(pos, CodeDuplicateName, "name %q is already defined in this scope", name)
	}
	b := &binding{name: name, typ: typ, mutable: mutable}
	s.bindings[name] = b
	return b, nil
}

// checker holds one file's view while checking it: its own words, the
// modules it uses, and every loaded module. words, uses, and module swap
// per file; modules stays for the whole compile. session.go builds the same
// view from the REPL's accumulated words and modules.
type checker struct {
	words   map[string]*word
	modules map[string]*module
	uses    []string
	module  string // the module being checked; "" for the entry file and the REPL
}

var hostWords = []string{
	"print", "int.to-string", "text.reverse", "text.to-int", "read-line", "range",
	"text.length", "text.line-count", "text.word-count", "text.upper", "text.lower",
	"text.trim", "text.contains", "text.index-of", "text.slice", "text.replace",
	"buffer.new", "buffer.add-line", "buffer.line-count", "buffer.line", "buffer.set-line", "buffer.to-text",
	"file.read-all", "file.read-lines", "file.open", "file.read-line", "file.close", "file.lines",
	"console.arg", "console.args-count",
}

// HostWord reports whether name is a word provided by the host, not the program.
func HostWord(name string) bool {
	for _, word := range hostWords {
		if word == name {
			return true
		}
	}
	return false
}

// wordNames lists the words visible in the current file: the host words,
// its own words, and its used modules' importable words qualified. The
// suggestion hints draw from this list.
func (c *checker) wordNames() []string {
	names := append([]string{}, hostWords...)
	for name := range c.words {
		names = append(names, name)
	}
	for _, use := range c.uses {
		if m := c.modules[use]; m != nil {
			for _, w := range m.order {
				if !strings.HasPrefix(w.name, "_") {
					names = append(names, m.name+"."+w.name)
				}
			}
		}
	}
	return names
}

// ModuleSource is one importable module file: its path, for diagnostics, and
// its source text.
type ModuleSource struct {
	Path   string
	Source string
}

// module is one imported file: its use name, its path, and its word table.
type module struct {
	name  string
	path  string
	words map[string]*word
	order []*word // source order, so the first broken body reports first
	uses  []string
}

// reservedModulePrefixes are the host operations' prefixes; a module named
// one of them would shadow every qualified host word.
var reservedModulePrefixes = []string{"text", "int", "buffer", "file", "console"}

func reservedModule(name string) bool {
	for _, prefix := range reservedModulePrefixes {
		if name == prefix {
			return true
		}
	}
	return false
}

// Compile parses and checks every word before the program can perform any I/O.
func Compile(file, source string) (*Program, error) {
	return CompileModules(file, source, nil)
}

// CompileModules compiles a multi-file program: the entry source plus every
// module its use directives name, loaded through resolve following the
// sibling <name>.zhex convention. A nil resolve rejects each use with
// use-missing. The compiler stays free of file I/O; callers own the disk.
func CompileModules(entryPath, entrySource string, resolve func(name string) (ModuleSource, bool)) (*Program, error) {
	p, err := parse(entryPath, entrySource)
	if err != nil {
		return nil, annotateSource(err, map[string]string{entryPath: entrySource})
	}
	sources := map[string]string{entryPath: entrySource}
	modules, order, err := loadModules(p.uses, nil, entryPath, resolve, useMissing, sources)
	if err != nil {
		return nil, annotateSource(err, sources)
	}
	c := &checker{words: make(map[string]*word), modules: modules}
	// Modules check in load order — dependencies first — and the entry last.
	for _, m := range order {
		c.module, c.words, c.uses = m.name, m.words, m.uses
		if err := c.checkAdopted(m.uses); err != nil {
			return nil, annotateSource(err, sources)
		}
		if err := c.checkWords(m.order); err != nil {
			return nil, annotateSource(err, sources)
		}
	}
	c.module, c.words, c.uses = "", make(map[string]*word), useNames(p)
	if err = c.register(p.words); err != nil {
		return nil, annotateSource(err, sources)
	}
	if err = c.checkAdopted(useNames(p)); err != nil {
		return nil, annotateSource(err, sources)
	}
	if err = c.checkWords(p.words); err != nil {
		return nil, annotateSource(err, sources)
	}
	if _, err = c.block(p.body, newScope(nil)); err != nil {
		return nil, annotateSource(err, sources)
	}
	p.sources = sources
	return p, nil
}

func useNames(p *Program) []string {
	names := make([]string, len(p.uses))
	for i, u := range p.uses {
		names[i] = u.name
	}
	return names
}

// loadModules resolves the use graph depth-first, dependencies before the
// files that use them, and returns the modules in that order — the order
// they check in. known seeds already-loaded modules, so a repeat use is a
// no-op; missing builds the use-missing diagnostic for the importing
// context. sources collects every loaded file's text so a diagnostic can be
// annotated against the file it points into.
func loadModules(uses []use, known map[string]*module, importingPath string, resolve func(name string) (ModuleSource, bool), missing func(u use, importingPath string) error, sources map[string]string) (map[string]*module, []*module, error) {
	byName := make(map[string]*module, len(known))
	for name, m := range known {
		byName[name] = m
	}
	var order []*module
	var stack []string // use names being loaded, for cycle chains
	var paths []string
	if importingPath != "" {
		paths = append(paths, importingPath)
	}
	var load func(uses []use, path string) error
	load = func(uses []use, path string) error {
		for _, u := range uses {
			for i, name := range stack {
				if name != u.name {
					continue
				}
				chain := make([]string, 0, len(stack)-i+1)
				for _, n := range stack[i:] {
					chain = append(chain, byName[n].path)
				}
				return withSpanRange(errorAt(u.pos, CodeUseCycle, "module cycle: %s", strings.Join(append(chain, byName[u.name].path), " -> ")), u.pos, u.end)
			}
			if byName[u.name] != nil {
				continue // already loaded; a duplicate use is a no-op
			}
			if reservedModule(u.name) {
				return withSpanRange(withHint(errorAt(u.pos, CodeUseReserved, "module name %q is reserved", u.name),
					"rename the module file — a module cannot be named text, int, buffer, file, or console"), u.pos, u.end)
			}
			src, ok := ModuleSource{}, false
			if resolve != nil {
				src, ok = resolve(u.name)
			}
			if !ok {
				return missing(u, path)
			}
			for i, active := range paths {
				if filepath.Clean(src.Path) == filepath.Clean(active) {
					chain := append(append([]string{}, paths[i:]...), src.Path)
					return withSpanRange(errorAt(u.pos, CodeUseCycle, "module cycle: %s", strings.Join(chain, " -> ")), u.pos, u.end)
				}
			}
			sources[src.Path] = src.Source
			mp, err := parse(src.Path, src.Source)
			if err != nil {
				return err
			}
			if len(mp.body) > 0 {
				return withSpanRange(errorAt(mp.body[0].pos, CodeModuleBody,
					"imported files declare words only; move the statements into the file you run"), mp.body[0].pos, mp.body[0].expr.end)
			}
			m := &module{name: u.name, path: src.Path, words: make(map[string]*word)}
			for _, w := range mp.words {
				if m.words[w.name] != nil || HostWord(w.name) {
					return errorAt(w.pos, CodeDuplicateWord, "word %q is already defined", w.name)
				}
				w.module = u.name
				m.words[w.name] = w
				m.order = append(m.order, w)
			}
			for _, dep := range mp.uses {
				m.uses = append(m.uses, dep.name)
			}
			byName[u.name] = m
			stack = append(stack, u.name)
			paths = append(paths, src.Path)
			if err := load(mp.uses, src.Path); err != nil {
				return err
			}
			stack = stack[:len(stack)-1]
			paths = paths[:len(paths)-1]
			order = append(order, m) // post-order: dependencies check first
		}
		return nil
	}
	if err := load(uses, importingPath); err != nil {
		return nil, nil, err
	}
	return byName, order, nil
}

// useMissing reports a use no resolver could supply; the searched path is the
// sibling <name>.zhex next to the importing file.
func useMissing(u use, importingPath string) error {
	searched := filepath.Join(filepath.Dir(importingPath), u.name+".zhex")
	return withSpanRange(withHint(errorAt(u.pos, CodeUseMissing, "module %q not found; searched %s", u.name, searched),
		fmt.Sprintf("create %s next to %s, or fix the module name", searched, importingPath)), u.pos, u.end)
}

// annotateSource attaches the excerpt from the file a diagnostic points into;
// Run annotates against the entry source only, so module files carry their own.
func annotateSource(err error, sources map[string]string) error {
	if d, ok := err.(*diagnostic); ok {
		if source, found := sources[d.pos.file]; found {
			return annotate(err, source)
		}
	}
	return err
}

// checkProgram type-checks one REPL entry against the session's accumulated
// words and top-level scope, returning the entry's result type. The session
// replaces redefined words instead of rejecting them, so adoption collisions
// were resolved before this runs; word bodies never see the top-level scope.
func (c *checker) checkProgram(p *Program, top *scope) (valueType, error) {
	if err := c.register(p.words); err != nil {
		return "", err
	}
	if err := c.checkWords(p.words); err != nil {
		return "", err
	}
	return c.block(p.body, top)
}

// register adds a file's own words to the current view, rejecting
// duplicates and host collisions.
func (c *checker) register(words []*word) error {
	for _, w := range words {
		if c.words[w.name] != nil || HostWord(w.name) {
			return errorAt(w.pos, CodeDuplicateWord, "word %q is already defined", w.name)
		}
		c.words[w.name] = w
	}
	return nil
}

// checkWords type-checks word bodies against the current file's view.
func (c *checker) checkWords(words []*word) error {
	for _, w := range words {
		env := newScope(nil)
		for i := range w.params {
			param := &w.params[i]
			binding, err := env.define(param.pos, param.name, param.typ, false)
			if err != nil {
				return err
			}
			param.binding = binding
		}
		result, err := c.block(w.body, env)
		if err != nil {
			return err
		}
		if result != w.result {
			return errorAt(w.pos, CodeWordResult, "word %q return type is %s; declared %s", w.name, result, w.result)
		}
	}
	return nil
}

// checkAdopted rejects a file's own word that spells a used module's
// importable word: both would answer to the same qualified name.
func (c *checker) checkAdopted(uses []string) error {
	for _, use := range uses {
		m := c.modules[use]
		if m == nil {
			continue
		}
		for _, w := range m.order {
			if strings.HasPrefix(w.name, "_") {
				continue
			}
			if own := c.words[m.name+"."+w.name]; own != nil {
				return errorAt(own.pos, CodeDuplicateWord, "word %q is already defined", m.name+"."+w.name)
			}
		}
	}
	return nil
}

// resolveWord resolves a call name in the current file's view: its own
// words, bare or as written, then — for a qualified name whose module this
// file uses — the module's table. A word from another module whose name
// starts with an underscore comes back with its module for private-use.
func (c *checker) resolveWord(name string) (*word, *module) {
	if w := c.words[name]; w != nil {
		return w, nil
	}
	owner, rest, qualified := strings.Cut(name, ".")
	if !qualified || !c.used(owner) {
		return nil, nil
	}
	m := c.modules[owner]
	if m == nil {
		return nil, nil
	}
	w := m.words[rest]
	if w == nil {
		return nil, nil
	}
	if strings.HasPrefix(w.name, "_") && w.module != c.module {
		return w, m
	}
	return w, nil
}

func (c *checker) used(name string) bool {
	for _, use := range c.uses {
		if use == name {
			return true
		}
	}
	return false
}

func (c *checker) block(body []*statement, env *scope) (valueType, error) {
	typ := unitType
	for _, s := range body {
		var err error
		typ, err = c.statement(s, env)
		if err != nil {
			return "", err
		}
	}
	return typ, nil
}

func (c *checker) statement(s *statement, env *scope) (valueType, error) {
	typ, err := c.expression(s.expr, env)
	if err != nil {
		return "", err
	}
	switch s.kind {
	case "expression":
		return typ, nil
	case "let", "var":
		s.binding, err = env.define(s.pos, s.name, typ, s.kind == "var")
		if err == nil {
			s.binding.origin = originOf(s.expr)
		}
		return unitType, err
	case "assign":
		s.binding = env.find(s.name)
		if s.binding == nil {
			return "", withHint(errorAt(s.pos, CodeUnknownName, "unknown name %q", s.name), nameHint(s.name, append(env.names(), c.wordNames()...)))
		}
		if !s.binding.mutable {
			return "", errorAt(s.pos, CodeImmutableName, "cannot assign to immutable name %q", s.name)
		}
		if typ != s.binding.typ {
			err := errorAt(s.pos, CodeAssignmentType, "%s", narrated(fmt.Sprintf("assignment type %s does not match %s", typ, s.binding.typ), narrate(s.expr)))
			return "", withSpanRange(err, s.pos, s.expr.end)
		}
		// Narration must track the current value, not the defining one.
		s.binding.origin = originOf(s.expr)
		return unitType, nil
	case "if":
		if typ != boolType {
			err := withHint(errorAt(s.expr.pos, CodeConditionType, "%s", narrated(fmt.Sprintf("condition must have type Bool, found %s", typ), narrate(s.expr))), tryHint(typ))
			return "", withSpan(err, s.expr)
		}
		left, err := c.block(s.body, newScope(env))
		if err != nil {
			return "", err
		}
		right, err := c.block(s.otherwise, newScope(env))
		if err != nil {
			return "", err
		}
		if left != right {
			return "", errorAt(s.pos, CodeBranchType, "branch result types must match: %s and %s", left, right)
		}
		return left, nil
	case "for":
		end, err := c.expression(s.end, env)
		if err != nil {
			return "", err
		}
		if typ != intType || end != intType {
			return "", errorAt(s.pos, CodeRangeType, "range bounds must have type Int")
		}
		inner := newScope(env)
		s.binding, err = inner.define(s.pos, s.name, intType, false)
		if err != nil {
			return "", err
		}
		_, err = c.block(s.body, inner)
		return unitType, err
	case "for-lines":
		if typ != stringType {
			return "", errorAt(s.pos, CodeRangeType, "file.lines path must have type String")
		}
		inner := newScope(env)
		s.binding, err = inner.define(s.pos, s.name, stringType, false)
		if err != nil {
			return "", err
		}
		_, err = c.block(s.body, inner)
		return unitType, err
	case "flow":
		for _, step := range s.steps {
			typ, err = c.call(step, []valueType{typ})
			if err != nil {
				return "", err
			}
		}
		return typ, nil
	}
	return "", errorAt(s.pos, CodeUnsupportedCode, "unsupported statement %q", s.kind)
}

func (c *checker) expression(x *expression, env *scope) (valueType, error) {
	var typ valueType
	switch x.kind {
	case "int":
		typ = intType
	case "bool":
		typ = boolType
	case "string":
		typ = stringType
	case "name":
		x.binding = env.find(x.text)
		if x.binding == nil {
			return "", withSpan(withHint(errorAt(x.pos, CodeUnknownName, "unknown name %q", x.text), nameHint(x.text, append(env.names(), c.wordNames()...))), x)
		}
		typ = x.binding.typ
	case "call":
		args := make([]valueType, len(x.args))
		for i, arg := range x.args {
			var err error
			args[i], err = c.expression(arg, env)
			if err != nil {
				return "", err
			}
		}
		return c.call(x, args)
	case "try":
		inner, err := c.expression(x.left, env)
		if err != nil {
			return "", err
		}
		unwrapped, ok := resultInner(inner)
		if !ok {
			return "", withSpan(errorAt(x.pos, CodeTryInput, "try requires a Result, found %s", inner), x)
		}
		typ = unwrapped
	case "unary":
		inner, err := c.expression(x.left, env)
		if err != nil {
			return "", err
		}
		typ = intType
		if x.text == "not" {
			typ = boolType
		}
		if inner != typ {
			return "", valueError(x.pos, x, CodeUnaryType, fmt.Sprintf("operator %q requires type %s, found %s", x.text, typ, inner), tryHint(inner), x.left)
		}
	case "binary":
		left, err := c.expression(x.left, env)
		if err != nil {
			return "", err
		}
		right, err := c.expression(x.right, env)
		if err != nil {
			return "", err
		}
		if left != right {
			// Narrate the result-typed operand first: it is the surprising one.
			nLeft, nRight := x.left, x.right
			if isResult(right) && !isResult(left) {
				nLeft, nRight = x.right, x.left
			}
			return "", valueError(x.pos, x, CodeOperandTypes, fmt.Sprintf("operator %q has incompatible operand types %s and %s", x.text, left, right), tryHint(left, right), nLeft, nRight)
		}
		typ = left
		switch x.text {
		case "and", "or":
			if left != boolType {
				return "", valueError(x.pos, x, CodeBoolOperand, fmt.Sprintf("operator %q requires type Bool", x.text), tryHint(left), x.left, x.right)
			}
		case "==", "!=":
			if left != intType && left != stringType && left != boolType {
				return "", valueError(x.pos, x, CodeCompareType, fmt.Sprintf("cannot compare type %s", left), tryHint(left), x.left, x.right)
			}
			typ = boolType
		case "<", "<=", ">", ">=":
			if left != intType {
				return "", valueError(x.pos, x, CodeOrderingType, "ordering requires type Int", tryHint(left), x.left, x.right)
			}
			typ = boolType
		case "+":
			if left != intType && left != stringType {
				return "", valueError(x.pos, x, CodeAdditionType, "addition requires type Int or String", tryHint(left), x.left, x.right)
			}
		default:
			if left != intType {
				return "", valueError(x.pos, x, CodeArithmeticType, "arithmetic requires type Int", tryHint(left), x.left, x.right)
			}
		}
	default:
		return "", errorAt(x.pos, CodeUnsupportedCode, "unsupported expression %q", x.kind)
	}
	x.typ = typ
	return typ, nil
}

func (c *checker) call(x *expression, args []valueType) (valueType, error) {
	var params []valueType
	var result valueType
	if w, m := c.resolveWord(x.text); w != nil {
		if m != nil {
			return "", withHint(withSpan(errorAt(x.pos, CodePrivateUse, "%q is private to module %q", w.name, m.name), x),
				fmt.Sprintf("remove the leading underscore in %s to export it", m.path))
		}
		x.word = w
		result = w.result
		for _, p := range w.params {
			params = append(params, p.typ)
		}
	} else {
		switch x.text {
		case "print":
			if len(args) != 1 {
				return "", withSpan(errorAt(x.pos, CodeArgumentCount, "print requires 1 argument, found %d", len(args)), x)
			}
			if args[0] != intType && args[0] != stringType && args[0] != boolType {
				var span *expression
				if len(x.args) > 0 {
					span = x.args[0]
				}
				return "", valueError(x.pos, span, CodePrintType, fmt.Sprintf("print does not support type %s", args[0]), printHint(args[0]), x.args...)
			}
			params = args
			result = unitType
		case "int.to-string":
			params = []valueType{intType}
			result = stringType
		case "text.reverse":
			params = []valueType{stringType}
			result = stringType
		case "text.to-int":
			params = []valueType{stringType}
			result = intParseResultType
		case "text.length", "text.line-count", "text.word-count":
			params = []valueType{stringType}
			result = intType
		case "text.upper", "text.lower", "text.trim":
			params = []valueType{stringType}
			result = stringType
		case "text.contains":
			params = []valueType{stringType, stringType}
			result = boolType
		case "text.index-of":
			params = []valueType{stringType, stringType}
			result = intType
		case "text.slice":
			params = []valueType{stringType, intType, intType}
			result = stringType
		case "text.replace":
			params = []valueType{stringType, stringType, stringType}
			result = stringType
		case "buffer.new":
			result = bufferType
		case "buffer.add-line":
			params = []valueType{bufferType, stringType}
			result = unitType
		case "buffer.line-count":
			params = []valueType{bufferType}
			result = intType
		case "buffer.line":
			params = []valueType{bufferType, intType}
			result = stringBufferResultType
		case "buffer.set-line":
			params = []valueType{bufferType, intType, stringType}
			result = unitBufferResultType
		case "buffer.to-text":
			params = []valueType{bufferType}
			result = stringType
		case "file.read-all":
			params = []valueType{stringType}
			result = stringFileResultType
		case "file.read-line":
			params = []valueType{fileType}
			result = stringFileResultType
		case "file.read-lines":
			params = []valueType{stringType}
			result = bufferFileResultType
		case "file.open":
			params = []valueType{stringType}
			result = fileResultType
		case "file.close":
			params = []valueType{fileType}
			result = unitType
		case "console.arg":
			params = []valueType{intType}
			result = stringResultType
		case "console.args-count":
			result = intType
		case "read-line":
			params = []valueType{stringType}
			result = stringResultType
		case "range", "file.lines":
			return "", errorAt(x.pos, CodeRangeContext, "%s is only available in a for loop", x.text)
		default:
			return "", withSpan(withHint(errorAt(x.pos, CodeUnknownWord, "unknown word %q", x.text), suggestWordHint(x.text, c.wordNames())), x)
		}
	}
	if len(args) != len(params) {
		return "", withSpan(errorAt(x.pos, CodeArgumentCount, "%s requires %d arguments, found %d", x.text, len(params), len(args)), x)
	}
	for i, typ := range args {
		if typ != params[i] {
			var span *expression
			var operands []*expression
			if i < len(x.args) {
				span = x.args[i]
				operands = x.args[i : i+1]
			}
			msg := fmt.Sprintf("argument %d of %s requires type %s, found %s", i+1, x.text, params[i], typ)
			return "", valueError(x.pos, span, CodeArgumentType, msg, tryHint(typ), operands...)
		}
	}
	x.typ = result
	if len(args) > 0 {
		x.inputType = args[0]
	}
	return result, nil
}

// valueError composes the standard value-type mismatch diagnostic: a narrated
// message, an optional hint, and an optional caret span. The message is
// runtime-built, so it is never passed to errorAt as a format string; the
// operands feed the provenance narration.
func valueError(pos position, span *expression, code Code, msg, hint string, operands ...*expression) error {
	err := withHint(errorAt(pos, code, "%s", narrated(msg, narrate(operands...))), hint)
	if span != nil {
		err = withSpan(err, span)
	}
	return err
}
