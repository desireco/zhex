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
	if !strings.Contains(got, "\n    print(a-b)\n") || !strings.Contains(got, "\n          ^") {
		t.Fatalf("missing excerpt or caret: %q", got)
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
	got := runError(t, "let n = console.read-int(\"N: \") try\nlet m = console.read-int(\"M: \")\nprint(n + m)\n")
	if !strings.Contains(got, "incompatible operand types Int and Result<Int, InputError>") {
		t.Fatalf("wrong diagnostic: %q", got)
	}
	if !strings.Contains(got, "add postfix try") {
		t.Fatalf("missing try hint: %q", got)
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
