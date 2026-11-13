package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
		if diagnostics.String() != "usage: zhex run <file.zhex> [--report=text|json] [--trace]\n" {
			t.Fatalf("usage %q", diagnostics.String())
		}
	}
}

func TestCommandUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"run"}, {"repl"}, {"run", "a", "b"}} {
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
}
