package lang

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestAnnotatedBindingsCheckDeclaredType(t *testing.T) {
	source := "let count: Int = 2\nvar name: String = \"Ada\"\nname = \"Grace\"\nprint(count)\nprint(name)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\nGrace\n" {
		t.Fatalf("output %q", out.String())
	}
	for _, bad := range []string{"let n: Int = \"wrong\"\n", "var n: Bool = 1\n", "let n: Unknown = 1\n"} {
		if _, err := Compile("test.zhex", bad); err == nil {
			t.Errorf("accepted invalid annotation: %q", bad)
		}
	}
}

func TestNamedRecordsConstructAndReadFields(t *testing.T) {
	source := "type Customer = record\n    name: String\n    age: Int\nword age(c: Customer) -> Int\n    c.age\nlet customer: Customer = Customer(\"Ada\", 36)\nprint(customer.name)\nprint(age(customer))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Ada\n36\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestNamedRecordsRejectInvalidDeclarationsAndUse(t *testing.T) {
	cases := []string{
		"type C = record\n    x: Int\n    x: String\n",
		"type C = record\n    x: Unknown\n",
		"type C = record\n    x: C\n",
		"type C = record\n    x: Int\nlet c = C(\"wrong\")\n",
		"type C = record\n    x: Int\nlet c = C()\n",
		"type C = record\n    x: Int\nlet c = C(1)\nprint(c.missing)\n",
		"type C = record\n    x: Int\ntype D = record\n    x: Int\nlet c: C = D(1)\n",
		"type C = record\n    x: Int\nlet c = C(1)\nc.x = 2\n",
	}
	for _, source := range cases {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted invalid record: %q", source)
		}
	}
}

func TestRecordRejectsConcreteErrorWithoutExplicitWrap(t *testing.T) {
	source := "type Box = record\n    problem: Error\nlet n = text.to-int(\"bad\") catch problem\n    let box = Box(problem)\n    0\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("record erased concrete error family")
	}
}

func TestRecordCanReturnThroughResultAndTrace(t *testing.T) {
	source := "type Inner = record\n    count: Int\ntype Outer = record\n    inner: Inner\nword make() -> Result<Outer, Error>\n    ok(Outer(Inner(4)))\nlet x = make() try\nprint(x.inner.count)\n"
	var out, trace bytes.Buffer
	if err := RunWithTrace("test.zhex", source, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n" || !strings.Contains(trace.String(), "record(Outer)") {
		t.Fatalf("output %q, trace %q", out.String(), trace.String())
	}
}

