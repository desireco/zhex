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
