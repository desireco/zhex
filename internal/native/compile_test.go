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
		"let raw-retries = \"abc\"\nlet retries = text.to-int raw-retries catch\n    print(\"That is not a whole number. I'll use 3 retries.\")\n    3\nprint(\"Retries: \" + int.to-string(retries))\n", "")
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

// TestNativeRebuildReplacesStaleRuntime: compiling again into the same
// directory is the normal rebuild — the runtime directory is rewritten from
// scratch, so files from an earlier run never join the build.
func TestNativeRebuildReplacesStaleRuntime(t *testing.T) {
	requireGo(t)
	outDir := filepath.Join(t.TempDir(), "program")
	if err := Compile("program.zhex", "print(\"one\")\n", outDir, nil); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(outDir, "zhexrt", "stale.go")
	if err := os.WriteFile(stale, []byte("this is not Go source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "zhexrt", "int.go"), []byte("package zhexrt\n\n// an obsolete copy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Compile("program.zhex", "print(\"two\")\n", outDir, nil); err != nil {
		t.Fatalf("rebuild over a stale runtime: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale runtime file survived the rebuild: %v", err)
	}
	out, diag, code := runBinary(t, filepath.Join(outDir, binaryName(outDir)), "")
	if out != "two\n" || diag != "" || code != 0 {
		t.Fatalf("stdout=%q stderr=%q status=%d", out, diag, code)
	}
}

// TestBinaryNameSuffixesWindowsTargets: the built file needs .exe to run on
// Windows; the target GOOS is the environment's when cross-compiling, else
// the host's.
func TestBinaryNameSuffixesWindowsTargets(t *testing.T) {
	t.Setenv("GOOS", "windows")
	if got := binaryName(filepath.Join("out", "program")); got != "program.exe" {
		t.Fatalf("windows target binary name = %q, want program.exe", got)
	}
	t.Setenv("GOOS", "linux")
	if got := binaryName(filepath.Join("out", "program")); got != "program" {
		t.Fatalf("linux target binary name = %q, want program", got)
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

// TestNativeFileWriteBytes pins the native byte write end to end: the
// happy path lands on disk (verified through file.size, since 0xFF alone
// is not valid UTF-8 and file.read would refuse it), and the range
// failure renders exactly as the interpreter's diagnostic.
func TestNativeFileWriteBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "b.bin")
	diffCase(t, "bytes",
		"file.write-bytes(\""+path+"\", [0, 255, 65, 128]) try\n"+
			"print(\"size: \" + file.size(\""+path+"\") try)\n", "")
	diffCase(t, "bytes-range",
		"file.write-bytes(\""+path+"\", [65, 256]) try\nprint(\"unreachable\")\n", "")
}

// TestNativeTextSentences pins the #218 sentence layer on the native
// target: the boundary law's output and the collection-limit diagnostic
// render exactly as the interpreter's, the zhexrt body being the shared one.
func TestNativeTextSentences(t *testing.T) {
	diffCase(t, "sentences",
		"print(text.join(text.sentences(\"One. Two! Three?\"), \"|\"))\n"+
			"print(text.join(text.sentences(\"He said \\\"Stop.\\\" Then left. Wait... Really?\"), \"|\"))\n", "")
	diffCase(t, "sentences-limit",
		"print(list.length(text.sentences(text.repeat(\"a. \", 1000001))))\n", "")
}

// TestNativeListEnds pins #159 on the native target: list.pop removes and
// returns the last element (the sanctioned exception), list.first/last read
// the ends, the flow receiver form threads the stack, an empty end fails
// list-range with the family diagnostic, and pop during iteration refuses
// before any change — every row differential against the interpreter.
func TestNativeListEnds(t *testing.T) {
	diffCase(t, "list-ends",
		"let xs = [1, 2, 3]\nprint(list.first(xs))\nprint(list.last(xs))\nprint(list.pop(xs))\nprint(list.length(xs))\n"+
			"let words = [\"a\", \"b\", \"c\"]\nprint(list.pop(words))\nprint(list.last(words))\n"+
			"let stack: List<Int> = list.new()\nlet top = flow stack\n    list.append 4\n    list.append 5\n    list.pop\nprint(top)\nprint(list.length(stack))\n"+
			"var total = 0\nwhile list.length(xs) > 0\n    total = total + list.pop(xs)\nprint(total)\n", "")
	diffCase(t, "list-ends-empty", "let xs: List<Int> = []\nprint(list.pop(xs))\nprint(\"unreachable\")\n", "")
	diffCase(t, "list-ends-first-empty", "let xs: List<String> = []\nprint(list.first(xs))\nprint(\"unreachable\")\n", "")
	diffCase(t, "list-ends-iterating", "let xs = [1, 2]\nfor v in xs\n    list.pop(xs)\nprint(\"unreachable\")\n", "")
}

// TestNativeBranchStatementOrdinaryLines pins #226 on the native target: a
// branch on an ordinary line drops what its arms produce, so the arms may
// end with any type - in a loop body, at the top level, nested in an arm,
// and as every line of a Unit word - and a statement match follows the
// same rule. The mutation words keep their host-word receiver paths.
func TestNativeBranchStatementOrdinaryLines(t *testing.T) {
	diffCase(t, "branch-loop",
		"let xs = [1]\nfor n in range(0, 4)\n    if n > 1\n        list.append xs, n\nprint(int.to-string(list.length(xs)))\n", "")
	diffCase(t, "branch-arms",
		"let xs = [1]\nif xs[0] == 1\n    list.append(xs, 9)\n    print(1)\nelse\n    print(\"one\")\nif xs[0] == 99\n    print(\"never\")\nelif xs[0] == 1\n    print(42)\nelse\n    print(\"other\")\nprint(int.to-string(list.length(xs)))\n", "")
	diffCase(t, "branch-nested",
		"let total = 0\nfor n in [1, 2, 3]\n    if n > 1\n        if n > 2\n            print(int.to-string(n))\n        else\n            print(\"small\")\n    else\n        print(\"first\")\n", "")
	diffCase(t, "branch-unit-words",
		"word store(xs: List<Int>, keep: Bool) -> Unit\n    if keep\n        list.append(xs, 7)\n    else\n        print(\"dropped\")\n\nlet xs = [1]\nstore(xs, true)\nstore(xs, false)\nword fill(xs: List<Int>) -> Unit\n    list.append(xs, 5)\nfill(xs)\nword unit() -> Unit\n    1 + 1\nunit()\nprint(int.to-string(list.length(xs)))\n", "")
	diffCase(t, "branch-match-statement",
		"type Shape = choice\n    Dot\n    Ring(radius: Int)\n\nlet shapes = [Shape.Dot, Shape.Ring(3)]\nlet seen: List<Shape> = []\nfor s in shapes\n    match s\n        case Shape.Dot\n            print(\"dot\")\n        case Shape.Ring(radius)\n            list.append(seen, Shape.Dot)\n            print(int.to-string(radius))\nprint(int.to-string(list.length(seen)))\n", "")
	diffCase(t, "branch-map-statement",
		"let m = {\"a\": 1}\nif map.contains(m, \"a\")\n    map.put(m, \"b\", 2)\nelse\n    print(\"kept\")\nprint(int.to-string(map.length(m)))\n", "")
}