func TestReplKeepsRecordDefinitions(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := session.Enter("type Customer = record\n    name: String\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("let c: Customer = Customer(\"Ada\")\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := session.Enter("c.name\n"); err != nil || echo != "Ada" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
	if _, err := session.Enter("type Broken = record\n    x: Missing\n"); err == nil {
		t.Fatal("accepted invalid record")
	}
	if _, err := session.Enter("let b = Broken(1)\n"); err == nil {
		t.Fatal("adopted invalid record")
	}
}

func TestImportedRecordTypesAreNominalAndQualified(t *testing.T) {
	modules := map[string]ModuleSource{
		"people": {Path: "people.zhex", Source: "type Customer = record\n    name: String\nword make(n: String) -> Customer\n    Customer(n)\n"},
		"other":  {Path: "other.zhex", Source: "type Customer = record\n    name: String\n"},
	}
	resolve := func(name string) (ModuleSource, bool) { m, ok := modules[name]; return m, ok }
	source := "use people\nlet a: people.Customer = people.Customer(\"Ada\")\nlet b = people.make(\"Grace\")\nprint(a.name)\nprint(b.name)\n"
	var out bytes.Buffer
	if err := RunWithModules("main.zhex", source, strings.NewReader(""), &out, nil, nil, resolve); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Ada\nGrace\n" {
		t.Fatalf("output %q", out.String())
	}
	bad := "use people\nuse other\nlet a: people.Customer = other.Customer(\"Ada\")\n"
	if _, err := CompileModules("main.zhex", bad, resolve); err == nil {
		t.Fatal("accepted different nominal module types")
	}
}

func TestImportedPrivateRecordIsInaccessible(t *testing.T) {
	resolve := func(name string) (ModuleSource, bool) {
		return ModuleSource{Path: "people.zhex", Source: "type _Secret = record\n    n: Int\n"}, true
	}
	for _, source := range []string{"use people\nlet x = people._Secret(1)\n", "use people\nword read(x: people._Secret) -> Int\n    1\n"} {
		if _, err := CompileModules("main.zhex", source, resolve); err == nil || !strings.Contains(err.Error(), "private") {
			t.Fatalf("private record diagnostic: %v", err)
		}
	}
}

func TestReplAdoptsImportedRecordTypes(t *testing.T) {
	resolve := func(name string) (ModuleSource, bool) {
		return ModuleSource{Path: "people.zhex", Source: "type Customer = record\n    name: String\nword make(n: String) -> Customer\n    Customer(n)\n"}, true
	}
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil, resolve)
	if _, err := session.Enter("use people\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("let c: people.Customer = people.make(\"Ada\")\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := session.Enter("c.name\n"); err != nil || echo != "Ada" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestRecordTypeVisibilityAndDeclarationCollisions(t *testing.T) {
	modules := map[string]ModuleSource{
		"outer": {Path: "outer.zhex", Source: "use inner\nword make() -> inner.Customer\n    inner.Customer(\"Ada\")\n"},
		"inner": {Path: "inner.zhex", Source: "type Customer = record\n    name: String\n"},
	}
	resolve := func(name string) (ModuleSource, bool) { m, ok := modules[name]; return m, ok }
	var out bytes.Buffer
	if err := RunWithModules("main.zhex", "use outer\nlet c = outer.make()\nprint(c.name)\n", strings.NewReader(""), &out, nil, nil, resolve); err != nil || out.String() != "Ada\n" {
		t.Fatalf("error %v output %q", err, out.String())
	}
	if _, err := CompileModules("main.zhex", "use outer\nlet c = inner.Customer(\"Ada\")\n", resolve); err == nil {
		t.Fatal("transitive record became directly visible")
	}
	for _, source := range []string{
		"type C = record\n    x: Int\nword C() -> Int\n    1\n",
		"type A = record\n    b: B\ntype B = record\n    a: A\n",
		"type outer.C = record\n    x: Int\n",
		"type C = record\n    x.y: Int\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted invalid declaration: %q", source)
		}
	}
}

func TestPublicModuleAPIRejectsPrivateRecordLeaks(t *testing.T) {
	cases := []string{
		"type _Secret = record\n    n: Int\nword expose() -> _Secret\n    _Secret(1)\n",
		"type _Secret = record\n    n: Int\nword expose(x: _Secret) -> Int\n    1\n",
		"type _Secret = record\n    n: Int\ntype Public = record\n    secret: _Secret\n",
	}
	for _, module := range cases {
		resolve := func(name string) (ModuleSource, bool) { return ModuleSource{Path: "m.zhex", Source: module}, true }
		if _, err := CompileModules("main.zhex", "use m\n", resolve); err == nil || !strings.Contains(err.Error(), "private") {
			t.Errorf("public API leaked private type: %v", err)
		}
	}
}

func TestPrivateRecordsRemainUsableInsideModule(t *testing.T) {
	source := "type _Secret = record\n    n: Int\nword _make() -> _Secret\n    _Secret(7)\nword value() -> Int\n    let s = _make()\n    s.n\n"
	resolve := func(name string) (ModuleSource, bool) { return ModuleSource{Path: "m.zhex", Source: source}, true }
	var out bytes.Buffer
	if err := RunWithModules("main.zhex", "use m\nprint(m.value())\n", strings.NewReader(""), &out, nil, nil, resolve); err != nil || out.String() != "7\n" {
		t.Fatalf("error %v output %q", err, out.String())
	}
}

func TestTypedListLiteralsAndEmptyLists(t *testing.T) {
	source := "let nums: List<Int> = [1, 2, 3]\nlet empty: List<String> = []\nlet other: List<Int> = list.new()\nlet nested: List<List<Int>> = [[1], [2, 3]]\nprint(list.length(nums))\nprint(list.length(empty))\nprint(list.length(other))\nprint(list.length(nested))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n0\n0\n2\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestListTypesRejectInvalidLiterals(t *testing.T) {
	for _, source := range []string{
		"let x = []\n", "let x = list.new()\n", "let x = [1, \"two\"]\n", "let x: List<String> = [1]\n", "let x: List<Missing> = []\n", "let x = [text.to-int(\"1\")]\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted invalid list: %q", source)
		}
	}
}

