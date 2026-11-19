package lang

import (
	"fmt"
	"go/format"
	"sort"
	"strconv"
	"strings"
)

// CompileNativeSource checks the program exactly like CompileWat and
// emits the Go source of a standalone native program — the Option 1
// report on #6: the typed tree maps almost line-for-line onto Go, words
// become functions with concrete signatures, fallible words return
// (value, *zhexrt.Error), and hard runtime failures panic into zhexrt.Run
// which renders them exactly as zhex run's diagnostics. Constructs
// outside the compiled subset fail with a positioned
// not-available-in-build diagnostic. The caller (internal/native) writes
// this main.go beside a go.mod and the zhexrt runtime sources and builds
// with the Go toolchain.
func CompileNativeSource(entryPath, entrySource string, resolve func(string) (ModuleSource, bool)) (mainGo string, err error) {
	p, cerr := CompileModules(entryPath, entrySource, resolve)
	if cerr != nil {
		return "", cerr
	}
	defer func() {
		if r := recover(); r != nil {
			if d, ok := r.(*diagnostic); ok {
				mainGo, err = "", annotateSource(d, p.sources)
				return
			}
			panic(r)
		}
	}()
	e := &nativeEmitter{p: p}
	e.emit()
	// gofmt the artifact: the emitter writes flat preambles and blocks
	// without managing columns, and readable output is the product.
	source, ferr := format.Source([]byte(e.b.String()))
	if ferr != nil {
		source = []byte(e.b.String()) // keep the raw text for debugging; tests compile it and fail loudly
	}
	return string(source), nil
}

// nativeFence rejects an out-of-subset construct with the same pinned
// diagnostic and milestone-neutral hint the wasm backend uses.
func nativeFence(pos position, what string) {
	panic(withHint(errorAt(pos, CodeNotAvailableInBuild, "%s is not available in this build", what),
		"this construct is not compiled by zhex compile yet; run it with zhex run instead"))
}

// nativeFunc is the per-word emission context: how errors return from the
// current function, the local-name scope, hoisted expression preambles,
// and the pending cleanups every early return must run.
type nativeFunc struct {
	word      *word
	fallible  bool // returns (T, *zhexrt.Error); the entry returns *zhexrt.Error
	locals    map[string]string
	localUsed map[string]bool
	pre       *strings.Builder
	refs      map[*binding]bool // bindings the body reads
}

// nativeEmitter walks a checked Program and writes Go.
type nativeEmitter struct {
	p     *Program
	b     strings.Builder
	depth int

	words    []*word
	seenWord map[*word]bool
	records  []*recordType
	byID     map[string]*recordType

	goNames   map[string]string
	usedNames map[string]bool

	posVars map[string]string

	tempCount  int
	flowFirst  string
	fn         *nativeFunc

	// bindingRefs collects the bindings a function body reads, so lets
	// Zhex never uses can evaluate for effect without a Go declaration
	// (Go rejects unused declarations; Zhex allows them).
	bindingRefs map[*binding]bool
}

func (e *nativeEmitter) line(format string, args ...any) {
	e.b.WriteString(strings.Repeat("\t", e.depth))
	fmt.Fprintf(&e.b, format, args...)
	e.b.WriteByte('\n')
}

// pre appends a hoisted statement for the expression being emitted; the
// final gofmt pass indents, so the preamble can be flat.
func (e *nativeEmitter) pre(format string, args ...any) {
	fmt.Fprintf(e.fn.pre, format, args...)
	e.fn.pre.WriteByte('\n')
}

// ---------------------------------------------------------------- names

// goReserved blocks Go keywords, predeclared identifiers, and the
// emitter's own scaffolding from mangling collisions.
var goReserved = map[string]bool{
	"break": true, "case": true, "chan": true, "const": true, "continue": true,
	"default": true, "defer": true, "else": true, "fallthrough": true, "for": true,
	"func": true, "go": true, "goto": true, "if": true, "import": true,
	"interface": true, "map": true, "package": true, "range": true, "return": true,
	"select": true, "struct": true, "switch": true, "type": true, "var": true,
	"append": true, "cap": true, "clear": true, "close": true, "complex": true,
	"copy": true, "delete": true, "imag": true, "len": true, "make": true,
	"max": true, "min": true, "new": true, "panic": true, "print": true,
	"println": true, "real": true, "recover": true, "any": true, "bool": true,
	"byte": true, "comparable": true, "error": true, "float32": true,
	"float64": true, "int": true, "int16": true, "int32": true, "int64": true,
	"int8": true, "rune": true, "string": true, "uint": true, "uint16": true,
	"uint32": true, "uint64": true, "uint8": true, "uintptr": true, "true": true,
	"false": true, "iota": true, "nil": true, "init": true, "main": true,
	"zhexMain": true, "zero": true, "stringer": true,
}

