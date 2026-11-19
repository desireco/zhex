package lang

import (
	"fmt"
	"strings"
)

// CompileWat checks the program exactly like CompileModules (checker errors
// return unchanged) and emits the full WebAssembly text: the skin's host
// imports, the runtime preludes, the skin's host layer, program-specific
// types, one wasm function per word, zhex_main for the top-level statements,
// and the exports (internal/wasm/DESIGN.md §1, §7; DESIGN-M3 §1). Constructs
// outside the compiled subset fail with a positioned not-available-in-build
// diagnostic. The skin selects the host layer (browser by default, D4).
func CompileWat(entryPath, entrySource string, resolve func(string) (ModuleSource, bool), skin WatSkin) (wat string, err error) {
	p, cerr := CompileModules(entryPath, entrySource, resolve)
	if cerr != nil {
		return "", cerr
	}
	defer func() {
		if r := recover(); r != nil {
			if d, ok := r.(*diagnostic); ok {
				wat, err = "", annotateSource(d, p.sources)
				return
			}
			panic(r)
		}
	}()
	e := &watEmitter{p: p, skin: skin}
	return e.emit(), nil
}

// watFence rejects an out-of-subset construct with the pinned diagnostic.
// The hint is milestone-neutral so it does not churn per milestone
// (DESIGN-M2 §6).
func watFence(pos position, what string) {
	panic(withHint(errorAt(pos, CodeNotAvailableInBuild, "%s is not available in this build", what),
		"this construct is not compiled by zhex compile yet; run it with zhex run instead"))
}

// watCleanup resets one traversal's $active counter on an early word return,
// mirroring eval's deferred active-- on the returnValue path.
type watCleanup struct{ local, structType string }

// watEmitter walks a checked Program and writes WAT. Statements are
// stack-neutral; a block pushes its tail statement's value (nothing for
// Unit), mirroring eval.block. Locals are discovered during emission and
// declared in the function header afterwards.
type watEmitter struct {
	p       *Program
	b       strings.Builder
	depth   int
	pos     position
	idCount int
	skin    WatSkin

	words    []*word
	seenWord map[*word]bool
	records  []*recordType
	byID     map[string]*recordType

	// per-function state
	locals         []string
	localCount     int
	bindings       map[*binding]string
	temps          map[string]string
	resultLocal    string
	wordInferred   bool
	cleanups       []watCleanup
	streams        []string  // open file.stream handles, closed on early word return
	flowCursor     string    // flow step's incoming value local; "" for Unit
	flowCursorType valueType // the cursor value's Zhex type

	thunks     []string // site-local closure thunk texts, appended after $zhex_main
	thunkNames []string // the thunks' function names, for the elem declare
}

func (e *watEmitter) line(format string, args ...any) {
	e.b.WriteString(strings.Repeat("  ", e.depth))
	fmt.Fprintf(&e.b, format, args...)
	e.b.WriteByte('\n')
}

// ---------------------------------------------------------------- collection

