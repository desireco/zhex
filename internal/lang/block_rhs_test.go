package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestBlockValuesInBindingsAndAssignment(t *testing.T) {
	source := `let clean = flow "  ADA  "
    text.trim
    text.lower
var value = if true
    1
else
    2
value = match Option.Some(3)
    case Option.Some(n)
        n
    case Option.None
        0
print(clean)
print(value)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "ada\n3\n" {
		t.Fatal(out.String())
	}
}

func TestBlockValueExpectedTypesBranchesAndEffects(t *testing.T) {
	source := `let values: List<Int> = if true
    []
else
    [1]
var selected: Option<Int> = Option.None
selected = match Option.Some(2)
    case Option.Some(n)
        Option.Some(n)
    case Option.None
        Option.None
word parse(raw: String) -> Int
    let value = flow raw
        text.trim
        text.to-int try
    value
print(parse(" 7 ") try)
print(list.length(values))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n0\n" {
		t.Fatal(out.String())
	}
}

func TestBlockValuesEvaluateOnlyChosenBranchAndAllowNever(t *testing.T) {
	source := `word mark(n: Int) -> Int
    print(n)
    n
let n = if true
    mark(1)
else
    mark(2)
let other: Int = if false
    console.exit(3)
else
    n
print(other)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n1\n" {
		t.Fatal(out.String())
	}
}

func TestBlockValuesRejectInvalidBranchesAndLostResults(t *testing.T) {
	for _, source := range []string{
		"let x = if true\n    1\nelse\n    false\n",
		"let x = if true\n    text.to-int(\"bad\")\nelse\n    text.to-int(\"ok\")\n",
		"let x = text.to-int(\"bad\") catch problem\n    let leak = if true\n        problem\n    else\n        problem\n    0\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted invalid block value: %s", source)
		}
	}
}

func TestMatchHeaderSignalsIncompleteForRepl(t *testing.T) {
	if _, err := Compile("repl", "let x = match Option.Some(1)\n"); !IsIncomplete(err) {
		t.Fatalf("match header is not incomplete: %v", err)
	}
}