// goMangle spells one Zhex name as a Go identifier: hyphens and dots
// become underscores, other runes escape, keywords and predeclared names
// take a trailing underscore.
func goMangle(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_':
			b.WriteRune(r)
		case r == '-' || r == '.':
			b.WriteByte('_')
		default:
			fmt.Fprintf(&b, "_u%04X", r)
		}
	}
	out := b.String()
	if strings.HasPrefix(out, "__") {
		out = "_" + out // keep the __ prefix for emitter temps
	}
	if goReserved[out] {
		out += "_"
	}
	return out
}

// packageName returns the Go identifier for a package-level Zhex name,
// deduplicated across words, records, and scaffolding.
func (e *nativeEmitter) packageName(key, name string) string {
	if got, ok := e.goNames[key]; ok {
		return got
	}
	candidate := goMangle(name)
	for i := 2; e.usedNames[candidate]; i++ {
		candidate = fmt.Sprintf("%s_%d", goMangle(name), i)
	}
	e.goNames[key] = candidate
	e.usedNames[candidate] = true
	return candidate
}

// localName returns the Go identifier for a binding inside the current
// function, deduplicated against other locals, the package-level names,
// and the emitter's reserved words.
func (e *nativeEmitter) localName(name string) string {
	if got, ok := e.fn.locals[name]; ok {
		return got
	}
	candidate := goMangle(name)
	for i := 2; e.fn.localUsed[candidate] || e.usedNames[candidate]; i++ {
		candidate = fmt.Sprintf("%s_%d", goMangle(name), i)
	}
	e.fn.locals[name] = candidate
	e.fn.localUsed[candidate] = true
	return candidate
}

func (e *nativeEmitter) temp() string {
	e.tempCount++
	return fmt.Sprintf("__t%d", e.tempCount)
}

// posVar returns the package-level position variable for a source
// position, declaring it on first use.
func (e *nativeEmitter) posVar(pos position) string {
	key := fmt.Sprintf("%s:%d:%d", pos.file, pos.line, pos.column)
	if got, ok := e.posVars[key]; ok {
		return got
	}
	name := fmt.Sprintf("_p%d_%d", pos.line, pos.column)
	for i := 2; e.usedNames[name]; i++ {
		name = fmt.Sprintf("_p%d_%d_%d", pos.line, pos.column, i)
	}
	e.posVars[key] = name
	e.usedNames[name] = true
	return name
}

// writePosVars writes the position variable declarations after the walk,
// when every use is known, in first-use order.
func (e *nativeEmitter) writePosVars() {
	if len(e.posVars) == 0 {
		return
	}
	e.line("// Runtime positions: every operation that can fail reports its source.")
	keys := make([]string, 0, len(e.posVars))
	for key := range e.posVars {
		keys = append(keys, key)
	}
	order := map[string]int{}
	for i, key := range keys {
		order[e.posVars[key]] = i
	}
	sort.Slice(keys, func(i, j int) bool { return e.posVars[keys[i]] < e.posVars[keys[j]] })
	for _, key := range keys {
		parts := strings.SplitN(key, ":", 3)
		line, _ := strconv.Atoi(parts[1])
		column, _ := strconv.Atoi(parts[2])
		e.line("var %s = zhexrt.At(%q, %d, %d)", e.posVars[key], parts[0], line, column)
	}
}

// ---------------------------------------------------------------- types

// goType maps a Zhex type to its Go representation. The checker is
// monomorphic, so the mapping is total on the compiled subset: Int is the
// interpreter's two-limb struct, Results collapse to one runtime repr,
// and anything unsupported fences.
func (e *nativeEmitter) goType(pos position, t valueType) string {
	switch t {
	case intType:
		return "zhexrt.Int"
	case boolType:
		return "bool"
	case stringType:
		return "string"
	case unitType:
		return "zhexrt.Unit"
	}
	if inner, ok := resultInner(t); ok {
		return "zhexrt.Result[" + e.goType(pos, inner) + "]"
	}
	nativeFence(pos, fmt.Sprintf("%s values", t))
	return ""
}