// collect gathers every reachable word and record: entry words in
// declaration order, then module words in first-reference order, walking
// the entry body first so its callees (and their constructors) are enqueued
// before any loop over the growing word list runs (module files have no
// top-level statements, so unreferenced module words are dead and never
// emitted). Entry records are always emitted.
func (e *watEmitter) collect() {
	e.seenWord = map[*word]bool{}
	e.byID = map[string]*recordType{}
	// The host-predeclared Json tree resolves like a user choice for match
	// and constructor emission; its variant structs are prelude-owned, so it
	// is never added to e.records (DESIGN-M3 §6).
	e.byID[builtinJSON.id()] = builtinJSON
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

func (e *watEmitter) addWord(w *word) {
	if w == nil || e.seenWord[w] {
		return
	}
	e.seenWord[w] = true
	e.words = append(e.words, w)
}

func (e *watEmitter) addRecord(r *recordType) {
	if r == nil || r.builtin || r == builtinOption || e.byID[r.id()] != nil {
		return
	}
	if r.typeParam != "" {
		watFence(r.pos, "generic choice "+r.id())
	}
	e.byID[r.id()] = r
	e.records = append(e.records, r)
}

func (e *watEmitter) walkStatements(list []*statement) {
	for _, s := range list {
		e.walkExpr(s.expr)
		e.walkExpr(s.end)
		e.walkStatements(s.body)
		e.walkStatements(s.otherwise)
		for _, step := range s.steps {
			e.walkExpr(step)
		}
	}
}

func (e *watEmitter) walkExpr(x *expression) {
	if x == nil {
		return
	}
	e.addWord(x.word)
	e.addRecord(x.record)
	e.walkExpr(x.left)
	e.walkExpr(x.right)
	for _, a := range x.args {
		e.walkExpr(a)
	}
	for i := range x.arms {
		e.walkStatements(x.arms[i].body)
	}
	e.walkStatements(x.handler)
	if x.statement != nil {
		e.walkStatements([]*statement{x.statement})
	}
}

// ------------------------------------------------------------------- module

func (e *watEmitter) emit() string {
	e.collect()
	var out strings.Builder
	// Imports precede every definition (wasm-tools rule); the prelude trio is
	// byte-identical for both skins with the browser-seam defs stripped, and
	// only the host layer differs (DESIGN-M3 §1, watnames_m3's emit recipe).
	out.WriteString(watHostImports(e.skin))
	out.WriteString(watStripSeamDefs(watPrelude + watPreludeM2 + watPreludeM3 + watPreludeM4))
	out.WriteString(watHostFuncs(e.skin))
	out.WriteString("\n;; program types\n")
	for _, r := range e.records {
		e.emitRecordType(&out, r)
	}
	out.WriteString("\n;; words\n")
	for _, w := range e.words {
		out.WriteString(e.wordFunction(w))
	}
	out.WriteString(e.mainFunction())
	if len(e.thunks) > 0 {
		out.WriteString("\n;; closure thunks\n")
		for _, t := range e.thunks {
			out.WriteString(t)
		}
		fmt.Fprintf(&out, "(elem declare func %s)\n", strings.Join(e.thunkNames, " "))
	}
	out.WriteString(`
	(export "memory" (memory $zhex_mem))
(export "zhex_main" (func $zhex_main))
(export "zhex_last_error" (func $zhex_last_error))
`)
	return out.String()
}

func (e *watEmitter) emitRecordType(out *strings.Builder, r *recordType) {
	e.pos = r.pos
	if r.choice {
		for i := range r.variants {
			v := &r.variants[i]
			fmt.Fprintf(out, "(type %s (struct", watVariantName(r, i))
			// WasmGC declared types are structural: same-shape siblings
			// canonicalize together and ref.test cannot tell them apart, so
			// variant i is padded with i leading i64 fields plus an i32
			// discriminant — a later variant differs at position i by
			// construction.
			for d := 0; d < i; d++ {
				fmt.Fprintf(out, " (field $d%d i64)", d)
			}
			fmt.Fprintf(out, " (field $v i32)")
			for j, f := range v.fields {
				fmt.Fprintf(out, " (field $f%d %s)", j, e.watLocalType(f.typ))
			}
			out.WriteString("))\n")
		}
		return
	}
	fmt.Fprintf(out, "(type %s (struct", watRecordName(r))
	for i, f := range r.fields {
		fmt.Fprintf(out, " (field $f%d %s)", i, e.watLocalType(f.typ))
	}
	out.WriteString("))\n")
}

// ---------------------------------------------------------------- functions

func (e *watEmitter) resetFunc(resultType string, inferred bool) {
	e.b.Reset()
	e.locals, e.localCount, e.bindings, e.temps = nil, 0, map[*binding]string{}, map[string]string{}
	e.resultLocal, e.wordInferred, e.cleanups, e.streams = "", inferred, nil, nil
	e.flowCursor, e.flowCursorType = "", unitType
	if resultType != "" {
		e.resultLocal = "$r"
		e.locals = append(e.locals, "$r "+nullableRef(resultType))
	}
}

func (e *watEmitter) funcText(name string, params []string, result string) string {
	head := "(func " + name
	for _, p := range params {
		head += " " + p
	}
	if result != "" {
		head += " (result " + result + ")"
	}
	var h strings.Builder
	h.WriteString(head + "\n")
	for _, l := range e.locals {
		h.WriteString("  (local " + l + ")\n")
	}
	h.WriteString(e.b.String())
	h.WriteString(")\n")
	return h.String()
}

// wordResult is the word's wasm result type; "" means no result.
func (e *watEmitter) wordResult(w *word) string {
	if w.inferred {
		return "(ref " + watTypeResult + ")"
	}
	if w.result == unitType {
		return ""
	}
	return e.watValueType(w.result)
}

func (e *watEmitter) wordFunction(w *word) string {
	e.pos = w.pos
	result := e.wordResult(w)
	e.resetFunc(result, w.inferred)
	var params []string
	for i, p := range w.params {
		name := fmt.Sprintf("$p%d_%s", i, watMangle(p.name))
		if p.typ == unitType {
			params = append(params, fmt.Sprintf("(param %s i32)", name))
			continue
		}
		e.bindings[p.binding] = name
		params = append(params, fmt.Sprintf("(param %s %s)", name, e.watLocalType(p.typ)))
	}
	e.line("call " + watEnter)
	e.line("block $zhex_ret")
	e.depth++
	e.emitTail(w.body, w)
	e.depth--
	e.line("end")
	e.line("call " + watLeave)
	if result != "" {
		e.line("local.get $r")
		if w.inferred {
			e.line("ref.as_non_null")
		} else {
			e.assertNonNull(w.result)
		}
	}
	return e.funcText(watWordName(w), params, result)
}

// emitTail emits the body block and stores its tail value into $r, wrapping
// a plain success value once for an inferred Result word (eval.call's
// `result.result == nil` wrap, made static: a tail whose type is already a
// Result — ok/err constructor or propagating try — passes through unwrapped).
func (e *watEmitter) emitTail(body []*statement, w *word) {
	tail := e.blockValueType(body)
	e.blockValue(body)
	if e.resultLocal == "" || tail == neverType {
		return
	}
	if e.wordInferred && !isResult(tail) {
		e.wrapSuccess(tail)
	}
	e.line("local.set $r")
}

func (e *watEmitter) mainFunction() string {
	e.pos = position{}
	e.resetFunc("", false)
	e.line("call " + watEnvInit) // seed the module-side env first (DESIGN-M3 §7)
	e.line("block $zhex_ret")
	e.depth++
	for _, s := range e.p.body {
		e.statementNeutral(s)
	}
	// The reserved entry word (#134): an empty entry body calls main, and a
	// Result return unwraps like a top-level try (fail below; resultLocal "").
	if w := entryMainWord(e.p); w != nil {
		e.pos = w.pos
		e.line("call " + watWordName(w))
		if inner, fallible := resultInner(w.result); fallible {
			res := e.temp("res", "(ref null "+watTypeResult+")")
			e.line("local.set " + res)
			e.emitResultUnwrap(res, inner)
			if !isUnitValue(inner) {
				e.line("drop")
			}
		} else if w.result != unitType {
			e.line("drop")
		}
	}
	e.depth--
	e.line("end")
	return e.funcText("$zhex_main", nil, "")
}

// ------------------------------------------------------------------ locals

// local returns the binding's wasm local; Unit bindings have none ("").
func (e *watEmitter) local(b *binding) string {
	if name, ok := e.bindings[b]; ok {
		return name
	}
	name := ""
	if b.typ != unitType {
		e.localCount++
		name = fmt.Sprintf("$l%d_%s", e.localCount, watMangle(b.name))
		e.locals = append(e.locals, name+" "+e.watLocalType(b.typ))
	}
	e.bindings[b] = name
	return name
}

// temp returns a function-local temporary keyed by purpose; the same key
// always reuses one local, so keys must be unique per purpose.
func (e *watEmitter) temp(key, typ string) string {
	if name, ok := e.temps[key]; ok {
		return name
	}
	name := "$t_" + key
	e.temps[key] = name
	e.locals = append(e.locals, name+" "+typ)
	return name
}

// ------------------------------------------------------------ value types

// watValueType is the WAT type of a Zhex type in params, locals, and block
// results (DESIGN.md §2). Unit has no WAT type; callers treat it specially.
func (e *watEmitter) watValueType(t valueType) string {
	switch t {
	case intType:
		return "i64"
	case boolType:
		return "i32"
	case stringType:
		return "(ref " + watTypeString + ")"
	case durationType:
		return "i64" // unboxed nanoseconds, Int's repr (DESIGN-M3 §5)
	case bufferType:
		return "(ref " + watTypeBuffer + ")"
	case fileType:
		return "(ref " + watTypeFile + ")"
	case errorType:
		// Errors are the caught Result payload, an opaque eq reference in
		// this build (DESIGN-M2 §2d); nothing compiles can read one yet.
		return "(ref null eq)"
	}
	if isResult(t) {
		return "(ref " + watTypeResult + ")"
	}
	if kind, inner, ok := collectionInner(t); ok {
		if kind == "List" {
			return "(ref " + watListType(watSpecOf(inner)) + ")"
		}
		return "(ref " + watArrayType(watSpecOf(inner)) + ")"
	}
	if base, args, applied := typeApplication(t); applied {
		if base == "@function" {
			return "(ref null " + watTypeClosure + ")" // uniform closure repr (§4a)
		}
		if base == "Option" {
			return "(ref null eq)" // Some_X and None share no named supertype
		}
		if base == "Map" && len(args) == 2 {
			return "(ref " + watMapType(mapKeySpec(args[0]), watSpecOf(args[1])) + ")"
		}
		watFence(e.pos, "generic type "+base)
	}
	if r := e.byID[string(t)]; r != nil {
		if r.choice {
			return "(ref null eq)" // struct-per-variant, so the value is the supertype
		}
		return "(ref " + watRecordName(r) + ")"
	}
	watFence(e.pos, string(t))
	return ""
}

// watSpecOf is the element specialization of a collection or Option payload:
// i64 for Int and Duration (unboxed i64 ns), i32 for Bool, ref for everything
// else including Unit.
func watSpecOf(t valueType) watSpec {
	switch t {
	case intType, durationType:
		return watSpecI64
	case boolType:
		return watSpecI32
	}
	return watSpecRef
}

// nullableRef widens a concrete ref type to its nullable storage form; i64,
// i32, and already-nullable types pass through. Every local, param, record
// field, and temp uses the nullable form: V8 rejects non-defaultable locals
// read after a block end even when set on every path, so no local is ever
// non-defaultable. Values are never actually null — the checker guarantees
// a binding is defined before every read.
func nullableRef(typ string) string {
	if strings.HasPrefix(typ, "(ref null ") {
		return typ
	}
	return strings.Replace(typ, "(ref ", "(ref null ", 1)
}

// watLocalType is a value type's storage form (locals, params, fields).
func (e *watEmitter) watLocalType(t valueType) string {
	return nullableRef(e.watValueType(t))
}

// assertNonNull follows a local.get of concrete ref type t with
// ref.as_non_null, restoring the expression invariant that every pushed
// ref is non-null; eqref values pass through (their consumers are nullable),
// and closures assert too — the callback helpers take them non-null.
func (e *watEmitter) assertNonNull(t valueType) {
	if _, _, isFn := functionTypeParts(t); isFn || nullableRef(e.watValueType(t)) != e.watValueType(t) {
		e.line("ref.as_non_null")
	}
}

// refTarget is the non-null ref type to cast to when a value of type t
// arrives as a plain eq reference (collection read, Result payload); ""
// when the value already is one (choices, Error).
func (e *watEmitter) refTarget(t valueType) string {
	switch t {
	case stringType:
		return "(ref " + watTypeString + ")"
	case bufferType:
		return "(ref " + watTypeBuffer + ")"
	case fileType:
		return "(ref " + watTypeFile + ")"
	}
	if isResult(t) {
		return "(ref " + watTypeResult + ")"
	}
	if kind, inner, ok := collectionInner(t); ok {
		if kind == "List" {
			return "(ref " + watListType(watSpecOf(inner)) + ")"
		}
		return "(ref " + watArrayType(watSpecOf(inner)) + ")"
	}
	if base, args, applied := typeApplication(t); applied {
		if base == "@function" {
			return "(ref " + watTypeClosure + ")"
		}
		if base == "Option" {
			return ""
		}
		if base == "Map" && len(args) == 2 {
			return "(ref " + watMapType(mapKeySpec(args[0]), watSpecOf(args[1])) + ")"
		}
	}
	if r := e.byID[string(t)]; r != nil && !r.choice {
		return "(ref " + watRecordName(r) + ")"
	}
	return ""
}

func (e *watEmitter) castFromEqref(t valueType) {
	if target := e.refTarget(t); target != "" {
		e.line("ref.cast " + target)
	}
}

// wrapSuccess turns a success value on the stack into an ok Result struct
// (§2 as amended: Int payloads box as $zhex_some_i64 because ref.i31 holds
// 31 bits; Bool as i31; Unit is null).
func (e *watEmitter) wrapSuccess(t valueType) {
	if t == unitType {
		e.line("i32.const 0")
		e.line("ref.null eq")
		e.line("struct.new " + watTypeResult)
		return
	}
	tmp := e.temp("wrap_"+string(t), e.watLocalType(t))
	e.line("local.set " + tmp)
	e.line("i32.const 0")
	e.line("local.get " + tmp)
	switch t {
	case intType, durationType:
		e.line("struct.new " + watSomeType(watSpecI64))
	case boolType:
		e.line("ref.i31")
	}
	e.line("struct.new " + watTypeResult)
}

// unboxPayload turns a Result payload eq reference on the stack into a value
// of type t (the inverse of wrapSuccess; ref payloads cast back to their
// concrete struct type, choice payloads stay eq references).
func (e *watEmitter) unboxPayload(t valueType) {
	switch t {
	case intType, durationType:
		e.line("ref.cast (ref " + watSomeType(watSpecI64) + ")")
		e.line("struct.get " + watSomeType(watSpecI64) + " $value")
	case boolType:
		e.line("call " + watUnboxI32)
	default:
		e.castFromEqref(t)
	}
}

// --------------------------------------------------------------- statements

// statementValueType is a statement's value type, mirroring the checker:
// expression → its type, if → the matching branch type (neverType when a
// branch only returns), flow → the last step's effective type, everything
// else Unit.
func (e *watEmitter) statementValueType(s *statement) valueType {
	switch s.kind {
	case "expression":
		return s.expr.typ
	case "if":
		left := e.blockValueType(s.body)
		right := unitType
		if s.otherwise != nil {
			right = e.blockValueType(s.otherwise)
		}
		if left == neverType {
			return right
		}
		if right == neverType {
			return left
		}
		return left
	case "flow":
		last := s.steps[len(s.steps)-1]
		if inner, fallible := resultInner(last.typ); fallible && last.propagate {
			return inner
		}
		return last.typ
	case "return":
		return neverType
	}
	return unitType
}

func (e *watEmitter) blockValueType(body []*statement) valueType {
	if len(body) == 0 {
		return unitType
	}
	return e.statementValueType(body[len(body)-1])
}

// blockValue emits the statements and leaves the last one's value on the
// stack (nothing for Unit), mirroring eval.block.
func (e *watEmitter) blockValue(body []*statement) {
	for i, s := range body {
		if i == len(body)-1 {
			e.statementValue(s)
			return
		}
		e.statementNeutral(s)
	}
}

func (e *watEmitter) statementNeutral(s *statement) {
	typ := e.statementValueType(s)
	e.statementValue(s)
	if typ != unitType && typ != neverType {
		e.line("drop")
	}
}

func (e *watEmitter) statementValue(s *statement) {
	e.pos = s.pos
	switch s.kind {
	case "expression":
		e.expr(s.expr)
	case "return":
		e.returnStatement(s)
	case "let", "var", "assign":
		e.expr(s.expr)
		if name := e.local(s.binding); name != "" {
			e.line("local.set " + name)
		}
	case "if":
		e.ifStatement(s)
	case "for":
		e.forRange(s)
	case "for-collection":
		e.forCollection(s)
	case "flow":
		e.flowStatement(s)
	case "for-lines":
		e.forLines(s)
	case "while":
		e.whileStatement(s)
	default:
		watFence(s.pos, s.kind)
	}
}

// returnStatement stores the value (wrapping it for an inferred word, as
// eval.call does for returnValue) and exits through the single epilogue,
// first decrementing every active traversal's counter — eval's defers run
// on the return path too.
func (e *watEmitter) returnStatement(s *statement) {
	// Open file.stream handles close before the traversal counters release —
	// eval's inner statement defer runs before the outer traversal's (LIFO).
	for i := len(e.streams) - 1; i >= 0; i-- {
		e.line("local.get " + e.streams[i])
		e.line("call " + watStreamClose)
	}
	for _, c := range e.cleanups {
		e.bumpActive(c.local, c.structType, -1)
	}
	typ := s.expr.typ
	e.expr(s.expr)
	if e.resultLocal != "" {
		if e.wordInferred && !isResult(typ) {
			e.wrapSuccess(typ)
		}
		e.line("local.set $r")
	} else if typ != unitType && typ != neverType {
		e.line("drop")
	}
	e.line("br $zhex_ret")
}

func (e *watEmitter) ifStatement(s *statement) {
	typ := e.statementValueType(s)
	if typ == neverType {
		typ = unitType // both branches only return; no value crosses
	}
	e.expr(s.expr) // Bool condition
	if typ != unitType {
		e.line("if (result %s)", e.watValueType(typ))
	} else {
		e.line("if")
	}
	e.depth++
	e.blockValue(s.body)
	e.depth--
	if s.otherwise != nil {
		e.line("else")
		e.depth++
		e.blockValue(s.otherwise)
		e.depth--
	}
	e.line("end")
}

// nextID mints unique label/temp suffixes.
func (e *watEmitter) nextID() string {
	e.idCount++
	return fmt.Sprintf("%d", e.idCount)
}

// nextIDInt is the numeric mint behind nextID, for the $zjson_schema<N>
// builder and thunk names (watnames_m3).
func (e *watEmitter) nextIDInt() int {
	e.idCount++
	return e.idCount
}

// forRange emits `for n in range(start, end)`: a plain i64 counter loop,
// empty when start >= end (eval.statement "for").
func (e *watEmitter) forRange(s *statement) {
	k := e.nextID()
	n := e.local(s.binding)
	end := e.temp("end"+k, "i64")
	e.expr(s.expr)
	e.line("local.set " + n)
	e.expr(s.end)
	e.line("local.set " + end)
	e.line("block $exit" + k)
	e.depth++
	e.line("loop $loop" + k)
	e.depth++
	e.line("local.get " + n)
	e.line("local.get " + end)
	e.line("i64.lt_s")
	e.line("if")
	e.depth++
	for _, st := range s.body {
		e.statementNeutral(st)
	}
	e.line("local.get " + n)
	e.line("i64.const 1")
	e.line("i64.add")
	e.line("local.set " + n)
	e.line("br $loop" + k)
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
}

// whileStatement emits `while cond` — a Unit statement with no break or
// continue. The condition is evaluated before the first pass and again after
// every pass, and the loop consumes no depth budget: the interpreter's while
// touches neither the counter nor any limit (DESIGN-M2 §5). A return inside
// the body branches to $zhex_ret, crossing the loop labels legally.
func (e *watEmitter) whileStatement(s *statement) {
	k := e.nextID()
	c := e.temp("while"+k, "i32")
	e.expr(s.expr)
	e.line("local.set " + c)
	e.line("block $exit" + k)
	e.depth++
	e.line("loop $loop" + k)
	e.depth++
	e.line("local.get " + c)
	e.line("i32.eqz")
	e.line("br_if $exit" + k)
	for _, st := range s.body {
		e.statementNeutral(st)
	}
	e.expr(s.expr)
	e.line("local.set " + c)
	e.line("br $loop" + k)
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
}

func (e *watEmitter) bumpActive(local, structType string, delta int) {
	e.line("local.get " + local)
	e.line("local.get " + local)
	e.line("struct.get " + structType + " $active")
	e.line("i32.const 1")
	if delta > 0 {
		e.line("i32.add")
	} else {
		e.line("i32.sub")
	}
	e.line("struct.set " + structType + " $active")
}

// forCollection emits `for v in list|array` (dual bindings included): it
// snapshots the items array, increments the wrapper's $active counter, loops,
// then decrements — mirroring eval's defer, with a counter so nested
// traversals of one collection work. Writes and appends during traversal
// fail 10 inside the prelude helpers.
func (e *watEmitter) forCollection(s *statement) {
	if keyType, valType, isMap := mapTypes(s.expr.typ); isMap {
		e.forMap(s, keyType, valType)
		return
	}
	kind, inner, ok := collectionInner(s.expr.typ)
	if !ok {
		watFence(s.pos, string(s.expr.typ))
	}
	k := e.nextID()
	spec := watSpecOf(inner)
	unit := isUnitValue(inner)
	e.expr(s.expr)
	recvType := watListType(spec)
	if kind == "Array" {
		recvType = watArrayType(spec)
	}
	recv := e.temp("recv"+k, nullableRef("(ref "+recvType+")"))
	items := e.temp("items"+k, "(ref null "+watArrType(spec)+")")
	i := e.temp("i"+k, "i32")
	e.line("local.set " + recv)
	e.line("local.get " + recv)
	e.line("struct.get " + recvType + " $items")
	e.line("local.set " + items)
	e.bumpActive(recv, recvType, 1)
	e.cleanups = append(e.cleanups, watCleanup{recv, recvType})
	e.line("i32.const 0")
	e.line("local.set " + i)
	e.line("block $exit" + k)
	e.depth++
	e.line("loop $loop" + k)
	e.depth++
	e.line("local.get " + i)
	e.line("local.get " + items)
	e.line("array.len")
	e.line("i32.lt_u")
	e.line("if")
	e.depth++
	if kind == "List" {
		// List dual binding: the first value is the zero-based index.
		e.readElem(items, i, inner)
		if s.secondBinding != nil {
			e.line("local.get " + i)
			e.line("i64.extend_i32_u")
			e.line("local.set " + e.local(s.binding))
		}
	} else if s.secondBinding != nil {
		// Array dual binding: the first value is the coordinates list,
		// rebuilt from the flat index exactly as eval does (axis
		// remainders, last axis first).
		e.emitCoords(recv, recvType, i, k, e.local(s.binding))
		e.readElem(items, i, inner)
	} else {
		e.readElem(items, i, inner)
	}
	if !unit {
		if s.secondBinding != nil {
			e.line("local.set " + e.local(s.secondBinding))
		} else {
			e.line("local.set " + e.local(s.binding))
		}
	}
	for _, st := range s.body {
		e.statementNeutral(st)
	}
	e.line("local.get " + i)
	e.line("i32.const 1")
	e.line("i32.add")
	e.line("local.set " + i)
	e.line("br $loop" + k)
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
	e.cleanups = e.cleanups[:len(e.cleanups)-1]
	e.bumpActive(recv, recvType, -1)
}

// forMap emits `for v in m` (values) and `for k, v in m` (key first, value
// second — eval's asymmetric binding order): snapshot the keys/vals arrays
// into locals, raise the guard counter for the traversal, loop, restore
// (DESIGN-M2 §3d). Mutating helpers fail 10 through the guard.
func (e *watEmitter) forMap(s *statement, keyType, valType valueType) {
	k := e.nextID()
	kk, v := mapKeySpec(keyType), watSpecOf(valType)
	mapT := watMapType(kk, v)
	recv := e.temp("recv"+k, nullableRef("(ref "+mapT+")"))
	keys := e.temp("items"+k, "(ref null "+mapKeyArrType(kk)+")")
	vals := e.temp("vals"+k, "(ref null "+watArrType(v)+")")
	i := e.temp("i"+k, "i32")
	e.expr(s.expr)
	e.line("local.set " + recv)
	e.line("local.get " + recv)
	e.line("struct.get " + mapT + " $keys")
	e.line("local.set " + keys)
	e.line("local.get " + recv)
	e.line("struct.get " + mapT + " $vals")
	e.line("local.set " + vals)
	e.bumpActive(recv, mapT, 1)
	e.cleanups = append(e.cleanups, watCleanup{recv, mapT})
	e.line("i32.const 0")
	e.line("local.set " + i)
	e.line("block $exit" + k)
	e.depth++
	e.line("loop $loop" + k)
	e.depth++
	e.line("local.get " + i)
	e.line("local.get " + keys)
	e.line("array.len")
	e.line("i32.lt_u")
	e.line("if")
	e.depth++
	if s.secondBinding != nil {
		// dual: key first (cast out of the keys array), value second
		e.line("local.get " + keys)
		e.line("local.get " + i)
		e.line("array.get " + mapKeyArrType(kk))
		e.castFromEqref(keyType)
		e.line("local.set " + e.local(s.binding))
		if !isUnitValue(valType) {
			e.line("local.get " + vals)
			e.line("local.get " + i)
			e.line("array.get " + watArrType(v))
			e.castFromEqref(valType)
			e.line("local.set " + e.local(s.secondBinding))
		}
	} else if !isUnitValue(valType) {
		// single: values only
		e.line("local.get " + vals)
		e.line("local.get " + i)
		e.line("array.get " + watArrType(v))
		e.castFromEqref(valType)
		e.line("local.set " + e.local(s.binding))
	}
	for _, st := range s.body {
		e.statementNeutral(st)
	}
	e.line("local.get " + i)
	e.line("i32.const 1")
	e.line("i32.add")
	e.line("local.set " + i)
	e.line("br $loop" + k)
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
	e.cleanups = e.cleanups[:len(e.cleanups)-1]
	e.bumpActive(recv, mapT, -1)
}

// forLines emits `for line in file.stream(path)`: open once, loop
// $zhex_stream_line until its ok-null payload (clean EOF), close on every
// exit — the loop's own and an early word return (the streams list, which
// returnStatement drains). Open, read, and UTF-8 failures are hard staged
// code-17 errors inside the helpers (DESIGN-M3 §3).
func (e *watEmitter) forLines(s *statement) {
	k := e.nextID()
	handle := e.temp("fh"+k, "i32")
	line := e.temp("fl"+k, "(ref null "+watTypeString+")")
	e.expr(s.expr)
	e.line("call " + watStreamOpen)
	e.line("local.set " + handle)
	e.streams = append(e.streams, handle)
	e.line("block $exit" + k)
	e.depth++
	e.line("loop $loop" + k)
	e.depth++
	e.line("local.get " + handle)
	e.line("call " + watStreamLine)
	e.line("struct.get " + watTypeResult + " $payload") // ok-null = EOF, ok-string = the chomped line
	e.line("ref.cast (ref null " + watTypeString + ")")
	e.line("local.set " + line)
	e.line("local.get " + line)
	e.line("ref.is_null")
	e.line("br_if $exit" + k)
	e.line("local.get " + line)
	e.line("ref.as_non_null")
	e.line("local.set " + e.local(s.binding))
	for _, st := range s.body {
		e.statementNeutral(st)
	}
	e.line("br $loop" + k)
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
	e.line("local.get " + handle)
	e.line("call " + watStreamClose)
	e.streams = e.streams[:len(e.streams)-1]
}

// readElem pushes items[i]; Unit elements push nothing.
func (e *watEmitter) readElem(items, i string, inner valueType) {
	if isUnitValue(inner) {
		return
	}
	e.line("local.get " + items)
	e.line("local.get " + i)
	e.line("array.get " + watArrType(watSpecOf(inner)))
	e.castFromEqref(inner)
}

// emitCoords builds the dual-binding coordinates List<Int> for the flat
// index in local i and stores it into bindingLocal.
func (e *watEmitter) emitCoords(recv, arrayType, i, k, bindingLocal string) {
	coords := e.temp("coords"+k, nullableRef("(ref "+watTypeArrI64+")"))
	n := e.temp("n"+k, "i64")
	axis := e.temp("axis"+k, "i32")
	e.line("local.get " + recv)
	e.line("struct.get " + arrayType + " $shape")
	e.line("array.len")
	e.line("array.new_default " + watTypeArrI64)
	e.line("local.set " + coords)
	e.line("local.get " + i)
	e.line("i64.extend_i32_u")
	e.line("local.set " + n)
	e.line("local.get " + recv)
	e.line("struct.get " + arrayType + " $shape")
	e.line("array.len")
	e.line("i32.const 1")
	e.line("i32.sub")
	e.line("local.set " + axis)
	e.line("block $axexit" + k)
	e.depth++
	e.line("loop $axloop" + k)
	e.depth++
	e.line("local.get " + axis)
	e.line("i32.const 0")
	e.line("i32.lt_s")
	e.line("if")
	e.depth++
	e.line("br $axexit" + k)
	e.depth--
	e.line("end")
	shapeAxis := func() {
		e.line("local.get " + recv)
		e.line("struct.get " + arrayType + " $shape")
		e.line("local.get " + axis)
		e.line("array.get " + watTypeArrI64)
	}
	e.line("local.get " + coords)
	e.line("local.get " + axis)
	e.line("local.get " + n)
	shapeAxis()
	e.line("i64.rem_s")
	e.line("array.set " + watTypeArrI64)
	e.line("local.get " + n)
	shapeAxis()
	e.line("i64.div_s")
	e.line("local.set " + n)
	e.line("local.get " + axis)
	e.line("i32.const 1")
	e.line("i32.sub")
	e.line("local.set " + axis)
	e.line("br $axloop" + k)
	e.depth--
	e.line("end")
	e.depth--
	e.line("end")
	e.line("local.get " + coords)
	e.line("i32.const 0")
	e.line("struct.new " + watListType(watSpecI64))
	e.line("local.set " + bindingLocal)
}

// -------------------------------------------------------------- expressions

func (e *watEmitter) expr(x *expression) {
	e.pos = x.pos
	switch x.kind {
	case "int":
		if !x.number.Fits64() {
			// The WAT lane carries Ints as i64; the interpreter's 128-bit
			// width fences here rather than truncating the literal.
			watFence(x.pos, "integers beyond the signed 64-bit range")
		}
		e.line("i64.const %d", x.number.AsInt64())
	case "bool":
		if x.text == "true" {
			e.line("i32.const 1")
		} else {
			e.line("i32.const 0")
		}
	case "string":
		e.stringConst(x.text)
	case "name":
		e.nameExpr(x)
	case "list":
		e.listLiteral(x)
	case "binary":
		e.binaryExpr(x)
	case "unary":
		e.unaryExpr(x)
	case "call":
		e.call(x)
	case "index":
		e.indexExpr(x)
	case "field":
		e.fieldExpr(x)
	case "index-write":
		e.indexWrite(x)
	case "match":
		e.matchExpr(x)
	case "try":
		e.tryExpr(x)
	case "catch":
		e.catchExpr(x)
	case "statement-value":
		e.statementValue(x.statement)
	case "flow-value":
		if e.flowCursor != "" {
			e.line("local.get " + e.flowCursor)
			e.assertNonNull(e.flowCursorType)
		}
	case "map":
		e.mapLiteral(x)
	default:
		watFence(x.pos, x.kind)
	}
}

func (e *watEmitter) stringConst(text string) {
	runes := []rune(text)
	for _, r := range runes {
		e.line("i32.const %d", r)
	}
	e.line("array.new_fixed %s %d", watTypeString, len(runes))
}

// nameExpr reads a binding (with its field path) or builds a payload-less
// choice value (struct-per-variant; Option.None is the shared empty struct).
func (e *watEmitter) nameExpr(x *expression) {
	if x.record != nil && x.record.choice {
		if x.record == builtinOption {
			if x.variant == 0 {
				e.line("struct.new " + watTypeNone)
				return
			}
			watFence(x.pos, x.text) // Some requires a payload
		}
		e.variantPadding(x.variant)
		e.line("struct.new " + watVariantName(x.record, x.variant))
		return
	}
	name := e.local(x.binding)
	if name == "" {
		return // Unit binding
	}
	e.line("local.get " + name)
	e.assertNonNull(x.binding.typ)
	t := x.binding.typ
	for at, idx := range x.fields {
		r := e.byID[string(t)]
		if r == nil || r.choice {
			watFence(x.pos, string(t))
		}
		fieldType := r.fields[idx].typ
		if at == len(x.fields)-1 {
			if fieldType == unitType {
				return // a Unit field carries no value
			}
			e.line("struct.get %s $f%d", watRecordName(r), idx)
			e.assertNonNull(fieldType)
			return
		}
		e.line("struct.get %s $f%d", watRecordName(r), idx)
		t = fieldType
	}
}

// variantPadding pushes variant i's leading i64 padding and discriminant.
func (e *watEmitter) variantPadding(i int) {
	for d := 0; d < i; d++ {
		e.line("i64.const 0")
	}
	e.line("i32.const %d", i)
}

func (e *watEmitter) listLiteral(x *expression) {
	_, inner, _ := collectionInner(x.typ)
	unit := isUnitValue(inner)
	for _, a := range x.args {
		e.expr(a)
		if unit {
			e.line("ref.null eq")
		}
	}
	e.line("array.new_fixed %s %d", watArrType(watSpecOf(inner)), len(x.args))
	e.line("i32.const 0")
	e.line("struct.new " + watListType(watSpecOf(inner)))
}

// mapKeySpec is a map key's specialization: Int → i64, Bool → i32,
// String → str (DESIGN-M2 §3a); the checker allows only these key types.
func mapKeySpec(t valueType) watMapKey {
	switch t {
	case intType:
		return watMapKeyI64
	case boolType:
		return watMapKeyI32
	}
	return watMapKeyStr
}

// mapKeyArrType is the keys-array type of one key specialization.
func mapKeyArrType(k watMapKey) string {
	if k == watMapKeyStr {
		return watTypeArrStr
	}
	return watArrType(watSpec(k))
}

// mapSpecs is the repr pair of a map.* call's receiver (params[0]).
func (e *watEmitter) mapSpecs(x *expression) (watMapKey, watSpec) {
	keyType, valType, _ := mapTypes(x.inputType)
	return mapKeySpec(keyType), watSpecOf(valType)
}

// mapLiteral builds an empty map of the contextually checked repr pair,
// then puts each pair left to right — key first, then value — so duplicate
// constant keys behave as put (overwrite in place, DESIGN-M2 §3c).
func (e *watEmitter) mapLiteral(x *expression) {
	keyType, valType, _ := mapTypes(x.typ)
	k, v := mapKeySpec(keyType), watSpecOf(valType)
	m := e.temp("map"+e.nextID(), nullableRef("(ref "+watMapType(k, v)+")"))
	e.line("call " + watMapNew(k, v))
	e.line("local.set " + m)
	for i := 0; i < len(x.args); i += 2 {
		e.line("local.get " + m)
		e.line("ref.as_non_null")
		e.expr(x.args[i])
		e.expr(x.args[i+1])
		if isUnitValue(valType) {
			e.line("ref.null eq")
		}
		e.line("call " + watMapPut(k, v))
		e.line("local.set " + m) // put passes the same map through
	}
	e.line("local.get " + m)
	e.line("ref.as_non_null")
}

func (e *watEmitter) binaryExpr(x *expression) {
	switch x.text {
	case "and", "or":
		// Short-circuit on a tee'd left operand: false and b → 0,
		// true or b → 1 (Bool values are always 0/1 here).
		t := e.temp("bool", "i32")
		e.expr(x.left)
		e.line("local.tee " + t)
		e.line("if (result i32)")
		e.depth++
		if x.text == "or" {
			e.line("i32.const 1") // true or b → true
		} else {
			e.expr(x.right) // true and b → b
		}
		e.depth--
		e.line("else")
		e.depth++
		if x.text == "or" {
			e.expr(x.right) // false or b → b
		} else {
			e.line("i32.const 0") // false and b → false
		}
		e.depth--
		e.line("end")
	case "==", "!=":
		e.expr(x.left)
		e.expr(x.right)
		switch x.left.typ {
		case intType:
			if x.text == "==" {
				e.line("i64.eq")
			} else {
				e.line("i64.ne")
			}
		case boolType:
			if x.text == "==" {
				e.line("i32.eq")
			} else {
				e.line("i32.ne")
			}
		case stringType:
			e.line("call " + watStrEq)
			if x.text == "!=" {
				e.line("i32.eqz")
			}
		}
	case "<", "<=", ">", ">=":
		op := map[string]string{"<": "lt_s", "<=": "le_s", ">": "gt_s", ">=": "ge_s"}[x.text]
		e.expr(x.left)
		e.expr(x.right)
		e.line("i64." + op)
	case "+":
		if x.typ == stringType {
			// The string-context Int exception renders an Int operand the
			// way int.to-string does (eval.binary).
			e.expr(x.left)
			if x.left.typ == intType {
				e.line("call " + watIntToString)
			}
			e.expr(x.right)
			if x.right.typ == intType {
				e.line("call " + watIntToString)
			}
			e.line("call " + watStrConcat)
			return
		}
		e.expr(x.left)
		e.expr(x.right)
		e.line("call " + watAdd)
	case "-":
		e.expr(x.left)
		e.expr(x.right)
		e.line("call " + watSub)
	case "*":
		e.expr(x.left)
		e.expr(x.right)
		e.line("call " + watMul)
	case "/":
		e.expr(x.left)
		e.expr(x.right)
		e.line("call " + watDiv)
	case "%":
		e.expr(x.left)
		e.expr(x.right)
		e.line("i64.rem_s")
	default:
		watFence(x.pos, x.text)
	}
}

func (e *watEmitter) unaryExpr(x *expression) {
	e.expr(x.left)
	switch x.text {
	case "not":
		e.line("i32.eqz")
	case "-":
		e.line("call " + watNeg)
	default:
		watFence(x.pos, x.text)
	}
}

func (e *watEmitter) call(x *expression) {
	e.pos = x.pos
	if x.checkedCollection {
		watFence(x.pos, "checked "+x.text)
	}
	if x.partial {
		e.partialCreation(x)
		return
	}
	if x.binding != nil {
		e.closureInvoke(x)
		return
	}
	if x.record != nil {
		e.constructorCall(x)
		return
	}
	if x.word != nil {
		e.wordCall(x)
		return
	}
	e.builtinCall(x)
}

// watReprType is the WAT type of one element repr in a closure signature:
// closures are repr-level (DESIGN-M2 §4a), so ref params/results are
// (ref null eq) and the thunk casts to concrete types where types are known.
func watReprType(s watSpec) string {
	switch s {
	case watSpecI64:
		return "i64"
	case watSpecI32:
		return "i32"
	}
	return "(ref null eq)"
}

// boxEnv boxes a captured value for the closure env array: Int as
// $zhex_some_i64, Bool as i31, Unit pushes null (there is no value to box),
// every other repr is the reference itself (DESIGN-M2 §4b, M1 payload boxing).
func (e *watEmitter) boxEnv(t valueType) {
	switch t {
	case intType, durationType:
		e.line("struct.new " + watSomeType(watSpecI64))
	case boolType:
		e.line("ref.i31")
	case unitType:
		e.line("ref.null eq")
	}
}

// envGet pushes env slot i of the closure in $clo and unboxes it to type t.
func (e *watEmitter) envGet(i int, t valueType) {
	e.line("local.get $clo")
	e.line("struct.get " + watTypeClosure + " $env")
	e.line("ref.as_non_null")
	e.line("i32.const %d", i)
	e.line("array.get " + watTypeArrRef)
	e.unboxPayload(t)
}

// partialCreation emits a closure value at a partial site: the supplied
// arguments evaluate left to right (holes skipped, standing as null slots)
// into an env array of target arity, paired with a site-local thunk
// (DESIGN-M2 §4b). Since M4 the target may be a closure binding
// (re-partial application): the callee evaluates first, the env array gains
// one slot carrying the target closure ref itself, and the thunk
// call_ref's the target through its own signature (DESIGN-M4 §3b). Unit
// results compile — the thunk and the target use the grid's void rows (§3a).
func (e *watEmitter) partialCreation(x *expression) {
	fnParams, fnResult, _ := functionTypeParts(x.typ)
	var paramTypes []valueType
	if x.binding != nil {
		paramTypes, _, _ = functionTypeParts(x.binding.typ)
	} else if x.word != nil {
		if len(x.args) < len(x.word.params) {
			watFence(x.pos, "word parameter defaults")
		}
		for _, p := range x.word.params {
			paramTypes = append(paramTypes, p.typ)
		}
	} else {
		if !watThunkableBuiltin(x.text) {
			watFence(x.pos, x.text)
		}
		for _, a := range x.args {
			paramTypes = append(paramTypes, a.typ)
		}
	}
	id := e.nextID()
	env := e.temp("env"+id, nullableRef("(ref "+watTypeArrRef+")"))
	tgt := ""
	if x.binding != nil {
		// the callee evaluates first; its ref rides the env array's last slot
		tgt = e.temp("envt"+id, nullableRef("(ref "+watTypeClosure+")"))
		clo := e.local(x.binding)
		e.line("local.get " + clo)
		e.line("ref.as_non_null")
		e.line("local.set " + tgt)
	}
	for _, a := range x.args {
		if a.kind == "hole" {
			e.line("ref.null eq")
			continue
		}
		e.expr(a)
		e.boxEnv(a.typ)
	}
	if x.binding != nil {
		e.line("local.get " + tgt)
		e.line("ref.as_non_null")
		e.line("array.new_fixed %s %d", watTypeArrRef, len(x.args)+1)
	} else {
		e.line("array.new_fixed %s %d", watTypeArrRef, len(x.args))
	}
	e.line("local.set " + env)
	thunk := fmt.Sprintf("$zcthunk%s", id)
	e.line("ref.func " + thunk)
	e.line("local.get " + env)
	e.line("ref.as_non_null")
	e.line("struct.new " + watTypeClosure)
	e.appendThunk(thunk, x, fnParams, fnResult, paramTypes)
}

// appendThunk builds the site-local thunk function: it takes the closure as
// its first parameter plus one parameter per hole, unboxes the captured env
// slots per the static target signature, and calls the target directly.
// Thunks are plain functions — the target word's own enter/leave keeps depth
// parity, and builtins never counted (DESIGN-M2 §4b).
func (e *watEmitter) appendThunk(name string, x *expression, fnParams []valueType, fnResult valueType, paramTypes []valueType) {
	savedB, savedDepth := e.b, e.depth
	savedLocals, savedCount := e.locals, e.localCount
	savedBindings, savedTemps := e.bindings, e.temps
	e.b.Reset()
	e.depth, e.locals, e.localCount = 1, nil, 0
	e.bindings, e.temps = map[*binding]string{}, map[string]string{}
	defer func() {
		e.b, e.depth = savedB, savedDepth
		e.locals, e.localCount = savedLocals, savedCount
		e.bindings, e.temps = savedBindings, savedTemps
	}()

	holeAt := map[int]int{} // arg index → thunk parameter index
	for i, p := range x.holes {
		holeAt[p] = i
	}
	params := []string{"(param $clo (ref " + watTypeClosure + "))"}
	for i, t := range fnParams {
		params = append(params, fmt.Sprintf("(param $a%d %s)", i, watReprType(watSpecOf(t))))
	}
	result := ""
	if !isUnitValue(fnResult) {
		result = watReprType(watSpecOf(fnResult))
	}
	tgt := ""
	if x.binding != nil {
		// the target closure rides env slot n; call_ref wants it as arg 0
		// ahead of the filled params (DESIGN-M4 §3b)
		tgt = e.temp("rp_tgt", nullableRef("(ref "+watTypeClosure+")"))
		e.line("local.get $clo")
		e.line("struct.get " + watTypeClosure + " $env")
		e.line("ref.as_non_null")
		e.line("i32.const %d", len(paramTypes))
		e.line("array.get " + watTypeArrRef)
		e.line("ref.cast (ref " + watTypeClosure + ")")
		e.line("local.set " + tgt)
		e.line("local.get " + tgt)
		e.line("ref.as_non_null")
	}
	for i, t := range paramTypes {
		if at, isHole := holeAt[i]; isHole {
			if t == unitType {
				if x.word != nil {
					e.line("i32.const 0") // the word's Unit placeholder
					continue
				}
				e.line("ref.null eq") // a builtin's Unit argument
				continue
			}
			e.line("local.get $a%d", at)
			e.castFromEqref(t)
			continue
		}
		if t == unitType {
			if x.word != nil {
				e.line("i32.const 0")
				continue
			}
			e.line("ref.null eq")
			continue
		}
		e.envGet(i, t)
	}
	if x.word != nil {
		e.line("call " + watWordName(x.word))
	} else if x.binding != nil {
		// re-partial: the filled args and the target (arg 0) are on the
		// stack; call_ref through the target's own signature (§3b)
		targetParams, targetResult, _ := functionTypeParts(x.binding.typ)
		specs := make([]watSpec, len(targetParams))
		for i, p := range targetParams {
			specs[i] = watSpecOf(p)
		}
		cloSpec := watSpecOf(targetResult)
		if isUnitValue(targetResult) {
			cloSpec = watCloVoid
		}
		cloName := watCloType(specs, cloSpec)
		e.line("local.get " + tgt)
		e.line("ref.as_non_null")
		e.line("struct.get " + watTypeClosure + " $fn")
		e.line("ref.cast (ref " + cloName + ")")
		e.line("call_ref " + cloName)
		if !isUnitValue(targetResult) {
			e.castFromEqref(targetResult)
		}
	} else {
		e.builtinCore(x)
	}
	e.thunks = append(e.thunks, e.funcText(name, params, result))
	e.thunkNames = append(e.thunkNames, name)
}

// closureInvoke emits a fully-applied call of a bound closure: the closure
// itself is argument 0, the args follow, then the typed funcref and call_ref
// (DESIGN-M2 §4c; stack order — the funcref is call_ref's last operand).
// A Unit result casts to the grid's void row and leaves nothing behind
// (DESIGN-M4 §3a).
func (e *watEmitter) closureInvoke(x *expression) {
	fnParams, fnResult, _ := functionTypeParts(x.binding.typ)
	specs := make([]watSpec, len(fnParams))
	for i, p := range fnParams {
		specs[i] = watSpecOf(p)
	}
	cloSpec := watSpecOf(fnResult)
	if isUnitValue(fnResult) {
		cloSpec = watCloVoid
	}
	cloName := watCloType(specs, cloSpec)
	clo := e.local(x.binding)
	e.line("local.get " + clo)
	e.line("ref.as_non_null")
	for _, arg := range x.args {
		e.expr(arg)
		if isUnitValue(arg.typ) {
			e.line("ref.null eq") // Unit has no value; the eq param carries null
		}
	}
	e.line("local.get " + clo)
	e.line("struct.get " + watTypeClosure + " $fn")
	e.line("ref.cast (ref " + cloName + ")")
	e.line("call_ref " + cloName)
	if !isUnitValue(fnResult) {
		e.castFromEqref(fnResult)
	}
}

// watThunkableBuiltin reports whether a builtin partial can be thunked: the
// emitter-supported text family plus int.to-string and print (the §3a
// flagship Unit-closure witness `print _`; others fence by name).
func watThunkableBuiltin(text string) bool {
	switch text {
	case "text.reverse", "text.length", "text.slice", "text.index-of", "text.contains",
		"text.upper", "text.lower", "text.trim", "text.split", "text.join", "text.replace",
		"text.starts-with", "text.ends-with", "text.line-count", "text.word-count",
		"text.to-int", "int.to-string", "print":
		return true
	}
	return false
}

// builtinCore emits a builtin's call lines with every argument already on
// the stack (used by thunk bodies; args were env-unboxed or param-loaded).
func (e *watEmitter) builtinCore(x *expression) {
	switch x.text {
	case "text.reverse":
		e.line("call " + watTextReverse)
	case "text.length":
		e.line("array.len")
		e.line("i64.extend_i32_u")
	case "text.slice":
		e.line("call " + watTextSlice)
	case "text.index-of":
		e.line("call " + watTextIndexOf)
	case "text.contains":
		e.line("call " + watTextContains)
	case "text.upper":
		e.line("call " + watTextUpper)
	case "text.lower":
		e.line("call " + watTextLower)
	case "text.trim":
		e.line("call " + watTextTrim)
	case "text.split":
		e.line("call " + watTextSplit)
	case "text.join":
		e.line("call " + watTextJoin)
	case "text.replace":
		e.line("call " + watTextReplace)
	case "text.starts-with":
		e.line("call " + watTextStartsWith)
	case "text.ends-with":
		e.line("call " + watTextEndsWith)
	case "text.line-count":
		e.line("call " + watTextLineCount)
	case "text.word-count":
		e.line("call " + watTextWordCount)
	case "text.to-int":
		e.line("call " + watTextToInt)
	case "int.to-string":
		e.line("call " + watIntToString)
	case "print":
		e.printCore(x, true)
	default:
		watFence(x.pos, x.text)
	}
}

func (e *watEmitter) wordCall(x *expression) {
	w := x.word
	if len(x.args) < len(w.params) {
		watFence(x.pos, "word parameter defaults")
	}
	for i, p := range w.params {
		e.expr(x.args[i])
		if p.typ == unitType {
			e.line("i32.const 0") // Unit parameter placeholder (§2)
		}
	}
	e.line("call " + watWordName(w))
}

// constructorCall builds records (positional or all-named; named arguments
// evaluate in source order, stored in field order) and choice variants.
func (e *watEmitter) constructorCall(x *expression) {
	r := x.record
	if r.choice {
		if r == builtinOption {
			if x.variant == 0 {
				e.line("struct.new " + watTypeNone)
				return
			}
			payload := x.typeArg
			if payload == "" {
				for _, a := range x.args {
					if a.kind != "flow-value" {
						payload = a.typ
						break
					}
				}
			}
			e.expr(x.args[len(x.args)-1])
			if payload == unitType {
				e.line("ref.null eq")
			}
			e.line("struct.new " + watSomeType(watSpecOf(payload)))
			return
		}
		v := &r.variants[x.variant]
		e.variantPadding(x.variant)
		for i := range v.fields {
			e.expr(x.args[i])
			if v.fields[i].typ == unitType {
				e.line("ref.null eq")
			}
		}
		e.line("struct.new " + watVariantName(r, x.variant))
		return
	}
	if len(x.args) < len(r.fields) {
		watFence(x.pos, "record field defaults")
	}
	if len(x.argNames) == 0 {
		for i := range r.fields {
			e.expr(x.args[i])
			if r.fields[i].typ == unitType {
				e.line("ref.null eq")
			}
		}
		e.line("struct.new " + watRecordName(r))
		return
	}
	// All-named: evaluate in source order into per-field temporaries, then
	// build the struct in declaration order (eval evaluates args, then
	// scatters them through argFields).
	at := make([]int, len(r.fields))
	for j, field := range x.argFields {
		at[field] = j
	}
	tmps := make([]string, len(r.fields))
	for j, arg := range x.args {
		field := x.argFields[j]
		e.expr(arg)
		if r.fields[field].typ == unitType {
			continue
		}
		tmps[field] = e.temp(fmt.Sprintf("ctor%s_%d", e.nextID(), field), e.watLocalType(r.fields[field].typ))
		e.line("local.set " + tmps[field])
	}
	for i := range r.fields {
		if r.fields[i].typ == unitType {
			e.line("ref.null eq")
			continue
		}
		e.line("local.get " + tmps[i])
	}
	e.line("struct.new " + watRecordName(r))
}

func (e *watEmitter) builtinCall(x *expression) {
	switch x.text {
	case "print":
		e.printCall(x)
	case "int.to-string":
		e.expr(x.args[0])
		e.line("call " + watIntToString)
	case "duration.new":
		e.expr(x.args[0]) // passthrough: Duration is Int's unboxed i64 repr
	case "duration.parse":
		e.expr(x.args[0])
		e.line("call " + watDurationParse)
	case "duration.to-string":
		e.expr(x.args[0])
		e.line("call " + watDurationToString)
	case "read-line":
		e.expr(x.args[0])
		e.line("call " + watReadLine)
	case "error.message":
		e.expr(x.args[0]) // the caught payload is a plain eq reference
		e.line("call " + watErrorMessage)
	case "error.detail":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watErrorDetail)
	case "file.join":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watPathJoin)
	case "file.base", "file.extension", "file.directory-of":
		e.expr(x.args[0])
		e.line("call " + map[string]string{
			"file.base":         watPathBase,
			"file.extension":    watPathExtension,
			"file.directory-of": watPathDirOf,
		}[x.text])
	case "file.read":
		e.expr(x.args[0])
		e.line("call " + watFileRead)
	case "file.open":
		e.expr(x.args[0])
		e.line("call " + watFileOpen)
	case "file.read-line":
		e.expr(x.args[0])
		e.line("call " + watFileReadLine)
	case "file.close":
		e.expr(x.args[0])
		e.line("call " + watFileClose)
	case "file.write":
		e.expr(x.args[0])
		e.expr(x.args[1])
		if len(x.args) > 2 {
			e.expr(x.args[2]) // an explicit append flag; defaults fill omissions
		}
		e.pushBuiltinDefaults(x)
		e.line("call " + watFileWrite)
	case "file.list":
		e.expr(x.args[0])
		e.line("call " + watFileList)
	case "file.exists":
		// the plain Bool form; the checked try/catch form fences with the
		// other checked collection operations (DESIGN-M3 §8)
		e.expr(x.args[0])
		e.line("call " + watFileExists)
	case "file.size":
		e.expr(x.args[0])
		e.line("call " + watFileSize)
	case "file.make-folder":
		e.expr(x.args[0])
		if len(x.args) > 1 {
			e.expr(x.args[1]) // an explicit parents flag; defaults fill omissions
		}
		e.pushBuiltinDefaults(x)
		e.line("call " + watFileMakeFolder)
	case "file.delete":
		e.expr(x.args[0])
		e.line("call " + watFileDelete)
	case "file.delete-folder":
		e.expr(x.args[0])
		if len(x.args) > 1 {
			e.expr(x.args[1]) // an explicit recursive flag; defaults fill omissions
		}
		e.pushBuiltinDefaults(x)
		e.line("call " + watFileDeleteFolder)
	case "file.move":
		e.expr(x.args[0])
		e.expr(x.args[1])
		if len(x.args) > 2 {
			e.expr(x.args[2]) // an explicit overwrite flag; defaults fill omissions
		}
		e.pushBuiltinDefaults(x)
		e.line("call " + watFileMove)
	case "buffer.new":
		e.line("call " + watBufferNew)
	case "buffer.add-line":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watBufferAddLine)
	case "buffer.line-count":
		e.expr(x.args[0])
		e.line("call " + watBufferLineCount)
	case "buffer.line":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watBufferLine)
	case "buffer.set-line":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.expr(x.args[2])
		e.line("call " + watBufferSetLine)
	case "buffer.to-text":
		e.expr(x.args[0])
		e.line("call " + watBufferToText)
	case "json.parse":
		e.expr(x.args[0])
		e.line("call " + watJsonParse)
	case "json.encode":
		e.expr(x.args[0])
		if x.jsonSchema == nil {
			e.line("ref.null " + watTypeJsonSchema) // Json tree path: no schema
		} else {
			e.boxEnv(x.args[0].typ) // uniform eqref value, M1 boxing
			e.emitSchemaCall(x.jsonSchema)
		}
		e.line("call " + watJsonEncode)
	case "json.decode":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.boxEnv(x.args[1].typ) // uniform eqref defaults, M1 boxing
		e.emitSchemaCall(x.jsonSchema)
		e.line("call " + watJsonDecode)
	case "xml.decode":
		// mirrors json.decode (DESIGN-M4 §2): boxed defaults, the M3 schema
		// builder (which also registers node names for `expected <Type>`),
		// then the pure prelude decode; works as a flow step the same way.
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.boxEnv(x.args[1].typ)
		e.emitSchemaCall(x.jsonSchema)
		e.line("call " + watXmlDecode)
	case "env.get":
		e.expr(x.args[0])
		e.line("call " + watEnvGet) // Option<String> as a plain eq reference
	case "env.load":
		if len(x.args) > 0 { // the only builtin whose default can be the sole argument
			e.expr(x.args[0])
		}
		e.pushBuiltinDefaults(x)
		e.line("call " + watEnvLoad)
	case "ok":
		e.expr(x.args[0])
		e.wrapSuccess(x.args[0].typ)
	case "err":
		e.expr(x.args[0])
		tmp := e.temp("errarg", e.watLocalType(errorType))
		e.line("local.set " + tmp)
		e.line("i32.const 1")
		e.line("local.get " + tmp)
		e.line("struct.new " + watTypeResult)
	case "list.new":
		_, inner, _ := collectionInner(x.typ)
		e.line("call " + watListNew(watSpecOf(inner)))
	case "list.append":
		_, inner, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.expr(x.args[1])
		if isUnitValue(inner) {
			e.line("ref.null eq")
		}
		e.line("call " + watListAppend(watSpecOf(inner)))
	case "list.length":
		_, inner, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.line("call " + watListLength(watSpecOf(inner)))
	case "array.new":
		_, inner, _ := collectionInner(x.typ)
		e.expr(x.args[0])
		e.expr(x.args[1])
		if isUnitValue(inner) {
			e.line("ref.null eq")
		}
		e.line("call " + watArrayNew(watSpecOf(inner)))
	case "array.length":
		_, inner, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.line("call " + watArrayLength(watSpecOf(inner)))
	case "text.reverse", "text.upper", "text.lower", "text.trim", "text.line-count", "text.word-count":
		e.expr(x.args[0])
		e.line("call " + map[string]string{
			"text.reverse":    watTextReverse,
			"text.upper":      watTextUpper,
			"text.lower":      watTextLower,
			"text.trim":       watTextTrim,
			"text.line-count": watTextLineCount,
			"text.word-count": watTextWordCount,
		}[x.text])
	case "text.length":
		// Inlined: the string is a code-point array (DESIGN-M2 §1).
		e.expr(x.args[0])
		e.line("array.len")
		e.line("i64.extend_i32_u")
	case "text.slice":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.expr(x.args[2])
		e.line("call " + watTextSlice)
	case "text.index-of":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watTextIndexOf)
	case "text.contains":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watTextContains)
	case "text.split":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watTextSplit)
	case "text.join":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watTextJoin)
	case "text.replace":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.expr(x.args[2])
		e.line("call " + watTextReplace)
	case "text.starts-with":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watTextStartsWith)
	case "text.ends-with":
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watTextEndsWith)
	case "text.to-int":
		e.expr(x.args[0])
		e.line("call " + watTextToInt)
	case "map.length":
		k, v := e.mapSpecs(x)
		e.expr(x.args[0])
		e.line("call " + watMapLength(k, v))
	case "map.contains":
		k, v := e.mapSpecs(x)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watMapContains(k, v))
	case "map.put":
		k, v := e.mapSpecs(x)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.expr(x.args[2])
		if isUnitValue(x.args[2].typ) {
			e.line("ref.null eq")
		}
		e.line("call " + watMapPut(k, v)) // returns the same map (flows)
	case "map.remove":
		k, v := e.mapSpecs(x)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watMapRemove(k, v)) // returns the same map (flows)
	case "map.into":
		k, v := e.mapSpecs(x)
		e.expr(x.args[0]) // source (guard targets only)
		e.expr(x.args[1])
		e.line("call " + watMapInto(k, v)) // returns the target
	case "list.map":
		if _, cbResult, _ := functionTypeParts(x.args[1].typ); isUnitValue(cbResult) {
			// no void-result row exists in the callback grid (DESIGN-M4 §3a)
			watFence(x.pos, "callbacks with Unit results")
		}
		_, in, _ := collectionInner(x.inputType)
		_, out, _ := collectionInner(x.typ)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watListMap(watSpecOf(in), watSpecOf(out)))
	case "list.filter":
		_, in, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watListFilter(watSpecOf(in)))
	case "list.select":
		_, in, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watListSelect(watSpecOf(in)))
	case "list.fold":
		if _, stepResult, _ := functionTypeParts(x.args[2].typ); isUnitValue(stepResult) {
			watFence(x.pos, "callbacks with Unit results") // no void row (§3a)
		}
		_, in, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.expr(x.args[2])
		e.line("call " + watListFold(watSpecOf(in), watSpecOf(x.args[1].typ)))
		e.castFromEqref(x.args[1].typ) // the fold helper returns (ref null eq)
	case "list.find":
		_, in, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watListFind(watSpecOf(in)))
	case "list.any":
		_, in, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watListAny(watSpecOf(in)))
	case "list.all":
		_, in, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watListAll(watSpecOf(in)))
	case "list.sort":
		_, in, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.line("call " + watListSort(watSpecOf(in)))
	case "list.sort-by":
		_, in, _ := collectionInner(x.inputType)
		_, key, _ := functionTypeParts(x.args[1].typ)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watListSortBy(watSpecOf(in), watSpecOf(key)))
	case "array.select":
		_, in, _ := collectionInner(x.inputType)
		e.expr(x.args[0])
		e.expr(x.args[1])
		e.line("call " + watArraySelect(watSpecOf(in)))
	default:
		watFence(x.pos, x.text)
	}
}

