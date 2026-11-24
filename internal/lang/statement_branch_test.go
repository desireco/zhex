package lang

import (
	"strings"
	"testing"
)

// #226: a value that nothing reads is dropped. An if or match that stands
// as an ordinary line may let its arms end with any type; wherever the
// value is read the arms must still agree, as before.

// runOK runs a program that must succeed and returns its stdout.
func runOK(t *testing.T, source string) string {
	t.Helper()
	var out strings.Builder
	if err := Run("ok.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatalf("expected a clean run, got: %v", err)
	}
	return out.String()
}

// render is the corpus's way to print a list of ints (print itself takes
// only Int, String, and Bool).
func render(xs string) string {
	return "print(text.join(list.map(" + xs + ", int.to-string(_)), \", \"))\n"
}

// The ticket's case 1: a loop body's if appends through one arm only.
func TestStatementIfArmsMayDifferInLoop(t *testing.T) {
	got := runOK(t, "let xs = [1]\nfor n in range(0, 4)\n    if n > 1\n        list.append xs, n\n"+render("xs"))
	if got != "1, 2, 3\n" {
		t.Fatalf("output %q, want 1, 2, 3", got)
	}
}

func TestStatementIfArmAgainstNothing(t *testing.T) {
	got := runOK(t, "let xs = [1]\nif xs[0] == 1\n    list.append(xs, 2)\nprint(int.to-string(list.length(xs)))\n")
	if got != "2\n" {
		t.Fatalf("output %q, want 2", got)
	}
}

func TestStatementIfArmsOfDifferentTypes(t *testing.T) {
	got := runOK(t, "if 2 > 1\n    print(1)\nelse\n    print(\"one\")\nprint(\"done\")\n")
	if got != "1\ndone\n" {
		t.Fatalf("output %q, want 1, done", got)
	}
	got = runOK(t, "if 1 > 2\n    print(1)\nelse\n    print(\"one\")\nprint(\"done\")\n")
	if got != "one\ndone\n" {
		t.Fatalf("output %q, want one, done", got)
	}
}

func TestStatementIfElifChainArmsMayDiffer(t *testing.T) {
	got := runOK(t, "let xs = [1]\nlet n = 2\nif n == 1\n    list.append(xs, 1)\nelif n == 2\n    print(42)\nelse\n    print(\"other\")\nprint(int.to-string(list.length(xs)))\n")
	if got != "42\n1\n" {
		t.Fatalf("output %q, want 42 then 1", got)
	}
}

func TestStatementIfNestedInLoopAndArm(t *testing.T) {
	got := runOK(t, "let xs = [1]\nfor n in range(0, 2)\n    if n == 0\n        if n == 0\n            list.append(xs, 2)\n        else\n            print(\"inner else\")\n    else\n        print(\"outer else\")\nprint(int.to-string(list.length(xs)))\n")
	if got != "outer else\n2\n" {
		t.Fatalf("output %q, want 2", got)
	}
	got = runOK(t, "let xs = [1]\nif xs[0] == 1\n    if xs[0] == 1\n        list.append(xs, 3)\n    else\n        print(\"no\")\nelse\n    print(\"outer\")\nprint(int.to-string(list.length(xs)))\n")
	if got != "2\n" {
		t.Fatalf("output %q, want 2", got)
	}
}

// #226 decision 3: match follows the same rule as if.
func TestStatementMatchArmsMayDiffer(t *testing.T) {
	source := "type Shape = choice\n    Dot\n    Ring(radius: Int)\n\nlet shapes = [Shape.Dot, Shape.Ring(2)]\nlet seen: List<Shape> = []\nfor s in shapes\n    match s\n        case Shape.Dot\n            print(\"dot\")\n        case Shape.Ring(radius)\n            list.append(seen, Shape.Dot)\n            print(int.to-string(radius))\nprint(int.to-string(list.length(seen)))\nprint(\"done\")\n"
	got := runOK(t, source)
	if got != "dot\n2\n1\ndone\n" {
		t.Fatalf("output %q, want dot, 2, done", got)
	}
}

