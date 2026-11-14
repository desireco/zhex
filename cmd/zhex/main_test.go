package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zhex/internal/lang"
)

func TestRunCommand(t *testing.T) {
	for _, tc := range []struct {
		name, source, input, output, diagnostic string
		code                                    int
	}{
		{"interactive", "let name = read-line(\"Name: \") try\nprint(name)\n", "Ada\n", "Name: Ada\n", "", 0},
		{"static error", "print(1)\nmissing(2)\n", "", "", "unknown word", 1},
		{"input error", "let n = text.to-int(read-line(\"N: \") try) try\nprint(n)\n", "abc\n", "N: ", "decimal integer", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "program.zhex")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			var out, diagnostics bytes.Buffer
			code := run([]string{"run", path}, strings.NewReader(tc.input), &out, &diagnostics)
			if code != tc.code || out.String() != tc.output || tc.diagnostic != "" && !strings.Contains(diagnostics.String(), tc.diagnostic) {
				t.Fatalf("code %d, output %q, stderr %q", code, out.String(), diagnostics.String())
			}
			if tc.code == 0 && diagnostics.Len() != 0 {
				t.Fatalf("unexpected stderr %q", diagnostics.String())
			}
		})
	}
}

func TestRunCommandReportsJSON(t *testing.T) {
	for _, tc := range []struct {
		name, source, input, output, wantCode string
	}{
		{"checker error", "print(a-b)\n", "", "", "unknown-name"},
		{"runtime error", "print(1 / 0)\n", "", "", "division-zero"},
		{"input error", "let n = text.to-int(\"abc\") try\nprint(n)\n", "", "", "text-to-int"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "program.zhex")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			var out, diagnostics bytes.Buffer
			code := run([]string{"run", path, "--report=json"}, strings.NewReader(tc.input), &out, &diagnostics)
			if code != 1 || out.String() != tc.output {
				t.Fatalf("code %d, output %q", code, out.String())
			}
			for _, want := range []string{`"code":"` + tc.wantCode + `"`, `"severity":"error"`, `"file":"`, `"hint":`} {
				if !strings.Contains(diagnostics.String(), want) {
					t.Fatalf("stderr %q misses %s", diagnostics.String(), want)
				}
			}
			if strings.Count(strings.TrimSuffix(diagnostics.String(), "\n"), "\n") != 0 {
				t.Fatalf("JSON report must be a single line: %q", diagnostics.String())
			}
		})
	}
}

func TestRunCommandRejectsMalformedReportFlag(t *testing.T) {
	for _, args := range [][]string{{"run", "--report=xml"}, {"run", "a.zhex", "--report="}, {"run", "--report=json"}} {
		var out, diagnostics bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &diagnostics); code != 2 || !strings.Contains(diagnostics.String(), "usage:") {
			t.Fatalf("args %v: code %d stderr %q", args, code, diagnostics.String())
		}
	}
}

func TestRunCommandTrace(t *testing.T) {
	path := "../../examples/composition.zhex"
	var out, diagnostics bytes.Buffer
	code := run([]string{"run", path, "--trace"}, strings.NewReader(""), &out, &diagnostics)
	if code != 0 || out.String() != "32\n" {
		t.Fatalf("code %d, output %q, stderr %q", code, out.String(), diagnostics.String())
	}
	// The full event stream is pinned by TestTraceComposition in internal/lang.
	lines := strings.Split(strings.TrimSuffix(diagnostics.String(), "\n"), "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 events, got %d: %q", len(lines), diagnostics.String())
	}
	for i, want := range map[int]string{
		0: `{"event":"flow-step","word":"square","value":"4","file":"` + path + `","line":9}`,
		9: `{"event":"flow-step","word":"print","value":"32","file":"` + path + `","line":11}`,
	} {
		if lines[i] != want {
			t.Fatalf("event %d is %q, want %q", i, lines[i], want)
		}
	}

	// A traced-off run is byte-identical: same stdout, nothing on stderr.
	var plainOut, plainErr bytes.Buffer
	if code := run([]string{"run", path}, strings.NewReader(""), &plainOut, &plainErr); code != 0 || plainOut.String() != "32\n" || plainErr.Len() != 0 {
		t.Fatalf("traced off: code %d, output %q, stderr %q", code, plainOut.String(), plainErr.String())
	}
}

