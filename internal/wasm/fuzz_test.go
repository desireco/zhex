package wasm

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"zhex/internal/lang"
)

// Deterministic fuzz differential (DESIGN-M4 §7): 50 generated in-subset
// programs per run, each compared interpreter-vs-compiled (browser skin) on
// stdout bytes and exit status. The interpreter is the oracle — expected
// output is derived, never stored. Determinism rules: one
// rand.New(rand.NewSource(base+i)) per program, never the auto-seeded
// global rand; no time or os entropy; the generator iterates slices only
// (Go map iteration order must not reach the output).

const (
	fuzzProgramCount  = 50
	fuzzSeedBase      = 20261115
	fuzzMaxStatements = 60
	fuzzMaxSource     = 8 << 10
)

// fuzzAtoms build string literals of at most 12 code points over ASCII,
// multibyte, NBSP, and escapes (DESIGN-M4 §7 bounds). Raw NBSP inside a
// string literal is legal Zhex; only outside strings is it a source error.
var fuzzAtoms = []string{"a", "b", "abc", "Zx", " pad ", "é", "😀", " ", "line", "42", "q", "x y", "a\\\"b", "n\\n", "e"}

// fuzzTrioProbe names the three M4 trio constructs in one witness: a
// re-partial over a closure binding, a Unit-result closure, and a closure
// binding as a flow step. When the compiled target accepts it, the fuzz
// grammar widens automatically (DESIGN-M4 §3, §7).
const fuzzTrioProbe = `word fuzz-add(a: Int, b: Int) -> Int
    a + b
let g: (Int, Int) -> Int = fuzz-add(_, _)
let h: (Int) -> Int = g(_, 2)
let show: (Int) -> Unit = print _
show h(5)
flow 7
    h
    show
`