// #226 decision 1: the last line of a -> Unit word is an ordinary line,
// like its other lines — the three tail kinds from the ticket.
func TestUnitWordTailIsOrdinary(t *testing.T) {
	// an if tail whose arms disagree
	got := runOK(t, "word store(xs: List<Int>, keep: Bool) -> Unit\n    if keep\n        list.append(xs, 7)\n    else\n        print(\"dropped\")\n\nlet xs = [1]\nstore(xs, true)\nstore(xs, false)\n"+render("xs"))
	if got != "dropped\n1, 7\n" {
		t.Fatalf("output %q, want dropped then 1, 7", got)
	}
	// a bare mutation call tail
	got = runOK(t, "word add(xs: List<Int>) -> Unit\n    list.append(xs, 5)\n\nlet xs = [1]\nadd(xs)\n"+render("xs"))
	if got != "1, 5\n" {
		t.Fatalf("output %q, want 1, 5", got)
	}
	// a plain value tail
	got = runOK(t, "word f() -> Unit\n    1 + 1\nf()\nprint(\"ran\")\n")
	if got != "ran\n" {
		t.Fatalf("output %q, want ran", got)
	}
}

func TestUnitWordTailIfArmsAgreeStillRuns(t *testing.T) {
	got := runOK(t, "word fill(xs: List<Int>) -> Unit\n    if xs[0] == 1\n        list.append(xs, 2)\n    else\n        list.append(xs, 3)\n\nlet xs = [1]\nfill(xs)\n"+render("xs"))
	if got != "1, 2\n" {
		t.Fatalf("output %q, want 1, 2", got)
	}
}

// Wherever the value is read the arms must still agree, as before.
func TestValuePositionBranchMismatchStillRejected(t *testing.T) {
	// binding right-hand side
	got := runError(t, "let v = if 1 == 1\n    1\nelse\n    \"one\"\nprint(v)\n")
	if !strings.Contains(got, "branch result types must match: Int and String") {
		t.Fatalf("binding RHS: missing branch mismatch: %q", got)
	}
	// assignment right-hand side
	got = runError(t, "var v = 0\nv = if 1 == 1\n    1\nelse\n    \"one\"\n")
	if !strings.Contains(got, "branch result types must match: Int and String") {
		t.Fatalf("assignment RHS: missing branch mismatch: %q", got)
	}
	// the tail of a word whose result is a value type
	got = runError(t, "word pick(flag: Bool) -> Int\n    if flag\n        1\n    else\n        \"one\"\n")
	if !strings.Contains(got, "branch result types must match: Int and String") {
		t.Fatalf("value word tail: missing branch mismatch: %q", got)
	}
	// a match in the same positions
	got = runError(t, "type Shape = choice\n    Dot\n    Ring(radius: Int)\n\nlet v = match Shape.Dot\n    case Shape.Dot\n        1\n    case Shape.Ring(radius)\n        \"ring\"\n")
	if !strings.Contains(got, "match case result types must match") {
		t.Fatalf("binding match: missing branch mismatch: %q", got)
	}
}

// #226 decision 2: a word with no declared result whose last line is an
// if with arms of different types stays an error.
func TestUnannotatedWordTailBranchMismatchStaysError(t *testing.T) {
	got := runError(t, "word pick(flag)\n    if flag\n        1\n    else\n        \"one\"\n")
	if !strings.Contains(got, "incompatible inferred types") {
		t.Fatalf("missing inference error: %q", got)
	}
}