func TestRunCommandTraceErrorPath(t *testing.T) {
	source := "word double(n: Int) -> Int\n    n + n\n\nprint(double(2))\nprint(1 / 0)\n"
	path := filepath.Join(t.TempDir(), "program.zhex")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	code := run([]string{"run", path, "--trace"}, strings.NewReader(""), &out, &diagnostics)
	if code != 1 || out.String() != "4\n" {
		t.Fatalf("code %d, output %q, stderr %q", code, out.String(), diagnostics.String())
	}
	want := strings.Join([]string{
		`{"event":"word-call","word":"double","file":"` + path + `","line":4,"args":["2"]}`,
		`{"event":"binding","name":"n","value":"2","file":"` + path + `","line":1}`,
		`{"event":"word-return","word":"double","value":"4","file":"` + path + `","line":1}`,
		path + ":5:9: division or remainder by zero",
		"    print(1 / 0)",
		"            ^",
	}, "\n") + "\n"
	if diagnostics.String() != want {
		t.Fatalf("stderr:\n%s\nwant:\n%s", diagnostics.String(), want)
	}
}

func TestRunCommandTraceWithJSONReport(t *testing.T) {
	source := "word double(n: Int) -> Int\n    n + n\n\nprint(double(2))\nprint(1 / 0)\n"
	path := filepath.Join(t.TempDir(), "program.zhex")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	code := run([]string{"run", path, "--trace", "--report=json"}, strings.NewReader(""), &out, &diagnostics)
	if code != 1 || out.String() != "4\n" {
		t.Fatalf("code %d, output %q, stderr %q", code, out.String(), diagnostics.String())
	}
	lines := strings.Split(strings.TrimSuffix(diagnostics.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("stderr must be three events plus the report: %q", diagnostics.String())
	}
	for i, want := range []string{
		`{"event":"word-call","word":"double","file":"` + path + `","line":4,"args":["2"]}`,
		`{"event":"binding","name":"n","value":"2","file":"` + path + `","line":1}`,
		`{"event":"word-return","word":"double","value":"4","file":"` + path + `","line":1}`,
	} {
		if lines[i] != want {
			t.Fatalf("line %d is %q, want %q", i, lines[i], want)
		}
	}
	if !strings.Contains(lines[3], `"code":"division-zero"`) || !strings.Contains(lines[3], `"severity":"error"`) {
		t.Fatalf("report line %q", lines[3])
	}
}

func TestRunCommandRejectsMalformedTraceFlag(t *testing.T) {
	for _, args := range [][]string{{"run", "a.zhex", "--trace=on"}, {"run", "--trace", "--report=xml", "a.zhex"}} {
		var out, diagnostics bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &diagnostics); code != 2 {
			t.Fatalf("args %v: code %d stderr %q", args, code, diagnostics.String())
		}
		if diagnostics.String() != "usage: zhex run <file.zhex> [--report=text|json] [--trace] [args...] | zhex repl [--trace]\n" {
			t.Fatalf("usage %q", diagnostics.String())
		}
	}
}

