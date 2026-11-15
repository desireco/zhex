package lang

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The repl.* seam is per-run session machinery for the #65 dogfood prototype:
// it is registered by RegisterReplSeam and never appears in hostWords, the
// catalog, or a plain compile.

func seamSession(output *bytes.Buffer) *Session {
	s := NewSession(strings.NewReader(""), output, nil, nil)
	s.SetDiagnostics(output)
	s.RegisterReplSeam()
	return s
}

// zstring renders a Zhex string literal for embedding entries in test source.
func zstring(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return `"` + replacer.Replace(text) + `"`
}

func TestReplSeamStaysOutOfPlainPrograms(t *testing.T) {
	if _, err := Compile("t.zhex", "print(repl.words())"); err == nil {
		t.Fatal("plain compile accepted repl.words")
	}
	s := NewSession(strings.NewReader(""), &bytes.Buffer{}, nil, nil)
	if _, err := s.Enter("repl.words()"); err == nil {
		t.Fatal("unregistered session accepted repl.words")
	}
	if HostWord("repl.enter") {
		t.Fatal("repl.enter registered as a host word")
	}
	for _, name := range hostWords {
		if strings.HasPrefix(name, "repl.") {
			t.Fatalf("hostWords gained %q", name)
		}
	}
}

func TestReplSeamWordsListStaysClean(t *testing.T) {
	s := seamSession(&bytes.Buffer{})
	if _, err := s.Enter("word bye() -> Unit\n    console.exit(0)\n"); err != nil {
		t.Fatal(err)
	}
	sigs := s.Words()
	if len(sigs) != 1 || sigs[0] != "bye() -> Unit" {
		t.Fatalf("words list %q", sigs)
	}
}

func TestReplSeamPerRunSessionsAreIndependent(t *testing.T) {
	first, second := seamSession(&bytes.Buffer{}), seamSession(&bytes.Buffer{})
	if _, err := first.Enter("word mine() -> Int\n    1\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Enter("mine()"); err == nil {
		t.Fatal("word leaked into a second session")
	}
}

func TestReplSeamEnterOutcomes(t *testing.T) {
	cases := []struct {
		name  string
		entry string
	}{
		{"ran echo", "1 + 1"},
		{"ran unit", "print(\"x\")"},
		{"failed check error", "let x = \"a\" + true"},
		{"runtime error keeps effects", "print(\"a\")\n1 / 0"},
		{"session state visible", "word helper() -> Int\n    5\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A twin session reproduces the outcome the seam entry will see,
			// so the twin's output plus its Enter result is the expectation.
			var twinOut bytes.Buffer
			twin := seamSession(&twinOut)
			echo, err := twin.Enter(tc.entry)
			want := twinOut.String()
			if err != nil {
				want += err.Error() + "\n"
			} else if echo != "" {
				want += echo + "\n"
			}
			var out bytes.Buffer
			s := seamSession(&out)
			matcher := "match repl.enter(" + zstring(tc.entry) + ")\n" +
				"    case EnterResult.Ran(echo)\n        if echo != \"\"\n            print(echo)\n" +
				"    case EnterResult.Failed(report)\n        print(report)\n" +
				"    case EnterResult.Exited(status)\n        print(status)\n"
			if _, err := s.Enter(matcher); err != nil {
				t.Fatal(err)
			}
			if out.String() != want {
				t.Fatalf("seam output %q, want %q", out.String(), want)
			}
		})
	}
}

func TestReplSeamEnterRedefinedNote(t *testing.T) {
	var out bytes.Buffer
	s := seamSession(&out)
	if _, err := s.Enter("word f() -> Int\n    1\n"); err != nil {
		t.Fatal(err)
	}
	matcher := "match repl.enter(" + zstring("word f() -> Int\n    2\n") + ")\n" +
		"    case EnterResult.Ran(echo)\n        if echo != \"\"\n            print(echo)\n" +
		"    case EnterResult.Failed(report)\n        print(report)\n" +
		"    case EnterResult.Exited(status)\n        print(status)\n"
	if _, err := s.Enter(matcher); err != nil {
		t.Fatal(err)
	}
	// Enter itself writes the redefined note to the session output.
	if out.String() != "redefined f\n" {
		t.Fatalf("output %q", out.String())
	}
}

func TestReplSeamEnterExitedPropagatesStatus(t *testing.T) {
	for _, status := range []int{0, 3} {
		var out bytes.Buffer
		s := seamSession(&out)
		source := "match repl.enter(" + zstring("console.exit("+strconv.Itoa(status)+")") + ")\n" +
			"    case EnterResult.Ran(echo)\n        print(echo)\n" +
			"    case EnterResult.Failed(report)\n        print(report)\n" +
			"    case EnterResult.Exited(code)\n        console.exit(code)\n"
		_, err := s.Enter(source)
		var exit *ExitError
		if !errors.As(err, &exit) || exit.Status != status {
			t.Fatalf("status %d: got %v", status, err)
		}
		if out.Len() != 0 {
			t.Fatalf("exit printed %q", out.String())
		}
	}
}