func TestNestedTypeDepthIsBounded(t *testing.T) {
	typ := strings.Repeat("List<", 260) + "Int" + strings.Repeat(">", 260)
	if _, err := Compile("test.zhex", "let x: "+typ+" = []\n"); err == nil || !strings.Contains(err.Error(), "nesting limit") {
		t.Fatalf("missing nesting limit: %v", err)
	}
}

func TestListExpectedTypesFlowIntoCallsAndReturns(t *testing.T) {
	source := "type Bag = record\n    items: List<Int>\nword empty() -> List<Int>\n    []\nword size(items: List<Int>) -> Int\n    list.length(items)\nlet bag = Bag([])\nprint(list.length(bag.items))\nprint(size(list.new()))\nprint(list.length(empty()))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n0\n0\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestRecordsCannotRecurThroughCollections(t *testing.T) {
	for _, source := range []string{"type Tree = record\n    children: List<Tree>\n", "type A = record\n    b: Array<B>\ntype B = record\n    a: List<A>\n"} {
		if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "recursive") {
			t.Errorf("recursive type accepted: %v", err)
		}
	}
}

func TestListCannotHideConcreteError(t *testing.T) {
	source := "let n = text.to-int(\"bad\") catch problem\n    let errors: List<Error> = [problem]\n    0\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("list erased concrete error family")
	}
}

func TestGenericTypeNamesCannotBeDeclaredAsRecords(t *testing.T) {
	for _, name := range []string{"List", "Array", "Result", "InputError", "ParseError", "FileError", "BufferError"} {
		if _, err := Compile("test.zhex", "type "+name+" = record\n    n: Int\n"); err == nil {
			t.Errorf("accepted reserved type %s", name)
		}
	}
}

func TestRecordConstructorTypoGetsSuggestion(t *testing.T) {
	source := "type Customer = record\n    name: String\nlet c = Custmer(\"Ada\")\n"
	if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), `did you mean "Customer"`) {
		t.Fatalf("missing constructor suggestion: %v", err)
	}
}

func TestListAppendIndexAndAliasing(t *testing.T) {
	source := "let items: List<Int> = [1, 2]\nlet alias = items\nlist.append(alias, 3) try\n(items[1] = 9) try\nprint(list.length(items))\nprint(alias[1] try)\nprint(items[2] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n9\n3\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestListFailuresAreCheckedAndAtomic(t *testing.T) {
	source := "let items = [1, 2]\n(items[5] = 9) catch problem\n    print(error.code(problem))\n    print(error.detail(problem, \"index\"))\nlet n = items[-1] catch problem\n    print(error.code(problem))\n    0\nprint(items[1] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "list-range\n5\nlist-range\n2\n" {
		t.Fatalf("output %q", out.String())
	}
	for _, bad := range []string{"let x = [1]\nx[0]\n", "let x = [1]\n(x[0] = \"bad\") try\n", "let x = [1]\nlist.append(x, \"bad\") try\n", "let x = [1]\nprint(x[true] try)\n"} {
		if _, err := Compile("test.zhex", bad); err == nil {
			t.Errorf("accepted invalid list operation %q", bad)
		}
	}
}

func TestListNestedMutationAndContext(t *testing.T) {
	source := "let rows: List<List<Int>> = []\nlist.append(rows, []) try\nlet first = rows[0] try\nlist.append(first, 7) try\nlet row = rows[0] try\nprint(row[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestRecordTypeGraphDepthIsBounded(t *testing.T) {
	var source strings.Builder
	for i := 0; i < 260; i++ {
		fmt.Fprintf(&source, "type T%d = record\n    next: T%d\n", i, i+1)
	}
	source.WriteString("type T260 = record\n    value: Int\n")
	if _, err := Compile("test.zhex", source.String()); err == nil || !strings.Contains(err.Error(), "nesting limit") {
		t.Fatalf("missing type graph limit: %v", err)
	}
	source.Reset()
	source.WriteString("type T260 = record\n    value: Int\n")
	for i := 259; i >= 0; i-- {
		fmt.Fprintf(&source, "type T%d = record\n    next: T%d\n", i, i+1)
	}
	if _, err := Compile("test.zhex", source.String()); err == nil || !strings.Contains(err.Error(), "nesting limit") {
		t.Fatalf("missing reversed type graph limit: %v", err)
	}
}