func TestReplSessionScript(t *testing.T) {
	script := strings.Join([]string{
		"word greet(name: String) -> String",
		"    \"Hello, \" + name + \"!\"",
		"",
		"greet(\"Ada\")",
		"word greet(name: String) -> String",
		"    \"Hi, \" + name + \"!\"",
		"",
		"greet(\"Grace\")",
		"let bad = missing(1)",
		"let who = read-line(\"Name: \") try",
		"Linus",
		"greet(who)",
	}, "\n") + "\n"
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, diagnostics.String())
	}
	for _, want := range []string{
		"Zhex REPL — end multi-line entries with a blank line; bye() or Ctrl-D exits.\n",
		"....> ",
		"Hello, Ada!\n",
		"redefined greet\n",
		"Hi, Grace!\n",
		"Name: ",
		"Hi, Linus!\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout %q misses %q", out.String(), want)
		}
	}
	// Exactly one diagnostic — "Linus" was consumed by the entry's read-line, not the loop.
	if n := strings.Count(diagnostics.String(), "repl:"); n != 1 || !strings.Contains(diagnostics.String(), "unknown word") || strings.Contains(diagnostics.String(), "Linus") {
		t.Fatalf("stderr %q", diagnostics.String())
	}
}

func TestReplEchoesSingleLines(t *testing.T) {
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader("1 + 1\n"), &out, &diagnostics)
	if code != 0 || !strings.Contains(out.String(), "zhex> ") || !strings.Contains(out.String(), "2\n") {
		t.Fatalf("code %d, stdout %q", code, out.String())
	}
	if diagnostics.Len() != 0 {
		t.Fatalf("unexpected stderr %q", diagnostics.String())
	}
}

func TestReplInvalidEntryKeepsSession(t *testing.T) {
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader("missing(1)\n1 + 1\n"), &out, &diagnostics)
	if code != 0 || !strings.Contains(diagnostics.String(), "unknown word") || !strings.Contains(out.String(), "2\n") {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out.String(), diagnostics.String())
	}
}

func TestReplWordsCommand(t *testing.T) {
	script := "words\nword double(n: Int) -> Int\n    n + n\n\nwords\n"
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics)
	if code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("code %d, stderr %q", code, diagnostics.String())
	}
	for _, want := range []string{"bye() -> Unit\n", "double(n: Int) -> Int\n"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout %q misses %q", out.String(), want)
		}
	}
}

// The words intercept beats a user binding of that name: `words` lists
// signatures, never the binding's value.
func TestReplWordsShadowsBinding(t *testing.T) {
	script := "word double(n: Int) -> Int\n    n + n\n\nlet words = 1\nwords\n"
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics)
	if code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("code %d, stderr %q", code, diagnostics.String())
	}
	if !strings.Contains(out.String(), "double(n: Int) -> Int\n") {
		t.Fatalf("stdout %q misses signatures", out.String())
	}
	if strings.Contains(out.String(), "1\n") {
		t.Fatalf("stdout %q echoes the binding, not the signatures", out.String())
	}
}

func TestReplEofDuringContinuationExits(t *testing.T) {
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader("word f() -> Int\n    1\n"), &out, &diagnostics)
	if code != 0 || !strings.Contains(out.String(), "....> ") || diagnostics.Len() != 0 {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out.String(), diagnostics.String())
	}
}

func TestReplTrace(t *testing.T) {
	script := "word double(n: Int) -> Int\n    n + n\n\ndouble(21)\n"
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl", "--trace"}, strings.NewReader(script), &out, &diagnostics)
	if code != 0 || !strings.Contains(out.String(), "42\n") {
		t.Fatalf("code %d, stdout %q", code, out.String())
	}
	for _, want := range []string{`{"event":"word-call","word":"double"`, `"file":"repl"`} {
		if !strings.Contains(diagnostics.String(), want) {
			t.Fatalf("stderr %q misses %q", diagnostics.String(), want)
		}
	}
}

func TestReplFinalLineWithoutNewline(t *testing.T) {
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader("1 + 1"), &out, &diagnostics)
	if code != 0 || !strings.Contains(out.String(), "2\n") || diagnostics.Len() != 0 {
		t.Fatalf("code %d, stdout %q, stderr %q", code, out.String(), diagnostics.String())
	}
}

func TestReplRejectsArguments(t *testing.T) {
	for _, args := range [][]string{{"repl", "extra"}, {"repl", "--report=json"}} {
		var out, diagnostics bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &diagnostics); code != 2 || !strings.Contains(diagnostics.String(), "usage:") {
			t.Fatalf("args %v: code %d, stderr %q", args, code, diagnostics.String())
		}
	}
}

