package lang

import (
	"strings"
	"testing"
)

func runError(t *testing.T, source string) string {
	t.Helper()
	err := Run("bad.zhex", source, strings.NewReader(""), &strings.Builder{})
	if err == nil {
		t.Fatal("expected an error")
	}
	return err.Error()
}

func TestDiagnosticsShowExcerptCaretAndHyphenHint(t *testing.T) {
	got := runError(t, "let a = 3\nlet b = 4\nprint(a-b)\n")
	if !strings.HasPrefix(got, `bad.zhex:3:7: unknown name "a-b"`) {
		t.Fatalf("first line changed: %q", got)
	}
	if !strings.Contains(got, "\n    print(a-b)\n") {
		t.Fatalf("missing excerpt: %q", got)
	}
	if !strings.Contains(got, "\n          ^^^\nhint:") {
		t.Fatalf("caret does not span a-b: %q", got)
	}
	if !strings.Contains(got, `hint: "a-b" is one name`) || !strings.Contains(got, `write "a - b" with spaces`) {
		t.Fatalf("missing hyphen hint: %q", got)
	}
}

func TestDiagnosticsSuggestCloseNamesAndWords(t *testing.T) {
	if got := runError(t, "let fibo = 3\nprint(fib)\n"); !strings.Contains(got, `did you mean "fibo"?`) {
		t.Fatalf("missing name suggestion: %q", got)
	}
	if got := runError(t, "prnt(\"x\")\n"); !strings.Contains(got, `did you mean "print"?`) {
		t.Fatalf("missing word suggestion: %q", got)
	}
}

func TestDiagnosticsSuggestTryForResultOperands(t *testing.T) {
	got := runError(t, "let n = text.to-int(\"1\") try\nprint(n + text.to-int(\"2\"))\n")
	if !strings.Contains(got, "incompatible operand types Int and Result<Int, ParseError>") {
		t.Fatalf("wrong diagnostic: %q", got)
	}
	if !strings.Contains(got, "add postfix try") {
		t.Fatalf("missing try hint: %q", got)
	}
	// Every Result type earns the hint, whatever operation produced it.
	for _, source := range []string{"print(file.open(\"x\"))\n", "print(console.arg(0))\n"} {
		if got := runError(t, source); !strings.Contains(got, "add postfix try") {
			t.Fatalf("missing try hint for %q: %q", source, got)
		}
	}
}

// TestEveryResultTypeUnwrapsRendersAndHints pins the one Result mapping from
// three sides: resultInner maps each Result const to its success type,
// postfix try accepts it, traceValue renders it, and the try hint fires —
// a new const inherits all three by construction and cannot skip an arm.
func TestEveryResultTypeUnwrapsRendersAndHints(t *testing.T) {
	for _, tc := range []struct {
		typ      valueType
		producer string // empty when no host word returns the type yet
		inner    valueType
	}{
		{intResultType, "", intType},
		{stringResultType, "let x = read-line(\"\") try\n", stringType},
		{intParseResultType, "let x = text.to-int(\"1\") try\n", intType},
		{bufferFileResultType, "let x = file.read-lines(\"/x\") try\n", bufferType},
		{stringBufferResultType, "let x = buffer.line(buffer.new(), 0) try\n", stringType},
		{unitBufferResultType, "let x = buffer.set-line(buffer.new(), 0, \"a\") try\n", unitType},
		{stringFileResultType, "let x = file.read-all(\"/x\") try\n", stringType},
		{fileResultType, "let x = file.open(\"/x\") try\n", fileType},
	} {
		t.Run(string(tc.typ), func(t *testing.T) {
			if inner, ok := resultInner(tc.typ); !ok || inner != tc.inner {
				t.Fatalf("resultInner(%s) = (%s, %v), want (%s, true)", tc.typ, inner, ok, tc.inner)
			}
			if tryHint(tc.typ) == "" {
				t.Fatalf("try hint must fire for %s", tc.typ)
			}
			success := value{}
			switch tc.inner {
			case bufferType:
				success.buffer = &bufferValue{}
			case fileType:
				success.file = &fileValue{}
			}
			if got := traceValue(value{result: &inputResult{value: success}}, tc.typ); !strings.HasPrefix(got, "ok(") {
				t.Fatalf("success render for %s: %q", tc.typ, got)
			}
			failure := value{result: &inputResult{err: errorAt(position{}, CodeUnsupportedCode, "boom")}}
			if got := traceValue(failure, tc.typ); got != `err("boom")` {
				t.Fatalf("failure render for %s: %q", tc.typ, got)
			}
			if tc.producer != "" {
				if _, err := Compile("ok.zhex", tc.producer); err != nil {
					t.Fatalf("try must accept %s: %v", tc.typ, err)
				}
			}
		})
	}
}

