package lang

import (
	"bytes"
	"strings"
	"testing"
)

// The #236 stack words: Stack<T> is the shared-mutable LIFO whose top is
// the end of the backing store. push/dup/swap/drop return the receiver,
// pop returns the removed top — the sanctioned return-side exception — and
// every underflow is the family's new stack-range diagnostic.

func TestStackNewPushPopPeekLengthRoundTrip(t *testing.T) {
	source := "let s: Stack<Int> = stack.new\nstack.push(s, 10)\nstack.push(s, 20)\nstack.push(s, 30)\nprint(stack.length(s))\nprint(stack.peek(s))\nprint(stack.pop(s))\nprint(stack.peek(s))\nprint(stack.length(s))\nprint(stack.pop(s))\nprint(stack.pop(s))\nprint(stack.length(s))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n30\n30\n20\n2\n20\n10\n0\n" {
		t.Fatal(out.String())
	}
}

func TestStackPopIsEmptyLastInFirstOut(t *testing.T) {
	source := "let s: Stack<Int> = stack.new\nvar total = 0\nfor n in [1, 2, 3, 4]\n    stack.push(s, n * 10)\nwhile stack.length(s) > 0\n    total = total * 100 + stack.pop(s)\nprint(total)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "40302010\n" {
		t.Fatal(out.String())
	}
}

func TestStackUnderflowFailsStackRange(t *testing.T) {
	for _, tc := range []struct {
		word   string
		index  string
		length string
	}{
		{"stack.pop", "-1", "0"},
		{"stack.peek", "-1", "0"},
		{"stack.dup", "-1", "0"},
		{"stack.drop", "-1", "0"},
		{"stack.swap", "-2", "0"},
	} {
		source := "let s: Stack<Int> = stack.new\n" + tc.word + "(s)\n"
		err := Run("bad.zhex", source, strings.NewReader(""), &strings.Builder{})
		if err == nil || !strings.Contains(err.Error(), "stack index is out of range") {
			t.Fatalf("%s: wanted the range diagnostic, got %v", tc.word, err)
		}
		d, ok := err.(*diagnostic)
		if !ok || d.code != CodeStackRange {
			t.Fatalf("%s: wanted code stack-range, got %v", tc.word, err)
		}
		if d.operation != tc.word {
			t.Fatalf("%s: wanted operation %s, got %q", tc.word, tc.word, d.operation)
		}
		if d.details["index"] != tc.index || d.details["length"] != tc.length {
			t.Fatalf("%s: wanted details index %s length %s, got %v", tc.word, tc.index, tc.length, d.details)
		}
	}
}

func TestStackSwapOnOneElementFailsStackRange(t *testing.T) {
	source := `let s: Stack<Int> = stack.new
stack.push(s, 9)
stack.swap(s)
`
	err := Run("bad.zhex", source, strings.NewReader(""), &strings.Builder{})
	if err == nil || !strings.Contains(err.Error(), "stack index is out of range") {
		t.Fatalf("wanted range diagnostic, got %v", err)
	}
	d, ok := err.(*diagnostic)
	if !ok || d.code != CodeStackRange || d.operation != "stack.swap" {
		t.Fatalf("wanted stack-range from stack.swap, got %v", err)
	}
	if d.details["index"] != "-2" || d.details["length"] != "1" {
		t.Fatalf("wanted details index -2 length 1, got %v", d.details)
	}
}

func TestStackTryCatchSelectsResult(t *testing.T) {
	source := `let empty: Stack<Int> = stack.new
let a = stack.pop(empty) catch problem
    print(error.code(problem))
    0
let b = stack.peek(empty) catch problem
    print(error.code(problem))
    0
word top-of(s: Stack<Int>) -> Int
    stack.peek(s) try
let seeded: Stack<Int> = stack.new
stack.push(seeded, 8)
print(top-of(seeded) try)
word drain-one(s: Stack<Int>) -> Int
    stack.pop(s) try
let n = drain-one(empty) catch problem
    print(error.code(problem))
    0
print(n)
`
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "stack-range\nstack-range\n8\nstack-range\n0\n" {
		t.Fatal(out.String())
	}
}

func TestStackDupSwapDropReturnTheReceiver(t *testing.T) {
	source := "var s: Stack<Int> = stack.new\ns = stack.push(s, 10)\ns = stack.push(s, 20)\ns = stack.push(s, 30)\ns = stack.swap(s)\nprint(stack.peek(s))\ns = stack.dup(s)\nprint(stack.length(s))\ns = stack.drop(s)\nprint(stack.length(s))\nprint(stack.pop(s))\nprint(stack.peek(s))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "20\n4\n3\n20\n30\n" {
		t.Fatal(out.String())
	}
}

func TestStackWithFlowIdiomResolvesTheNamespace(t *testing.T) {
	source := "let s: Stack<Int> = stack.new\nlet top = flow s\nwith stack\n    push 10\n    push 20\n    swap\n    push 5\n    dup\n    drop\n    pop\nprint(top)\nprint(stack.length(s))\nprint(stack.peek(s))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "5\n2\n10\n" {
		t.Fatal(out.String())
	}
}

