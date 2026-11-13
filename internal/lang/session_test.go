package lang

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestIsIncomplete(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         bool
	}{
		{"word header", "word double(n: Int) -> Int", true},
		{"if header", "if true", true},
		{"for header", "for n in range(1, 5)", true},
		{"flow header", "flow 4", true},
		{"dangling operator", "let x = 1 +", false},
		{"open parenthesis", "let x = (1", false},
		{"missing name", "let = 1", false},
		{"unterminated string", "let x = \"oops", false},
		{"top-level indent", "    1", false},
		{"valid program", "let x = 1\nprint(x)\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile("repl", tc.source)
			if IsIncomplete(err) != tc.want {
				t.Fatalf("IsIncomplete(%v) = %t, want %t", err, IsIncomplete(err), tc.want)
			}
		})
	}
}

func TestSessionEnterEchoes(t *testing.T) {
	for _, tc := range []struct {
		name, source, echo, output string
	}{
		{"integer sum", "1 + 2", "3", ""},
		{"string", "\"hi\"", "hi", ""},
		{"bool", "true", "true", ""},
		{"print", "print(2)", "", "2\n"},
		{"let", "let x = 5", "", ""},
		{"word definition", "word id(n: Int) -> Int\n    n\n", "", ""},
		// Multi-statement entries echo the LAST statement's value only.
		{"last statement wins", "let a = 2\na * 3", "6", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			session := NewSession(strings.NewReader(""), &out, nil)
			echo, err := session.Enter(tc.source + "\n")
			if err != nil || echo != tc.echo || out.String() != tc.output {
				t.Fatalf("echo %q, output %q, error %v", echo, out.String(), err)
			}
		})
	}
}

func TestSessionKeepsDefinitionsAcrossEntries(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	if echo, err := session.Enter("word double(n: Int) -> Int\n    n + n\n"); err != nil || echo != "" {
		t.Fatalf("define: echo %q, error %v", echo, err)
	}
	if echo, err := session.Enter("double(21)"); err != nil || echo != "42" {
		t.Fatalf("call: echo %q, error %v", echo, err)
	}
	if echo, err := session.Enter("let y = 2"); err != nil || echo != "" {
		t.Fatalf("let: echo %q, error %v", echo, err)
	}
	if echo, err := session.Enter("y * 3"); err != nil || echo != "6" {
		t.Fatalf("use: echo %q, error %v", echo, err)
	}
}

func TestSessionWordRedefinitionReplacesWithNote(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	if _, err := session.Enter("word double(n: Int) -> Int\n    n + n\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("word double(n: Int) -> Int\n    n * 3\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "redefined double\n") {
		t.Fatalf("output %q misses redefinition note", out.String())
	}
	if echo, err := session.Enter("double(2)"); err != nil || echo != "6" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionRejectsHostWordShadowing(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	_, err := session.Enter("word print(n: Int) -> Int\n    n\n")
	if err == nil || err.(*diagnostic).code != CodeDuplicateWord {
		t.Fatalf("expected duplicate-word, got %v", err)
	}
	if echo, err := session.Enter("print(3)"); err != nil || echo != "" || out.String() != "3\n" {
		t.Fatalf("echo %q, output %q, error %v", echo, out.String(), err)
	}
}

func TestSessionFailedEntryDefinesNothing(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	if _, err := session.Enter("let x = missing(1)"); err == nil || err.(*diagnostic).code != CodeUnknownWord {
		t.Fatalf("expected unknown-word, got %v", err)
	}
	if _, err := session.Enter("x"); err == nil || err.(*diagnostic).code != CodeUnknownName {
		t.Fatalf("expected unknown-name for x, got %v", err)
	}
	if echo, err := session.Enter("1 + 1"); err != nil || echo != "2" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionValueRedefinitionShadows(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	for _, source := range []string{"let x = 1", "let x = \"two\""} {
		if _, err := session.Enter(source); err != nil {
			t.Fatal(err)
		}
	}
	if echo, err := session.Enter("x"); err != nil || echo != "two" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionVarAssignmentPersists(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	for _, source := range []string{"var count = 1", "count = 2"} {
		if _, err := session.Enter(source); err != nil {
			t.Fatal(err)
		}
	}
	if echo, err := session.Enter("count"); err != nil || echo != "2" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionDuplicateWordsInOneEntry(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	source := "word f(n: Int) -> Int\n    n\nword f(n: Int) -> Int\n    n\n"
	if _, err := session.Enter(source); err == nil || err.(*diagnostic).code != CodeDuplicateWord {
		t.Fatalf("expected duplicate-word, got %v", err)
	}
	if words := session.Words(); len(words) != 0 {
		t.Fatalf("session adopted %q from a failed entry", words)
	}
}

func TestSessionRuntimeErrorKeepsEarlierEffects(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	if _, err := session.Enter("var n = 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("n = 5\n1 / 0"); err == nil || err.(*diagnostic).code != CodeDivisionZero {
		t.Fatalf("expected division-zero, got %v", err)
	}
	if echo, err := session.Enter("n"); err != nil || echo != "5" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionRuntimeErrorKeepsSession(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	if _, err := session.Enter("1 / 0"); err == nil || err.(*diagnostic).code != CodeDivisionZero {
		t.Fatalf("expected division-zero, got %v", err)
	}
	if echo, err := session.Enter("1 + 1"); err != nil || echo != "2" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionResultEchoDoesNotPanic(t *testing.T) {
	session := NewSession(strings.NewReader("Ada\n"), io.Discard, nil)
	if echo, err := session.Enter("read-line(\"\")"); err != nil || echo != "ok(\"Ada\")" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
	empty := NewSession(strings.NewReader(""), io.Discard, nil)
	if echo, err := empty.Enter("read-line(\"\")"); err != nil || !strings.HasPrefix(echo, "err(") {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionWordsListsSignatures(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil)
	if words := session.Words(); len(words) != 0 {
		t.Fatalf("words %q", words)
	}
	if _, err := session.Enter("word add(a: Int, b: Int) -> Int\n    a + b\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("word double(n: Int) -> Int\n    n + n\n"); err != nil {
		t.Fatal(err)
	}
	want := []string{"add(a: Int, b: Int) -> Int", "double(n: Int) -> Int"}
	if words := session.Words(); len(words) != 2 || words[0] != want[0] || words[1] != want[1] {
		t.Fatalf("words %q, want %q", words, want)
	}
}

func TestSessionSharesBufferedInput(t *testing.T) {
	input := bufio.NewReader(strings.NewReader("command\nAda\nremaining\n"))
	if _, err := input.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	session := NewSession(input, &out, nil)
	if _, err := session.Enter("let name = read-line(\"Name: \") try\nprint(name)\n"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "Name: Ada\n" {
		t.Fatalf("output %q", out.String())
	}
	remaining, err := input.ReadString('\n')
	if err != nil || remaining != "remaining\n" {
		t.Fatalf("remaining input %q, error %v", remaining, err)
	}
}