func TestListLimitsAndActiveGuardLeaveDataUnchanged(t *testing.T) {
	full := &listValue{items: make([]value, maxCollectionElements)}
	r := runtime{}
	call := &expression{text: "list.append", pos: position{file: "test.zhex", line: 1, column: 1}}
	result, err := r.builtin(call, []value{{list: full}, {number: 1}})
	if err != nil || result.result == nil || result.result.err == nil || result.result.err.(*diagnostic).code != CodeCollectionLimit || len(full.items) != maxCollectionElements {
		t.Fatalf("limit failure: %v %+v", err, result)
	}
	active := &listValue{items: []value{{number: 2}}, active: 1}
	result = listWrite(call.pos, active, 0, value{number: 9})
	if result.result.err == nil || result.result.err.(*diagnostic).code != CodeCollectionIterating || active.items[0].number != 2 {
		t.Fatal("iteration guard changed list")
	}
}

func TestArrayResizePreservesCoordinates(t *testing.T) {
	source := "let grid = array.new([2, 3], 0) try\n(grid[0, 0] = 1) try\n(grid[0, 1] = 2) try\n(grid[1, 0] = 4) try\n(grid[1, 1] = 5) try\narray.resize(grid, [3, 2], 9) try\nprint(grid[0, 0] try)\nprint(grid[0, 1] try)\nprint(grid[1, 0] try)\nprint(grid[1, 1] try)\nprint(grid[2, 0] try)\nprint(array.rank(grid))\nprint(array.length(grid))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n2\n4\n5\n9\n2\n6\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestArrayFailuresAndZeroShapes(t *testing.T) {
	source := "let grid = array.new([2, 2], 7) try\n(grid[9, 0] = 1) catch problem\n    print(error.code(problem))\narray.resize(grid, [3], 0) catch problem\n    print(error.code(problem))\nprint(grid[0, 0] try)\nlet empty = array.new([9223372036854775807, 0], 1) try\nprint(array.length(empty))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "array-range\narray-rank\n7\n0\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestArrayThreeDimensionalResizeAndFreshShape(t *testing.T) {
	source := "let grid = array.new([2, 2, 2], 0) try\n(grid[1, 0, 1] = 8) try\narray.resize(grid, [2, 3, 3], 9) try\nprint(grid[1, 0, 1] try)\nprint(grid[1, 1, 2] try)\nlet shape = array.shape(grid)\n(shape[0] = 7) try\nprint(array.rank(grid))\nlet actual = array.shape(grid)\nprint(actual[0] try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "8\n9\n3\n2\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestArrayRejectsBadTypesAndRequiresHandling(t *testing.T) {
	for _, src := range []string{"array.new([2, 2], 0)\n", "let a = array.new([2, 2], 0) try\n(a[0, 0] = true) try\n", "let a = array.new([2, 2], 0) try\nprint(a[true, 0] try)\n", "let a = array.new([2, 2], 0) try\narray.resize(a, [2, 2], true) try\n"} {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted %q", src)
		}
	}
}

func TestCollectionLoopsPositionsAndMutationGuard(t *testing.T) {
	source := "let items = [3, 4]\nlet alias = items\nfor index, item in items\n    print(index)\n    print(item)\n    list.append(alias, 9) catch problem\n        print(error.code(problem))\nlist.append(items, 5) try\nlet grid = array.new([2, 2], 7) try\nfor coords, item in grid\n    print(coords[0] try)\n    print(coords[1] try)\n    print(item)\n    (grid[0, 0] = 8) catch problem\n        print(error.code(problem))\nprint(list.length(items))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n3\ncollection-iterating\n1\n4\ncollection-iterating\n0\n0\n7\ncollection-iterating\n0\n1\n7\ncollection-iterating\n1\n0\n7\ncollection-iterating\n1\n1\n7\ncollection-iterating\n3\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestCollectionLoopCoordinatesAreFreshAndGuardReleases(t *testing.T) {
	source := "let grid = array.new([1, 2], 3) try\nlet saved: List<List<Int>> = []\nfor coords, item in grid\n    list.append(saved, coords) try\nlet first = saved[0] try\n(first[1] = 8) try\nlet second = saved[1] try\nprint(second[1] try)\n(grid[0, 0] = 4) try\nprint(grid[0, 0] try)\nlet empty = array.new([0, 2], 1) try\nfor item in empty\n    print(item)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n4\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestCollectionLoopGuardUnwindsOnReturn(t *testing.T) {
	source := "word first(items: List<Int>) -> Result<Int, Error>\n    for item in items\n        return ok(item)\n    ok(0)\nlet items = [2]\nprint(first(items) try)\nlist.append(items, 3) try\nprint(list.length(items))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n2\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestCollectionLoopTypeAndScopeChecks(t *testing.T) {
	for _, src := range []string{"for n in 3\n    print(n)\n", "for n in [1]\n    n = 2\n", "for n in [1]\n    print(n)\nprint(n)\n", "for n, n in [1]\n    print(n)\n"} {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted %q", src)
		}
	}
}