// pushBuiltinDefaults emits the constant trailing defaults a builtin call
// site omitted, mirroring eval's runtime append from builtinTrailingDefaults
// (the values are constants, so emission is static).
// shortcut: an empty-string default would render as Bool false; add a kind
// marker to the table if one ever ships.
func (e *watEmitter) pushBuiltinDefaults(x *expression) {
	d, ok := builtinTrailingDefaults[x.text]
	if !ok || len(x.args) >= d.arity {
		return
	}
	first := len(x.args) - (d.arity - len(d.values))
	for _, v := range d.values[first:] {
		if v.text != "" {
			e.stringConst(v.text)
			continue
		}
		if v.boolean {
			e.line("i32.const 1")
		} else {
			e.line("i32.const 0")
		}
	}
}

// emitSchemaCall pushes one decode-schema reference for a typed json call
// site — `call $zjson_schema<N>` — and queues the builder function plus the
// make/get thunks its record nodes need: one per call site, no caching
// (DESIGN-M3 §6; the schema's func fields are watnames_m3's flagged
// deviation).
func (e *watEmitter) emitSchemaCall(s *jsonSchema) {
	name := watJsonSchemaBuilder(e.nextIDInt())
	e.appendSchemaFuncs(name, s)
	e.line("call " + name)
}