func TestRunCommandForwardsArgs(t *testing.T) {
	source := "print(console.args-count())\nprint(console.arg(0) try)\nprint(console.arg(1) try)\n"
	path := filepath.Join(t.TempDir(), "program.zhex")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	code := run([]string{"run", path, "alpha", "beta"}, strings.NewReader(""), &out, &diagnostics)
	if code != 0 || out.String() != "2\nalpha\nbeta\n" || diagnostics.Len() != 0 {
		t.Fatalf("code %d, output %q, stderr %q", code, out.String(), diagnostics.String())
	}

	// Flags keep working alongside positional program arguments; the trace
	// stream keeps flowing next to them.
	tracedPath := filepath.Join(t.TempDir(), "traced.zhex")
	if err := os.WriteFile(tracedPath, []byte("let first = console.arg(0) try\nprint(first)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var tracedOut, tracedErr bytes.Buffer
	code = run([]string{"run", tracedPath, "--trace", "alpha", "beta"}, strings.NewReader(""), &tracedOut, &tracedErr)
	if code != 0 || tracedOut.String() != "alpha\n" {
		t.Fatalf("code %d, output %q", code, tracedOut.String())
	}
	if !strings.Contains(tracedErr.String(), `{"event":`) {
		t.Fatalf("missing trace events: %q", tracedErr.String())
	}

	// Unknown --flags stay usage errors, before or after positionals.
	for _, args := range [][]string{{"run", path, "--bogus"}, {"run", path, "alpha", "--bogus"}} {
		var out, diagnostics bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &diagnostics); code != 2 || !strings.Contains(diagnostics.String(), "usage:") {
			t.Fatalf("args %v: code %d stderr %q", args, code, diagnostics.String())
		}
	}
}

func TestCommandUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"run"}, {"rep"}, {"run", "--bogus", "a.zhex"}, {"run", "a.zhex", "--bogus"}} {
		var out, diagnostics bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &diagnostics); code != 2 || !strings.Contains(diagnostics.String(), "usage:") {
			t.Fatalf("args %v: code %d stderr %q", args, code, diagnostics.String())
		}
	}
	var out, diagnostics bytes.Buffer
	if code := run([]string{"run", filepath.Join(t.TempDir(), "missing.zhex")}, strings.NewReader(""), &out, &diagnostics); code != 1 || !strings.Contains(diagnostics.String(), "missing.zhex") {
		t.Fatalf("missing file: code %d stderr %q", code, diagnostics.String())
	}
}

func TestExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zhex")
	build := exec.Command("go", "build", "-o", path, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	cmd := exec.Command(path, "run", "../../examples/fibonacci.zhex")
	cmd.Stdin = strings.NewReader("10\n")
	output, err := cmd.CombinedOutput()
	if err != nil || string(output) != "Fibonacci index (0..92): 55\n" {
		t.Fatalf("output %q, error %v", output, err)
	}
	cmd = exec.Command(path, "run", "../../examples/fibonacci.zhex")
	cmd.Stdin = strings.NewReader("wrong\n")
	err = cmd.Run()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got %v", err)
	}
	exitSource := filepath.Join(t.TempDir(), "exit.zhex")
	if err := os.WriteFile(exitSource, []byte("console.exit(23)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(path, "run", exitSource)
	if err = cmd.Run(); err == nil {
		t.Fatal("expected exit 23")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 23 {
		t.Fatalf("expected exit 23, got %v", err)
	}
	if err := os.WriteFile(exitSource, []byte("console.fail(\"failed\")\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(path, "run", exitSource)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 || stdout.Len() != 0 || stderr.String() != "failed\n" {
		t.Fatalf("fail: error %v stdout %q stderr %q", err, stdout.String(), stderr.String())
	}
	cmd = exec.Command(path, "repl")
	cmd.Stdin = strings.NewReader("1 + 1\n")
	output, err = cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "2") {
		t.Fatalf("repl: output %q, error %v", output, err)
	}
}

const siblingMathModule = `word gcd(a: Int, b: Int) -> Int
    if b == 0
        a
    else
        gcd(b, a % b)
word _wrap(n: Int) -> Int
    n
`

func TestRunSiblingModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "math.zhex"), []byte(siblingMathModule), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.zhex"), []byte("use math\nprint(math.gcd(54, 18))\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	code := run([]string{"run", filepath.Join(dir, "main.zhex")}, strings.NewReader(""), &out, &diagnostics)
	if code != 0 || out.String() != "18\n" || diagnostics.Len() != 0 {
		t.Fatalf("code %d, output %q, stderr %q", code, out.String(), diagnostics.String())
	}

	// A missing sibling names the searched path next to the program file.
	if err := os.WriteFile(filepath.Join(dir, "lonely.zhex"), []byte("use stats\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out2, diagnostics2 bytes.Buffer
	code = run([]string{"run", filepath.Join(dir, "lonely.zhex")}, strings.NewReader(""), &out2, &diagnostics2)
	if code != 1 || !strings.Contains(diagnostics2.String(), `module "stats" not found; searched `+filepath.Join(dir, "stats.zhex")) {
		t.Fatalf("code %d, stderr %q", code, diagnostics2.String())
	}
}

func TestReplUseSessionScript(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "math.zhex"), []byte(siblingMathModule), 0o600); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { os.Chdir(old) }()
	script := strings.Join([]string{
		"use math",
		"math.gcd(54, 18)",
		"math._wrap(1)",
		"words",
		"1 + 1",
	}, "\n") + "\n"
	var out, diagnostics bytes.Buffer
	code := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics)
	if code != 0 {
		t.Fatalf("code %d, stderr %q", code, diagnostics.String())
	}
	for _, want := range []string{"18\n", "math.gcd(a: Int, b: Int) -> Int\n", "2\n"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout %q misses %q", out.String(), want)
		}
	}
	if strings.Contains(out.String(), "math._wrap") {
		t.Fatalf("module-private words must not be listed: %q", out.String())
	}
	if !strings.Contains(diagnostics.String(), `"_wrap" is private to module "math"`) ||
		!strings.Contains(diagnostics.String(), "hint: remove the leading underscore in math.zhex to export it") {
		t.Fatalf("stderr %q misses the private-use diagnostic", diagnostics.String())
	}
}

func TestConsoleExitReturnsRequestedStatus(t *testing.T) {
	for _, tc := range []struct {
		source string
		status int
	}{
		{"console.exit(0)\nprint(\"after\")\n", 0},
		{"word stop() -> Unit\n    console.exit(23)\nstop()\nprint(\"after\")\n", 23},
		{"let n = text.to-int(\"bad\") catch problem\n    console.exit(255)\nprint(n)\n", 255},
	} {
		path := filepath.Join(t.TempDir(), "exit.zhex")
		if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		var out, diagnostics bytes.Buffer
		status := run([]string{"run", path, "--report=json"}, strings.NewReader(""), &out, &diagnostics)
		if status != tc.status || out.Len() != 0 || diagnostics.Len() != 0 {
			t.Fatalf("status %d, output %q, stderr %q", status, out.String(), diagnostics.String())
		}
	}
}

func TestConsoleExitRejectsInvalidStatus(t *testing.T) {
	for _, code := range []string{"-1", "256"} {
		path := filepath.Join(t.TempDir(), "exit.zhex")
		if err := os.WriteFile(path, []byte("console.exit("+code+")\n"), 0600); err != nil {
			t.Fatal(err)
		}
		var out, diagnostics bytes.Buffer
		if status := run([]string{"run", path, "--report=json"}, strings.NewReader(""), &out, &diagnostics); status != 1 || !strings.Contains(diagnostics.String(), `"code":"console-exit-status"`) {
			t.Fatalf("status %d, stderr %q", status, diagnostics.String())
		}
	}
}