func TestCaughtErrorsCannotBeLaunderedThroughUserWords(t *testing.T) {
	source := "word collect(problem: Error) -> List<Error>\n    [problem]\nlet n = text.to-int(\"bad\") catch problem\n    let errors = collect(problem)\n    0\n"
	if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "error.wrap") {
		t.Fatalf("accepted concrete error laundering: %v", err)
	}
	safe := "word collect(problem: Error) -> List<Error>\n    [problem]\nlet n = text.to-int(\"bad\") catch problem\n    let errors = collect(error.wrap(\"converted\", \"converted\", problem))\n    print(list.length(errors))\n    0\n"
	var out bytes.Buffer
	if err := Run("test.zhex", safe, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestArrayErrorDetailsAndOrigin(t *testing.T) {
	source := "let grid = array.new([2, 3], 0) try\nlet x = grid[1, 7] catch problem\n    print(error.operation(problem))\n    print(error.detail(problem, \"axis\"))\n    print(error.detail(problem, \"index\"))\n    print(error.detail(problem, \"extent\"))\n    print(error.line(problem))\n    0\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "array.index\n1\n7\n3\n2\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestNestedCollectionLoopsHoldGuardUntilOuterEnds(t *testing.T) {
	source := "let xs = [1]\nfor x in xs\n    for y in xs\n        print(y)\n    list.append(xs, 2) catch problem\n        print(error.code(problem))\nlist.append(xs, 3) try\nprint(list.length(xs))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\ncollection-iterating\n2\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestCollectionGuardUnwindsAfterFailuresAndExits(t *testing.T) {
	for _, body := range []string{"print(1 / 0)", "console.exit(3)", "console.fail(\"done\")", "list.append(xs, 2) try"} {
		t.Run(body, func(t *testing.T) {
			var out bytes.Buffer
			session := NewSession(strings.NewReader(""), &out, nil, nil)
			if _, err := session.Enter("let xs = [1]\n"); err != nil {
				t.Fatal(err)
			}
			if _, err := session.Enter("for x in xs\n    " + body + "\n"); err == nil {
				t.Fatal("expected unwinding")
			}
			if _, err := session.Enter("list.append(xs, 3) try\n"); err != nil {
				t.Fatalf("guard remained active: %v", err)
			}
			if echo, err := session.Enter("list.length(xs)\n"); err != nil || echo != "2" {
				t.Fatalf("length %q, error %v", echo, err)
			}
		})
	}
}

func TestErrorValuedFlowSuggestsDirectCall(t *testing.T) {
	source := "word describe(problem: Error) -> String\n    error.code(problem)\nflow error.new(\"x\", \"x\")\n    describe\n"
	if _, err := Compile("test.zhex", source); err == nil || !strings.Contains(err.Error(), "direct user-word call") {
		t.Fatalf("missing actionable flow diagnostic: %v", err)
	}
}

func TestChoicesConstructAndMatch(t *testing.T) {
	source := "type Status = choice\n    Pending\n    Running\n    Failed(message: String)\nword describe(status: Status) -> String\n    match status\n        case Status.Pending\n            \"pending\"\n        case Status.Running\n            \"running\"\n        case Status.Failed(message)\n            message\nlet status = Status.Failed(\"offline\")\nprint(describe(status))\nlet text = match Status.Pending\n    case Status.Pending\n        \"waiting\"\n    case _\n        \"other\"\nprint(text)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "offline\nwaiting\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestChoicesRequireExhaustiveAndValidPatterns(t *testing.T) {
	prefix := "type Status = choice\n    Pending\n    Failed(message: String)\nlet status = Status.Pending\n"
	for _, body := range []string{"match status\n    case Status.Pending\n        print(1)\n", "match status\n    case Status.Pending\n        print(1)\n    case Status.Pending\n        print(2)\n    case _\n        print(3)\n", "match status\n    case Status.Failed\n        print(1)\n    case _\n        print(2)\n", "match status\n    case _\n        print(1)\n    case Status.Pending\n        print(2)\n"} {
		if _, err := Compile("test.zhex", prefix+body); err == nil {
			t.Errorf("accepted invalid match %q", body)
		}
	}
}

func TestChoiceModulePrivacyAndREPL(t *testing.T) {
	modules := map[string]ModuleSource{"model": {Path: "model.zhex", Source: "type Status = choice\n    Ready\n    Failed(message: String)\ntype _Hidden = choice\n    Secret\nword create() -> Status\n    Status.Failed(\"bad\")\n"}}
	resolve := func(name string) (ModuleSource, bool) { m, ok := modules[name]; return m, ok }
	source := "use model\nlet status = model.create()\nmatch status\n    case model.Status.Ready\n        print(\"ready\")\n    case model.Status.Failed(message)\n        print(message)\n"
	var out bytes.Buffer
	if err := RunWithModules("main.zhex", source, strings.NewReader(""), &out, nil, nil, resolve); err != nil {
		t.Fatal(err)
	}
	if out.String() != "bad\n" {
		t.Fatal(out.String())
	}
	if _, err := CompileModules("main.zhex", "use model\nlet x = model._Hidden.Secret\n", resolve); err == nil {
		t.Fatal("private choice accessible")
	} else if d, ok := err.(*diagnostic); !ok || d.code != CodePrivateUse {
		t.Fatalf("wanted private-use: %v", err)
	}
	session := NewSession(strings.NewReader(""), &out, nil, resolve)
	if _, err := session.Enter("type Mood = choice\n    Happy\n    Sad\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("let m = Mood.Happy\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("m\n"); err != nil {
		t.Fatal(err)
	}
}

func TestChoicePayloadAndMatchRestrictions(t *testing.T) {
	cases := []string{
		"type E = choice\n    Bad(reason: Error)\nlet x = text.to-int(\"bad\") catch problem\n    let e = E.Bad(problem)\n    0\n",
		"type E = choice\n    Bad(problem: Result<Int, Error>)\n",
		"type E = choice\n    A\n    A\n",
		"type E = choice\n    A\nword E.A() -> Int\n    0\n",
		"type E = choice\n    A\nlet x = E.A()\n",
		"type E = choice\n    A(x: Int)\nlet a = E.A(2)\nmatch a\n    case E.A(x)\n        x = 3\n",
		"type E = choice\n    A\nlet x = E.A\nprint(x)\n",
		"type E = choice\n    A\nlet x = E.A\nprint(x == x)\n",
		"type E = choice\n    A\n    B\nlet x = E.A\nmatch x\n    case E.A\n        1\n    case E.B\n        true\n",
	}
	for _, src := range cases {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted invalid choice:\n%s", src)
		}
	}
}

func TestMatchEvaluatesSubjectOnceAndSupportsNever(t *testing.T) {
	source := "type Status = choice\n    Good(value: Int)\n    Missing\nlet calls = [0]\nword next(calls: List<Int>) -> Status\n    let count = calls[0] try\n    (calls[0] = count + 1) try\n    Status.Good(7)\nlet value = match next(calls)\n    case Status.Good(value)\n        value\n    case Status.Missing\n        console.exit(4)\nprint(value)\nprint(calls[0] try)\n"
	// next needs a fallible signature for checked collection access.
	source = strings.Replace(source, "word next(calls: List<Int>) -> Status", "word next(calls: List<Int>) -> Result<Status, Error>", 1)
	source = strings.Replace(source, "    Status.Good(7)\n", "    ok(Status.Good(7))\n", 1)
	source = strings.Replace(source, "match next(calls)\n", "match next(calls) try\n", 1)
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n1\n" {
		t.Fatal(out.String())
	}
}

func TestChoiceTraceAndTypeNameShadow(t *testing.T) {
	source := "type Status = choice\n    Ready\nlet Status = 4\nlet state = Status.Ready\nmatch state\n    case Status.Ready\n        print(Status)\n"
	var out, trace bytes.Buffer
	if err := RunWithTrace("test.zhex", source, strings.NewReader(""), &out, &trace); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n" || !strings.Contains(trace.String(), "choice(Status.Ready)") {
		t.Fatalf("output %q trace %q", out.String(), trace.String())
	}
}

func TestMatchCanReturnResultWithoutDiscard(t *testing.T) {
	source := "type Switch = choice\n    On\n    Off\nword convert(s: Switch) -> Result<Int, Error>\n    match s\n        case Switch.On\n            ok(1)\n        case Switch.Off\n            err(error.new(\"off\", \"off\"))\nprint(convert(Switch.On) try)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n" {
		t.Fatal(out.String())
	}
	bad := "type Switch = choice\n    On\nword run() -> Result<Int, Error>\n    for n in range(0, 1)\n        match Switch.On\n            case Switch.On\n                err(error.new(\"lost\", \"lost\"))\n    ok(1)\n"
	if _, err := Compile("test.zhex", bad); err == nil {
		t.Fatal("discarded Result from match in loop")
	}
}

func TestMatchCannotEraseCaughtErrorFamily(t *testing.T) {
	source := "type Status = choice\n    Ready\nlet n = text.to-int(\"bad\") catch problem\n    let erased = match Status.Ready\n        case Status.Ready\n            problem\n    let errors: List<Error> = [erased]\n    0\n"
	if _, err := Compile("test.zhex", source); err == nil {
		t.Fatal("match erased caught error family")
	}
	safe := strings.Replace(source, "            problem\n", "            error.wrap(\"converted\", \"converted\", problem)\n", 1)
	if _, err := Compile("test.zhex", safe); err != nil {
		t.Fatal(err)
	}
}

func TestMatchPropagatesExpectedTypeIntoBranches(t *testing.T) {
	source := "type Flag = choice\n    On\n    Off\nlet xs: List<Int> = match Flag.On\n    case Flag.On\n        []\n    case Flag.Off\n        [1]\nprint(list.length(xs))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n" {
		t.Fatal(out.String())
	}
}

func TestMatchSupportsFlowAsBranchResult(t *testing.T) {
	source := "type Status = choice\n    Ready(name: String)\n    Missing\nword label(status: Status) -> String\n    match status\n        case Status.Ready(name)\n            flow name\n                text.upper\n        case Status.Missing\n            \"unknown\"\nprint(label(Status.Ready(\"ada\")))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "ADA\n" {
		t.Fatal(out.String())
	}
}