// appendSchemaFuncs emits the $zjson_schema<N> builder (post-order node
// construction, one local per schema node) and, per record node, the
// $zjson_make<N>/get<N> thunks converting between the record struct and the
// boxed-fields eqref array. Everything lands in the thunks section.
func (e *watEmitter) appendSchemaFuncs(name string, root *jsonSchema) {
	savedB, savedDepth := e.b, e.depth
	savedLocals, savedCount := e.locals, e.localCount
	savedBindings, savedTemps := e.bindings, e.temps
	e.b.Reset()
	e.depth, e.locals, e.localCount = 1, nil, 0
	e.bindings, e.temps = map[*binding]string{}, map[string]string{}
	defer func() {
		e.b, e.depth = savedB, savedDepth
		e.locals, e.localCount = savedLocals, savedCount
		e.bindings, e.temps = savedBindings, savedTemps
	}()

	leafKind := map[string]int{"int": watJsonSchemaInt, "bool": watJsonSchemaBool, "string": watJsonSchemaString}
	innerKind := map[string]int{"list": watJsonSchemaList, "map": watJsonSchemaMap, "option": watJsonSchemaOption}
	var records []struct {
		thunk  string
		isMake bool
		r      *recordType
	}
	nodes := 0
	newLocal := func(typ string) string {
		local := fmt.Sprintf("$sn%d", nodes)
		nodes++
		e.locals = append(e.locals, local+" (ref null "+typ+")")
		return local
	}
	// Each schema node registers its Zhex type name beside the node ref in
	// the M4 names table (reset per build): the xml decode renders
	// `expected <Type>` failures with the interpreter's jsonSchema.typ
	// string — record names and aliases included — and nothing else reads
	// it (DESIGN-M4 §2b).
	e.locals = append(e.locals, "$zxml_tname (ref null "+watTypeString+")")
	e.line("(global.set " + watXmlNames + " (ref.null " + watTypeArrRef + "))")
	var build func(s *jsonSchema) string
	build = func(s *jsonSchema) string {
		local := newLocal(watTypeJsonSchema)
		switch s.kind {
		case "int", "bool", "string":
			e.line("i32.const %d", leafKind[s.kind])
			e.line("ref.null " + watTypeJsonSchema)
			e.line("ref.null " + watTypeArrRef)
			e.line("ref.null " + watTypeJsonMakeFn)
			e.line("ref.null " + watTypeJsonGetFn)
		case "list", "map", "option":
			child := build(s.element)
			e.line("i32.const %d", innerKind[s.kind])
			e.line("local.get " + child)
			e.line("ref.null " + watTypeArrRef)
			e.line("ref.null " + watTypeJsonMakeFn)
			e.line("ref.null " + watTypeJsonGetFn)
		case "record":
			r := e.byID[string(s.typ)]
			if r == nil {
				watFence(e.pos, string(s.typ))
			}
			children := make([]string, len(s.fields))
			for i := range s.fields {
				children[i] = build(s.fields[i].schema)
			}
			for i, f := range s.fields {
				e.stringConst(f.name)
				e.line("local.get " + children[i])
				e.line("struct.new " + watTypeJsonField)
			}
			e.line("array.new_fixed %s %d", watTypeArrRef, len(s.fields))
			arr := newLocal(watTypeArrRef)
			e.line("local.set " + arr)
			id := e.nextIDInt()
			makeName, getName := watJsonMakeThunk(id), watJsonGetThunk(id)
			records = append(records, struct {
				thunk  string
				isMake bool
				r      *recordType
			}{makeName, true, r})
			records = append(records, struct {
				thunk  string
				isMake bool
				r      *recordType
			}{getName, false, r})
			e.line("i32.const %d", watJsonSchemaRecord)
			e.line("ref.null " + watTypeJsonSchema)
			e.line("local.get " + arr)
			e.line("ref.func " + makeName)
			e.line("ref.func " + getName)
		default:
			watFence(e.pos, "json schema "+s.kind)
		}
		e.line("struct.new " + watTypeJsonSchema)
		e.line("local.set " + local)
		e.stringConst(string(s.typ))
		e.line("local.set $zxml_tname")
		e.line("(global.set " + watXmlNames + " (call $zjson_arr_push (global.get " + watXmlNames + ") (local.get " + local + ")))")
		e.line("(global.set " + watXmlNames + " (call $zjson_arr_push (global.get " + watXmlNames + ") (local.get $zxml_tname)))")
		return local
	}
	rootLocal := build(root)
	e.line("local.get " + rootLocal)
	e.line("ref.as_non_null")
	e.thunks = append(e.thunks, e.funcText(name, nil, "(ref "+watTypeJsonSchema+")"))
	for _, rec := range records {
		// The thunks take everything through their params — no locals of
		// their own, so the builder's $sn locals must not leak into them —
		// and both are declared with the shared functype index: wasm-tools
		// does not match a separately declared, structurally identical
		// signature against the schema field's (ref null $zjson_*_fn).
		e.b.Reset()
		e.locals = nil
		if rec.isMake {
			// make: read the boxed fields in declaration order, unbox, and
			// build the record struct.
			for i, f := range rec.r.fields {
				e.line("local.get $f")
				e.line("i32.const %d", i)
				e.line("array.get " + watTypeArrRef)
				e.unboxPayload(f.typ)
			}
			e.line("struct.new " + watRecordName(rec.r))
			e.thunks = append(e.thunks, e.funcText(rec.thunk,
				[]string{"(type " + watTypeJsonMakeFn + ")", "(param $f (ref " + watTypeArrRef + "))"}, "(ref null eq)"))
		} else {
			// get: read each field (re-casting the receiver per read), box,
			// and return the fields array.
			for i, f := range rec.r.fields {
				e.line("local.get $v")
				e.line("ref.cast (ref %s)", watRecordName(rec.r))
				e.line("struct.get %s $f%d", watRecordName(rec.r), i)
				e.boxEnv(f.typ)
			}
			e.line("array.new_fixed %s %d", watTypeArrRef, len(rec.r.fields))
			e.thunks = append(e.thunks, e.funcText(rec.thunk,
				[]string{"(type " + watTypeJsonGetFn + ")", "(param $v (ref null eq))"}, "(ref null "+watTypeArrRef+")"))
		}
		e.thunkNames = append(e.thunkNames, rec.thunk) // ref.func targets must be declared
	}
}