// ---------------------------------------------------------------- walk

// collect gathers every reachable word and record exactly like the WAT
// emitter's collect: entry words in declaration order, then module words
// in first-reference order, walking the entry body first.
func (e *nativeEmitter) collect() {
	e.seenWord = map[*word]bool{}
	e.byID = map[string]*recordType{}
	e.goNames = map[string]string{}
	e.usedNames = map[string]bool{}
	e.posVars = map[string]string{}
	for _, r := range e.p.records {
		e.addRecord(r)
	}
	for _, w := range e.p.words {
		e.addWord(w)
	}
	e.walkStatements(e.p.body)
	for i := 0; i < len(e.words); i++ {
		e.walkStatements(e.words[i].body)
	}
}

func (e *nativeEmitter) addWord(w *word) {
	if w == nil || e.seenWord[w] {
		return
	}
	e.seenWord[w] = true
	e.words = append(e.words, w)
}

func (e *nativeEmitter) addRecord(r *recordType) {
	if r == nil || r.builtin || e.byID[r.id()] != nil {
		return
	}
	e.records = append(e.records, r)
	e.byID[r.id()] = r
}

// walkStatements enqueues words and records referenced by expressions.
func (e *nativeEmitter) walkStatements(body []*statement) {
	for _, s := range body {
		e.walkStatement(s)
	}
}

func (e *nativeEmitter) walkStatement(s *statement) {
	if e.bindingRefs != nil && s.binding != nil && s.kind == "assign" {
		e.bindingRefs[s.binding] = true
	}
	e.walkExpression(s.expr)
	e.walkExpression(s.end)
	e.walkStatements(s.body)
	e.walkStatements(s.otherwise)
	for _, step := range s.steps {
		e.walkExpression(step)
	}
	if s.expr != nil {
		switch s.expr.kind {
		case "match":
			for _, arm := range s.expr.arms {
				e.walkStatements(arm.body)
			}
		case "catch":
			e.walkStatements(s.expr.handler)
		}
	}
}

func (e *nativeEmitter) walkExpression(x *expression) {
	if x == nil {
		return
	}
	if e.bindingRefs != nil && x.kind == "name" && x.binding != nil {
		e.bindingRefs[x.binding] = true
	}
	if x.word != nil {
		e.addWord(x.word)
	}
	if x.record != nil {
		e.addRecord(x.record)
	}
	e.walkExpression(x.left)
	e.walkExpression(x.right)
	for _, arg := range x.args {
		e.walkExpression(arg)
	}
	e.walkStatements(x.handler)
	if x.statement != nil {
		e.walkStatement(x.statement)
	}
	for _, arm := range x.arms {
		e.walkStatements(arm.body)
	}
}

// ---------------------------------------------------------------- emit

func (e *nativeEmitter) emit() {
	e.collect()
	e.emitBody()
	body := e.b.String()
	e.b = strings.Builder{}
	e.emitHeader()
	e.writePosVars()
	e.line("")
	e.b.WriteString(body)
}

// emitHeader writes the package clause, the zhexrt import, and the
// embedded sources the caret rendering reads.
func (e *nativeEmitter) emitHeader() {
	e.line("// Code generated by zhex compile --target=native. Edit the Zhex source, not this file.")
	e.line("package main")
	e.line("")
	e.line("import \"zhexprog/zhexrt\"")
	e.line("")
	e.line("func init() {")
	e.depth++
	for path, source := range e.p.sources {
		e.line("zhexrt.Sources[%q] = %s", path, strconv.Quote(source))
	}
	e.depth--
	e.line("}")
	e.line("")
}

// emitBody writes the words and the entry; the header and position table
// are assembled around it afterwards.
func (e *nativeEmitter) emitBody() {
	for _, w := range e.words {
		e.emitWord(w)
		e.line("")
	}
	e.emitEntry()
}

// wordKey identifies a word for package-level naming.
func wordKey(w *word) string {
	if w.module == "" {
		return "w:" + w.name
	}
	return "w:" + w.module + "." + w.name
}

