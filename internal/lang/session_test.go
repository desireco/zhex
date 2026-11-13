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
			session := NewSession(strings.NewReader(""), &out, nil, nil)
			echo, err := session.Enter(tc.source + "\n")
			if err != nil || echo != tc.echo || out.String() != tc.output {
				t.Fatalf("echo %q, output %q, error %v", echo, out.String(), err)
			}
		})
	}
}

func TestSessionKeepsDefinitionsAcrossEntries(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(strings.NewReader(""), &out, nil, nil)
	if _, err := session.Enter("1 / 0"); err == nil || err.(*diagnostic).code != CodeDivisionZero {
		t.Fatalf("expected division-zero, got %v", err)
	}
	if echo, err := session.Enter("1 + 1"); err != nil || echo != "2" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionRequiresHandlingResultInSameEntry(t *testing.T) {
	session := NewSession(strings.NewReader("Ada\n"), io.Discard, nil, nil)
	if _, err := session.Enter("read-line(\"\")"); err == nil {
		t.Fatal("bare Result accepted")
	}
	if _, err := session.Enter("let pending = read-line(\"\")"); err == nil {
		t.Fatal("unhandled Result persisted")
	}
	if echo, err := session.Enter("let pending = read-line(\"\")\nlet name = pending try\nname\n"); err != nil || echo != "Ada" {
		t.Fatalf("echo %q, error %v", echo, err)
	}
}

func TestSessionWordsListsSignatures(t *testing.T) {
	var out bytes.Buffer
	session := NewSession(strings.NewReader(""), &out, nil, nil)
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
	session := NewSession(input, &out, nil, nil)
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

func TestSessionUseAdoptsModule(t *testing.T) {
	var out strings.Builder
	session := NewSession(strings.NewReader(""), &out, nil, mapResolver(map[string]string{"math.zhex": privateMathModule}))
	if _, err := session.Enter("use math\n"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Fatalf("adoption must be silent: %q", out.String())
	}
	if echo, err := session.Enter("math.gcd(54, 18)\n"); err != nil || echo != "18" {
		t.Fatalf("echo %q, err %v", echo, err)
	}
}

func TestSessionDoubleUseIsNoOp(t *testing.T) {
	var out strings.Builder
	session := NewSession(strings.NewReader(""), &out, nil, mapResolver(map[string]string{"math.zhex": privateMathModule}))
	if _, err := session.Enter("use math\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("use math\n"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "" {
		t.Fatalf("second use must print nothing: %q", out.String())
	}
	if n := strings.Count(strings.Join(session.Words(), "\n"), "math.gcd"); n != 1 {
		t.Fatalf("math.gcd listed %d times, want 1: %v", n, session.Words())
	}
}

func TestSessionWordsListQualifiedSignatures(t *testing.T) {
	session := NewSession(strings.NewReader(""), &strings.Builder{}, nil, mapResolver(map[string]string{"math.zhex": privateMathModule}))
	if _, err := session.Enter("use math\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("word double(n: Int) -> Int\n    n * 2\n"); err != nil {
		t.Fatal(err)
	}
	words := strings.Join(session.Words(), "\n")
	for _, want := range []string{"math.gcd(a: Int, b: Int) -> Int", "double(n: Int) -> Int"} {
		if !strings.Contains(words, want) {
			t.Fatalf("words %q misses %q", words, want)
		}
	}
	if strings.Contains(words, "_wrap") {
		t.Fatalf("module-private words must not be listed: %q", words)
	}
}

func TestSessionPrivateUseFromLaterEntry(t *testing.T) {
	session := NewSession(strings.NewReader(""), &strings.Builder{}, nil, mapResolver(map[string]string{"math.zhex": privateMathModule}))
	if _, err := session.Enter("use math\n"); err != nil {
		t.Fatal(err)
	}
	_, err := session.Enter("math._wrap(1)\n")
	if err == nil {
		t.Fatal("expected private-use")
	}
	if !strings.HasPrefix(err.Error(), `repl:1:1: "_wrap" is private to module "math"`) {
		t.Fatalf("first line changed: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "hint: remove the leading underscore in math.zhex to export it") {
		t.Fatalf("missing hint: %q", err.Error())
	}
	if echo, err := session.Enter("1 + 1\n"); err != nil || echo != "2" {
		t.Fatalf("session must survive: echo %q, err %v", echo, err)
	}
}

func TestSessionAdoptionReplacesDottedWordWithNote(t *testing.T) {
	var out strings.Builder
	session := NewSession(strings.NewReader(""), &out, nil, mapResolver(map[string]string{"math.zhex": runtimeMathModule}))
	if _, err := session.Enter("word math.halve(n: Int) -> Int\n    n\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("use math\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "redefined math.halve\n") {
		t.Fatalf("missing redefined note: %q", out.String())
	}
	if echo, err := session.Enter("math.halve(84)\n"); err != nil || echo != "42" {
		t.Fatalf("the module's word must win: echo %q, err %v", echo, err)
	}
}

func TestSessionDottedWordReplacesAdoptedWithNote(t *testing.T) {
	var out strings.Builder
	session := NewSession(strings.NewReader(""), &out, nil, mapResolver(map[string]string{"math.zhex": runtimeMathModule}))
	if _, err := session.Enter("use math\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Enter("word math.halve(n: Int) -> Int\n    n + 1\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "redefined math.halve\n") {
		t.Fatalf("missing redefined note: %q", out.String())
	}
	if echo, err := session.Enter("math.halve(84)\n"); err != nil || echo != "85" {
		t.Fatalf("the session word must win: echo %q, err %v", echo, err)
	}
}

func TestSessionUseMissingNamesWorkingDirectory(t *testing.T) {
	session := NewSession(strings.NewReader(""), &strings.Builder{}, nil, mapResolver(nil))
	_, err := session.Enter("use math\n")
	if err == nil {
		t.Fatal("expected use-missing")
	}
	if !strings.HasPrefix(err.Error(), `repl:1:1: module "math" not found; searched math.zhex`) {
		t.Fatalf("first line changed: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "hint: create math.zhex in the working directory, or fix the module name") {
		t.Fatalf("missing hint: %q", err.Error())
	}
	if len(session.Words()) != 0 {
		t.Fatalf("nothing must be adopted: %v", session.Words())
	}
	if echo, err := session.Enter("1 + 1\n"); err != nil || echo != "2" {
		t.Fatalf("session must survive: echo %q, err %v", echo, err)
	}
}

func TestSessionAdoptionChecksModuleBodies(t *testing.T) {
	session := NewSession(strings.NewReader(""), &strings.Builder{}, nil, mapResolver(map[string]string{
		"math.zhex": "word bad() -> Int\n    true\n",
	}))
	_, err := session.Enter("use math\n")
	if err == nil || !strings.HasPrefix(err.Error(), `math.zhex:1:6: word "bad" return type is Bool; declared Int`) {
		t.Fatalf("module body must be checked: %v", err)
	}
	if len(session.Words()) != 0 {
		t.Fatalf("nothing must be adopted: %v", session.Words())
	}
}

func TestSessionRuntimeErrorInModuleWordAnnotatesModuleSource(t *testing.T) {
	session := NewSession(strings.NewReader(""), &strings.Builder{}, nil, mapResolver(map[string]string{"math.zhex": runtimeMathModule}))
	if _, err := session.Enter("use math\n"); err != nil {
		t.Fatal(err)
	}
	_, err := session.Enter("math.boom()\n")
	if err == nil || !strings.HasPrefix(err.Error(), "math.zhex:2:7: division or remainder by zero") {
		t.Fatalf("module runtime error must annotate math.zhex: %v", err)
	}
	if !strings.Contains(err.Error(), "\n        1 / 0\n") {
		t.Fatalf("excerpt must come from the module file: %q", err.Error())
	}
}

// F1: a module's own imports stay private to it in the session too; naming
// the module directly later adopts it without reloading.
func TestSessionTransitiveUseDoesNotLeak(t *testing.T) {
	files := map[string]string{
		"a.zhex": "use b\nword f(n: Int) -> Int\n    b.g(n)\n",
		"b.zhex": "word g(n: Int) -> Int\n    n\n",
	}
	session := NewSession(strings.NewReader(""), &strings.Builder{}, nil, mapResolver(files))
	if _, err := session.Enter("use a\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := session.Enter("a.f(2)\n"); err != nil || echo != "2" {
		t.Fatalf("direct module must work: echo %q, err %v", echo, err)
	}
	_, err := session.Enter("b.g(2)\n")
	if err == nil || !strings.Contains(err.Error(), `unknown word "b.g"`) {
		t.Fatalf("transitive module must not leak: %v", err)
	}
	words := strings.Join(session.Words(), "\n")
	if !strings.Contains(words, "a.f(n: Int) -> Int") || strings.Contains(words, "b.g") {
		t.Fatalf("words must list only directly used modules: %q", words)
	}
	// The trap the review warned about: b is already loaded, so adoption must
	// merge from the module tables, not from this entry's freshly loaded set.
	if _, err := session.Enter("use b\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := session.Enter("b.g(2)\n"); err != nil || echo != "2" {
		t.Fatalf("use b must adopt the already-loaded module: echo %q, err %v", echo, err)
	}
	words = strings.Join(session.Words(), "\n")
	if !strings.Contains(words, "b.g(n: Int) -> Int") {
		t.Fatalf("words after use b: %q", words)
	}
}

// F2: an entry that adopts a module and then fails at runtime adopts
// nothing, and the session survives.
func TestSessionFailedUseRollsBackAdoption(t *testing.T) {
	session := NewSession(strings.NewReader(""), &strings.Builder{}, nil, mathFiles(runtimeMathModule))
	_, err := session.Enter("use math\nprint(math.boom())\n")
	if err == nil || !strings.Contains(err.Error(), "division or remainder by zero") {
		t.Fatalf("wanted the runtime error: %v", err)
	}
	if _, err := session.Enter("math.gcd(54, 18)\n"); err == nil || !strings.Contains(err.Error(), `unknown word "math.gcd"`) {
		t.Fatalf("failed entry must adopt nothing: %v", err)
	}
	if len(session.Words()) != 0 {
		t.Fatalf("words after failed adoption: %v", session.Words())
	}
	if echo, err := session.Enter("1 + 1\n"); err != nil || echo != "2" {
		t.Fatalf("session must survive: echo %q, err %v", echo, err)
	}
}

func TestSessionNestedUseMissing(t *testing.T) {
	session := NewSession(strings.NewReader(""), &strings.Builder{}, nil, mapResolver(map[string]string{
		"a.zhex": "use b\nword f(n: Int) -> Int\n    n\n",
	}))
	_, err := session.Enter("use a\n")
	if err == nil || !strings.HasPrefix(err.Error(), `a.zhex:1:1: module "b" not found; searched b.zhex`) {
		t.Fatalf("nested missing module must report use-missing: %v", err)
	}
	if len(session.Words()) != 0 {
		t.Fatalf("nothing must be adopted: %v", session.Words())
	}
	if echo, err := session.Enter("1 + 1\n"); err != nil || echo != "2" {
		t.Fatalf("session must survive: echo %q, err %v", echo, err)
	}
}
