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
		{"interactive", "let name = console.read-line(\"Name: \") try\nprint(name)\n", "Ada\n", "Name: Ada\n", "", 0},
		{"static error", "print(1)\nmissing(2)\n", "", "", "unknown word", 1},
		{"input error", "let n = console.read-int(\"N: \") try\nprint(n)\n", "abc\n", "N: ", "decimal integer", 1},
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
		{"input error", "let n = console.read-int(\"N: \") try\nprint(n)\n", "abc\n", "N: ", "console-int"},
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