// A bare Result stays an error everywhere, ordinary line or not.
func TestBareResultStaysRejected(t *testing.T) {
	// an ordinary statement
	got := runError(t, "text.to-int(\"x\")\n")
	if !strings.Contains(got, "Result must be handled or propagated") {
		t.Fatalf("statement: missing Result error: %q", got)
	}
	// an if arm
	got = runError(t, "if 1 == 1\n    text.to-int(\"x\")\n")
	if !strings.Contains(got, "Result must be handled or propagated") {
		t.Fatalf("arm: missing Result error: %q", got)
	}
	// a match arm
	got = runError(t, "type Shape = choice\n    Dot\n    Ring(radius: Int)\n\nlet s = Shape.Dot\nmatch s\n    case Shape.Dot\n        text.to-int(\"x\")\n    case _\n        print(\"other\")\n")
	if !strings.Contains(got, "Result must be handled or propagated") {
		t.Fatalf("match arm: missing Result error: %q", got)
	}
	// the tail of a -> Unit word
	got = runError(t, "word f() -> Unit\n    text.to-int(\"x\")\n")
	if !strings.Contains(got, "Result must be handled or propagated") {
		t.Fatalf("Unit tail: missing Result error: %q", got)
	}
}

// Result arms as the tail of a Result word stay legal.
func TestResultWordTailsStayLegal(t *testing.T) {
	got := runOK(t, "word parse(flag: Bool) -> Result<Int, Error>\n    if flag\n        ok(42)\n    else\n        err(error.new(\"bad\", \"flag down\"))\n\nprint(parse(true) try)\nprint(parse(false) catch 0)\n")
	if got != "42\n0\n" {
		t.Fatalf("output %q, want 42 then 0", got)
	}
	got = runOK(t, "type Shape = choice\n    Dot\n    Ring(radius: Int)\n\nword size(s: Shape) -> Result<Int, Error>\n    match s\n        case Shape.Dot\n            ok(0)\n        case Shape.Ring(radius)\n            ok(radius)\n\nprint(size(Shape.Ring(3)) try)\n")
	if got != "3\n" {
		t.Fatalf("output %q, want 3", got)
	}
}

// The REPL echo reads the entry's value: agreeing arms echo, disagreeing
// arms keep failing.
func TestReplEchoReadsBranchArms(t *testing.T) {
	var out strings.Builder
	session := NewSession(strings.NewReader(""), &out, nil, nil)
	echo, err := session.Enter("if 1 == 1\n    42\nelse\n    42\n")
	if err != nil || echo != "42" {
		t.Fatalf("agreeing arms: echo %q, error %v", echo, err)
	}
	_, err = session.Enter("if 1 == 1\n    42\nelse\n    \"one\"\n")
	if err == nil || !strings.Contains(err.Error(), "branch result types must match: Int and String") {
		t.Fatalf("disagreeing arms: error %v, want the branch mismatch", err)
	}
}

// The definition of an ordinary line must agree between return inference
// and the checking pass: the same statement shape is an ordinary line in a
// file's top level (nothing reads it) and a read value at the REPL prompt
// (its echo), whatever else the program makes inference look at.
func TestOrdinaryLineAgreesAcrossInferenceAndCheck(t *testing.T) {
	// The file runs: both passes treat the entry-tail if as an ordinary
	// line, even though the unannotated word makes inference walk the
	// entry too — and today both passes reject exactly this program.
	got := runOK(t, "word widen(n)\n    n\n\nlet xs = [1]\nwiden(3)\nif xs[0] == 1\n    list.append(xs, 2)\n    print(int.to-string(list.length(xs)))\nelse\n    print(\"no\")\n")
	if got != "2\n" {
		t.Fatalf("output %q, want 2", got)
	}
	// The same program typed at the REPL prompt fails: the echo reads the
	// entry's value, and here the unannotated word makes inference walk
	// the entry too — so both passes see the same line and only the echo
	// turns it into a read.
	var out strings.Builder
	session := NewSession(strings.NewReader(""), &out, nil, nil)
	source := "word widen(n)\n    n\n\nlet xs = [1]\nwiden(3)\nif xs[0] == 1\n    list.append(xs, 2)\nelse\n    print(\"no\")\n"
	_, err := session.Enter(source)
	if err == nil || !strings.Contains(err.Error(), "branch result types must match") {
		t.Fatalf("REPL entry with a mismatching tail if: error %v, want the branch mismatch", err)
	}
}
