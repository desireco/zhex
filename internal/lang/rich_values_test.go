package lang

import (
	"bytes"
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
