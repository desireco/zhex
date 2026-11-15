package lang

import (
	"fmt"
	"strings"
)

// CompileWat checks the program exactly like CompileModules (checker errors
// return unchanged) and emits the full WebAssembly text: the runtime prelude,
// program-specific types, one wasm function per word, zhex_main for the
// top-level statements, and the exports (internal/wasm/DESIGN.md §1, §7).
// Constructs outside the wasm milestone-1 subset fail with a positioned
// not-available-in-build diagnostic.
func CompileWat(entryPath, entrySource string, resolve func(string) (ModuleSource, bool)) (wat string, err error) {
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
	e := &watEmitter{p: p}
	return e.emit(), nil
}

// watFence rejects an out-of-subset construct with the pinned diagnostic.
func watFence(pos position, what string) {
	panic(withHint(errorAt(pos, CodeNotAvailableInBuild, "%s is not available in this build", what),
		"this construct is outside the wasm milestone-1 subset; run it with zhex run instead"))
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
	flowCursor     string    // flow step's incoming value local; "" for Unit
	flowCursorType valueType // the cursor value's Zhex type
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
	out.WriteString(watPrelude)
	out.WriteString("\n;; program types\n")
	for _, r := range e.records {
		e.emitRecordType(&out, r)
	}
	out.WriteString("\n;; words\n")
	for _, w := range e.words {
		out.WriteString(e.wordFunction(w))
	}
	out.WriteString(e.mainFunction())
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
	e.resultLocal, e.wordInferred, e.cleanups = "", inferred, nil
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
	e.line("block $zhex_ret")
	e.depth++
	for _, s := range e.p.body {
		e.statementNeutral(s)
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
// i64 for Int, i32 for Bool, ref for everything else including Unit.
func watSpecOf(t valueType) watSpec {
	switch t {
	case intType:
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
// ref is non-null; eqref values pass through (their consumers are nullable).
func (e *watEmitter) assertNonNull(t valueType) {
	if nullableRef(e.watValueType(t)) != e.watValueType(t) {
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
	case intType:
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
	case intType:
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
		watFence(s.pos, "file.stream")
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
		e.line("i64.const %d", x.number)
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
	if x.binding != nil {
		watFence(x.pos, "function values")
	}
	if x.partial {
		watFence(x.pos, "partials")
	}
	for _, a := range x.args {
		if a.kind == "hole" {
			watFence(x.pos, "partials")
		}
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
		e.line("call " + watMapRemove(k, v)) // Unit
	case "map.into":
		k, v := e.mapSpecs(x)
		e.expr(x.args[0]) // source (guard targets only)
		e.expr(x.args[1])
		e.line("call " + watMapInto(k, v)) // returns the target
	default:
		watFence(x.pos, x.text)
	}
}

// printCall emits print for the four printable types (Int, Bool, String,
// Unit); anything else the interpreter formats is out of the M1 subset.
func (e *watEmitter) printCall(x *expression) {
	switch x.inputType {
	case intType:
		e.expr(x.args[0])
		e.line("call " + watPrintInt)
	case stringType:
		e.expr(x.args[0])
		e.line("call " + watPrintStr)
	case boolType:
		e.expr(x.args[0])
		e.line("if")
		e.depth++
		e.stringConst("true")
		e.line("call " + watPrintStr)
		e.depth--
		e.line("else")
		e.depth++
		e.stringConst("false")
		e.line("call " + watPrintStr)
		e.depth--
		e.line("end")
	case unitType:
		e.expr(x.args[0])
		e.stringConst("")
		e.line("call " + watPrintStr)
	default:
		watFence(x.pos, "print of "+string(x.inputType))
	}
}

// indexExpr emits a plain list/array read (or a map lookup returning an
// Option); the checked (try/catch) form is fenced.
func (e *watEmitter) indexExpr(x *expression) {
	if x.checkedIndex {
		watFence(x.pos, "checked index reads")
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
		// (fail 12); anything else keeps M1's generic unhandled failure
		// (the defensive err() case, DESIGN-M2 §2d).
		e.line("local.get " + res)
		e.line("struct.get " + watTypeResult + " $payload")
		e.line("ref.test (ref " + watTypeErr + ")")
		e.line("if")
		e.depth++
		e.line("i32.const %d", watFailTextToInt)
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
	}
	e.depth--
	e.line("end")
}

// flowStatement runs the seed into a cursor local, then each call-shaped
// step with the cursor as its first argument (args[0] is the tree's
// incoming-value slot); the last step's value is the flow's value.
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
		if step.binding != nil {
			watFence(step.pos, "function values")
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