// printCall emits print for the printable types (Int, Bool, String, Unit,
// Duration). Since M3 the module owns the newline: every path composes into
// $zhex_print_line, and the host's print import sends raw bytes
// (DESIGN-M3 §1 Surprises 2).
func (e *watEmitter) printCall(x *expression) {
	e.expr(x.args[0])
	e.printCore(x, false)
}

// printCore emits print's lines with the argument already on the stack —
// shared by direct calls and the `print _` partial's thunk body (DESIGN-M4
// §3a). A thunk's Unit parameter carries a null placeholder the core drops;
// an expression of Unit pushes nothing, so only the thunk drops.
func (e *watEmitter) printCore(x *expression, stackHasArg bool) {
	switch x.inputType {
	case intType:
		e.line("call " + watIntToString)
		e.line("call " + watPrintLine)
	case stringType:
		e.line("call " + watPrintLine)
	case boolType:
		e.line("if")
		e.depth++
		e.stringConst("true")
		e.line("call " + watPrintLine)
		e.depth--
		e.line("else")
		e.depth++
		e.stringConst("false")
		e.line("call " + watPrintLine)
		e.depth--
		e.line("end")
	case unitType:
		if stackHasArg {
			e.line("drop")
		}
		e.stringConst("")
		e.line("call " + watPrintLine)
	default:
		watFence(x.pos, "print of "+string(x.inputType))
	}
}

