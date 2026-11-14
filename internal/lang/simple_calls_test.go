package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestSimpleWordHeadersAndCalls(t *testing.T) {
	source := `word add left: Int, right: Int = 4
    left + right
word seven
    7
let sum = add 1, 2
print sum
print 1 + 1
let n = seven
print n
flow 2
    add
    print
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n2\n7\n6\n" {
		t.Fatal(out.String())
	}
}

func TestSimpleCallsKeepValueShadowingAndChecks(t *testing.T) {
	source := `word answer
    7
let answer = 9
print answer
print(answer())
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "9\n7\n" {
		t.Fatal(out.String())
	}
	for _, source := range []string{
		"word add a: Int, b: Int\n    a + b\nadd 1\n",
		"print text.length \"no nesting\"\n",
	} {
		if _, err := Compile("test.zhex", source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}

func TestSimpleCallsPropagateAtCallBoundary(t *testing.T) {
	source := `word parse raw: String
    text.to-int(raw) try
word outer raw: String
    parse raw try
let n = outer "7" try
print n
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestSimpleCallAmbiguitiesKeepExistingParsing(t *testing.T) {
	source := `let values = [8, 5]
let difference = values [0] - values [1]
print difference
word display(n: Int)
    print(n)
display(-1)
display([3][0])
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n-1\n3\n" {
		t.Fatal(out.String())
	}
	for _, bad := range []string{"print\"no whitespace\"\n", "print text.length \"ambiguous\"\n"} {
		if _, err := Compile("test.zhex", bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestBareZeroCallStillChecksFailures(t *testing.T) {
	source := `word parse
    text.to-int("7") try
word outer
    parse try
print(outer() try)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}

func TestSimpleCallsWorkAcrossModulesAndFlowSeeds(t *testing.T) {
	files := mapResolver(map[string]string{"math.zhex": "word increment value\n    value + 1\nword seed\n    6\n"})
	source := "use math\nflow math.seed\n    math.increment\n    print\n"
	program, err := CompileModules("main.zhex", source, files)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := program.Run(strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "7\n" {
		t.Fatal(out.String())
	}
}