// wordGoName is the word's qualified Zhex name, for diagnostics.
func wordGoName(w *word) string {
	if w.module == "" {
		return w.name
	}
	return w.module + "." + w.name
}

// emitWord writes one Zhex word as a Go function. A pure word returns its
// value; a word with an inferred effective Result signature returns
// (value, *zhexrt.Error) where a non-nil error is the Result's failure; a
// word with an explicit Result signature returns the Result directly and
// escaping failures become its Err.
func (e *nativeEmitter) emitWord(w *word) {
	for _, param := range w.params {
		if param.defaultExpr != nil {
			nativeFence(param.pos, "word parameter defaults")
		}
	}
	e.fn = &nativeFunc{word: w, fallible: w.inferred, locals: map[string]string{}, localUsed: map[string]bool{}, refs: e.collectRefs(w.body)}
	e.fn.pre = &strings.Builder{}
	resultType := e.goType(w.pos, w.result)
	if w.inferred {
		resultType = e.goType(w.pos, w.success)
	}

	var params []string
	for _, param := range w.params {
		params = append(params, e.localName(param.name)+" "+e.goType(param.pos, param.typ))
	}
	name := e.packageName(wordKey(w), wordGoName(w))
	if w.inferred {
		e.line("func %s(%s) (%s, *zhexrt.Error) {", name, strings.Join(params, ", "), resultType)
	} else {
		e.line("func %s(%s) %s {", name, strings.Join(params, ", "), resultType)
	}
	e.depth++
	e.line("zhexrt.EnterWord(%s, %q)", e.posVar(w.pos), wordGoName(w))
	e.line("defer zhexrt.LeaveWord()")
	e.line("var __result %s", resultType)
	e.statements(w.body, "__result")
	if w.inferred {
		e.line("return __result, nil")
	} else {
		e.line("return __result")
	}
	e.depth--
	e.line("}")
}

// collectRefs prewalks a body collecting every binding it reads, so the
// emitter can skip Go declarations Zhex itself never uses.
func (e *nativeEmitter) collectRefs(body []*statement) map[*binding]bool {
	e.bindingRefs = map[*binding]bool{}
	e.walkStatements(body)
	refs := e.bindingRefs
	e.bindingRefs = nil
	return refs
}

// failReturn emits the early-error return for the current function shape:
// the entry returns the diagnostic, a fallible word pairs it with the
// zero of its success type, and an explicit-Result word makes it the
// returned Result's Err.
func (e *nativeEmitter) failReturn(errExpr string) {
	if e.fn.word == nil {
		e.pre("return %s", errExpr)
		return
	}
	if e.fn.fallible {
		e.pre("return zhexrt.Zero[%s](), %s", e.goType(e.fn.word.pos, e.fn.word.success), errExpr)
		return
	}
	inner, _ := resultInner(e.fn.word.result)
	e.pre("return zhexrt.Result[%s]{Err: %s}", e.goType(e.fn.word.pos, inner), errExpr)
}

// emitEntry writes zhexMain (the top-level statements, or the entry's
// zero-argument main word) and the Go main that runs it.
func (e *nativeEmitter) emitEntry() {
	e.fn = &nativeFunc{locals: map[string]string{}, localUsed: map[string]bool{}, refs: e.collectRefs(e.p.body)}
	e.fn.pre = &strings.Builder{}
	e.line("func zhexMain() *zhexrt.Error {")
	e.depth++
	if len(e.p.body) == 0 {
		if main := entryMainWord(e.p); main != nil {
			name := e.packageName(wordKey(main), wordGoName(main))
			if main.inferred {
				e.line("_, err := %s()", name)
				e.line("return err")
			} else if _, ok := resultInner(main.result); ok {
				e.line("return %s().Err", name)
			} else {
				e.line("%s()", name)
				e.line("return nil")
			}
		}
	} else {
		e.statements(e.p.body, "")
		e.line("return nil")
	}
	e.depth--
	e.line("}")
	e.line("")
	e.line("func main() {")
	e.depth++
	e.line("zhexrt.Run(zhexMain)")
	e.depth--
	e.line("}")
}

