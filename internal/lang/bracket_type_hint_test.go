package lang

import (
	"bytes"
	"strings"
	"testing"
)

// #223: two lanes hand-wrote List[String] where List<String> belongs and
// then chased a false parser bug. The bracket form in a type-annotation
// position keeps today's expected-token message and gains a hint naming
// the angle-bracket rewrite.
func TestBracketTypeHintOnLetVarAnnotations(t *testing.T) {
	got := runError(t, "let xs: List[Int] = list.new()\n")
	if !strings.Contains(got, `expected =, found "["`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "hint: types use angle brackets — write List<Int>, not List[Int]") {
		t.Fatalf("missing let-form hint: %q", got)
	}
	got = runError(t, "var ys: Array[Int] = array.new([2], 0)\n")
	if !strings.Contains(got, "write Array<Int>, not Array[Int]") {
		t.Fatalf("missing var-form hint: %q", got)
	}
	got = runError(t, "let o: Option[Int] = Option.None\n")
	if !strings.Contains(got, "write Option<Int>, not Option[Int]") {
		t.Fatalf("missing Option hint: %q", got)
	}
}

func TestBracketTypeHintOnWordParameter(t *testing.T) {
	got := runError(t, "word first(xs: List[Int]) -> Int\n    0\n")
	if !strings.Contains(got, `expected ), found "["`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "hint: types use angle brackets — write List<Int>, not List[Int]") {
		t.Fatalf("missing parameter hint: %q", got)
	}
}

func TestBracketTypeHintOnReturnType(t *testing.T) {
	got := runError(t, "word names() -> List[String]\n    list.new()\n")
	if !strings.Contains(got, `expected newline, found "["`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "hint: types use angle brackets — write List<String>, not List[String]") {
		t.Fatalf("missing return hint: %q", got)
	}
}

// Nested and two-parameter forms rewrite the whole bracketed span, and a
// bracket form nested inside a correct angle form still gets the hint.
func TestBracketTypeHintNestedAndTwoParameterForms(t *testing.T) {
	got := runError(t, "let m: Map[String, List[Int]] = {\"a\": list.new()}\n")
	if !strings.Contains(got, "hint: types use angle brackets — write Map<String, List<Int>>, not Map[String, List[Int]]") {
		t.Fatalf("missing Map hint: %q", got)
	}
	got = runError(t, "let m: Map<String, List[Int]> = {\"a\": list.new()}\n")
	if !strings.Contains(got, `expected >, found "["`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "hint: types use angle brackets — write List<Int>, not List[Int]") {
		t.Fatalf("missing inner-form hint: %q", got)
	}
}

// Record fields are type-annotation positions too, so the same hint rides
// their expected-newline failure.
func TestBracketTypeHintOnRecordField(t *testing.T) {
	got := runError(t, "type Box = record\n    items: List[Int]\n")
	if !strings.Contains(got, `expected newline, found "["`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "write List<Int>, not List[Int]") {
		t.Fatalf("missing record-field hint: %q", got)
	}
}

// Indexing, collection literals, and a bracket after a name that is not a
// built-in generic keep today's message without the hint.
func TestBracketUsesOutsideGenericTypeAnnotations(t *testing.T) {
	var out bytes.Buffer
	source := "let xs = [1, 2]\nprint(xs[0])\n"
	if err := Run("ok.zhex", source, strings.NewReader(""), &out); err != nil {
		t.Fatalf("clean program rejected: %v", err)
	}
	if out.String() != "1\n" {
		t.Fatalf("output changed: %q", out.String())
	}
	got := runError(t, "let n = 1\nprint(n[0])\n")
	if strings.Contains(got, "types use angle brackets") {
		t.Fatalf("indexing earned the hint: %q", got)
	}
	got = runError(t, "let x: Box[Int] = 1\n")
	if !strings.Contains(got, `expected =, found "["`) {
		t.Fatalf("first line changed: %q", got)
	}
	if strings.Contains(got, "hint:") {
		t.Fatalf("non-generic name earned a hint: %q", got)
	}
}