func TestReplConsoleExitReturnsStatus(t *testing.T) {
	var out, diagnostics bytes.Buffer
	status := run([]string{"repl"}, strings.NewReader("console.exit(42)\nprint(\"after\")\n"), &out, &diagnostics)
	if status != 42 || strings.Contains(out.String(), "after") || diagnostics.Len() != 0 {
		t.Fatalf("status %d, output %q, stderr %q", status, out.String(), diagnostics.String())
	}
}

func TestConsoleFailWritesStderrAndExits(t *testing.T) {
	for _, source := range []string{
		"print(\"before\")\nconsole.fail(\"failed\")\nprint(\"after\")\n",
		"let n = text.to-int(\"bad\") catch problem\n    print(\"before\")\n    console.fail(\"failed\")\nprint(n)\n",
	} {
		path := filepath.Join(t.TempDir(), "fail.zhex")
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		var out, diagnostics bytes.Buffer
		status := run([]string{"run", path, "--report=json"}, strings.NewReader(""), &out, &diagnostics)
		if status != 1 || out.String() != "before\n" || diagnostics.String() != "failed\n" {
			t.Fatalf("status %d, stdout %q, stderr %q", status, out.String(), diagnostics.String())
		}
	}
}

func TestReplConsoleFailWritesStderrAndExits(t *testing.T) {
	var out, diagnostics bytes.Buffer
	status := run([]string{"repl"}, strings.NewReader("console.fail(\"failed\")\nprint(\"after\")\n"), &out, &diagnostics)
	if status != 1 || strings.Contains(out.String(), "after") || diagnostics.String() != "failed\n" {
		t.Fatalf("status %d, stdout %q, stderr %q", status, out.String(), diagnostics.String())
	}
}

func TestConsoleFailRequiresString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.zhex")
	if err := os.WriteFile(path, []byte("console.fail(42)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	if status := run([]string{"run", path}, strings.NewReader(""), &out, &diagnostics); status != 1 || !strings.Contains(diagnostics.String(), "requires type String") {
		t.Fatalf("status %d stderr %q", status, diagnostics.String())
	}
}

func TestReplAcceptsRecordDefinition(t *testing.T) {
	script := "type Customer = record\n    name: String\n\nlet c: Customer = Customer(\"Ada\")\nc.name\n"
	var out, diagnostics bytes.Buffer
	status := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics)
	if status != 0 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "Ada\n") {
		t.Fatalf("status %d stdout %q stderr %q", status, out.String(), diagnostics.String())
	}
}

func TestReplByeIsOrdinaryRedefinableWord(t *testing.T) {
	var out, diagnostics bytes.Buffer
	script := "words\nbye()\nprint(\"after\")\n"
	if code := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics); code != 0 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "bye() -> Unit") || strings.Contains(out.String(), "after") {
		t.Fatalf("exit %d stdout %q stderr %q", code, out.String(), diagnostics.String())
	}
	out.Reset()
	diagnostics.Reset()
	script = "word bye() -> Unit\n    print(\"custom\")\n\nbye()\n"
	if code := run([]string{"repl"}, strings.NewReader(script), &out, &diagnostics); code != 0 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "redefined bye") || !strings.Contains(out.String(), "custom\n") {
		t.Fatalf("exit %d stdout %q stderr %q", code, out.String(), diagnostics.String())
	}
	if _, err := lang.Compile("file.zhex", "bye()\n"); err == nil {
		t.Fatal("bye unexpectedly available in file program")
	}
}

func TestReplBareByeExitsNormally(t *testing.T) {
	var out, diagnostics bytes.Buffer
	status := run([]string{"repl"}, strings.NewReader("bye\nprint(7)\nbye()\n"), &out, &diagnostics)
	if status != 0 || diagnostics.Len() != 0 || strings.Contains(out.String(), "7\n") {
		t.Fatalf("exit %d stdout %q stderr %q", status, out.String(), diagnostics.String())
	}
}