// statements emits a Zhex block; target is the lvalue receiving the
// block's value ("" discards it).
func (e *nativeEmitter) statements(body []*statement, target string) {
	for i, s := range body {
		last := i == len(body)-1
		if last && target != "" && blockValueKind(s) {
			e.statement(s, target)
		} else {
			e.statement(s, "")
		}
	}
}

// blockValueKind reports whether a statement can carry its block's value.
func blockValueKind(s *statement) bool {
	switch s.kind {
	case "expression", "if", "flow", "return":
		return true
	}
	return false
}

// statement emits one statement; target receives its value when the
// statement is its block's tail ("" discards).
func (e *nativeEmitter) statement(s *statement, target string) {
	switch s.kind {
	case "expression":
		e.fn.pre = &strings.Builder{}
		v := e.expr(s.expr)
		e.flushPre()
		if target != "" && s.expr.typ != unitType {
			e.line("%s = %s", target, v)
		} else if !isCallShaped(s.expr) {
			e.line("_ = %s", v)
		} else {
			e.line("%s", v)
		}
	case "return":
		e.fn.pre = &strings.Builder{}
		v := e.expr(s.expr)
		e.flushPre()
		if e.fn.word != nil && e.fn.fallible {
			e.line("return %s, nil", v)
		} else {
			e.line("return %s", v)
		}
	case "let", "var", "assign":
		if s.kind != "assign" && !e.fn.refs[s.binding] {
			// never referenced later: evaluate the value for its effect
			e.fn.pre = &strings.Builder{}
			v := e.expr(s.expr)
			e.flushPre()
			if isCallShaped(s.expr) {
				e.line("%s", v)
			} else {
				e.line("_ = %s", v)
			}
			return
		}
		name := e.localName(s.name)
		if s.expr != nil && s.expr.kind == "statement-value" {
			e.line("var %s %s", name, e.goType(s.pos, s.binding.typ))
			e.statement(s.expr.statement, name)
			return
		}
		e.fn.pre = &strings.Builder{}
		v := e.expr(s.expr)
		e.flushPre()
		if s.kind == "assign" {
			e.line("%s = %s", name, v)
		} else {
			e.line("%s := %s", name, v)
		}
	case "if":
		e.fn.pre = &strings.Builder{}
		cond := e.expr(s.expr)
		e.flushPre()
		e.line("if %s {", cond)
		e.depth++
		e.statements(s.body, target)
		e.depth--
		if len(s.otherwise) != 0 {
			e.line("} else {")
			e.depth++
			e.statements(s.otherwise, target)
			e.depth--
		}
		e.line("}")
	case "flow":
		e.emitFlow(s, target)
	default:
		nativeFence(s.pos, s.kind+" statements")
	}
}

// flushPre writes the hoisted expression preamble ahead of the statement
// line being assembled.
func (e *nativeEmitter) flushPre() {
	text := e.fn.pre.String()
	e.fn.pre.Reset()
	if text != "" {
		e.b.WriteString(text)
	}
}

// isCallShaped reports whether the expression's value is a plain call
// that may stand alone as a Go statement; try and catch materialize
// temps, so they need an explicit discard even though they wrap calls.
func isCallShaped(x *expression) bool {
	return x != nil && x.kind == "call"
}

// emitFlow writes a threaded flow or a with block as straight-line calls.
func (e *nativeEmitter) emitFlow(s *statement, target string) {
	if s.with != "" {
		nativeFence(s.withPos, "with blocks")
	}
	e.fn.pre = &strings.Builder{}
	cursor := e.expr(s.expr)
	e.flushPre()
	for index, step := range s.steps {
		e.fn.pre = &strings.Builder{}
		last := index == len(s.steps)-1
		value := e.stepValue(step, cursor)
		e.flushPre()
		if last && target != "" && step.typ != unitType {
			e.line("%s = %s", target, value)
			continue
		}
		if last || step.typ == unitType {
			// a Unit step (or the discarded tail) runs for its effect
			if isCallShaped(step) {
				e.line("%s", value)
			} else {
				e.line("_ = %s", value)
			}
			cursor = "zhexrt.Unit{}"
			continue
		}
		next := e.temp()
		e.line("%s := %s", next, value)
		cursor = next
	}
}

// stepValue emits one flow step receiving first as its first argument.
func (e *nativeEmitter) stepValue(step *expression, first string) string {
	e.flowFirst = first
	defer func() { e.flowFirst = "" }()
	return e.expr(step)
}

