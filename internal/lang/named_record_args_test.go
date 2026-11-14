package lang

import (
	"bytes"
	"strings"
	"testing"
)

func TestNamedRecordArgumentsUseSourceOrderAndDefaults(t *testing.T) {
	source := `word mark(value: Int) -> Int
    print(value)
    value
type Settings = record
    retries: Int
    delay: Int = mark(30)
    verbose: Bool = false
let a = Settings(verbose: true, retries: mark(1))
print(a.retries)
print(a.delay)
print(a.verbose)
let b = Settings(retries: mark(2), delay: mark(3))
print(b.delay)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n30\n1\n30\ntrue\n2\n3\n3\n" {
		t.Fatal(out.String())
	}
}

func TestNamedRecordArgumentsRejectInvalidCallsBeforeEffects(t *testing.T) {
	for _, call := range []string{"Settings(verbose: true)", "Settings(retries: 1, retries: 2)", "Settings(retry: 1)", "Settings(retries: false)", "Settings(1, verbose: true)", "print(value: 1)", "Option.Some(value: 1)"} {
		source := "type Settings = record\n    retries: Int\n    verbose: Bool = false\nprint(\"before\")\nlet x = " + call + "\n"
		var out bytes.Buffer
		if err := Run("test.zhex", source, strings.NewReader(""), &out); err == nil || out.Len() != 0 {
			t.Errorf("accepted/evaluated %s: error %v out %q", call, err, out.String())
		}
	}
}

func TestNamedRecordArgsContextEffectsAndFlow(t *testing.T) {
	source := `type Config = record
    retries: Int
    tags: List<String> = []
word show(c: Config) -> Unit
    print(c.retries)
    print(list.length(c.tags))
word parse(raw: String) -> Config
    Config(tags: [], retries: text.to-int(raw) try)
flow Config(
    tags: [],
    retries: 5
)
    show
let c = parse("bad") catch problem
    Config(retries: 7)
print(c.retries)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n0\n7\n" {
		t.Fatal(out.String())
	}
	bad := "type Config = record\n    retries: Int\nflow 1\n    Config(retries: 2)\n"
	if _, err := Compile("test.zhex", bad); err == nil {
		t.Fatal("accepted named flow step mixed with its implicit positional argument")
	}
}

func TestNamedRecordArgsPreserveFreshDefaultsAndRejectErrorLaundering(t *testing.T) {
	source := `type Config = record
    required: Int
    flags: List<Int> = []
    delay: Int = 0
let a = Config(delay: 5, required: 1)
let b = Config(required: 2)
list.append(a.flags, 9)
print(list.length(b.flags))
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "0\n" {
		t.Fatal(out.String())
	}
	bad := "type Box = record\n    n: Int\n    problem: Error\nlet n = text.to-int(\"bad\") catch problem\n    let box = Box(problem: problem, n: 1)\n    0\n"
	if _, err := Compile("test.zhex", bad); err == nil {
		t.Fatal("named fields laundered a concrete Error")
	}
}