func TestChoiceConstructorCannotShadowImportedWord(t *testing.T) {
	resolve := func(name string) (ModuleSource, bool) {
		if name == "Status" {
			return ModuleSource{Path: "Status.zhex", Source: "word Failed(message: String) -> String\n    message\n"}, true
		}
		return ModuleSource{}, false
	}
	source := "use Status\ntype Status = choice\n    Failed(message: String)\n"
	if _, err := CompileModules("main.zhex", source, resolve); err == nil {
		t.Fatal("choice constructor shadowed imported word")
	}
}

func TestChoicesRejectPrivatePayloadLeakAndRecursiveGraph(t *testing.T) {
	modules := map[string]ModuleSource{"model": {Path: "model.zhex", Source: "type _Hidden = record\n    value: Int\ntype Public = choice\n    Revealed(value: _Hidden)\n"}}
	resolve := func(name string) (ModuleSource, bool) { m, ok := modules[name]; return m, ok }
	if _, err := CompileModules("main.zhex", "use model\n", resolve); err == nil {
		t.Fatal("public choice leaks private type")
	}
	for _, src := range []string{"type A = choice\n    Child(value: List<A>)\n", "type A = choice\n    Child(value: B)\ntype B = record\n    value: A\n"} {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted recursive choice graph: %q", src)
		}
	}
}