// ---------------------------------------------------------------- expressions

// expr emits an expression, hoisting statements it needs into the
// current preamble, and returns its Go expression string.
func (e *nativeEmitter) expr(x *expression) string {
	switch x.kind {
	case "int":
		if x.number.Fits64() {
			return fmt.Sprintf("zhexrt.I64(%d)", x.number.AsInt64())
		}
		return fmt.Sprintf("zhexrt.Int{Lo: %d, Hi: %d}", x.number.Lo, x.number.Hi)
	case "bool":
		return x.text
	case "string":
		return strconv.Quote(x.text)
	case "name":
		if x.record != nil && x.record.choice {
			nativeFence(x.pos, "choices")
		}
		v := e.localName(x.text)
		if len(x.fields) != 0 {
			nativeFence(x.pos, "record fields")
		}
		return v
	case "unary":
		v := e.expr(x.left)
		if x.text == "not" {
			return "!" + v
		}
		return fmt.Sprintf("zhexrt.Neg(%s, %s)", e.posVar(x.pos), v)
	case "binary":
		return e.binary(x)
	case "call":
		return e.call(x)
	case "try":
		return e.tryExpr(x)
	case "catch":
		return e.catchExpr(x)
	case "statement-value":
		nativeFence(x.pos, "block values in expression position")
	}
	nativeFence(x.pos, x.kind+" expressions")
	return ""
}

func (e *nativeEmitter) binary(x *expression) string {
	left := e.expr(x.left)
	switch x.text {
	case "and", "or":
		save := e.fn.pre
		e.fn.pre = &strings.Builder{}
		right := e.expr(x.right)
		rightPre := e.fn.pre.String()
		e.fn.pre = save
		op := "&&"
		if x.text == "or" {
			op = "||"
		}
		if rightPre == "" {
			return fmt.Sprintf("%s %s %s", left, op, right)
		}
		// Short-circuit: the right side's preamble must not run when the
		// left side already decides.
		temp := e.temp()
		e.pre("%s := %s", temp, left)
		e.pre("if %s %s {", temp, op)
		e.pre("%s", rightPre)
		e.pre("%s = %s", temp, right)
		e.pre("}")
		return temp
	}
	right := e.expr(x.right)
	switch x.text {
	case "==", "!=":
		op := x.text
		return fmt.Sprintf("%s %s %s", left, op, right)
	case "<", "<=", ">", ">=":
		order := map[string]string{"<": "< 0", "<=": "<= 0", ">": "> 0", ">=": ">= 0"}[x.text]
		return fmt.Sprintf("zhexrt.Cmp(%s, %s) %s", left, right, order)
	case "+":
		if x.typ == stringType {
			if x.left.typ == intType {
				left = "zhexrt.Str(" + left + ")"
			}
			if x.right.typ == intType {
				right = "zhexrt.Str(" + right + ")"
			}
			return left + " + " + right
		}
		return fmt.Sprintf("zhexrt.Add(%s, %s, %s)", e.posVar(x.pos), left, right)
	case "-":
		return fmt.Sprintf("zhexrt.Sub(%s, %s, %s)", e.posVar(x.pos), left, right)
	case "*":
		return fmt.Sprintf("zhexrt.Mul(%s, %s, %s)", e.posVar(x.pos), left, right)
	case "/":
		return fmt.Sprintf("zhexrt.Div(%s, %s, %s)", e.posVar(x.pos), left, right)
	case "%":
		return fmt.Sprintf("zhexrt.Rem(%s, %s, %s)", e.posVar(x.pos), left, right)
	}
	nativeFence(x.pos, x.text+" expressions")
	return ""
}

// isFallibleCall reports whether x is a call to a word with an inferred
// effective Result signature (the (value, error) Go shape).
func isFallibleCall(x *expression) bool {
	return x.kind == "call" && x.word != nil && x.word.inferred && x.binding == nil && !x.partial
}

// callValue emits a user-word call: pure words are plain expressions;
// fallible words materialize a (value, error) pair.
func (e *nativeEmitter) callValue(x *expression) (v, errv string) {
	args := e.callArgs(x)
	name := e.packageName(wordKey(x.word), wordGoName(x.word))
	call := fmt.Sprintf("%s(%s)", name, strings.Join(args, ", "))
	if !x.word.inferred {
		return call, ""
	}
	v, errv = e.temp(), e.temp()
	e.pre("%s, %s := %s", v, errv, call)
	return v, errv
}

