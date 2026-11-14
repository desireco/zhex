package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestPartialsCaptureArgumentsAndCallFunctions(t *testing.T) {
	source := `word add left: Int, right: Int -> Int
    left + right
let plus-five = add _, 5
let five-plus: (Int) -> Int = add 5, _
print plus-five(7)
let n = five-plus 7
print n
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n12\n" {
		t.Fatal(out.String())
	}
}

func TestPartialsUseDefaultsOnInvocation(t *testing.T) {
	source := `word suffix() -> String
    print("default")
    "!"
word greet name: String, mark: String = suffix() -> String
    name + mark
let greeter = greet _
print greeter("Ada")
print greeter("Grace")
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "default\nAda!\ndefault\nGrace!\n" {
		t.Fatal(out.String())
	}
}

func TestPartialsInferFromWordsAndWorkInFlow(t *testing.T) {
	source := `word add left, right
    left + right
let plus-five = add _, 5
flow 7
    plus-five
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatal(out.String())
	}
}

func TestCallableParameterAndPartialOfPartial(t *testing.T) {
	source := `word add a: Int, b: Int, c: Int -> Int
    a + b + c
word apply(op: (Int) -> Int, n: Int) -> Int
    op(n)
let two-holes = add _, 2, _
let plus-five = two-holes 3, _
print(apply(plus-five, 7))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatal(out.String())
	}
}

func TestPartialCapturesValuesAndSharedCollections(t *testing.T) {
	source := `word describe(values: List<Int>, n: Int) -> Int
    list.length(values) + n
var n = 2
let values = [1]
let describe-now = describe values, _
let add-n = describe _, n
n = 9
list.append(values, 2)
print(describe-now(3))
print(add-n(values))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n4\n" {
		t.Fatal(out.String())
	}
}

func TestPartialCapturesOnceAndRejectsMissingRequired(t *testing.T) {
	source := `word mark(n: Int) -> Int
    print(n)
    n
word add(a: Int, b: Int) -> Int
    a + b
let p = add mark(5), _
print(p(1))
print(p(2))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n6\n7\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "word add(a: Int, b: Int) -> Int\n    a + b\nlet p = add 1\n"); err == nil {
		t.Fatal("accepted implicit currying")
	}
}

func TestFlowCanCreatePartial(t *testing.T) {
	source := `word add(a: Int, b: Int) -> Int
    a + b
let plus-five = flow 5
    add _
print(plus-five(7))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "12\n" {
		t.Fatal(out.String())
	}
}
