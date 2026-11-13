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
	got := runError(t, "let n = text.to-int(\"1\") try\nlet m = text.to-int(\"2\")\nprint(n + m)\n")
	if !strings.Contains(got, "incompatible operand types Int and Result<Int, ParseError>") {
		t.Fatalf("wrong diagnostic: %q", got)
	}
	if !strings.Contains(got, "add postfix try") {
		t.Fatalf("missing try hint: %q", got)
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
		{"result operand narrated first", "let n = text.to-int(\"1\") try\nlet m = text.to-int(\"2\")\nprint(n + m)\n",
			`bad.zhex:3:9: operator "+" has incompatible operand types Int and Result<Int, ParseError>; m comes from text.to-int`},
		{"condition", "let line = read-line(\"x\")\nif line\n    print(1)\n",
			`bad.zhex:2:4: condition must have type Bool, found Result<String, InputError>; line comes from read-line`},
		{"print", "let line = read-line(\"x\")\nprint(line)\n",
			`bad.zhex:2:1: print does not support type Result<String, InputError>; line comes from read-line`},
		{"word result", "word double(n: Int) -> Int\n    n + n\nlet d = double(2)\nprint(d + \"a\")\n",
			`bad.zhex:4:9: operator "+" has incompatible operand types Int and String; d comes from double`},
		{"assignment", "let m = text.to-int(\"2\")\nvar a = 1\na = m\n",
			`bad.zhex:3:1: assignment type Result<Int, ParseError> does not match Int; m comes from text.to-int`},
		{"argument", "let m = text.to-int(\"2\")\nprint(int.to-string(m))\n",
			`bad.zhex:2:7: argument 1 of int.to-string requires type Int, found Result<Int, ParseError>; m comes from text.to-int`},
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
			`bad.zhex:1:11: try requires an input Result, found Int`,
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