// callArgs emits the argument expressions in order, substituting the flow
// cursor for the synthetic flow-value placeholder.
func (e *nativeEmitter) callArgs(x *expression) []string {
	var args []string
	for _, arg := range x.args {
		if arg.kind == "flow-value" {
			args = append(args, e.flowFirst)
			continue
		}
		args = append(args, e.expr(arg))
	}
	return args
}

func (e *nativeEmitter) call(x *expression) string {
	if x.partial || x.binding != nil {
		nativeFence(x.pos, "partials")
	}
	if x.record != nil {
		nativeFence(x.pos, "record constructors")
	}
	if x.word != nil {
		v, errv := e.callValue(x)
		if errv == "" {
			return v
		}
		// plain context: the call's value is its Result
		result := fmt.Sprintf("zhexrt.Result[%s]{Value: %s, Err: %s}", e.goType(x.pos, x.word.success), v, errv)
		return result
	}
	return e.builtin(x)
}

// tryExpr unwraps a Result and propagates its failure to the word
// boundary (or reports it at the top level).
func (e *nativeEmitter) tryExpr(x *expression) string {
	if isFallibleCall(x.left) {
		v, errv := e.callValue(x.left)
		e.pre("if %s != nil {", errv)
		e.failReturn(errv)
		e.pre("}")
		return v
	}
	r := e.expr(x.left)
	temp := e.temp()
	e.pre("%s := %s", temp, r)
	e.pre("if %s.Err != nil {", temp)
	e.failReturn(temp + ".Err")
	e.pre("}")
	return temp + ".Value"
}

// catchExpr handles a failed Result with a fallback value or block; the
// optional binding receives the error.
func (e *nativeEmitter) catchExpr(x *expression) string {
	inner, _ := resultInner(x.left.typ)
	temp := e.temp()
	if isFallibleCall(x.left) {
		v, errv := e.callValue(x.left)
		e.pre("%s := zhexrt.Result[%s]{Value: %s, Err: %s}", temp, e.goType(x.pos, inner), v, errv)
	} else {
		e.pre("%s := %s", temp, e.expr(x.left))
	}
	out := e.temp()
	e.pre("var %s %s", out, e.goType(x.pos, x.typ))
	e.pre("if %s.Err != nil {", temp)
	if x.binding != nil && e.fn.refs[x.binding] {
		e.pre("%s := %s.Err", e.localName(x.binding.name), temp)
	}
	// The handler is a block: emit it into a scratch buffer so its
	// statements (and their own preambles) land inside the if.
	savedB, savedPre := e.b, e.fn.pre
	e.b, e.fn.pre = strings.Builder{}, &strings.Builder{}
	e.statements(x.handler, out)
	handler := e.b.String() + e.fn.pre.String()
	e.b, e.fn.pre = savedB, savedPre
	e.fn.pre.WriteString(handler)
	e.pre("} else {")
	e.pre("%s = %s.Value", out, temp)
	e.pre("}")
	return out
}

// builtin emits the compiled builtins; anything else fences with its
// name, so the diagnostic names exactly what the program used.
func (e *nativeEmitter) builtin(x *expression) string {
	pos := e.posVar(x.pos)
	args := e.callArgs(x)
	switch x.text {
	case "print":
		arg := args[0]
		if x.args[0].typ == intType {
			return fmt.Sprintf("zhexrt.PrintInt(%s, %s)", pos, arg)
		}
		if x.args[0].typ != stringType {
			nativeFence(x.pos, "print of "+string(x.args[0].typ))
		}
		return fmt.Sprintf("zhexrt.Print(%s, %s)", pos, arg)
	case "read-line":
		return fmt.Sprintf("zhexrt.ReadLine(%s, %s)", pos, e.expr(x.args[0]))
	case "text.to-int":
		return fmt.Sprintf("zhexrt.ToInt(%s, %s)", pos, e.expr(x.args[0]))
	case "int.to-string":
		return fmt.Sprintf("zhexrt.Str(%s)", e.expr(x.args[0]))
	}
	nativeFence(x.pos, x.text)
	return ""
}