func TestReplSeamConsoleFailKeepsDiagnosticsWriter(t *testing.T) {
	var out, errs bytes.Buffer
	s := NewSession(strings.NewReader(""), &out, nil, nil)
	s.SetDiagnostics(&errs)
	s.RegisterReplSeam()
	source := "match repl.enter(" + zstring("console.fail(\"bad\")") + ")\n" +
		"    case EnterResult.Ran(echo)\n        print(echo)\n" +
		"    case EnterResult.Failed(report)\n        print(report)\n" +
		"    case EnterResult.Exited(code)\n        console.exit(code)\n"
	_, err := s.Enter(source)
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Status != 1 {
		t.Fatalf("got %v", err)
	}
	if errs.String() != "bad\n" {
		t.Fatalf("diagnostics %q", errs.String())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout %q", out.String())
	}
}

func TestReplSeamProbes(t *testing.T) {
	sources := []string{
		"word f() -> Int\n",
		"1 + 1\n",
		"let x = [1,\n",
		"let x = [1]\n",
		"word f() -> Int\n    42\n",
	}
	for _, src := range sources {
		_, cerr := Compile("repl", src)
		diagnosis := ""
		if cerr != nil {
			diagnosis = cerr.Error()
		}
		_, open := DelimiterIndent(src, cerr)
		want := strconv.FormatBool(IsIncomplete(cerr)) + "\n" +
			strconv.FormatBool(open) + "\n" + diagnosis + "\n"
		var out bytes.Buffer
		s := seamSession(&out)
		entry := "print(repl.incomplete(" + zstring(src) + "))\n" +
			"print(repl.open-delimiter(" + zstring(src) + "))\n" +
			"print(repl.diagnose(" + zstring(src) + "))\n"
		if _, err := s.Enter(entry); err != nil {
			t.Fatal(err)
		}
		if out.String() != want {
			t.Fatalf("%q: probes gave %q, want %q", src, out.String(), want)
		}
	}
}

func TestReplSeamSaveLoadRoundTripInProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.zhex")
	var out bytes.Buffer
	s := seamSession(&out)
	for _, entry := range []string{
		"word double(n: Int) -> Int\n    n * 2\n",
		"let base = 21\n",
		"let saved = repl.save(" + zstring(path) + ") try\n",
	} {
		if _, err := s.Enter(entry); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("save did not write %s: %v", path, err)
	}
	fresh := seamSession(&bytes.Buffer{})
	if _, err := fresh.Enter("let loaded = repl.load(" + zstring(path) + ") try\n"); err != nil {
		t.Fatal(err)
	}
	if echo, err := fresh.Enter("double(base)"); err != nil || echo != "42" {
		t.Fatalf("echo %q, err %v", echo, err)
	}
}

func TestReplSeamSaveReportsUnwritablePath(t *testing.T) {
	var out bytes.Buffer
	s := seamSession(&out)
	if _, err := s.Enter("let saved = repl.save(" + zstring(filepath.Join(t.TempDir(), "missing-folder", "s.zhex")) + ") catch problem\n    print(error.code(problem))\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "file-write") {
		t.Fatalf("output %q", out.String())
	}
}

func TestReplSeamLoadReportsMissingFile(t *testing.T) {
	var out bytes.Buffer
	s := seamSession(&out)
	if _, err := s.Enter("let done = repl.load(" + zstring(filepath.Join(t.TempDir(), "missing.zhex")) + ") catch problem\n    print(error.code(problem))\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "file-open") {
		t.Fatalf("output %q", out.String())
	}
}

func TestReplSeamLoadRejectsBrokenEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.zhex")
	if err := os.WriteFile(path, []byte("let x = \"a\" + true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	s := seamSession(&out)
	if _, err := s.Enter("let n = 1\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enter("let loaded = repl.load(" + zstring(path) + ") catch problem\n    print(error.code(problem))\n"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "operand-types") {
		t.Fatalf("output %q", out.String())
	}
	if echo2, err := s.Enter("n"); err != nil || echo2 != "1" {
		t.Fatalf("session state lost: %q, %v", echo2, err)
	}
}

func TestReplSeamLoadPropagatesExit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exit.zhex")
	if err := os.WriteFile(path, []byte("console.exit(7)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	s := seamSession(&out)
	_, err := s.Enter("let loaded = repl.load(" + zstring(path) + ") try\n")
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Status != 7 {
		t.Fatalf("got %v", err)
	}
}

// A driver entry holds its state in bindings across a while loop; nested
// repl.enter calls prune the session frame between passes, so the engine must
// keep an in-flight entry's bindings framed (the active-scope stack).
func TestReplSeamDriverBindingsSurviveNestedEnter(t *testing.T) {
	var out bytes.Buffer
	s := seamSession(&out)
	source := "var pass = \"\"\nvar count = 0\nwhile count < 2\n" +
		"    let outcome = repl.enter(\"5 + 5\")\n" +
		"    match outcome\n" +
		"        case EnterResult.Ran(echo)\n            print(echo)\n" +
		"        case EnterResult.Failed(report)\n            print(report)\n" +
		"        case EnterResult.Exited(status)\n            console.exit(status)\n" +
		"    count = count + 1\n    pass = pass + \".\"\n" +
		"print(text.length(pass))\n"
	if _, err := s.Enter(source); err != nil {
		t.Fatal(err)
	}
	if out.String() != "10\n10\n2\n" {
		t.Fatalf("output %q", out.String())
	}
}