// fieldExpr emits the postfix field chain after any postfix receiver —
// xs[0].a.b — walking struct.gets exactly like nameExpr's dotted path.
func (e *watEmitter) fieldExpr(x *expression) {
	e.expr(x.left)
	t := x.left.typ
	for at, idx := range x.fields {
		r := e.byID[string(t)]
		if r == nil || r.choice {
			watFence(x.pos, string(t))
		}
		fieldType := r.fields[idx].typ
		if at == len(x.fields)-1 {
			if fieldType == unitType {
				e.line("drop")
				return // a Unit field carries no value
			}
			e.line("struct.get %s $f%d", watRecordName(r), idx)
			e.assertNonNull(fieldType)
			return
		}
		e.line("struct.get %s $f%d", watRecordName(r), idx)
		t = fieldType
	}
}

// indexExpr emits a plain list/array read (or a map lookup returning an
// Option); the checked List read is a Result helper (DESIGN-M4 §4) while
// checked Array reads and all checked writes stay fenced.
func (e *watEmitter) indexExpr(x *expression) {
	if x.checkedIndex {
		kind, inner, ok := collectionInner(x.left.typ)
		if !ok {
			watFence(x.pos, string(x.left.typ))
		}
		if kind != "List" {
			watFence(x.pos, "checked index reads") // arrays stay fenced (§4)
		}
		e.expr(x.left)
		e.expr(x.args[0])
		e.line("call " + watListGetChecked(watSpecOf(inner))) // Result<T, Error>, family 5
		return
	}
	if keyType, valType, isMap := mapTypes(x.left.typ); isMap {
		k, v := mapKeySpec(keyType), watSpecOf(valType)
		e.expr(x.left)
		e.expr(x.args[0])
		e.line("call " + watMapLookup(k, v)) // Option as a plain eq reference
		return
	}
	kind, inner, ok := collectionInner(x.left.typ)
	if !ok {
		watFence(x.pos, string(x.left.typ))
	}
	spec := watSpecOf(inner)
	e.expr(x.left)
	if kind == "List" {
		e.expr(x.args[0])
		e.line("call " + watListGet(spec))
	} else {
		for _, a := range x.args {
			e.expr(a)
		}
		e.line("array.new_fixed %s %d", watTypeArrI64, len(x.args))
		e.line("call " + watArrayGet(spec))
	}
	if spec == watSpecRef && !isUnitValue(inner) {
		e.castFromEqref(inner)
	}
}

