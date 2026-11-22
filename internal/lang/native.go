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
// and the pending cleanups every early return must run (traversal guards
// and open streams).
type nativeFunc struct {
	word      *word
	fallible  bool // returns (T, *zhexrt.Error); the entry returns *zhexrt.Error
	locals    map[string]string
	localUsed map[string]bool
	pre       *strings.Builder
	refs      map[*binding]bool // bindings the body reads
	cleanups  []string          // statements run before early returns inside their scope
}

// nativeEmitter walks a checked Program and writes Go.
type nativeEmitter struct {
	p     *Program
	b     strings.Builder
	depth int

	words      []*word
	seenWord   map[*word]bool
	records    []*recordType
	byID       map[string]*recordType
	allRecords map[string]*recordType

	goNames   map[string]string
	usedNames map[string]bool

	posVars map[string]string

	tempCount         int
	flowFirst         string
	defaultRoots      map[*expression]bool
	expandingDefaults map[*expression]bool
	fn                *nativeFunc

	// bindingRefs collects the bindings a function body reads, so lets
	// Zhex never uses can evaluate for effect without a Go declaration
	// (Go rejects unused declarations; Zhex allows them).
	bindingRefs map[*binding]bool

	recordFields map[string][]string // record id -> Go field names
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
	"zhexMain": true, "zhexrt": true, "zero": true, "stringer": true,
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
	for i := 2; e.usedNames[name] || e.fn != nil && e.fn.localUsed[name]; i++ {
		name = fmt.Sprintf("_p%d_%d_%d", pos.line, pos.column, i)
	}
	e.posVars[key] = name
	e.usedNames[name] = true
	return name
}

// writePosVars writes the position variable declarations after the walk,
// when every use is known.
func (e *nativeEmitter) writePosVars() {
	if len(e.posVars) == 0 {
		return
	}
	e.line("// Runtime positions: every operation that can fail reports its source.")
	keys := make([]string, 0, len(e.posVars))
	for key := range e.posVars {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return e.posVars[keys[i]] < e.posVars[keys[j]] })
	for _, key := range keys {
		columnSep := strings.LastIndexByte(key, ':')
		lineSep := strings.LastIndexByte(key[:columnSep], ':')
		line, _ := strconv.Atoi(key[lineSep+1 : columnSep])
		column, _ := strconv.Atoi(key[columnSep+1:])
		e.line("var %s = zhexrt.At(%q, %d, %d)", e.posVars[key], key[:lineSep], line, column)
	}
}

// ---------------------------------------------------------------- types

// goType maps a Zhex type to its Go representation. The checker is
// monomorphic, so the mapping is total on the compiled subset: Int is
// the interpreter's two-limb struct, collections are zhexrt generics
// over pointers (sharing semantics, GC-covered cycles), records and
// choices are the generated structs, and Results collapse to one runtime
// repr. Anything unsupported fences.
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
	case bufferType:
		return "*zhexrt.Buffer"
	case fileType:
		return "*zhexrt.File"
	case jsonType:
		return "*zhexrt.Json"
	case errorType:
		return "*zhexrt.Error"
	}
	if inner, ok := resultInner(t); ok {
		return "zhexrt.Result[" + e.goType(pos, inner) + "]"
	}
	if kind, inner, ok := collectionInner(t); ok {
		if kind == "List" {
			return "*zhexrt.List[" + e.goType(pos, inner) + "]"
		}
		return "*zhexrt.Array[" + e.goType(pos, inner) + "]"
	}
	if base, args, ok := typeApplication(t); ok {
		switch base {
		case "Option":
			return "zhexrt.Option[" + e.goType(pos, args[0]) + "]"
		case "Map":
			return "*zhexrt.Map[" + e.goType(pos, args[0]) + ", " + e.goType(pos, args[1]) + "]"
		}
	}
	if params, result, ok := functionTypeParts(t); ok {
		if anyResult(params) {
			nativeFence(pos, "fallible function values")
		}
		var ins []string
		for _, param := range params {
			ins = append(ins, e.goType(pos, param))
		}
		// a function value returning a Result carries the Result as its
		// Go return; try and catch unwrap it at the call sites
		return "func(" + strings.Join(ins, ", ") + ") " + e.goType(pos, result)
	}
	if record := e.byID[string(t)]; record != nil && !record.builtin {
		return "*" + e.packageName("t:"+record.id(), record.id())
	}
	switch t {
	case "Date", "Time", "Duration", "Instant", "TimeZone", "ZonedDateTime":
		nativeFence(pos, string(t)+" values")
	}
	nativeFence(pos, fmt.Sprintf("%s values", t))
	return ""
}