func TestChoiceConstructorCannotShadowImportedType(t *testing.T) {
	resolve := func(name string) (ModuleSource, bool) {
		if name == "Status" {
			return ModuleSource{Path: "Status.zhex", Source: "type Failed = record\n    message: String\n"}, true
		}
		return ModuleSource{}, false
	}
	source := "use Status\ntype Status = choice\n    Failed(message: String)\n"
	if _, err := CompileModules("main.zhex", source, resolve); err == nil {
		t.Fatal("choice constructor shadowed imported record constructor")
	}
}

func TestGenericChoiceInfersPayloadAndContextualNullary(t *testing.T) {
	source := "type Maybe<T> = choice\n    None\n    Some(value: T)\nlet value = Maybe.Some(7)\nlet empty: Maybe<Int> = Maybe.None\nword describe(m: Maybe<Int>) -> Int\n    match m\n        case Maybe.Some(value)\n            value\n        case Maybe.None\n            0\nprint(describe(value))\nprint(describe(empty))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n0\n" {
		t.Fatal(out.String())
	}
}

func TestGenericChoiceRejectsUnknownAndConflictingTypes(t *testing.T) {
	prefix := "type Maybe<T> = choice\n    None\n    Some(value: T)\n    Pair(left: T, right: T)\n"
	for _, src := range []string{"let x = Maybe.None\n", "let x = Maybe.Pair(1, \"x\")\n", "let x: Maybe<String> = Maybe.Some(1)\n", "let x: Maybe<Result<Int, Error>> = Maybe.None\n"} {
		if _, err := Compile("test.zhex", prefix+src); err == nil {
			t.Errorf("accepted %q", src)
		}
	}
}