func TestStackEqualityIsRejected(t *testing.T) {
	if _, err := Compile("test.zhex", "let a: Stack<Int> = stack.new\nlet b: Stack<Int> = stack.new\nprint(a == b)\n"); err == nil {
		t.Fatal("accepted stack equality")
	} else if !strings.Contains(err.Error(), "cannot compare type Stack<Int>") {
		t.Fatal(err.Error())
	}
}

func TestStackPrintShowsIndexOrderTopLast(t *testing.T) {
	source := "let s: Stack<Int> = stack.new\nstack.push(s, 10)\nstack.push(s, 20)\nprint(s)\nlet names: Stack<String> = stack.new\nstack.push(names, \"Ada\")\nstack.push(names, \"Bo\")\nprint(names)\nlet flags: Stack<Bool> = stack.new\nstack.push(flags, true)\nstack.push(flags, false)\nprint(flags)\nlet empty: Stack<Int> = stack.new\nprint(empty)\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[10, 20]\n[\"Ada\", \"Bo\"]\n[true, false]\n[]\n" {
		t.Fatal(out.String())
	}
	session := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, nil)
	if _, err := session.Enter("let s: Stack<Int> = stack.new\nstack.push(s, 1)\nstack.push(s, 2)\n"); err != nil {
		t.Fatal(err)
	}
	echo, err := session.Enter("s\n")
	if err != nil {
		t.Fatal(err)
	}
	if echo != "stack(2 elements)" {
		t.Fatal(echo)
	}
}

func TestStackJSONEncodeArrayDecodeRejected(t *testing.T) {
	source := "let s: Stack<Int> = stack.new\nstack.push(s, 10)\nstack.push(s, 20)\nlet js = json.encode(s) try\nprint js\nlet names: Stack<String> = stack.new\nstack.push(names, \"Ada\")\nlet js2 = json.encode(names) try\nprint js2\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "[10,20]\n[\"Ada\"]\n" {
		t.Fatal(out.String())
	}
	if _, err := Compile("test.zhex", "let s: Stack<Int> = stack.new\nlet back = json.decode(\"[1]\", s) try\n"); err == nil {
		t.Fatal("accepted json.decode into a Stack target")
	} else if !strings.Contains(err.Error(), "json.decode does not support type Stack<Int>") {
		t.Fatal(err.Error())
	}
}

func TestStackExceedsTheCollectionLimit(t *testing.T) {
	source := "var s: Stack<Int> = stack.new\nfor i in range(0, 1000001)\n    s = stack.push(s, i)\n"
	err := Run("bad.zhex", source, strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("accepted a stack past the element limit")
	}
	if code, message := codeOf(t, err); code != "collection-limit" || !strings.Contains(message, "stack exceeds element limit") {
		t.Fatalf("wanted collection-limit from stack.push, got %s: %s", code, message)
	}
}

func TestStackAliasesShareOneStore(t *testing.T) {
	source := "let s: Stack<Int> = stack.new\nstack.push(s, 1)\nlet alias = s\nstack.push(alias, 2)\nprint(stack.length(s))\nprint(stack.pop(s))\nprint(stack.length(alias))\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "2\n2\n1\n" {
		t.Fatal(out.String())
	}
}

func TestStackElementsFollowTheListDomain(t *testing.T) {
	source := "let xs = [1, 2]\nlet s: Stack<List<Int>> = stack.new\nstack.push(s, xs)\nprint(stack.length(s))\nlet ys = stack.pop(s)\nprint(list.length(ys))\nprint(ys[1])\n"
	var out bytes.Buffer
	if err := Run("test.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\n2\n2\n" {
		t.Fatal(out.String())
	}
}

func TestStackWordSignatureMismatches(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   string
	}{
		{"let s: Stack<Int> = stack.new\nstack.push(s, \"x\")\n", "stack.push item requires Int"},
		{"let s: Stack<Int> = stack.new\nlet x = stack.pop(s, 2)\n", "stack.pop requires 1 arguments"},
		{"print(stack.length(5))\n", "stack.length requires a Stack"},
		{"stack.values(5)\n", "unknown word"},
		{"let s = stack.new\n", "stack.new requires an expected Stack type"},
		{"let s: Stack = stack.new\n", "unknown type"},
		{"let s: Stack<Int> = stack.new\nlet x = stack.length(s) try\n", "try requires a Result, found Int"},
		{"let s: Stack<Int> = stack.new\nfor x in s\n    print(x)\n", "collection loop requires a List, Array, or Map"},
		{"let s: Stack<Int> = stack.new\nprint(s[0] try)\n", "indexing requires a List, Array, or Map"},
		{"let s: Stack<List<Int>> = stack.new\nprint(s)\n", "print does not support type Stack<List<Int>>"},
		{"use stack\nprint(1)\n", "module name \"stack\" is reserved"},
	} {
		if _, err := Compile("test.zhex", tc.source); err == nil {
			t.Errorf("accepted %q", tc.source)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: wanted %q, got %v", tc.source, tc.want, err)
		}
	}
}