// indexWrite emits a plain indexed write (`xs[i] = v` or `m[k] = v`); the
// parenthesized checked form is fenced (§6c).
func (e *watEmitter) indexWrite(x *expression) {
	if !x.plainWrite {
		watFence(x.pos, "checked index writes")
	}
	idx := x.left
	if keyType, valType, isMap := mapTypes(idx.left.typ); isMap {
		// = put with operation map.set: the helper guards, then overwrites
		// or appends; the write itself is Unit, so put's pass-through map
		// ref is dropped (DESIGN-M2 §3b).
		k, v := mapKeySpec(keyType), watSpecOf(valType)
		e.expr(idx.left)
		e.expr(idx.args[0])
		e.expr(x.right)
		if isUnitValue(valType) {
			e.line("ref.null eq")
		}
		e.line("call " + watMapPut(k, v))
		e.line("drop")
		return
	}
	kind, inner, ok := collectionInner(idx.left.typ)
	if !ok {
		watFence(x.pos, string(idx.left.typ))
	}
	e.expr(idx.left) // receiver, then indices, then the value (eval order)
	if kind == "List" {
		e.expr(idx.args[0])
		e.expr(x.right)
		e.line("call " + watListSet(watSpecOf(inner)))
		return
	}
	for _, a := range idx.args {
		e.expr(a)
	}
	e.line("array.new_fixed %s %d", watTypeArrI64, len(idx.args))
	e.expr(x.right)
	if isUnitValue(inner) {
		e.line("ref.null eq")
	}
	e.line("call " + watArraySet(watSpecOf(inner)))
}