func TestGenericChoiceNestedInferenceAndEmptyContext(t *testing.T) {
	source := "type Maybe<T> = choice\n    None\n    Some(value: T)\n    Pair(left: T, right: T)\nlet lists: Maybe<List<Int>> = Maybe.Some([])\nlet pair = Maybe.Pair([2], [3])\nmatch pair\n    case Maybe.Pair(a, b)\n        print(a[0] try)\n        print(b[0] try)\n    case _\n        print(0)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n3\n" {
		t.Fatal(out.String())
	}
}

func TestGenericChoicePrivacyCyclesAndErrorFamily(t *testing.T) {
	modules := map[string]ModuleSource{"model": {Path: "model.zhex", Source: "type _Secret = record\n    n: Int\ntype Maybe<T> = choice\n    None\n    Some(value: T)\ntype Public = record\n    item: Maybe<_Secret>\n"}}
	resolve := func(name string) (ModuleSource, bool) { m, ok := modules[name]; return m, ok }
	if _, err := CompileModules("main.zhex", "use model\n", resolve); err == nil {
		t.Fatal("generic argument leaked private type")
	}
	for _, src := range []string{"type Maybe<T> = choice\n    Again(item: Maybe<T>)\n", "type Maybe<Int> = choice\n    Some(value: Int)\n", "type Maybe<T> = choice\n    Some(value: T)\nlet n = text.to-int(\"bad\") catch problem\n    let x = Maybe.Some(problem)\n    0\n"} {
		if _, err := Compile("test.zhex", src); err == nil {
			t.Errorf("accepted invalid generic choice:\n%s", src)
		}
	}
}

func TestGenericChoiceModulesAndInferredWord(t *testing.T) {
	modules := map[string]ModuleSource{"model": {Path: "model.zhex", Source: "type Maybe<T> = choice\n    None\n    Some(value: T)\nword get() -> Maybe<Int>\n    Maybe.Some(4)\n"}}
	resolve := func(name string) (ModuleSource, bool) { m, ok := modules[name]; return m, ok }
	source := "use model\nlet empty: model.Maybe<Int> = model.Maybe.None\nlet value = model.get()\nmatch value\n    case model.Maybe.Some(n)\n        print(n)\n    case model.Maybe.None\n        print(0)\n"
	var out bytes.Buffer
	if err := RunWithModules("main.zhex", source, strings.NewReader(""), &out, nil, nil, resolve); err != nil {
		t.Fatal(err)
	}
	if out.String() != "4\n" {
		t.Fatal(out.String())
	}
}

func TestPrivateTypeInsideGenericArgumentIsPrivateUse(t *testing.T) {
	resolve := func(name string) (ModuleSource, bool) {
		if name == "model" {
			return ModuleSource{Path: "model.zhex", Source: "type _Secret = record\n    n: Int\ntype Maybe<T> = choice\n    None\n    Some(value: T)\n"}, true
		}
		return ModuleSource{}, false
	}
	_, err := CompileModules("main.zhex", "use model\nlet x: model.Maybe<model._Secret> = model.Maybe.None\n", resolve)
	if d, ok := err.(*diagnostic); !ok || d.code != CodePrivateUse {
		t.Fatalf("wanted private-use: %v", err)
	}
}

func TestGenericChoiceCannotHideResultObligation(t *testing.T) {
	prefix := "type Maybe<T> = choice\n    Some(value: T)\n"
	if _, err := Compile("test.zhex", prefix+"let x = Maybe.Some(text.to-int(\"bad\"))\n"); err == nil {
		t.Fatal("generic choice hid Result")
	}
	var out bytes.Buffer
	if err := Run("test.zhex", prefix+"let x = Maybe.Some(text.to-int(\"7\") try)\nmatch x\n    case Maybe.Some(value)\n        print(value)\n", strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}
