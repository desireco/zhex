// Package native orchestrates Zhex → Go-native compilation: Go emission
// lives in internal/lang (CompileNativeSource) and this package writes the
// self-contained program directory (main.go, go.mod, the zhexrt runtime
// sources) and builds it with the Go toolchain (#185, the Option 1
// report's M1 seam).
package native

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"zhex/internal/lang"
)

// requireGo skips the test when the Go toolchain is absent, so
// tool-less environments stay green (the internal/wasm requireTool
// convention).
func requireGo(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not installed; skipping")
	}
}

// compileAndBuild emits and builds source into a fresh temp directory and
// returns the binary path.
func compileAndBuild(t *testing.T, name, source string) string {
	t.Helper()
	requireGo(t)
	dir := t.TempDir()
	outDir := filepath.Join(dir, name)
	if err := Compile(name+".zhex", source, outDir, nil); err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	binary := filepath.Join(outDir, name)
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("built binary missing: %v", err)
	}
	return binary
}

// runBinary executes a built program with stdin and returns stdout,
// stderr, and the exit status.
func runBinary(t *testing.T, binary, stdin string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Stdin = strings.NewReader(stdin)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out.String(), stderr.String(), exit.ExitCode()
		}
		t.Fatalf("run %s: %v", binary, err)
	}
	return out.String(), stderr.String(), 0
}

// interpret runs the same source under the interpreter and returns its
// stdout plus rendered diagnostic (the corpus oracle).
func interpret(t *testing.T, name, source, stdin string) (string, string, int) {
	t.Helper()
	var out, diag bytes.Buffer
	err := lang.Run(name+".zhex", source, strings.NewReader(stdin), &out)
	code := 0
	if err != nil {
		var exit *lang.ExitError
		if errors.As(err, &exit) {
			code = exit.Status
		} else {
			code = 1
			fmt.Fprintln(&diag, err)
		}
	}
	return out.String(), diag.String(), code
}

// diffCase is one differential row: native and interpreted runs must agree
// byte-for-byte on stdout, stderr, and exit status.
func diffCase(t *testing.T, name, source, stdin string) {
	t.Helper()
	wantOut, wantDiag, wantCode := interpret(t, name, source, stdin)
	binary := compileAndBuild(t, name, source)
	out, stderr, code := runBinary(t, binary, stdin)
	if out != wantOut || code != wantCode || stderr != wantDiag {
		t.Fatalf("%s: native code=%d (want %d)\nnative stdout: %q\ninterp stdout: %q\nnative stderr: %q\ninterp stderr: %q",
			name, code, wantCode, out, wantOut, stderr, wantDiag)
	}
}

// TestNativeSmokeHello drives the M1 seam end to end: hello.zhex's shape —
// a word, a prompted read-line with try, a flow — compiles to Go, builds
// with go build, and matches the interpreter byte-for-byte.
func TestNativeSmokeHello(t *testing.T) {
	diffCase(t, "hello",
		"word greet(name: String) -> String\n    \"Hello, \" + name + \"!\"\n\nlet name = read-line \"Your name: \" try\n\nflow name\n    greet\n    print\n", "Ada\n")
}

// TestNativeArithmeticAndWords covers the M1 expression surface:
// arithmetic with overflow and division diagnostics, comparisons,
// recursion, word defaults of nothing, string building with the Int
// exception.
func TestNativeArithmeticAndWords(t *testing.T) {
	diffCase(t, "arith",
		"word double(n: Int) -> Int\n    n * 2\n\nword fib(n: Int) -> Int\n    if n < 2\n        n\n    else\n        fib(n - 1) + fib(n - 2)\n\nprint(int.to-string(fib(20)))\nprint(int.to-string(double(21)))\nprint(\"sum: \" + (2 + 3))\n", "")
}

// TestNativeRuntimeErrorDiagnostic pins the M4-critical seam early: a
// runtime failure renders exactly as zhex run's diagnostic — position,
// excerpt, caret — and exits 1.
func TestNativeRuntimeErrorDiagnostic(t *testing.T) {
	diffCase(t, "divzero", "let a = 1\nlet b = 0\nprint(a / b)\n", "")
}

// TestNativeTryCatch: catch with a block fallback and an unhandled
// top-level try.
func TestNativeTryCatch(t *testing.T) {
	diffCase(t, "recovery",
		"let rawRetries = \"abc\"\nlet retries = text.to-int rawRetries catch\n    print(\"That is not a whole number. I'll use 3 retries.\")\n    3\nprint(\"Retries: \" + int.to-string(retries))\n", "")
	diffCase(t, "unhandled", "let n = text.to-int(\"x\") try\nprint(\"unreachable\")\n", "")
}

// TestNativeMissingGoWritesSources: without go on PATH the sources and
// go.mod are still written and the instruction names the build command.
func TestNativeMissingGoWritesSources(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	dir := t.TempDir()
	outDir := filepath.Join(dir, "program")
	var message bytes.Buffer
	err := Compile("program.zhex", "print(\"hi\")\n", outDir, nil)
	var missing *MissingToolError
	if !errors.As(err, &missing) {
		t.Fatalf("compile without go: got %v, want MissingToolError", err)
	}
	fmt.Fprintln(&message, err)
	if !strings.Contains(message.String(), "go build") || !strings.Contains(message.String(), outDir) {
		t.Fatalf("instruction does not name the build: %q", message.String())
	}
	for _, name := range []string{filepath.Join(outDir, "main.go"), filepath.Join(outDir, "go.mod"), filepath.Join(outDir, "zhexrt", "int.go")} {
		if _, statErr := os.Stat(name); statErr != nil {
			t.Fatalf("%s missing: %v", name, statErr)
		}
	}
}

// TestNativeRuntimeDiagnostics pins the M4 contract on explicit failure
// fixtures: runtime errors render exactly as zhex run's diagnostics —
// first line, source excerpt, caret — never raw Go panics, and the exit
// status matches. Each row diffs against the interpreter oracle.
func TestNativeRuntimeDiagnostics(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"divzero", "let a = 1\nlet b = 0\nprint(a / b)\n"},
		{"overflow", "print(170141183460469231731687303715884105727 + 1)\n"},
		{"list-range", "let xs: List<Int> = [1]\nprint(xs[5])\n"},
		{"array-range", "let g: Array<Int> = array.new([2, 2], 0)\nprint(g[9, 0])\n"},
		{"collection-iterating", "word addup(values: List<Int>) -> Int\n    0\nlet xs: List<Int> = []\nlet _ = list.append(xs, 1)\nfor x in xs\n    let _ = list.append(xs, 2)\nprint(\"done\")\n"},
		{"unhandled-to-int", "let n = text.to-int(\"x\") try\nprint(\"unreachable\")\n"},
		{"text-slice-range", "print(text.slice(\"abc\", 9, 2))\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diffCase(t, tc.name, tc.source, "")
		})
	}
}