// TestProvenanceNarration pins the "; <name> comes from <origin>" suffix:
// when the checker knows where a mismatched value was defined, the message
// says so. Direct calls keep the origin on-screen, so no narration.
func TestProvenanceNarration(t *testing.T) {
	cases := []struct {
		name   string
		source string
		prefix string
	}{
		{"word result", "word double(n: Int) -> Int\n    n + n\nlet d = double(2)\nprint(d + \"a\")\n",
			`bad.zhex:4:9: operator "+" has incompatible operand types Int and String; d comes from double`},
		{"assignment", "let m = text.length(\"text\")\nvar a = \"hi\"\na = m\n", `bad.zhex:3:1: assignment type Int does not match String; m comes from text.length`},
		{"argument", "let m = text.length(\"text\")\nprint(text.upper(m))\n", `bad.zhex:2:7: argument 1 of text.upper requires type String, found Int; m comes from text.length`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runError(t, tc.source)
			if !strings.HasPrefix(got, tc.prefix) {
				t.Fatalf("missing narration: %q", got)
			}
		})
	}
	if got := runError(t, "print(text.to-int(\"1\") + 2)\n"); strings.Contains(got, "comes from") {
		t.Fatalf("direct call must not narrate: %q", got)
	}
	// Assignment refreshes the origin: narration tracks the CURRENT value.
	if got := runError(t, "var a = read-line(\"x\") try\na = \"hello\"\nprint(a == 1)\n"); strings.Contains(got, "comes from") {
		t.Fatalf("reassigned-to-literal must not narrate: %q", got)
	}
	if got := runError(t, "word double(n: Int) -> Int\n    n + n\nvar a = double(2)\na = text.to-int(\"2\") try\nprint(a == \"x\")\n"); !strings.Contains(got, "a comes from text.to-int") {
		t.Fatalf("reassignment must narrate the new origin: %q", got)
	}
}
func TestRuntimeDiagnosticsShowExcerpt(t *testing.T) {
	got := runError(t, "print(1 / 0)\n")
	if !strings.HasPrefix(got, "bad.zhex:1:") || !strings.Contains(got, "division or remainder by zero") {
		t.Fatalf("wrong runtime diagnostic: %q", got)
	}
	if !strings.Contains(got, "\n    print(1 / 0)\n") || !strings.Contains(got, "^") {
		t.Fatalf("missing excerpt or caret: %q", got)
	}
}

// TestCaretSpansWholeExpressions pins the multi-caret rungs: the caret
// underlines the whole offending expression while the first line keeps
// reporting the start column.
func TestCaretSpansWholeExpressions(t *testing.T) {
	cases := []struct {
		name                string
		source              string
		wantFirstLinePrefix string
		wantCaretLine       string
	}{
		{"binary", "print(1 + \"a\")\n",
			`bad.zhex:1:9: operator "+" has incompatible operand types Int and String`,
			"\n          ^^^^^^^"},
		{"argument", "print(int.to-string(\"x\"))\n",
			`bad.zhex:1:7: argument 1 of int.to-string requires type Int, found String`,
			"\n                        ^^^"},
		{"unary", "print(not 1)\n",
			`bad.zhex:1:7: operator "not" requires type Bool, found Int`,
			"\n          ^^^^^"},
		{"condition", "if read-line(\"x\")\n    print(1)\n",
			`bad.zhex:1:4: condition must have type Bool, found Result<String, InputError>`,
			"\n       ^^^^^^^^^^^^^^"},
		{"try input", "let x = 1 try\nprint(x)\n",
			`bad.zhex:1:11: try requires a Result, found Int`,
			"\n            ^^^^^"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runError(t, tc.source)
			if !strings.HasPrefix(got, tc.wantFirstLinePrefix) {
				t.Fatalf("first line changed: %q", got)
			}
			if !strings.Contains(got, tc.wantCaretLine+"\nhint:") && !strings.HasSuffix(got, tc.wantCaretLine) {
				t.Fatalf("missing caret rung %q: %q", tc.wantCaretLine, got)
			}
		})
	}
}

func TestResultTypeParsingHandlesNestedGenericCommas(t *testing.T) {
	typ := valueType("Result<Map<String, Int>, ParseError>")
	if inner, ok := resultInner(typ); !ok || inner != "Map<String, Int>" {
		t.Fatalf("inner %q, ok %t", inner, ok)
	}
	if family := resultFamily(typ); family != "ParseError" {
		t.Fatalf("family %q", family)
	}
}

func TestUnknownPrintNameSuggestsCallSyntax(t *testing.T) {
	got := runError(t, "print1+1\n")
	if !strings.HasPrefix(got, `bad.zhex:1:1: unknown name "print1"`) || !strings.Contains(got, "write print(expression)") {
		t.Fatalf("missing call syntax hint: %s", got)
	}
}

func TestPrintWithWhitespaceSuggestsParentheses(t *testing.T) {
	got := runError(t, "print 1 + 1\n")
	if !strings.HasPrefix(got, `bad.zhex:1:7: expected newline, found "1"`) || !strings.Contains(got, "write print(expression)") {
		t.Fatalf("missing call syntax hint: %s", got)
	}
}

func TestBareKnownWordSuggestsCall(t *testing.T) {
	for _, tc := range []struct{ source, first, hint string }{
		{"word bye() -> Unit\n    print(1)\nbye\n", `bad.zhex:3:1: unknown name "bye"`, "call bye()"},
		{"word greet(name: String) -> Unit\n    print(name)\ngreet\n", `bad.zhex:3:1: unknown name "greet"`, "call greet(...)"},
	} {
		got := runError(t, tc.source)
		if !strings.HasPrefix(got, tc.first) || !strings.Contains(got, tc.hint) || strings.Contains(got, `did you mean`) {
			t.Errorf("unexpected diagnostic: %s", got)
		}
	}
	var out strings.Builder
	if err := Run("test.zhex", "word bye() -> Unit\n    print(1)\nlet bye = 3\nprint(bye)\n", strings.NewReader(""), &out); err != nil || out.String() != "3\n" {
		t.Fatalf("binding shadow: %q %v", out.String(), err)
	}
}

func TestBareBuiltinSuggestsCallSyntax(t *testing.T) {
	for _, tc := range []struct{ source, hint string }{{"instant.now\n", "call instant.now()"}, {"text.lower\n", "call text.lower(...)"}} {
		got := runError(t, tc.source)
		if !strings.Contains(got, tc.hint) {
			t.Errorf("missing hint: %s", got)
		}
	}
}