// matchExpr emits the ref.test if-chain in source arm order; the checker
// guarantees exhaustiveness, so the last arm is the final else (§7).
func (e *watEmitter) matchExpr(x *expression) {
	m := e.temp("m", "(ref null eq)")
	e.expr(x.left)
	e.line("local.set " + m)
	withValue := x.typ != unitType
	vtype := ""
	if withValue {
		vtype = e.watValueType(x.typ)
	}
	for i := 0; i < len(x.arms)-1; i++ {
		arm := &x.arms[i]
		e.line("local.get " + m)
		e.line("ref.test (ref %s)", e.variantStruct(x.left.typ, arm))
		if withValue {
			e.line("if (result %s)", vtype)
		} else {
			e.line("if")
		}
		e.depth++
		e.armBody(x, arm, m)
		e.depth--
		e.line("else")
		e.depth++
	}
	e.armBody(x, &x.arms[len(x.arms)-1], m)
	for i := 0; i < len(x.arms)-1; i++ {
		e.depth--
		e.line("end")
	}
}

// armBody casts the subject to the arm's variant struct, stores the payload
// bindings, and emits the arm's block value.
func (e *watEmitter) armBody(x *expression, arm *matchArm, m string) {
	if arm.variant >= 0 {
		v := e.variantStruct(x.left.typ, arm)
		for j, b := range arm.bindings {
			if b == nil {
				continue
			}
			field := fmt.Sprintf("$f%d", j) // payload names skip the padding fields
			if strings.HasPrefix(v, "$zhex_some_") {
				field = "$value" // the prelude's Some struct names its payload
			}
			// each payload read re-casts: struct.get consumes its receiver
			e.line("local.get " + m)
			e.line("ref.cast (ref %s)", v)
			e.line("struct.get %s %s", v, field)
			if b.typ == unitType {
				e.line("drop")
			} else {
				// the generic Some's payload is eqref; narrow it to the
				// binding's type (Option<String> and friends)
				if v == watTypeSomeRef {
					e.line("ref.cast %s", e.watLocalType(b.typ))
				}
				e.line("local.set " + e.local(b))
			}
		}
	}
	e.blockValue(arm.body)
}

// variantStruct is the wasm struct type of one arm's variant: user choices
// are struct-per-variant; Option uses the per-repr Some struct and the
// shared None struct (§2).
func (e *watEmitter) variantStruct(subject valueType, arm *matchArm) string {
	base, args, applied := typeApplication(subject)
	lookup := string(subject)
	if applied {
		lookup = base
	}
	if lookup == "Option" {
		if arm.variant == 0 {
			return watTypeNone
		}
		payload := valueType(unitType)
		if len(args) == 1 {
			payload = args[0]
		} else if len(arm.bindings) > 0 && arm.bindings[0] != nil {
			payload = arm.bindings[0].typ
		}
		return watSomeType(watSpecOf(payload))
	}
	r := e.byID[lookup]
	if r == nil {
		watFence(e.pos, string(subject))
	}
	return watVariantName(r, arm.variant)
}

// tryExpr unwraps a Result: the success payload on tag 0, else the failure
// exits here — br $zhex_ret with the struct stored (word) or fail 11 (top
// level), mirroring eval's propagation to the boundary.
func (e *watEmitter) tryExpr(x *expression) {
	res := e.temp("res", "(ref null "+watTypeResult+")")
	e.expr(x.left)
	e.line("local.set " + res)
	e.emitResultUnwrap(res, x.typ)
}

func (e *watEmitter) catchExpr(x *expression) {
	res := e.temp("res", "(ref null "+watTypeResult+")")
	e.expr(x.left)
	e.line("local.set " + res)
	e.line("local.get " + res)
	e.line("struct.get " + watTypeResult + " $tag")
	e.line("i32.eqz")
	if x.typ == unitType {
		e.line("if")
	} else {
		e.line("if (result %s)", e.watValueType(x.typ))
	}
	e.depth++
	e.line("local.get " + res)
	e.line("struct.get " + watTypeResult + " $payload")
	if x.typ != unitType {
		e.unboxPayload(x.typ)
	} else {
		// the result-less if leaves nothing: discard the Unit payload
		e.line("drop")
	}
	e.depth--
	e.line("else")
	e.depth++
	if x.binding != nil {
		// eval's catch binds the err payload — value{fault: v.result.err} —
		// not the Result struct (DESIGN-M2 §2d); it stays opaque here.
		e.line("local.get " + res)
		e.line("struct.get " + watTypeResult + " $payload")
		e.line("local.set " + e.local(x.binding))
	}
	e.blockValue(x.handler)
	e.depth--
	e.line("end")
}

// emitResultUnwrap leaves the unwrapped success value on the stack, or exits
// on a failure payload; shared by try and fallible flow steps.
func (e *watEmitter) emitResultUnwrap(res string, inner valueType) {
	e.line("local.get " + res)
	e.line("struct.get " + watTypeResult + " $tag")
	e.line("i32.eqz")
	if isUnitValue(inner) {
		e.line("if")
	} else {
		e.line("if (result %s)", e.watValueType(inner))
	}
	e.depth++
	e.line("local.get " + res)
	e.line("struct.get " + watTypeResult + " $payload")
	if !isUnitValue(inner) {
		e.unboxPayload(inner)
	} else {
		// the result-less if leaves nothing: discard the Unit payload
		e.line("drop")
	}
	e.depth--
	e.line("else")
	e.depth++
	if e.resultLocal != "" {
		e.line("local.get " + res)
		e.line("local.set $r")
		e.line("br $zhex_ret")
	} else {
		// Top level: a real err value stages its message for the runner
		// (fail 16 since M3 — 12 stays the to-int mint; anything else keeps
		// M1's generic unhandled failure, the defensive err() case,
		// DESIGN-M2 §2d amended by DESIGN-M3 §8).
		e.line("local.get " + res)
		e.line("struct.get " + watTypeResult + " $payload")
		e.line("ref.test (ref " + watTypeErr + ")")
		e.line("if")
		e.depth++
		e.line("i32.const %d", watFailUnhandledResult)
		e.line("local.get " + res)
		e.line("struct.get " + watTypeResult + " $payload")
		e.line("ref.cast (ref " + watTypeErr + ")")
		e.line("struct.get " + watTypeErr + " " + watErrFieldMessage)
		e.line("ref.as_non_null")
		e.line("call " + watFailMsg)
		e.line("unreachable")
		e.depth--
		e.line("else")
		e.depth++
		e.line("i32.const %d", watFailUnhandled)
		e.line("call " + watFail)
		e.line("unreachable") // the trap never returns; satisfy the result type
		e.depth--
		e.line("end")
		e.line("unreachable") // both inner paths trap; the outer arm never falls through
	}
	e.depth--
	e.line("end")
}

// flowStatement runs the seed into a cursor local, then each call-shaped
// step with the cursor as its first argument (args[0] is the tree's
// incoming-value slot); the last step's value is the flow's value. A with
// block is the same shape with a cursor that never updates: every step
// reads the seed local, and non-final step values drop. A closure binding
// is a legal step — e.call routes it to closureInvoke with the cursor
// already in args[0], and steps whose extra args hold holes route to
// partialCreation (DESIGN-M4 §3c).
func (e *watEmitter) flowStatement(s *statement) {
	fid := e.nextID()
	curType := s.expr.typ
	cur := ""
	e.expr(s.expr)
	if !isUnitValue(curType) {
		cur = e.temp("c"+fid+"_0", e.watLocalType(curType))
		e.line("local.set " + cur)
	}
	for idx, step := range s.steps {
		if step.checkedCollection {
			watFence(step.pos, "checked "+step.text)
		}
		e.flowCursor, e.flowCursorType = cur, curType
		e.call(step)
		nextType := step.typ
		if inner, fallible := resultInner(nextType); fallible {
			res := e.temp("res", "(ref null "+watTypeResult+")")
			e.line("local.set " + res)
			e.emitResultUnwrap(res, inner)
			nextType = inner
		}
		if idx == len(s.steps)-1 {
			return // the last step's value is the flow's value
		}
		if s.with != "" {
			if !isUnitValue(nextType) {
				e.line("drop")
			}
			continue
		}
		if !isUnitValue(nextType) {
			next := e.temp("c"+fid+"_"+fmt.Sprint(idx+1), e.watLocalType(nextType))
			e.line("local.set " + next)
			cur, curType = next, nextType
		} else {
			cur, curType = "", unitType
		}
	}
}

// isUnitValue reports whether t is Unit (no WAT value).
func isUnitValue(t valueType) bool { return t == unitType }