// anyResult reports whether any parameter type is a Result.
func anyResult(params []valueType) bool {
	for _, param := range params {
		if isResult(param) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- walk

// collect gathers every reachable word and record exactly like the WAT
// emitter's collect: entry words in declaration order, then module words
// in first-reference order, walking the entry body first.
func (e *nativeEmitter) collect() {
	e.seenWord = map[*word]bool{}
	e.byID = map[string]*recordType{}
	e.allRecords = collectRecords(e.p.modules)
	for _, r := range e.p.records {
		e.allRecords[r.id()] = r
	}
	e.goNames = map[string]string{}
	e.usedNames = map[string]bool{}
	e.posVars = map[string]string{}
	e.defaultRoots = map[*expression]bool{}
	e.expandingDefaults = map[*expression]bool{}
	e.recordFields = map[string][]string{}
	e.byID[builtinJSON.id()] = builtinJSON
	e.byID[builtinOption.id()] = builtinOption
	for _, r := range e.p.records {
		e.addRecord(r)
	}
	for _, w := range e.p.words {
		e.addWord(w)
	}
	e.walkStatements(e.p.body)
	for wordIndex, recordIndex := 0, 0; wordIndex < len(e.words) || recordIndex < len(e.records); {
		if wordIndex < len(e.words) {
			w := e.words[wordIndex]
			wordIndex++
			e.addType(w.result)
			e.walkStatements(w.body)
			for _, param := range w.params {
				e.addType(param.typ)
				if param.defaultExpr != nil {
					e.defaultRoots[param.defaultExpr] = true
				}
				e.walkExpression(param.defaultExpr)
			}
		}
		if recordIndex < len(e.records) {
			r := e.records[recordIndex]
			recordIndex++
			for _, field := range r.fields {
				e.addType(field.typ)
				if field.defaultExpr != nil {
					e.defaultRoots[field.defaultExpr] = true
				}
				e.walkExpression(field.defaultExpr)
			}
			for _, variant := range r.variants {
				for _, field := range variant.fields {
					e.addType(field.typ)
				}
			}
		}
	}
	// Reserve every function name before assigning locals, including words
	// emitted later that share a spelling with a parameter or binding.
	for _, w := range e.words {
		e.packageName(wordKey(w), wordGoName(w))
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
	if r == nil || e.byID[r.id()] != nil {
		return
	}
	if r.builtin {
		return
	}
	if r.typeParam != "" {
		kind := "record"
		if r.choice {
			kind = "choice"
		}
		nativeFence(r.pos, "generic "+kind+" "+r.name)
	}
	e.records = append(e.records, r)
	e.byID[r.id()] = r
}

// addType follows nominal types reachable through signatures and fields,
// including module records that never appear in a constructor call.
func (e *nativeEmitter) addType(t valueType) {
	if record := e.allRecords[string(t)]; record != nil {
		e.addRecord(record)
		return
	}
	if params, result, ok := functionTypeParts(t); ok {
		for _, param := range params {
			e.addType(param)
		}
		e.addType(result)
		return
	}
	if _, args, ok := typeApplication(t); ok {
		for _, arg := range args {
			e.addType(arg)
		}
	}
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
	e.addType(x.typ)
	if e.bindingRefs != nil && x.binding != nil && (x.kind == "name" || x.kind == "call") {
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

// collectRefs prewalks a body collecting every binding it reads, so the
// emitter can skip Go declarations Zhex itself never uses.
func (e *nativeEmitter) collectRefs(body []*statement) map[*binding]bool {
	e.bindingRefs = map[*binding]bool{}
	e.walkStatements(body)
	refs := e.bindingRefs
	e.bindingRefs = nil
	return refs
}

// ---------------------------------------------------------------- emit

func (e *nativeEmitter) emit() {
	e.collect()
	e.emitRecords()
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

// recordFieldNames returns the Go field names for a record's fields, in
// declaration order, deduplicated within the record. Choices flatten
// every variant's payload onto one struct, prefixed by variant name so
// spellings never collide.
func (e *nativeEmitter) recordFieldNames(r *recordType) []string {
	if got, ok := e.recordFields[r.id()]; ok {
		return got
	}
	var fields []parameter
	if r.choice {
		for _, variant := range r.variants {
			for _, field := range variant.fields {
				fields = append(fields, parameter{name: variant.name + "-" + field.name, typ: field.typ})
			}
		}
	} else {
		fields = r.fields
	}
	used := map[string]bool{}
	var names []string
	for _, field := range fields {
		name := goMangle(field.name)
		for i := 2; used[name]; i++ {
			name = fmt.Sprintf("%s_%d", goMangle(field.name), i)
		}
		used[name] = true
		names = append(names, name)
	}
	e.recordFields[r.id()] = names
	return names
}

// emitRecords writes one Go struct per reachable record and choice:
// field-for-field for records, a tag plus prefixed payload fields for
// choices.
func (e *nativeEmitter) emitRecords() {
	for _, r := range e.records {
		name := e.packageName("t:"+r.id(), r.id())
		if r.choice {
			e.line("// %s: tag values %s", r.id(), e.variantIndexComment(r))
			e.line("type %s struct {", name)
			e.depth++
			e.line("tag int")
			for i, variant := range r.variants {
				for j, field := range variant.fields {
					e.line("%s %s", e.recordFieldNames(r)[e.variantFieldBase(r, i)+j], e.goType(field.pos, field.typ))
				}
			}
			e.depth--
			e.line("}")
			e.line("")
			continue
		}
		e.line("type %s struct {", name)
		e.depth++
		names := e.recordFieldNames(r)
		for i, field := range r.fields {
			e.line("%s %s", names[i], e.goType(field.pos, field.typ))
		}
		e.depth--
		e.line("}")
		e.line("")
	}
}

func (e *nativeEmitter) variantIndexComment(r *recordType) string {
	var parts []string
	for i, variant := range r.variants {
		parts = append(parts, fmt.Sprintf("%d=%s", i, variant.name))
	}
	return strings.Join(parts, ", ")
}

// variantFieldBase is the first payload field index of a variant on the
// flattened choice struct.
func (e *nativeEmitter) variantFieldBase(r *recordType, variant int) int {
	base := 0
	for i := 0; i < variant; i++ {
		base += len(r.variants[i].fields)
	}
	return base
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
	e.emitCleanups()
	if w.inferred {
		e.line("return __result, nil")
	} else {
		e.line("return __result")
	}
	e.depth--
	e.line("}")
}

// pushCleanup registers a statement every early return inside the
// current scope must run first (traversal guards, open streams).
func (e *nativeEmitter) pushCleanup(statement string) {
	e.fn.cleanups = append(e.fn.cleanups, statement)
}

// popCleanup deregisters the innermost cleanup, emitting it for the
// normal path.
func (e *nativeEmitter) popCleanup() {
	last := len(e.fn.cleanups) - 1
	e.line("%s", e.fn.cleanups[last])
	e.fn.cleanups = e.fn.cleanups[:last]
}

// emitCleanups writes every pending cleanup, for an early return.
func (e *nativeEmitter) emitCleanups() {
	for i := len(e.fn.cleanups) - 1; i >= 0; i-- {
		e.line("%s", e.fn.cleanups[i])
	}
}

// failReturn emits the early-error return for the current function shape:
// the entry returns the diagnostic, a fallible word pairs it with the
// zero of its success type, and an explicit-Result word makes it the
// returned Result's Err.
func (e *nativeEmitter) failReturn(errExpr string) {
	for i := len(e.fn.cleanups) - 1; i >= 0; i-- {
		e.pre("%s", e.fn.cleanups[i])
	}
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
			}
		}
	} else {
		e.statements(e.p.body, "")
	}
	e.line("return nil")
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
		e.emitExprStatement(s.expr, target)
	case "return":
		e.fn.pre = &strings.Builder{}
		v := e.expr(s.expr)
		e.flushPre()
		e.emitCleanups()
		if e.fn.word != nil && e.fn.fallible {
			e.line("return %s, nil", v)
		} else {
			e.line("return %s", v)
		}
	case "let", "var", "assign":
		e.emitBinding(s)
	case "if":
		e.emitIf(s, target)
	case "match":
		e.emitMatch(s.expr, target)
	case "while":
		e.emitWhile(s)
	case "for":
		e.emitForRange(s)
	case "for-collection":
		e.emitForCollection(s)
	case "for-lines":
		e.emitForLines(s)
	case "flow":
		e.emitFlow(s, target)
	default:
		nativeFence(s.pos, s.kind+" statements")
	}
}

// neverReturns reports whether the expression is a console.exit or
// console.fail call: it terminates instead of producing the fallback
// value, so a block tail carrying it assigns nothing.
func neverReturns(x *expression) bool {
	return x != nil && x.kind == "call" && (x.text == "console.exit" || x.text == "console.fail")
}

// emitExprStatement evaluates an expression for its value or effect.
func (e *nativeEmitter) emitExprStatement(x *expression, target string) {
	if x.kind == "statement-value" {
		e.statement(x.statement, target)
		return
	}
	if x.kind == "match" {
		e.emitMatch(x, target)
		return
	}
	e.fn.pre = &strings.Builder{}
	v := e.expr(x)
	hoisted := e.fn.pre.Len() != 0
	e.flushPre()
	if target != "" && neverReturns(x) {
		e.line("%s", v)
		return
	}
	if target != "" && x.typ != unitType {
		e.line("%s = %s", target, v)
	} else if isCallShaped(x) && !hoisted {
		e.line("%s", v)
	} else {
		e.line("_ = %s", v)
	}
}

// emitBinding writes let/var/assign, including block-valued right-hand
// sides; a binding Zhex never reads evaluates for effect only.
func (e *nativeEmitter) emitBinding(s *statement) {
	if s.kind != "assign" && !e.fn.refs[s.binding] {
		e.emitExprStatement(s.expr, "")
		return
	}
	name := e.localName(s.name)
	if s.expr != nil && s.expr.kind == "statement-value" {
		if s.kind != "assign" {
			e.line("var %s %s", name, e.goType(s.pos, s.binding.typ))
		}
		e.statement(s.expr.statement, name)
		return
	}
	// the parser hands a let-bound match straight to the expression
	// grammar, unwrapped (statementExpression)
	if s.expr != nil && s.expr.kind == "match" {
		if s.kind != "assign" {
			e.line("var %s %s", name, e.goType(s.pos, s.binding.typ))
		}
		e.emitMatch(s.expr, name)
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
}

func (e *nativeEmitter) emitIf(s *statement, target string) {
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
}

// emitMatch writes a match as a switch over the subject's tag (choices,
// Json) or Some flag (Option), binding payloads per arm.
func (e *nativeEmitter) emitMatch(x *expression, target string) {
	e.fn.pre = &strings.Builder{}
	subject := e.expr(x.left)
	e.flushPre()
	temp := e.temp()
	e.line("%s := %s", temp, subject)
	subjectType := x.left.typ
	switch {
	case subjectType == jsonType:
		e.line("switch %s.Kind {", temp)
	case e.optionInner(subjectType) != "":
		e.line("switch {")
	case e.byID[string(subjectType)] != nil && e.byID[string(subjectType)].choice:
		e.line("switch %s.tag {", temp)
	default:
		nativeFence(x.pos, "match on "+string(subjectType))
	}
	for _, arm := range x.arms {
		if arm.variant < 0 {
			e.depth--
			e.line("default:")
			e.depth++
		} else {
			e.depth--
			switch {
			case subjectType == jsonType:
				e.line("case %d:", arm.variant)
			case e.optionInner(subjectType) != "":
				if arm.variant == 0 {
					e.line("case !%s.Some:", temp)
				} else {
					e.line("case %s.Some:", temp)
				}
			default:
				e.line("case %d:", arm.variant)
			}
			e.depth++
		}
		e.depth++
		e.emitMatchBindings(x, arm, temp)
		e.statements(arm.body, target)
		e.depth--
	}
	e.line("}")
}

// emitMatchBindings assigns the arm's payload names from the subject.
func (e *nativeEmitter) emitMatchBindings(x *expression, arm matchArm, temp string) {
	subjectType := x.left.typ
	if subjectType == jsonType {
		for _, b := range arm.bindings {
			if b == nil || !e.fn.refs[b] {
				continue
			}
			field := map[int]string{1: "Bool", 2: "Text", 3: "Text", 4: "List", 5: "Object"}[arm.variant]
			e.line("%s := %s.%s", e.localName(b.name), temp, field)
		}
		return
	}
	if e.optionInner(subjectType) != "" {
		for _, b := range arm.bindings {
			if b != nil && e.fn.refs[b] {
				e.line("%s := %s.Value", e.localName(b.name), temp)
			}
		}
		return
	}
	record := e.byID[string(subjectType)]
	names := e.recordFieldNames(record)
	base := e.variantFieldBase(record, arm.variant)
	for i, b := range arm.bindings {
		if b != nil && e.fn.refs[b] {
			e.line("%s := %s.%s", e.localName(b.name), temp, names[base+i])
		}
	}
}

// optionInner returns Option<T>'s inner type, or "".
func (e *nativeEmitter) optionInner(t valueType) valueType {
	if base, args, ok := typeApplication(t); ok && base == "Option" && len(args) == 1 {
		return args[0]
	}
	return ""
}

func (e *nativeEmitter) emitWhile(s *statement) {
	e.fn.pre = &strings.Builder{}
	cond := e.expr(s.expr)
	pre := e.fn.pre.String()
	e.fn.pre.Reset()
	if pre == "" {
		e.line("for %s {", cond)
		e.depth++
		e.statements(s.body, "")
		e.depth--
		e.line("}")
		return
	}
	// the condition's preamble must re-run each pass
	e.line("for {")
	e.depth++
	e.line("%s", pre)
	e.line("if !%s {", cond)
	e.depth++
	e.line("break")
	e.depth--
	e.line("}")
	e.statements(s.body, "")
	e.depth--
	e.line("}")
}

// emitForRange writes `for n in range(start, end)` — exclusive end.
func (e *nativeEmitter) emitForRange(s *statement) {
	e.fn.pre = &strings.Builder{}
	start := e.expr(s.expr)
	e.flushPre()
	startTemp := e.temp()
	e.line("%s := %s", startTemp, start)
	end := e.expr(s.end)
	e.flushPre()
	endTemp := e.temp()
	e.line("%s := %s", endTemp, end)
	start, end = startTemp, endTemp
	name := e.localName(s.name)
	e.line("for %s := %s; zhexrt.Cmp(%s, %s) < 0; %s, _ = zhexrt.AddOvf(%s, zhexrt.I64(1)) {", name, start, name, end, name, name)
	e.depth++
	e.statements(s.body, "")
	e.depth--
	e.line("}")
}

// emitForCollection writes `for v in list|array|map` and the dual-binding
// forms, with the interpreter's traversal guards as cleanups.
func (e *nativeEmitter) emitForCollection(s *statement) {
	e.fn.pre = &strings.Builder{}
	receiver := e.expr(s.expr)
	e.flushPre()
	temp := e.temp()
	e.line("%s := %s", temp, receiver)
	name := e.localName(s.name)
	second := ""
	if s.secondName != "" {
		second = e.localName(s.secondName)
	}
	if kind, _, ok := collectionInner(s.expr.typ); ok && kind == "List" {
		e.line("%s.Active++", temp)
		e.pushCleanup(fmt.Sprintf("%s.Active--", temp))
		index, item := "_", "_"
		if second != "" {
			if e.fn.refs[s.binding] {
				index = "__i"
			}
			if e.fn.refs[s.secondBinding] {
				item = second
			}
		} else if e.fn.refs[s.binding] {
			item = name
		}
		if index == "_" && item == "_" {
			e.line("for range %s.Items {", temp)
		} else {
			e.line("for %s, %s := range %s.Items {", index, item, temp)
		}
		e.depth++
		if index != "_" {
			e.line("%s := zhexrt.I64(int64(%s))", name, index)
		}
		e.statements(s.body, "")
		e.depth--
		e.line("}")
		e.popCleanup()
		return
	}
	if kind, _, ok := collectionInner(s.expr.typ); ok && kind == "Array" {
		e.line("%s.Active++", temp)
		e.pushCleanup(fmt.Sprintf("%s.Active--", temp))
		index, item := "_", "_"
		if second != "" {
			if e.fn.refs[s.binding] {
				index = "__i"
			}
			if e.fn.refs[s.secondBinding] {
				item = second
			}
		} else if e.fn.refs[s.binding] {
			item = name
		}
		if index == "_" && item == "_" {
			e.line("for range %s.Items {", temp)
		} else {
			e.line("for %s, %s := range %s.Items {", index, item, temp)
		}
		e.depth++
		if index != "_" {
			// Coordinates are a fresh list, matching the interpreter.
			e.line("__coords := make([]zhexrt.Int, len(%s.Shape))", temp)
			e.line("__n := int64(%s)", index)
			e.line("for __axis := len(%s.Shape) - 1; __axis >= 0; __axis-- {", temp)
			e.depth++
			e.line("__coords[__axis] = zhexrt.I64(__n %% %s.Shape[__axis])", temp)
			e.line("__n /= %s.Shape[__axis]", temp)
			e.depth--
			e.line("}")
			e.line("%s := &zhexrt.List[zhexrt.Int]{Items: __coords}", name)
		}
		e.statements(s.body, "")
		e.depth--
		e.line("}")
		e.popCleanup()
		return
	}
	// map: single binding iterates values, dual binding key then value
	e.line("%s.Active++", temp)
	e.pushCleanup(fmt.Sprintf("%s.Active--", temp))
	if !e.fn.refs[s.binding] && (second == "" || !e.fn.refs[s.secondBinding]) {
		e.line("for range %s.Entries {", temp)
		e.depth++
	} else if second != "" {
		e.line("for _, __entry := range %s.Entries {", temp)
		e.depth++
		if e.fn.refs[s.binding] {
			e.line("%s := __entry.Key", name)
		}
		if e.fn.refs[s.secondBinding] {
			e.line("%s := __entry.Value", second)
		}
	} else {
		e.line("for _, __entry := range %s.Entries {", temp)
		e.depth++
		if e.fn.refs[s.binding] {
			e.line("%s := __entry.Value", name)
		}
	}
	e.statements(s.body, "")
	e.depth--
	e.line("}")
	e.popCleanup()
}

// emitForLines writes `for line in file.stream(path)` — the stream always
// closes, including on early returns inside the body.
func (e *nativeEmitter) emitForLines(s *statement) {
	e.fn.pre = &strings.Builder{}
	path := e.expr(s.expr)
	e.flushPre()
	file := e.temp()
	e.line("%s := zhexrt.StreamOpen(%s, %s)", file, e.posVar(s.pos), path)
	e.pushCleanup(fmt.Sprintf("zhexrt.FileClose(%s)", file))
	name := e.localName(s.name)
	linePos := e.posVar(s.pos)
	e.line("for {")
	e.depth++
	next := "_"
	if e.fn.refs[s.binding] {
		next = e.temp()
	}
	e.line("%s, __ok := zhexrt.StreamLine(%s, %s)", next, linePos, file)
	e.line("if !__ok {")
	e.depth++
	e.line("break")
	e.depth--
	e.line("}")
	if next != "_" {
		e.line("%s := %s", name, next)
	}
	e.statements(s.body, "")
	e.depth--
	e.line("}")
	e.popCleanup()
}

// emitFlow writes a threaded flow or a with block as straight-line calls.
// A with block fixes the seed: every step receives it as the first
// argument instead of the previous step's result.
func (e *nativeEmitter) emitFlow(s *statement, target string) {
	e.fn.pre = &strings.Builder{}
	seed := e.expr(s.expr)
	e.flushPre()
	// materialize the seed once: a with block hands the same value to
	// every step, and a threaded flow starts from it
	seedTemp := e.temp()
	e.line("%s := %s", seedTemp, seed)
	cursor := seedTemp
	withBlock := s.with != ""
	for index, step := range s.steps {
		e.fn.pre = &strings.Builder{}
		last := index == len(s.steps)-1
		first := cursor
		if withBlock {
			first = seedTemp
		}
		value := e.stepValue(step, first)
		e.flushPre()
		if last && target != "" && step.typ != unitType {
			e.line("%s = %s", target, value)
			continue
		}
		// with-block steps discard their values (the seed is fixed); a
		// Unit step or the discarded tail runs for its effect
		if last || step.typ == unitType || withBlock {
			e.line("_ = %s", value)
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

// flushPre writes the hoisted expression preamble ahead of the statement
// line being assembled.
func (e *nativeEmitter) flushPre() {
	text := e.fn.pre.String()
	e.fn.pre.Reset()
	if text != "" {
		e.b.WriteString(text)
	}
}

// isCallShaped reports whether the expression is a plain call that may
// stand alone as a Go statement when its value is discarded.
func isCallShaped(x *expression) bool {
	return x != nil && x.kind == "call"
}

// ---------------------------------------------------------------- expressions

// expr emits an expression, hoisting statements it needs into the
// current preamble, and returns its Go expression string.
func (e *nativeEmitter) expr(x *expression) string {
	if e.defaultRoots[x] {
		if e.expandingDefaults[x] {
			nativeFence(x.pos, "recursive word defaults")
		}
		e.expandingDefaults[x] = true
		defer delete(e.expandingDefaults, x)
	}
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
		return e.nameExpr(x)
	case "flow-value":
		return e.flowFirst
	case "field":
		return e.fieldExpr(x)
	case "list":
		return e.listLiteral(x)
	case "map":
		return e.mapLiteral(x)
	case "index":
		return e.indexRead(x)
	case "index-write":
		return e.indexWrite(x)
	case "unary":
		v := e.expr(x.left)
		if x.text == "not" {
			return "!(" + v + ")"
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

// exprs preserves left-to-right evaluation when a later operand needs
// hoisted statements. Earlier inline operands run before that preamble;
// expressions without preambles keep their ordinary Go expression form.
func (e *nativeEmitter) exprs(expressions []*expression) []string {
	values := make([]string, 0, len(expressions))
	pending := 0
	for _, x := range expressions {
		saved := e.fn.pre
		e.fn.pre = &strings.Builder{}
		value := e.expr(x)
		pre := e.fn.pre.String()
		e.fn.pre = saved
		if pre != "" {
			for i := pending; i < len(values); i++ {
				temp := e.temp()
				e.pre("%s := %s", temp, values[i])
				values[i] = temp
			}
			pending = len(values)
			e.fn.pre.WriteString(pre)
		}
		values = append(values, value)
	}
	return values
}

// nameExpr reads a binding (with optional field path) or a bare choice
// constructor such as Option.None. The parser spells a field path as one
// dotted text, so the base local comes from the binding, not x.text.
func (e *nativeEmitter) nameExpr(x *expression) string {
	if x.record != nil && x.record.choice {
		return e.choiceConstructor(x.record, x.variant, nil, x)
	}
	base := e.localName(x.binding.name)
	if len(x.fields) == 0 {
		return base
	}
	record := x.binding.typ
	for _, index := range x.fields {
		r := e.byID[string(record)]
		if r == nil || r.choice {
			nativeFence(x.pos, "fields on "+string(record))
		}
		base = base + "." + e.recordFieldNames(r)[index]
		if index < len(r.fields) {
			record = r.fields[index].typ
		}
	}
	return base
}

// fieldExpr reads record fields off any receiver expression.
func (e *nativeEmitter) fieldExpr(x *expression) string {
	receiver := e.expr(x.left)
	record := x.left.typ
	for _, index := range x.fields {
		r := e.byID[string(record)]
		if r == nil || r.choice {
			nativeFence(x.pos, "fields on "+string(record))
		}
		receiver = receiver + "." + e.recordFieldNames(r)[index]
		if index < len(r.fields) {
			record = r.fields[index].typ
		}
	}
	return receiver
}

func (e *nativeEmitter) listLiteral(x *expression) string {
	_, inner, _ := collectionInner(x.typ)
	elementType := e.goType(x.pos, inner)
	items := e.exprs(x.args)
	return fmt.Sprintf("&zhexrt.List[%s]{Items: []%s{%s}}", elementType, elementType, strings.Join(items, ", "))
}

func (e *nativeEmitter) mapLiteral(x *expression) string {
	base, args, ok := typeApplication(x.typ)
	if !ok || base != "Map" {
		nativeFence(x.pos, "map literals")
	}
	temp := e.temp()
	e.pre("%s := zhexrt.NewMap[%s, %s]()", temp, e.goType(x.pos, args[0]), e.goType(x.pos, args[1]))
	values := e.exprs(x.args)
	for i := 0; i < len(values); i += 2 {
		e.pre("zhexrt.MapPut(%s, %s, %s)", temp, values[i], values[i+1])
	}
	return temp
}

// mapKeyType returns the Map's key type string, or "" when not a map.
func (e *nativeEmitter) mapKeyType(t valueType) string {
	if base, args, ok := typeApplication(t); ok && base == "Map" && len(args) == 2 {
		return string(args[0])
	}
	return ""
}

// indexRead emits m[k] (an Option), and the plain or checked list/array
// indexed read.
func (e *nativeEmitter) indexRead(x *expression) string {
	values := e.exprs(append([]*expression{x.left}, x.args...))
	receiver, indices := values[0], values[1:]
	if e.mapKeyType(x.left.typ) != "" {
		return fmt.Sprintf("zhexrt.MapLookup(%s, %s)", receiver, indices[0])
	}
	pos := e.posVar(x.pos)
	if kind, _, ok := collectionInner(x.left.typ); ok && kind == "List" {
		call := fmt.Sprintf("zhexrt.ListGet(%s, %s, %s)", pos, receiver, indices[0])
		if x.checkedIndex {
			return call
		}
		return e.hardUnwrap(call)
	}
	call := fmt.Sprintf("zhexrt.ArrayGet(%s, %s, []zhexrt.Int{%s})", pos, receiver, strings.Join(indices, ", "))
	if x.checkedIndex {
		return call
	}
	return e.hardUnwrap(call)
}

// indexWrite emits the plain indexed write (checked writes fence: the
// parenthesized form is the checked collection contract).
func (e *nativeEmitter) indexWrite(x *expression) string {
	if !x.plainWrite {
		nativeFence(x.pos, "checked indexed writes")
	}
	idx := x.left
	operands := append([]*expression{idx.left}, idx.args...)
	values := e.exprs(append(operands, x.right))
	receiver, indices := values[0], values[1:len(values)-1]
	value := values[len(values)-1]
	pos := e.posVar(x.pos)
	if e.mapKeyType(idx.left.typ) != "" {
		e.pre("zhexrt.MapSetHard(%s, %s, %s, %s)", pos, receiver, indices[0], value)
		return "zhexrt.Unit{}"
	}
	if kind, _, ok := collectionInner(idx.left.typ); ok && kind == "List" {
		return e.hardUnwrap(fmt.Sprintf("zhexrt.ListSet(%s, %s, %s, %s)", pos, receiver, indices[0], value))
	}
	return e.hardUnwrap(fmt.Sprintf("zhexrt.ArraySet(%s, %s, []zhexrt.Int{%s}, %s)", pos, receiver, strings.Join(indices, ", "), value))
}

// hardUnwrap converts a checked Result into its plain form: the value
// when it succeeded, a hard panic when it failed (plainReadResult).
func (e *nativeEmitter) hardUnwrap(resultExpr string) string {
	temp := e.temp()
	e.pre("%s := %s", temp, resultExpr)
	e.pre("if %s.Err != nil {", temp)
	e.pre("	panic(%s.Err)", temp)
	e.pre("}")
	return temp + ".Value"
}

// binaryPrec is one Zhex/Go operator precedence class (both languages
// agree on the ordering): or < and < comparisons < additive <
// multiplicative.
func binaryPrec(op string) int {
	switch op {
	case "or":
		return 1
	case "and":
		return 2
	case "==", "!=", "<", "<=", ">", ">=":
		return 3
	case "+", "-":
		return 4
	default:
		return 5
	}
}

// rightNeedsParen reports whether a binary right operand needs keeping in
// parentheses: lower precedence than its parent, or equal precedence
// under a non-commutative operator (the parser only nests those through
// explicit grouping).
func rightNeedsParen(op string, right *expression) bool {
	if right.kind != "binary" {
		return false
	}
	rp, pp := binaryPrec(right.text), binaryPrec(op)
	return rp < pp || rp == pp && (op == "-" || op == "/" || op == "%")
}

func (e *nativeEmitter) binary(x *expression) string {
	switch x.text {
	case "and", "or":
		left := e.expr(x.left)
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
			if rightNeedsParen(x.text, x.right) {
				right = "(" + right + ")"
			}
			return fmt.Sprintf("(%s %s %s)", left, op, right)
		}
		// Short-circuit: the right side's preamble must not run when the
		// left side already decides.
		temp := e.temp()
		e.pre("%s := %s", temp, left)
		if op == "&&" {
			e.pre("if %s {", temp)
		} else {
			e.pre("if !%s {", temp)
		}
		e.pre("%s", rightPre)
		e.pre("%s = %s", temp, right)
		e.pre("}")
		return temp
	}
	values := e.exprs([]*expression{x.left, x.right})
	left, right := values[0], values[1]
	if rightNeedsParen(x.text, x.right) {
		right = "(" + right + ")"
	}
	switch x.text {
	case "==", "!=":
		op := x.text
		return fmt.Sprintf("(%s %s %s)", left, op, right)
	case "<", "<=", ">", ">=":
		order := map[string]string{"<": "< 0", "<=": "<= 0", ">": "> 0", ">=": ">= 0"}[x.text]
		return fmt.Sprintf("(zhexrt.Cmp(%s, %s) %s)", left, right, order)
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
// fallible words materialize a (value, error) pair. Word parameter
// defaults evaluate inline at the call site (they are closed
// expressions).
func (e *nativeEmitter) callValue(x *expression) (v, errv string) {
	arguments := append([]*expression(nil), x.args...)
	for i := len(arguments); i < len(x.word.params); i++ {
		defaultExpr := x.word.params[i].defaultExpr
		if defaultExpr == nil {
			nativeFence(x.pos, "word parameter defaults")
		}
		arguments = append(arguments, defaultExpr)
	}
	args := e.exprs(arguments)
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
	var arguments []*expression
	for _, arg := range x.args {
		if arg.kind != "hole" {
			arguments = append(arguments, arg)
		}
	}
	return e.exprs(arguments)
}

func (e *nativeEmitter) call(x *expression) string {
	if x.partial {
		return e.partialExpr(x)
	}
	if x.binding != nil {
		// a captured function value invoked with the call's arguments
		args := e.callArgs(x)
		return fmt.Sprintf("%s(%s)", e.localName(x.binding.name), strings.Join(args, ", "))
	}
	if x.record != nil {
		if x.record.choice {
			return e.choiceConstructor(x.record, x.variant, e.callArgs(x), x)
		}
		return e.recordConstructor(x)
	}
	if x.word != nil {
		v, errv := e.callValue(x)
		if errv == "" {
			return v
		}
		// plain context: the call's value is its Result
		return fmt.Sprintf("zhexrt.Result[%s]{Value: %s, Err: %s}", e.goType(x.pos, x.word.success), v, errv)
	}
	return e.builtin(x)
}

// choiceConstructor builds one variant: Option and Json map to zhexrt
// literals, user choices to the generated struct.
func (e *nativeEmitter) choiceConstructor(r *recordType, variant int, args []string, x *expression) string {
	if r == builtinOption {
		if variant == 0 {
			return fmt.Sprintf("zhexrt.None[%s]()", e.goType(x.pos, e.optionInner(x.typ)))
		}
		return fmt.Sprintf("zhexrt.Some(%s)", args[0])
	}
	if r == builtinJSON {
		switch variant {
		case 0:
			return "&zhexrt.Json{Kind: 0}"
		case 1:
			return fmt.Sprintf("&zhexrt.Json{Kind: 1, Bool: %s}", args[0])
		case 2, 3:
			return fmt.Sprintf("&zhexrt.Json{Kind: %d, Text: %s}", variant, args[0])
		case 4:
			return fmt.Sprintf("&zhexrt.Json{Kind: 4, List: %s}", args[0])
		default:
			return fmt.Sprintf("&zhexrt.Json{Kind: 5, Object: %s}", args[0])
		}
	}
	name := e.packageName("t:"+r.id(), r.id())
	if len(r.variants[variant].fields) == 0 {
		return fmt.Sprintf("&%s{tag: %d}", name, variant)
	}
	names := e.recordFieldNames(r)
	base := e.variantFieldBase(r, variant)
	var fields []string
	for i, arg := range args {
		fields = append(fields, fmt.Sprintf("%s: %s", names[base+i], arg))
	}
	return fmt.Sprintf("&%s{tag: %d, %s}", name, variant, strings.Join(fields, ", "))
}

// recordConstructor builds a record literal, positional or all-named,
// filling omitted trailing defaults inline (closed expressions).
func (e *nativeEmitter) recordConstructor(x *expression) string {
	r := x.record
	name := e.packageName("t:"+r.id(), r.id())
	arguments := append([]*expression(nil), x.args...)
	indices := make([]int, 0, len(r.fields))
	present := make([]bool, len(r.fields))
	for i := range x.args {
		index := i
		if len(x.argNames) != 0 {
			index = x.argFields[i]
		}
		indices = append(indices, index)
		present[index] = true
	}
	for i, field := range r.fields {
		if present[i] {
			continue
		}
		if field.defaultExpr == nil {
			nativeFence(x.pos, "record field defaults")
		}
		arguments = append(arguments, field.defaultExpr)
		indices = append(indices, i)
	}
	values := e.exprs(arguments)
	fields := make([]string, 0, len(r.fields))
	for i, value := range values {
		index := indices[i]
		fields = append(fields, e.recordFieldNames(r)[index]+": "+value)
	}
	return fmt.Sprintf("&%s{%s}", name, strings.Join(fields, ", "))
}

// partialExpr turns a partial application into a Go closure: captured
// arguments evaluate once, in source order, at creation; holes become
// the closure's parameters in order.
func (e *nativeEmitter) partialExpr(x *expression) string {
	params, result, ok := functionTypeParts(x.typ)
	if !ok {
		nativeFence(x.pos, "partials without a function type")
	}
	if anyResult(params) {
		nativeFence(x.pos, "fallible partials")
	}
	if x.record != nil {
		nativeFence(x.pos, "constructor partials")
	}
	// evaluate the captured arguments once, in source order, at
	// creation: each lands in a temp the closure reads, so later
	// reassignment of a captured name never leaks in
	captured := make([]string, len(x.args))
	for i, arg := range x.args {
		if arg.kind == "hole" {
			continue
		}
		temp := e.temp()
		e.pre("%s := %s", temp, e.expr(arg))
		captured[i] = temp
	}
	// the closure's parameters are the holes in order; params lists the
	// RESULTING function's parameter types, so hole i maps to params[i]
	var paramNames []string
	var paramTypes []string
	for i := range x.holes {
		name := fmt.Sprintf("__a%d", i)
		paramNames = append(paramNames, name)
		paramTypes = append(paramTypes, name+" "+e.goType(x.pos, params[i]))
	}
	// splice holes and captured slots back into the call shape
	var callArgs []string
	holeAt := 0
	for i := range x.args {
		if x.args[i].kind == "hole" {
			callArgs = append(callArgs, paramNames[holeAt])
			holeAt++
			continue
		}
		callArgs = append(callArgs, captured[i])
	}
	var body string
	if x.word != nil {
		name := e.packageName(wordKey(x.word), wordGoName(x.word))
		var defaults []*expression
		for i := len(callArgs); i < len(x.word.params); i++ {
			if x.word.params[i].defaultExpr == nil {
				nativeFence(x.pos, "word parameter defaults")
			}
			defaults = append(defaults, x.word.params[i].defaultExpr)
		}
		// Omitted defaults run afresh when the closure is invoked.
		savedPre := e.fn.pre
		e.fn.pre = &strings.Builder{}
		callArgs = append(callArgs, e.exprs(defaults)...)
		defaultPre := e.fn.pre.String()
		e.fn.pre = savedPre
		if x.word.inferred {
			body = defaultPre + fmt.Sprintf("__v, __e := %s(%s)\nreturn zhexrt.Result[%s]{Value: __v, Err: __e}", name, strings.Join(callArgs, ", "), e.goType(x.pos, x.word.success))
		} else {
			body = defaultPre + fmt.Sprintf("return %s(%s)", name, strings.Join(callArgs, ", "))
		}
	} else {
		call := e.builtinCall(x, callArgs)
		if call == "" {
			nativeFence(x.pos, x.text+" partials")
		}
		if isCheckedCollectionOperation(x.text) && !x.checkedCollection {
			// the plain form of a checked operation: the closure hardens
			// the failure
			body = fmt.Sprintf("__r := %s\nif __r.Err != nil {\n	panic(__r.Err)\n}\nreturn __r.Value", call)
		} else {
			body = "return " + call
		}
	}
	return fmt.Sprintf("func(%s) %s {\n%s\n}", strings.Join(paramTypes, ", "), e.goType(x.pos, result), body)
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

// ---------------------------------------------------------------- builtins

// builtin emits the compiled builtins; anything else fences with its
// name, so the diagnostic names exactly what the program used. Checked
// collection operations in their plain form harden the failure.
func (e *nativeEmitter) builtin(x *expression) string {
	args := e.callArgs(x)
	call := e.builtinCall(x, args)
	if call == "" {
		nativeFence(x.pos, x.text)
	}
	if isCheckedCollectionOperation(x.text) && !x.checkedCollection {
		return e.hardUnwrap(call)
	}
	return call
}

// builtinCall preserves the interpreter's operation provenance on failed
// builtin Results, including calls deferred by a partial application.
func (e *nativeEmitter) builtinCall(x *expression, args []string) string {
	call := e.rawBuiltinCall(x, args)
	result := x.typ
	if _, output, ok := functionTypeParts(result); ok {
		result = output
	}
	if call != "" && (isResult(result) || isCheckedCollectionOperation(x.text)) {
		return fmt.Sprintf("zhexrt.WithOperation(%s, %q)", call, x.text)
	}
	return call
}

// rawBuiltinCall resolves one builtin to its zhexrt call, or "" when the
// name is outside the compiled vocabulary.
func (e *nativeEmitter) rawBuiltinCall(x *expression, args []string) string {
	pos := e.posVar(x.pos)
	need := func(n int) {
		if len(args) < n {
			nativeFence(x.pos, x.text)
		}
	}
	switch x.text {
	// console and input
	case "print":
		need(1)
		if x.args[0].typ == intType {
			return fmt.Sprintf("zhexrt.PrintInt(%s, %s)", pos, args[0])
		}
		if x.args[0].typ == boolType {
			return fmt.Sprintf("zhexrt.PrintBool(%s, %s)", pos, args[0])
		}
		return fmt.Sprintf("zhexrt.Print(%s, %s)", pos, args[0])
	case "read-line":
		need(1)
		return fmt.Sprintf("zhexrt.ReadLine(%s, %s)", pos, args[0])
	case "text.to-int":
		need(1)
		return fmt.Sprintf("zhexrt.ToInt(%s, %s)", pos, args[0])
	case "int.to-string":
		need(1)
		return fmt.Sprintf("zhexrt.Str(%s)", args[0])
	case "console.arg":
		need(1)
		return fmt.Sprintf("zhexrt.ArgsAt(%s, %s)", pos, args[0])
	case "console.args-count":
		return "zhexrt.ArgsCount()"
	case "console.exit":
		need(1)
		return fmt.Sprintf("zhexrt.ExitWith(%s, %s)", pos, args[0])
	case "console.fail":
		need(1)
		return fmt.Sprintf("zhexrt.FailWith(%s)", args[0])
	case "env.get":
		need(1)
		return fmt.Sprintf("zhexrt.EnvGet(%s)", args[0])
	case "env.load":
		need(1)
		return fmt.Sprintf("zhexrt.EnvLoad(%s, %s)", pos, args[0])

	// error words
	case "error.new":
		need(2)
		return fmt.Sprintf("zhexrt.ErrorNew(%s, %s, %s)", pos, args[0], args[1])
	case "error.wrap":
		need(3)
		return fmt.Sprintf("zhexrt.ErrorWrap(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "error.has-cause":
		need(1)
		return fmt.Sprintf("zhexrt.ErrorHasCause(%s)", args[0])
	case "error.cause":
		need(1)
		return fmt.Sprintf("zhexrt.ErrorCause(%s, %s)", pos, args[0])
	case "error.code":
		need(1)
		return fmt.Sprintf("zhexrt.ErrorCode(%s)", args[0])
	case "error.message":
		need(1)
		return fmt.Sprintf("zhexrt.ErrorMessage(%s)", args[0])
	case "error.operation":
		need(1)
		return fmt.Sprintf("zhexrt.ErrorOperation(%s)", args[0])
	case "error.file":
		need(1)
		return fmt.Sprintf("zhexrt.ErrorFile(%s)", args[0])
	case "error.line":
		need(1)
		return fmt.Sprintf("zhexrt.ErrorLine(%s)", args[0])
	case "error.column":
		need(1)
		return fmt.Sprintf("zhexrt.ErrorColumn(%s)", args[0])
	case "error.detail":
		need(2)
		return fmt.Sprintf("zhexrt.ErrorDetail(%s, %s)", args[0], args[1])

	// ok / err
	case "ok":
		need(1)
		return fmt.Sprintf("zhexrt.Ok(%s)", args[0])
	case "err":
		need(1)
		inner, _ := resultInner(x.typ)
		return fmt.Sprintf("zhexrt.Result[%s]{Err: %s}", e.goType(x.pos, inner), args[0])

	// text words
	case "text.reverse":
		need(1)
		return fmt.Sprintf("zhexrt.TextReverse(%s)", args[0])
	case "text.length":
		need(1)
		return fmt.Sprintf("zhexrt.TextLength(%s)", args[0])
	case "text.line-count":
		need(1)
		return fmt.Sprintf("zhexrt.TextLineCount(%s)", args[0])
	case "text.word-count":
		need(1)
		return fmt.Sprintf("zhexrt.TextWordCount(%s)", args[0])
	case "text.upper":
		need(1)
		return fmt.Sprintf("zhexrt.TextUpper(%s)", args[0])
	case "text.lower":
		need(1)
		return fmt.Sprintf("zhexrt.TextLower(%s)", args[0])
	case "text.trim":
		need(1)
		return fmt.Sprintf("zhexrt.TextTrim(%s)", args[0])
	case "text.contains":
		need(2)
		return fmt.Sprintf("zhexrt.TextContains(%s, %s)", args[0], args[1])
	case "text.index-of":
		need(2)
		return fmt.Sprintf("zhexrt.TextIndexOf(%s, %s)", args[0], args[1])
	case "text.slice":
		need(3)
		return fmt.Sprintf("zhexrt.TextSlice(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "text.replace":
		need(3)
		return fmt.Sprintf("zhexrt.TextReplace(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "text.split":
		need(2)
		return fmt.Sprintf("zhexrt.TextSplit(%s, %s, %s)", pos, args[0], args[1])
	case "text.join":
		need(2)
		return fmt.Sprintf("zhexrt.TextJoin(%s, %s)", args[0], args[1])
	case "text.to-code-points":
		need(1)
		return fmt.Sprintf("zhexrt.TextToCodePoints(%s, %s)", pos, args[0])
	case "text.from-code-points":
		need(1)
		return fmt.Sprintf("zhexrt.TextFromCodePoints(%s, %s)", pos, args[0])
	case "text.starts-with":
		need(2)
		return fmt.Sprintf("zhexrt.TextStartsWith(%s, %s)", args[0], args[1])
	case "text.ends-with":
		need(2)
		return fmt.Sprintf("zhexrt.TextEndsWith(%s, %s)", args[0], args[1])
	case "text.repeat":
		need(2)
		return fmt.Sprintf("zhexrt.TextRepeat(%s, %s, %s)", pos, args[0], args[1])
	case "text.pad-left":
		need(2)
		if len(args) == 2 {
			return fmt.Sprintf("zhexrt.TextPadLeft(%s, %s, %s, \" \")", pos, args[0], args[1])
		}
		return fmt.Sprintf("zhexrt.TextPadLeft(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "text.pad-right":
		need(2)
		if len(args) == 2 {
			return fmt.Sprintf("zhexrt.TextPadRight(%s, %s, %s, \" \")", pos, args[0], args[1])
		}
		return fmt.Sprintf("zhexrt.TextPadRight(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "text.wrap":
		need(2)
		return fmt.Sprintf("zhexrt.TextWrap(%s, %s, %s)", pos, args[0], args[1])
	case "text.insert":
		need(3)
		return fmt.Sprintf("zhexrt.TextInsert(%s, %s, %s)", args[0], args[1], args[2])
	case "text.remove":
		need(3)
		return fmt.Sprintf("zhexrt.TextRemove(%s, %s, %s)", args[0], args[1], args[2])
	case "text.sentences":
		need(1)
		return fmt.Sprintf("zhexrt.TextSentences(%s, %s)", pos, args[0])

	// list words
	case "list.new":
		_, inner, _ := collectionInner(x.typ)
		return fmt.Sprintf("&zhexrt.List[%s]{}", e.goType(x.pos, inner))
	case "list.append":
		need(2)
		return fmt.Sprintf("zhexrt.ListAppend(%s, %s, %s)", pos, args[0], args[1])
	case "list.insert":
		need(3)
		return fmt.Sprintf("zhexrt.ListInsert(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "list.length":
		need(1)
		return fmt.Sprintf("zhexrt.ListLength(%s)", args[0])
	case "list.map":
		need(2)
		return fmt.Sprintf("zhexrt.ListMap(%s, %s)", args[0], args[1])
	case "list.filter", "list.select":
		need(2)
		return fmt.Sprintf("zhexrt.ListFilter(%s, %s)", args[0], args[1])
	case "list.fold":
		need(3)
		return fmt.Sprintf("zhexrt.ListFold(%s, %s, %s)", args[0], args[1], args[2])
	case "list.find":
		need(2)
		return fmt.Sprintf("zhexrt.ListFind(%s, %s)", args[0], args[1])
	case "list.any":
		need(2)
		return fmt.Sprintf("zhexrt.ListAny(%s, %s)", args[0], args[1])
	case "list.all":
		need(2)
		return fmt.Sprintf("zhexrt.ListAll(%s, %s)", args[0], args[1])
	case "list.contains":
		need(2)
		return fmt.Sprintf("zhexrt.ListContains(%s, %s)", args[0], args[1])
	case "list.sort":
		need(1)
		if x.inputType == "List<String>" {
			return fmt.Sprintf("zhexrt.ListSortStrings(%s)", args[0])
		}
		return fmt.Sprintf("zhexrt.ListSortInts(%s)", args[0])
	case "list.sort-by":
		need(2)
		if e.sortKeyIsText(x) {
			return fmt.Sprintf("zhexrt.ListSortByString(%s, %s)", args[0], args[1])
		}
		return fmt.Sprintf("zhexrt.ListSortByInt(%s, %s)", args[0], args[1])

	// map words
	case "map.length":
		need(1)
		return fmt.Sprintf("zhexrt.MapLength(%s)", args[0])
	case "map.contains":
		need(2)
		return fmt.Sprintf("zhexrt.MapContains(%s, %s)", args[0], args[1])
	case "map.put":
		need(3)
		return fmt.Sprintf("zhexrt.MapPutChecked(%s, %q, %s, %s, %s)", pos, x.text, args[0], args[1], args[2])
	case "map.remove":
		need(2)
		return fmt.Sprintf("zhexrt.MapRemove(%s, %q, %s, %s)", pos, x.text, args[0], args[1])
	case "map.into":
		need(2)
		return fmt.Sprintf("zhexrt.MapInto(%s, %s, %s)", pos, args[0], args[1])
	case "map.keys":
		need(1)
		return fmt.Sprintf("zhexrt.MapKeys(%s, %q, %s)", pos, x.text, args[0])
	case "map.values":
		need(1)
		return fmt.Sprintf("zhexrt.MapValues(%s, %q, %s)", pos, x.text, args[0])

	// array words
	case "array.new":
		need(2)
		return fmt.Sprintf("zhexrt.ArrayNew(%s, %s, %s)", pos, args[0], args[1])
	case "array.resize":
		need(3)
		return fmt.Sprintf("zhexrt.ArrayResize(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "array.shape":
		need(1)
		return fmt.Sprintf("zhexrt.ArrayShape(%s)", args[0])
	case "array.rank":
		need(1)
		return fmt.Sprintf("zhexrt.ArrayRank(%s)", args[0])
	case "array.select":
		need(2)
		return fmt.Sprintf("zhexrt.ArraySelect(%s, %s, %s)", pos, args[0], args[1])
	case "array.length":
		need(1)
		return fmt.Sprintf("zhexrt.ArrayLength(%s)", args[0])

	// buffer words
	case "buffer.new":
		return "zhexrt.BufferNew()"
	case "buffer.add-line":
		need(2)
		return fmt.Sprintf("zhexrt.BufferAddLine(%s, %s)", args[0], args[1])
	case "buffer.line-count":
		need(1)
		return fmt.Sprintf("zhexrt.BufferLineCount(%s)", args[0])
	case "buffer.line":
		need(2)
		return fmt.Sprintf("zhexrt.BufferLine(%s, %s, %s)", pos, args[0], args[1])
	case "buffer.set-line":
		need(3)
		return fmt.Sprintf("zhexrt.BufferSetLine(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "buffer.to-text":
		need(1)
		return fmt.Sprintf("zhexrt.BufferToText(%s)", args[0])

	// file words
	case "file.read":
		need(1)
		return fmt.Sprintf("zhexrt.FileRead(%s, %s)", pos, args[0])
	case "file.open":
		need(1)
		return fmt.Sprintf("zhexrt.FileOpen(%s, %s)", pos, args[0])
	case "file.read-line":
		need(1)
		return fmt.Sprintf("zhexrt.FileReadLineOf(%s, %s)", pos, args[0])
	case "file.close":
		need(1)
		return fmt.Sprintf("zhexrt.FileClose(%s)", args[0])
	case "file.write":
		need(2)
		if len(args) == 2 {
			return fmt.Sprintf("zhexrt.FileWrite(%s, %s, %s, false)", pos, args[0], args[1])
		}
		return fmt.Sprintf("zhexrt.FileWrite(%s, %s, %s, %s)", pos, args[0], args[1], args[2])
	case "file.write-bytes":
		need(2)
		return fmt.Sprintf("zhexrt.FileWriteBytes(%s, %s, %s)", pos, args[0], args[1])
	case "file.list":
		need(1)
		return fmt.Sprintf("zhexrt.FileList(%s, %s)", pos, args[0])
	case "file.exists":
		need(1)
		return fmt.Sprintf("zhexrt.FileExists(%s, %s)", pos, args[0])
	case "file.size":
		need(1)
		return fmt.Sprintf("zhexrt.FileSize(%s, %s)", pos, args[0])
	case "file.join":
		need(2)
		return fmt.Sprintf("zhexrt.FileJoin(%s, %s)", args[0], args[1])
	case "file.base":
		need(1)
		return fmt.Sprintf("zhexrt.FileBase(%s)", args[0])
	case "file.extension":
		need(1)
		return fmt.Sprintf("zhexrt.FileExtension(%s)", args[0])
	case "file.directory-of":
		need(1)
		return fmt.Sprintf("zhexrt.FileDirectoryOf(%s)", args[0])
	case "file.make-folder":
		need(1)
		if len(args) == 1 {
			return fmt.Sprintf("zhexrt.FileMakeFolder(%s, %s, false)", pos, args[0])
		}
		return fmt.Sprintf("zhexrt.FileMakeFolder(%s, %s, %s)", pos, args[0], args[1])
	case "file.delete":
		need(1)
		return fmt.Sprintf("zhexrt.FileDelete(%s, %s)", pos, args[0])
	case "file.delete-folder":
		need(1)
		if len(args) == 1 {
			return fmt.Sprintf("zhexrt.FileDeleteFolder(%s, %s, false)", pos, args[0])
		}
		return fmt.Sprintf("zhexrt.FileDeleteFolder(%s, %s, %s)", pos, args[0], args[1])
	case "file.move":
		need(2)
		if len(args) == 2 {
			return fmt.Sprintf("zhexrt.FileMove(%s, %s, %s, false)", pos, args[0], args[1])
		}
		return fmt.Sprintf("zhexrt.FileMove(%s, %s, %s, %s)", pos, args[0], args[1], args[2])

	// json tree words
	case "json.parse":
		need(1)
		return fmt.Sprintf("zhexrt.JsonParse(%s, %s)", pos, args[0])
	case "json.encode":
		need(1)
		if x.jsonSchema == nil {
			return fmt.Sprintf("zhexrt.JsonEncode(%s, %s)", pos, args[0])
		}
	}
	return ""
}

// sortKeyIsText reports whether sort-by's key function returns String.
func (e *nativeEmitter) sortKeyIsText(x *expression) bool {
	if len(x.args) >= 2 {
		if _, result, ok := functionTypeParts(x.args[1].typ); ok {
			return result == stringType
		}
	}
	return false
}