var fuzzTrioAvailable = sync.OnceValue(func() bool {
	dir, err := os.MkdirTemp("", "zhex-fuzz-trio-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)
	return Compile("trio-probe.zhex", fuzzTrioProbe, filepath.Join(dir, "probe.wasm"), nil, lang.WatSkinBrowser) == nil
})

// fuzzBinding is one named value the generator knows the type of. nonempty
// marks list bindings known to hold at least one element (safe index reads);
// assignable marks var bindings (later name = expr statements are legal).
type fuzzBinding struct {
	name       string
	typ        string
	nonempty   bool
	assignable bool
}

// fuzzWord records a generated word's signature so call sites can pass
// literals of matching types.
type fuzzWord struct {
	name   string
	params []string
	result string
}

type fuzzGen struct {
	rng    *rand.Rand
	lines  []string
	binds  []fuzzBinding
	words  []fuzzWord
	local  []fuzzBinding // word parameters during body generation
	names  int
	stmts  int
	hasFib bool
	hasGcd bool
	trio   bool
}

func (g *fuzzGen) fresh() string {
	name := fmt.Sprintf("v%d", g.names)
	g.names++
	return name
}

func (g *fuzzGen) emit(s string) { g.lines = append(g.lines, s) }

// addStmt accounts n top-level statements against the budget.
func (g *fuzzGen) addStmt(n int) bool {
	if g.stmts+n > fuzzMaxStatements {
		return false
	}
	g.stmts += n
	return true
}

func (g *fuzzGen) bind(b fuzzBinding) { g.binds = append(g.binds, b) }

// namesOf collects bindings of typ from the word-local scope and the main
// scope, in emission order (slices keep the generator deterministic).
func (g *fuzzGen) namesOf(typ string) []string {
	var names []string
	for _, pool := range [][]fuzzBinding{g.local, g.binds} {
		for _, b := range pool {
			if b.typ == typ {
				names = append(names, b.name)
			}
		}
	}
	return names
}

func (g *fuzzGen) pick(typ string) string {
	if names := g.namesOf(typ); len(names) > 0 {
		return names[g.rng.Intn(len(names))]
	}
	return ""
}

// listNames returns list bindings of the element type; wantNonempty selects
// the index-safe ones.
func (g *fuzzGen) listNames(elem string, wantNonempty bool) []string {
	typ := "List<" + elem + ">"
	var names []string
	for _, b := range g.binds {
		if b.typ == typ && (!wantNonempty || b.nonempty) {
			names = append(names, b.name)
		}
	}
	return names
}

func (g *fuzzGen) pickList(elem string, wantNonempty bool) string {
	if names := g.listNames(elem, wantNonempty); len(names) > 0 {
		return names[g.rng.Intn(len(names))]
	}
	return ""
}

func (g *fuzzGen) maybe(p int) bool { return g.rng.Intn(100) < p }

// intLit mostly stays inside |v| < 2^62 with occasional Min/MaxInt64 probes
// (arithmetic on them may trap — the differential covers the trap path).
func (g *fuzzGen) intLit() string {
	switch g.rng.Intn(20) {
	case 0:
		return "9223372036854775807"
	case 1:
		return "-9223372036854775808"
	case 2:
		return fmt.Sprint(-(g.rng.Intn(1 << 61)))
	default:
		return fmt.Sprint(g.rng.Intn(1 << 40))
	}
}

func (g *fuzzGen) smallLit() string { return fmt.Sprint(g.rng.Intn(30)) }

// posLit is a non-negative literal: short-form call arguments cannot start
// with "-" (the parser reads name-minus-expression), so captured arguments
// in `word lit, _` partials draw from this.
func (g *fuzzGen) posLit() string { return fmt.Sprint(g.rng.Intn(1 << 40)) }

func (g *fuzzGen) digitsLit() string { return fmt.Sprint(g.rng.Intn(100000)) }

func (g *fuzzGen) strLit() string {
	parts := make([]string, 1+g.rng.Intn(3))
	for i := range parts {
		parts[i] = fuzzAtoms[g.rng.Intn(len(fuzzAtoms))]
	}
	s := strings.Join(parts, "")
	for len(parts) > 1 && utf8.RuneCountInString(s) > 12 {
		parts = parts[:len(parts)-1]
		s = strings.Join(parts, "")
	}
	return `"` + s + `"`
}

// mapKey draws keys from the small pool the generator also uses for map
// literals, so lookups hit both Some and None paths.
var fuzzMapKeys = []string{"ada", "grace", "lin", "nancy", "iris", "ghost"}

func (g *fuzzGen) mapKey() string { return fuzzMapKeys[g.rng.Intn(len(fuzzMapKeys))] }

func (g *fuzzGen) intOp() string {
	return []string{"+", "-", "*", "/", "%"}[g.rng.Intn(5)]
}

func (g *fuzzGen) cmpOp() string {
	return []string{"<", "<=", ">", ">="}[g.rng.Intn(4)]
}

func (g *fuzzGen) exprInt(d int) string {
	if d <= 0 {
		if name := g.pick("Int"); name != "" && g.maybe(60) {
			return name
		}
		return g.intLit()
	}
	for try := 0; try < 8; try++ {
		switch g.rng.Intn(14) {
		case 0:
			return g.intLit()
		case 1:
			if n := g.pick("Int"); n != "" {
				return n
			}
		case 2:
			return g.exprInt(d-1) + " " + g.intOp() + " " + g.exprInt(d-1)
		case 3:
			return "text.length(" + g.exprStr(d-1) + ")"
		case 4:
			return "text.line-count(" + g.exprStr(d-1) + ")"
		case 5:
			return "text.word-count(" + g.exprStr(d-1) + ")"
		case 6:
			return "text.index-of(" + g.exprStr(d-1) + ", " + g.strLit() + ")"
		case 7:
			if n := g.pick("List<Int>"); n != "" {
				return "list.length(" + n + ")"
			}
			if n := g.pick("List<String>"); n != "" {
				return "list.length(" + n + ")"
			}
		case 8:
			return "fuzz-double(" + g.exprInt(d-1) + ")"
		case 9:
			return "fuzz-add(" + g.exprInt(d-1) + ", " + g.exprInt(d-1) + ")"
		case 10:
			return "fuzz-key(" + g.exprStr(d-1) + ")"
		case 11:
			if g.hasFib {
				return "fib(" + g.smallLit() + ")"
			}
		case 12:
			if g.hasGcd {
				return "fuzz-gcd(" + g.smallLit() + ", " + g.smallLit() + ")"
			}
		case 13:
			return "fuzz-shape-area(Shape." + []string{"Circle", "Square"}[g.rng.Intn(2)] + "(" + g.exprInt(d-1) + "))"
		}
	}
	return g.intLit()
}

func (g *fuzzGen) exprStr(d int) string {
	if d <= 0 {
		if name := g.pick("String"); name != "" && g.maybe(60) {
			return name
		}
		return g.strLit()
	}
	for try := 0; try < 8; try++ {
		switch g.rng.Intn(12) {
		case 0:
			return g.strLit()
		case 1:
			if n := g.pick("String"); n != "" {
				return n
			}
		case 2:
			return g.exprStr(d-1) + " + " + g.exprStr(d-1)
		case 3:
			// string-context Int exception: String + Int concatenates
			return g.exprStr(d-1) + " + " + g.exprInt(d-1)
		case 4:
			return "text.upper(" + g.exprStr(d-1) + ")"
		case 5:
			return "text.lower(" + g.exprStr(d-1) + ")"
		case 6:
			return "text.reverse(" + g.exprStr(d-1) + ")"
		case 7:
			return "text.trim(" + g.exprStr(d-1) + ")"
		case 8:
			return "int.to-string(" + g.exprInt(d-1) + ")"
		case 9:
			return "text.slice(" + g.exprStr(d-1) + ", " + g.smallLit() + ", " + g.smallLit() + ")"
		case 10:
			return "text.replace(" + g.exprStr(d-1) + ", \"a\", \"o\")"
		case 11:
			if n := g.pickList("String", false); n != "" {
				return "text.join(" + n + ", \",\")"
			}
		}
	}
	return g.strLit()
}

func (g *fuzzGen) exprBool(d int) string {
	if d <= 0 {
		if name := g.pick("Bool"); name != "" && g.maybe(60) {
			return name
		}
		return []string{"true", "false"}[g.rng.Intn(2)]
	}
	for try := 0; try < 8; try++ {
		switch g.rng.Intn(13) {
		case 0:
			return []string{"true", "false"}[g.rng.Intn(2)]
		case 1:
			if n := g.pick("Bool"); n != "" {
				return n
			}
		case 2:
			return g.exprInt(d-1) + " " + g.cmpOp() + " " + g.exprInt(d-1)
		case 3:
			return g.exprInt(d-1) + " == " + g.exprInt(d-1)
		case 4:
			return g.exprStr(d-1) + " != " + g.exprStr(d-1)
		case 5:
			return g.exprBool(d-1) + " and " + g.exprBool(d-1)
		case 6:
			return g.exprBool(d-1) + " or " + g.exprBool(d-1)
		case 7:
			return "not " + g.exprBool(d-1)
		case 8:
			return "text.contains(" + g.exprStr(d-1) + ", " + g.strLit() + ")"
		case 9:
			return "text.starts-with(" + g.exprStr(d-1) + ", " + g.strLit() + ")"
		case 10:
			return "text.ends-with(" + g.exprStr(d-1) + ", " + g.strLit() + ")"
		case 11:
			return "fuzz-keep(" + g.exprInt(d-1) + ")"
		case 12:
			if n := g.pickList("Int", false); n != "" {
				return "list." + []string{"any", "all"}[g.rng.Intn(2)] + "(" + n + ", fuzz-keep(_))"
			}
		}
	}
	return "true"
}

// ---------------------------------------------------------------------------
// statement templates — each returns false when its prerequisites are
// missing (the driver retries); all stay type-correct by construction
// ---------------------------------------------------------------------------

func (g *fuzzGen) tPrint() bool {
	if !g.addStmt(1) {
		return false
	}
	if g.maybe(50) {
		g.emit("print " + g.exprStr(1))
	} else {
		g.emit("print " + g.exprInt(1))
	}
	return true
}

func (g *fuzzGen) tLet() bool {
	typ := []string{"Int", "String", "Bool"}[g.rng.Intn(3)]
	if !g.addStmt(1) {
		return false
	}
	name := g.fresh()
	switch typ {
	case "Int":
		g.emit("let " + name + " = " + g.exprInt(2))
	case "String":
		g.emit("let " + name + " = " + g.exprStr(2))
	default:
		g.emit("let " + name + " = " + g.exprBool(2))
	}
	g.bind(fuzzBinding{name: name, typ: typ})
	return true
}

func (g *fuzzGen) tVarAssign() bool {
	if !g.addStmt(1) {
		return false
	}
	typ := []string{"Int", "String", "Bool"}[g.rng.Intn(3)]
	name := g.fresh()
	var init string
	switch typ {
	case "Int":
		init = g.exprInt(1)
	case "String":
		init = g.exprStr(1)
	default:
		init = g.exprBool(1)
	}
	g.emit("var " + name + " = " + init)
	g.bind(fuzzBinding{name: name, typ: typ, assignable: true})
	if g.maybe(60) {
		if !g.addStmt(1) {
			return true
		}
		switch typ {
		case "Int":
			g.emit(name + " = " + g.exprInt(1))
		case "String":
			g.emit(name + " = " + g.exprStr(1))
		default:
			g.emit(name + " = " + g.exprBool(1))
		}
	}
	return true
}

func (g *fuzzGen) tIfPrint() bool {
	if !g.addStmt(1) {
		return false
	}
	g.emit("if " + g.exprBool(1))
	g.emit("    print " + g.exprStr(1))
	if g.maybe(50) {
		g.emit("elif " + g.exprBool(1))
		g.emit("    print " + g.exprInt(1))
	}
	g.emit("else")
	g.emit("    print " + g.exprStr(1))
	return true
}

func (g *fuzzGen) tIfBind() bool {
	if !g.addStmt(1) {
		return false
	}
	typ := []string{"Int", "String"}[g.rng.Intn(2)]
	name := g.fresh()
	value := func() string {
		if typ == "Int" {
			return g.exprInt(1)
		}
		return g.exprStr(1)
	}
	g.emit("let " + name + " = if " + g.exprBool(1))
	g.emit("    " + value())
	if g.maybe(40) {
		g.emit("elif " + g.exprBool(1))
		g.emit("    " + value())
	}
	g.emit("else")
	g.emit("    " + value())
	g.bind(fuzzBinding{name: name, typ: typ})
	return true
}

func (g *fuzzGen) tMatchChoice() bool {
	if !g.addStmt(2) {
		return false
	}
	shape := g.fresh()
	alt := []string{"Circle", "Square", "Label"}[g.rng.Intn(3)]
	arg := g.exprInt(1)
	if alt == "Label" {
		arg = g.exprStr(1)
	}
	g.emit("let " + shape + " = Shape." + alt + "(" + arg + ")")
	subject := "match " + shape
	if g.maybe(50) {
		if !g.addStmt(1) {
			return true
		}
		g.emit(subject)
		g.emit("    case Shape.Circle(radius)")
		g.emit("        print \"circle \" + int.to-string(radius)")
		g.emit("    case Shape.Square(side)")
		g.emit("        print \"square \" + int.to-string(side)")
		g.emit("    case Shape.Label(text)")
		g.emit("        print \"label \" + text")
		return true
	}
	name := g.fresh()
	g.emit("let " + name + " = " + subject)
	g.emit("    case Shape.Circle(radius)")
	g.emit("        radius * 2")
	g.emit("    case Shape.Square(side)")
	g.emit("        side + 1")
	g.emit("    case Shape.Label(text)")
	g.emit("        text.length(text)")
	g.emit("print int.to-string(" + name + ")")
	g.bind(fuzzBinding{name: name, typ: "Int"})
	return true
}

func (g *fuzzGen) tOptionMap() bool {
	m := g.pick("Map<String, Int>")
	if m == "" || !g.addStmt(1) {
		return false
	}
	key := g.mapKey()
	if g.maybe(50) {
		g.emit("match " + m + "[\"" + key + "\"]")
		g.emit("    case Option.Some(score)")
		g.emit("        print \"" + key + ": \" + int.to-string(score)")
		g.emit("    case Option.None")
		g.emit("        print \"" + key + ": none\"")
		return true
	}
	name := g.fresh()
	g.emit("let " + name + " = match " + m + "[\"" + key + "\"]")
	g.emit("    case Option.Some(score)")
	g.emit("        score + 1")
	g.emit("    case Option.None")
	g.emit("        0")
	g.emit("print int.to-string(" + name + ")")
	g.bind(fuzzBinding{name: name, typ: "Int"})
	return true
}

func (g *fuzzGen) tOptionFind() bool {
	xs := g.pickList("Int", false)
	if xs == "" || !g.addStmt(1) {
		return false
	}
	g.emit("match list.find(" + xs + ", fuzz-keep(_))")
	g.emit("    case Option.Some(first)")
	g.emit("        print \"first big: \" + int.to-string(first)")
	g.emit("    case Option.None")
	g.emit("        print \"none big\"")
	return true
}

func (g *fuzzGen) tWhile() bool {
	if !g.addStmt(2) {
		return false
	}
	c := g.fresh()
	n := 1 + g.rng.Intn(20)
	g.emit("var " + c + " = " + fmt.Sprint(n))
	g.emit("while " + c + " > 0")
	if g.maybe(50) {
		g.emit("    print int.to-string(" + c + ")")
	}
	if v := g.pickVar("Int"); v != "" && g.maybe(50) {
		g.emit("    " + v + " = " + v + " + " + c)
	}
	g.emit("    " + c + " = " + c + " - 1")
	return true
}

func (g *fuzzGen) pickVar(typ string) string {
	for _, b := range g.binds {
		if b.typ == typ && b.assignable {
			return b.name
		}
	}
	return ""
}

func (g *fuzzGen) tForRange() bool {
	if !g.addStmt(1) {
		return false
	}
	i := g.fresh()
	a, b := g.rng.Intn(101), g.rng.Intn(101)
	g.emit("for " + i + " in range(" + fmt.Sprint(a) + ", " + fmt.Sprint(b) + ")")
	if v := g.pickVar("Int"); v != "" && g.maybe(50) {
		g.emit("    " + v + " = " + v + " + " + i)
	} else {
		g.emit("    print int.to-string(" + i + ")")
	}
	return true
}

func (g *fuzzGen) tForList() bool {
	xs := g.pickList("Int", false)
	if xs == "" || !g.addStmt(1) {
		return false
	}
	x := g.fresh()
	g.emit("for " + x + " in " + xs)
	if v := g.pickVar("Int"); v != "" && g.maybe(50) {
		g.emit("    " + v + " = " + v + " + " + x)
	} else {
		g.emit("    print int.to-string(" + x + ")")
	}
	return true
}

func (g *fuzzGen) tForListStr() bool {
	xs := g.pickList("String", false)
	if xs == "" || !g.addStmt(1) {
		return false
	}
	x := g.fresh()
	g.emit("for " + x + " in " + xs)
	g.emit("    print text.upper(" + x + ")")
	return true
}

func (g *fuzzGen) tForMap() bool {
	m := g.pick("Map<String, Int>")
	if m == "" || !g.addStmt(1) {
		return false
	}
	k, v := g.fresh(), g.fresh()
	g.emit("for " + k + ", " + v + " in " + m)
	g.emit("    print " + k + " + \": \" + int.to-string(" + v + ")")
	return true
}

func (g *fuzzGen) tListLiteral() bool {
	if !g.addStmt(1) {
		return false
	}
	name := g.fresh()
	if g.maybe(50) {
		n := 1 + g.rng.Intn(4)
		elems := make([]string, n)
		for i := range elems {
			elems[i] = g.intLit()
		}
		g.emit("let " + name + ": List<Int> = [" + strings.Join(elems, ", ") + "]")
		g.bind(fuzzBinding{name: name, typ: "List<Int>", nonempty: true})
	} else {
		n := 1 + g.rng.Intn(3)
		elems := make([]string, n)
		for i := range elems {
			elems[i] = g.strLit()
		}
		g.emit("let " + name + ": List<String> = [" + strings.Join(elems, ", ") + "]")
		g.bind(fuzzBinding{name: name, typ: "List<String>", nonempty: true})
	}
	return true
}

func (g *fuzzGen) tAppend() bool {
	elem := []string{"Int", "String"}[g.rng.Intn(2)]
	xs := g.pickList(elem, false)
	if xs == "" || !g.addStmt(1) {
		return false
	}
	if elem == "Int" {
		g.emit("list.append(" + xs + ", " + g.exprInt(1) + ")")
	} else {
		g.emit("list.append(" + xs + ", " + g.exprStr(1) + ")")
	}
	// append mutates in place; the binding now holds at least one element
	for i := range g.binds {
		if g.binds[i].name == xs {
			g.binds[i].nonempty = true
		}
	}
	return true
}

func (g *fuzzGen) tIndexRead() bool {
	xs := g.pickList("Int", true)
	if xs == "" || !g.addStmt(1) {
		return false
	}
	name := g.fresh()
	g.emit("let " + name + " = " + xs + "[0]")
	g.bind(fuzzBinding{name: name, typ: "Int"})
	return true
}

func (g *fuzzGen) tListOps() bool {
	xi, xs := g.pickList("Int", false), g.pickList("String", false)
	if xi == "" && xs == "" {
		return false
	}
	if !g.addStmt(1) {
		return false
	}
	name := g.fresh()
	which := g.rng.Intn(7)
	switch {
	case which == 0 && xi != "":
		g.emit("let " + name + " = list.map(" + xi + ", fuzz-double(_))")
		g.bind(fuzzBinding{name: name, typ: "List<Int>"})
	case which == 1 && xs != "":
		g.emit("let " + name + " = list.map(" + xs + ", fuzz-up(_))")
		g.bind(fuzzBinding{name: name, typ: "List<String>"})
	case which == 2 && xi != "":
		g.emit("let " + name + " = list.filter(" + xi + ", fuzz-keep(_))")
		g.bind(fuzzBinding{name: name, typ: "List<Int>"})
	case which == 3 && xi != "":
		g.emit("let " + name + " = list.fold(" + xi + ", 0, fuzz-add(_, _))")
		g.bind(fuzzBinding{name: name, typ: "Int"})
	case which == 4 && xs != "":
		g.emit("let " + name + " = list.sort(" + xs + ")")
		g.bind(fuzzBinding{name: name, typ: "List<String>"})
	case which == 5 && xs != "":
		g.emit("let " + name + " = list.sort-by(" + xs + ", fuzz-key(_))")
		g.bind(fuzzBinding{name: name, typ: "List<String>"})
	case which == 6 && xi != "":
		g.emit("let " + name + " = list.length(" + xi + ")")
		g.bind(fuzzBinding{name: name, typ: "Int"})
	default:
		if xi != "" {
			g.emit("let " + name + " = list.length(" + xi + ")")
			g.bind(fuzzBinding{name: name, typ: "Int"})
		} else {
			g.emit("let " + name + " = list.length(" + xs + ")")
			g.bind(fuzzBinding{name: name, typ: "Int"})
		}
	}
	return true
}

func (g *fuzzGen) tMapLiteral() bool {
	if !g.addStmt(1) {
		return false
	}
	name := g.fresh()
	n := 1 + g.rng.Intn(3)
	entries := make([]string, n)
	used := map[string]bool{}
	for i := range entries {
		key := g.mapKey()
		for used[key] {
			key = g.mapKey()
		}
		used[key] = true
		entries[i] = "\"" + key + "\": " + g.intLit()
	}
	g.emit("let " + name + ": Map<String, Int> = {" + strings.Join(entries, ", ") + "}")
	g.bind(fuzzBinding{name: name, typ: "Map<String, Int>"})
	return true
}

func (g *fuzzGen) tMapOps() bool {
	m := g.pick("Map<String, Int>")
	if m == "" || !g.addStmt(1) {
		return false
	}
	switch g.rng.Intn(4) {
	case 0:
		name := g.fresh()
		g.emit("let " + name + " = map.put(" + m + ", \"" + g.mapKey() + "\", " + g.exprInt(1) + ")")
		g.bind(fuzzBinding{name: name, typ: "Map<String, Int>"})
	case 1:
		name := g.fresh()
		g.emit("let " + name + " = map.length(" + m + ")")
		g.bind(fuzzBinding{name: name, typ: "Int"})
	case 2:
		g.emit("map.remove(" + m + ", \"" + g.mapKey() + "\")")
	case 3:
		name := g.fresh()
		g.emit("let " + name + " = map.contains(" + m + ", \"" + g.mapKey() + "\")")
		g.bind(fuzzBinding{name: name, typ: "Bool"})
	}
	return true
}

func (g *fuzzGen) tText() bool {
	if !g.addStmt(1) {
		return false
	}
	name := g.fresh()
	switch g.rng.Intn(4) {
	case 0:
		g.emit("let " + name + " = text.split(" + g.exprStr(1) + ", \"a\")")
		// split always yields at least one part
		g.bind(fuzzBinding{name: name, typ: "List<String>", nonempty: true})
	case 1:
		g.emit("let " + name + " = text.reverse(" + g.exprStr(1) + ")")
		g.bind(fuzzBinding{name: name, typ: "String"})
	case 2:
		g.emit("let " + name + " = text.word-count(" + g.exprStr(1) + ")")
		g.bind(fuzzBinding{name: name, typ: "Int"})
	default:
		g.emit("let " + name + " = text.trim(" + g.exprStr(1) + ")")
		g.bind(fuzzBinding{name: name, typ: "String"})
	}
	return true
}

func (g *fuzzGen) tToInt() bool {
	if !g.addStmt(1) {
		return false
	}
	name := g.fresh()
	switch g.rng.Intn(3) {
	case 0:
		// try only on guaranteed digits: a top-level try failure ends the run
		g.emit("let " + name + " = text.to-int(\"" + g.digitsLit() + "\") try")
	case 1:
		g.emit("let " + name + " = text.to-int(" + g.exprStr(1) + ") catch 0")
	default:
		g.emit("let " + name + " = text.to-int(" + g.exprStr(1) + ") catch")
		g.emit("    print \"fallback\"")
		g.emit("    0")
	}
	g.bind(fuzzBinding{name: name, typ: "Int"})
	return true
}

func (g *fuzzGen) tFlowStmt() bool {
	if !g.addStmt(1) {
		return false
	}
	switch g.rng.Intn(5) {
	case 0:
		g.emit("flow " + g.exprInt(1))
		g.emit("    fuzz-double")
		g.emit("    int.to-string")
		g.emit("    print")
	case 1:
		g.emit("flow " + g.exprStr(1))
		g.emit("    text.trim")
		g.emit("    text.upper")
		g.emit("    print")
	case 2:
		// fallible step on a guaranteed-digit seed
		g.emit("flow \"" + g.digitsLit() + "\"")
		g.emit("    text.to-int try")
		g.emit("    print")
	case 3:
		// computed seed
		g.emit("flow fuzz-shape-area(Shape.Square(" + g.smallLit() + "))")
		g.emit("    int.to-string")
		g.emit("    print")
	case 4:
		if xs := g.pickList("String", false); xs != "" {
			g.emit("flow " + xs)
			g.emit("    text.join(\"/\")")
			g.emit("    print")
		} else {
			g.emit("flow " + g.exprInt(1))
			g.emit("    int.to-string")
			g.emit("    print")
		}
	}
	return true
}

func (g *fuzzGen) tFlowBind() bool {
	if !g.addStmt(1) {
		return false
	}
	name := g.fresh()
	g.emit("let " + name + " = flow " + g.exprStr(1))
	g.emit("    text.trim")
	g.emit("    text.replace(\"a\", \"o\")")
	g.bind(fuzzBinding{name: name, typ: "String"})
	return true
}

func (g *fuzzGen) tClosure() bool {
	if !g.addStmt(2) {
		return false
	}
	switch g.rng.Intn(3) {
	case 0:
		p := g.fresh()
		g.emit("let " + p + " = fuzz-add _, " + g.posLit())
		g.bind(fuzzBinding{name: p, typ: "(Int) -> Int"})
		g.emit("print int.to-string(" + p + "(" + g.smallLit() + "))")
	case 1:
		u := g.fresh()
		g.emit("let " + u + " = text.upper(_)")
		g.bind(fuzzBinding{name: u, typ: "(String) -> String"})
		g.emit("print " + u + "(" + g.exprStr(1) + ")")
	default:
		r := g.fresh()
		g.emit("let " + r + ": (Int) -> Int = fuzz-add " + g.posLit() + ", _")
		g.bind(fuzzBinding{name: r, typ: "(Int) -> Int"})
		g.emit("print int.to-string(" + r + "(" + g.smallLit() + "))")
	}
	return true
}

func (g *fuzzGen) tCallWord() bool {
	if len(g.words) == 0 || !g.addStmt(1) {
		return false
	}
	w := g.words[g.rng.Intn(len(g.words))]
	args := make([]string, len(w.params))
	for i, p := range w.params {
		switch p {
		case "Int":
			args[i] = g.smallLit()
		case "String":
			args[i] = g.strLit()
		default:
			args[i] = []string{"true", "false"}[g.rng.Intn(2)]
		}
	}
	call := w.name + "(" + strings.Join(args, ", ") + ")"
	switch w.result {
	case "Int":
		g.emit("print int.to-string(" + call + ")")
	case "String":
		g.emit("print " + call)
	default:
		g.emit("if " + call)
		g.emit("    print \"yes\"")
		g.emit("else")
		g.emit("    print \"no\"")
	}
	return true
}

// tTrio emits the M4 trio shapes (DESIGN-M4 §3): Unit-result closures,
// re-partials over closure bindings, and closure-valued flow steps. Only
// offered when the compiled target accepts the probe.
func (g *fuzzGen) tTrio() bool {
	if !g.trio || !g.addStmt(2) {
		return false
	}
	switch g.rng.Intn(3) {
	case 0:
		s := g.fresh()
		g.emit("let " + s + ": (Int) -> Unit = print _")
		g.bind(fuzzBinding{name: s, typ: "(Int) -> Unit"})
		g.emit(s + " " + g.smallLit())
	case 1:
		gp := g.fresh()
		g.emit("let " + gp + ": (Int, Int) -> Int = fuzz-add _, _")
		g.bind(fuzzBinding{name: gp, typ: "(Int, Int) -> Int"})
		h := g.fresh()
		if g.maybe(50) {
			g.emit("let " + h + " = " + gp + " _, " + g.smallLit())
		} else {
			g.emit("let " + h + " = " + gp + " " + g.smallLit() + ", _")
		}
		g.bind(fuzzBinding{name: h, typ: "(Int) -> Int"})
		g.emit("print int.to-string(" + h + "(" + g.smallLit() + "))")
	default:
		p := g.pick("(Int) -> Int")
		if p == "" {
			return false
		}
		g.emit("flow " + g.exprInt(1))
		g.emit("    " + p)
		g.emit("    int.to-string")
		g.emit("    print")
	}
	return true
}

// tUnitFlow seeds a flow with a Unit-result closure as its final step.
func (g *fuzzGen) tUnitFlow() bool {
	if !g.trio {
		return false
	}
	s := g.pick("(Int) -> Unit")
	if s == "" || !g.addStmt(1) {
		return false
	}
	g.emit("flow " + g.exprInt(1))
	g.emit("    " + s)
	return true
}

// fuzzGenerate builds one complete in-subset program from seed
// fuzzSeedBase+i (DESIGN-M4 §7): fixed types and callback words, 1-8
// randomly signed words, a typed prologue, guaranteed shape/Option/closure
// coverage, then random template fill under the statement budget.
func fuzzGenerate(i int) string {
	g := &fuzzGen{
		rng:  rand.New(rand.NewSource(fuzzSeedBase + int64(i))),
		trio: fuzzTrioAvailable(),
	}
	g.emit("# fuzz program " + fmt.Sprint(fuzzSeedBase+i))
	g.emit("type Shape = choice")
	g.emit("    Circle(radius: Int)")
	g.emit("    Square(side: Int)")
	g.emit("    Label(text: String)")
	g.emit("type Names = alias List<String>")
	g.emit("word fuzz-double(n: Int) -> Int")
	g.emit("    n * 2")
	g.emit("word fuzz-up(s: String) -> String")
	g.emit("    text.upper(s)")
	g.emit("word fuzz-key(s: String) -> Int")
	g.emit("    text.length(s)")
	g.emit("word fuzz-keep(n: Int) -> Bool")
	g.emit("    n > 10")
	g.emit("word fuzz-add(a: Int, b: Int) -> Int")
	g.emit("    a + b")
	g.emit("word fuzz-shape-area(shape: Shape) -> Int")
	g.emit("    match shape")
	g.emit("        case Shape.Circle(radius)")
	g.emit("            radius * 3")
	g.emit("        case Shape.Square(side)")
	g.emit("            side * side")
	g.emit("        case Shape.Label(text)")
	g.emit("            text.length(text)")
	g.hasFib = g.maybe(50)
	if g.hasFib {
		g.emit("word fib(n: Int) -> Int")
		g.emit("    if n < 2")
		g.emit("        n")
		g.emit("    else")
		g.emit("        fib(n - 1) + fib(n - 2)")
	}
	g.hasGcd = g.maybe(50)
	if g.hasGcd {
		g.emit("word fuzz-gcd(a: Int, b: Int) -> Int")
		g.emit("    if b == 0")
		g.emit("        a")
		g.emit("    else")
		g.emit("        fuzz-gcd(b, a % b)")
	}

	// 1-8 extra annotated words with generated signatures
	for w := 0; w < 1+g.rng.Intn(8); w++ {
		name := fmt.Sprintf("fw%d", w)
		nparams := 1 + g.rng.Intn(3)
		params := make([]fuzzBinding, nparams)
		pnames := []string{"a", "b", "c"}
		decl := make([]string, nparams)
		for p := range params {
			typ := []string{"Int", "String", "Bool"}[g.rng.Intn(3)]
			params[p] = fuzzBinding{name: pnames[p], typ: typ}
			decl[p] = pnames[p] + ": " + typ
		}
		result := []string{"Int", "String", "Bool"}[g.rng.Intn(3)]
		g.emit("word " + name + "(" + strings.Join(decl, ", ") + ") -> " + result)
		g.local = params
		var body string
		switch result {
		case "Int":
			body = g.exprInt(1)
		case "String":
			body = g.exprStr(1)
		default:
			body = g.exprBool(1)
		}
		g.local = nil
		g.emit("    " + body)
		types := make([]string, len(params))
		for p := range params {
			types[p] = params[p].typ
		}
		g.words = append(g.words, fuzzWord{name: name, params: types, result: result})
	}

	// typed prologue: every operand pool non-empty from the start
	prologue := []fuzzBinding{
		{name: "p0", typ: "Int"},
		{name: "p1", typ: "Int"},
		{name: "s0", typ: "String"},
		{name: "b0", typ: "Bool"},
	}
	g.emit("let p0 = " + g.smallLit())
	g.emit("let p1 = " + fmt.Sprint(100+g.rng.Intn(900)))
	g.emit("let s0 = " + g.strLit())
	g.emit("let b0 = " + []string{"true", "false"}[g.rng.Intn(2)])
	g.stmts += 4
	xs0, ys0, m0 := g.fresh(), g.fresh(), g.fresh()
	g.emit("let " + xs0 + ": List<Int> = [3, 1, " + g.smallLit() + ", 1]")
	g.emit("let " + ys0 + ": Names = [\"pear\", \"fig\", \"ada\"]")
	g.emit("let " + m0 + ": Map<String, Int> = {\"ada\": 1, \"grace\": 2}")
	g.stmts += 3
	prologue = append(prologue,
		fuzzBinding{name: xs0, typ: "List<Int>", nonempty: true},
		fuzzBinding{name: ys0, typ: "List<String>", nonempty: true},
		fuzzBinding{name: m0, typ: "Map<String, Int>"},
	)
	g.binds = append(g.binds, prologue...)

	// guaranteed coverage: one choice match, one Option match, one closure
	for _, t := range []func() bool{g.tMatchChoice, g.tOptionMap, g.tClosure} {
		for try := 0; try < 8 && !t(); try++ {
		}
	}

	templates := []func() bool{
		g.tPrint, g.tLet, g.tVarAssign, g.tIfPrint, g.tIfBind,
		g.tMatchChoice, g.tOptionMap, g.tOptionFind,
		g.tWhile, g.tForRange, g.tForList, g.tForListStr, g.tForMap,
		g.tListLiteral, g.tAppend, g.tIndexRead, g.tListOps,
		g.tMapLiteral, g.tMapOps, g.tText, g.tToInt,
		g.tFlowStmt, g.tFlowBind, g.tClosure, g.tCallWord,
		g.tTrio, g.tUnitFlow,
	}
	target := 24 + g.rng.Intn(24)
	for g.stmts < target {
		fired := false
		for try := 0; try < 8 && !fired; try++ {
			fired = templates[g.rng.Intn(len(templates))]()
		}
		if !fired {
			g.tPrint()
		}
	}
	return strings.Join(g.lines, "\n") + "\n"
}

// TestFuzzDeterminism pins the generator itself: same seed, same program,
// on every machine and every run (DESIGN-M4 §7), and distinct seeds produce
// distinct programs (a broken seeding scheme would silently collapse them).
func TestFuzzDeterminism(t *testing.T) {
	seen := make(map[string]int, fuzzProgramCount)
	for i := 0; i < fuzzProgramCount; i++ {
		a, b := fuzzGenerate(i), fuzzGenerate(i)
		if a != b {
			t.Fatalf("program %d differs between two runs with the same seed", fuzzSeedBase+i)
		}
		if prev, dup := seen[a]; dup {
			t.Fatalf("programs %d and %d are identical", fuzzSeedBase+prev, fuzzSeedBase+i)
		}
		seen[a] = i
		if len(a) > fuzzMaxSource {
			t.Fatalf("program %d is %d bytes, over the %d budget", fuzzSeedBase+i, len(a), fuzzMaxSource)
		}
	}
}

// TestFuzzDifferential runs the 50 deterministic programs through the
// interpreter (oracle) and the compiled browser module; stdout must match
// byte for byte and the exit status must agree. Traps (division by zero,
// overflow) are part of the grammar: both engines exit 1 on stdout-parity.
func TestFuzzDifferential(t *testing.T) {
	requireTool(t, "wasm-tools")
	for i := 0; i < fuzzProgramCount; i++ {
		i := i
		t.Run(fmt.Sprintf("seed%d", fuzzSeedBase+i), func(t *testing.T) {
			source := fuzzGenerate(i)
			var interpreted strings.Builder
			runErr := lang.Run("fuzz.zhex", source, strings.NewReader(""), &interpreted)
			wantOut, wantCode := interpreted.String(), exitCode(runErr)
			outPath := filepath.Join(t.TempDir(), "fuzz.wasm")
			if err := Compile("fuzz.zhex", source, outPath, nil, lang.WatSkinBrowser); err != nil {
				t.Fatalf("compile: %v\nsource:\n%s", err, source)
			}
			out, _, code := runNode(t, outPath)
			if out != wantOut || code != wantCode {
				t.Fatalf("code=%d (want %d)\ninterpreted: %q\ncompiled:    %q\nsource:\n%s",
					code, wantCode, wantOut, out, source)
			}
		})
	}
}
