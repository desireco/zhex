package wasm

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

// The missing-tool diagnostic is pinned verbatim in internal/wasm/DESIGN.md
// §8 (D6): a factual first line naming the tool and its code, then a
// one-line install hint covering the real install paths. Tested directly so
// it does not depend on PATH state.
func TestMissingToolMessage(t *testing.T) {
	want := "wasm-tools is not installed; zhex compile needs it to assemble the WebAssembly module (code: missing-tool)\n" +
		"hint: install it — cargo install wasm-tools (rustup), pacman -S wasm-tools (Arch), or brew install wasm-tools (macOS)\n"
	var err error = &MissingToolError{}
	if got := err.Error() + "\n"; got != want {
		t.Fatalf("missing-tool message mismatch:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestAssembleMissingToolLeavesNoArtifact(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	outPath := filepath.Join(t.TempDir(), "out.wasm")
	err := assemble("(module)", outPath)
	var missing *MissingToolError
	if !errors.As(err, &missing) {
		t.Fatalf("assemble without wasm-tools on PATH: got %v, want MissingToolError", err)
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Fatalf("output artifact created despite missing tool: %v", statErr)
	}
}

// requireTool skips the test when an external tool is absent, so tool-less
// environments stay green (DESIGN.md §9).
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is not installed; skipping", name)
	}
}

// runNode executes testdata/run.mjs against a compiled module and returns
// its stdout, stderr, and exit status.
func runNode(t *testing.T, wasmPath string) (string, string, int) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node is not installed; skipping")
	}
	cmd := exec.Command(node, filepath.Join("testdata", "run.mjs"), wasmPath)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out.String(), stderr.String(), exit.ExitCode()
		}
		t.Fatalf("node runner: %v", err)
	}
	return out.String(), stderr.String(), 0
}

// smokeWat stages the two bytes "hi" in linear memory and prints them — the
// minimal module honoring the runner contract (exports memory, zhex_main,
// zhex_last_error; imports env.print).
const smokeWat = `(module
  (import "env" "print" (func $env.print (param i32 i32)))
  (memory (export "memory") 1)
  (global $err (mut i32) (i32.const 0))
  (func (export "zhex_main")
    (i32.store8 (i32.const 0) (i32.const 104))
    (i32.store8 (i32.const 1) (i32.const 105))
    (call $env.print (i32.const 0) (i32.const 2)))
  (func (export "zhex_last_error") (result i32) (global.get $err)))`

// failWat mimics the prelude's deliberate trap: set the error code, then
// unreachable — the runner must map it to the pinned §6a stderr string.
const failWat = `(module
  (import "env" "print" (func $env.print (param i32 i32)))
  (memory (export "memory") 1)
  (global $err (mut i32) (i32.const 1))
  (func (export "zhex_main")
    (unreachable))
  (func (export "zhex_last_error") (result i32) (global.get $err)))`

// The runner contract stands alone from the emitter: assembling a
// hand-written module and executing it under node proves the wasm-tools
// invocation, the env.print import, and both trap paths before any Zhex
// source is involved.
func TestRunnerContract(t *testing.T) {
	requireTool(t, "wasm-tools")
	dir := t.TempDir()

	smoke := filepath.Join(dir, "smoke.wasm")
	if err := assemble(smokeWat, smoke); err != nil {
		t.Fatalf("assemble smoke module: %v", err)
	}
	if out, _, code := runNode(t, smoke); code != 0 || out != "hi\n" {
		t.Fatalf("smoke run: code=%d out=%q", code, out)
	}

	fail := filepath.Join(dir, "fail.wasm")
	if err := assemble(failWat, fail); err != nil {
		t.Fatalf("assemble fail module: %v", err)
	}
	out, stderr, code := runNode(t, fail)
	if code != 1 || out != "" || stderr != "runtime error: integer overflow\n" {
		t.Fatalf("fail run: code=%d out=%q stderr=%q", code, out, stderr)
	}
}

// stagedFailWat mimics the DESIGN-M2 §2c staged-message trap: message bytes
// live at memory offset 0, zhex_msg_len exports their length, and the module
// fails with the given code. withLen=false models a milestone-1 module that
// exports no accessor — fixed-table codes must still work there.
func stagedFailWat(code int, message string, withLen bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "(module\n  (import \"env\" \"print\" (func $env.print (param i32 i32)))\n  (memory (export \"memory\") 1)\n  (global $err (mut i32) (i32.const %d))\n  (func (export \"zhex_main\")\n", code)
	for i, c := range []byte(message) {
		fmt.Fprintf(&b, "    (i32.store8 (i32.const %d) (i32.const %d))\n", i, c)
	}
	b.WriteString("    (unreachable))\n  (func (export \"zhex_last_error\") (result i32) (global.get $err))\n")
	if withLen {
		fmt.Fprintf(&b, "  (func (export \"zhex_msg_len\") (result i32) (i32.const %d)))\n", len(message))
	} else {
		b.WriteString(")\n")
	}
	return b.String()
}

// The staged-message contract (DESIGN-M2 §2c): dynamic fail codes 12 and 15
// stage their message as UTF-8 at memory offset 0 and the runner decodes it;
// fixed codes 13 and 14 stay in the table and work without the accessor.
func TestRunnerStagedFailMessage(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, tc := range []struct {
		name    string
		code    int
		message string
		withLen bool
		stderr  string
	}{
		{
			name:    "text-to-int",
			code:    12,
			message: `"x" is not a signed 64-bit decimal integer`,
			withLen: true,
			stderr:  "runtime error: \"x\" is not a signed 64-bit decimal integer\n",
		},
		{
			name:    "text-to-int multibyte decodes as UTF-8",
			code:    12,
			message: `"é" is not a signed 64-bit decimal integer`,
			withLen: true,
			stderr:  "runtime error: \"é\" is not a signed 64-bit decimal integer\n",
		},
		{
			name:    "array-select rank",
			code:    15,
			message: "array.select requires a rank-2 array, found rank 3",
			withLen: true,
			stderr:  "runtime error: array.select requires a rank-2 array, found rank 3\n",
		},
		{
			name:    "replace-empty without the accessor",
			code:    13,
			message: "",
			withLen: false,
			stderr:  "runtime error: text.replace old text is empty\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wasmPath := filepath.Join(t.TempDir(), "staged.wasm")
			if err := assemble(stagedFailWat(tc.code, tc.message, tc.withLen), wasmPath); err != nil {
				t.Fatalf("assemble staged module: %v", err)
			}
			out, stderr, code := runNode(t, wasmPath)
			if code != 1 || out != "" || stderr != tc.stderr {
				t.Fatalf("code=%d out=%q stderr=%q, want code 1 and %q", code, out, stderr, tc.stderr)
			}
		})
	}
}

func TestAssembleRejectsInvalidWat(t *testing.T) {
	requireTool(t, "wasm-tools")
	err := assemble("(module\n  (bogus", filepath.Join(t.TempDir(), "bad.wasm"))
	if err == nil || !strings.HasPrefix(err.Error(), "wasm-tools failed:") {
		t.Fatalf("invalid WAT: got %v, want a wasm-tools failed error", err)
	}
}

// wasm-tools parse assembles type-invalid modules with exit 0 (DESIGN.md,
// Surprises 1); the pipeline's separate validate step must catch them and
// surface the tool's stderr.
func TestAssembleRejectsTypeInvalidModule(t *testing.T) {
	requireTool(t, "wasm-tools")
	outPath := filepath.Join(t.TempDir(), "bad.wasm")
	err := assemble("(module (func (result i32) (i64.const 0)))", outPath)
	if err == nil || !strings.HasPrefix(err.Error(), "wasm-tools failed:") ||
		!strings.Contains(err.Error(), "type mismatch") {
		t.Fatalf("type-invalid module: got %v, want a wasm-tools failed error naming the mismatch", err)
	}
	if _, statErr := os.Stat(outPath); !os.IsNotExist(statErr) {
		t.Fatalf("artifact left behind despite validation failure: %v", statErr)
	}
}

// exitCode maps an interpreter error to the status the CLI would exit with.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *lang.ExitError
	if errors.As(err, &exit) {
		return exit.Status
	}
	return 1
}

// compileFixture compiles one testdata fixture to a temp artifact.
func compileFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name+".zhex")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), name+".wasm")
	if err := Compile(path, string(source), outPath, nil); err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return outPath
}

// The differential gate (DESIGN.md §9): every fixture runs through the
// interpreter and the compiled module; stdout must match byte for byte and
// the exit status must agree.
func TestDifferential(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, name := range []string{"fib", "core", "lists", "choices", "arrays"} {
		t.Run(name, func(t *testing.T) {
			wasmPath := compileFixture(t, name)
			source, err := os.ReadFile(filepath.Join("testdata", name+".zhex"))
			if err != nil {
				t.Fatal(err)
			}
			var interpreted bytes.Buffer
			runErr := lang.Run(filepath.Join("testdata", name+".zhex"), string(source), strings.NewReader(""), &interpreted)
			out, _, code := runNode(t, wasmPath)
			if out != interpreted.String() {
				t.Fatalf("stdout mismatch:\ninterpreted: %q\ncompiled:    %q", interpreted.String(), out)
			}
			if code != exitCode(runErr) {
				t.Fatalf("exit status mismatch: interpreted %d, compiled %d", exitCode(runErr), code)
			}
		})
	}
}

// Runtime diagnostics in the artifact print the pinned §6a strings without
// positions; stdout produced before the failure still matches the
// interpreter byte for byte.
func TestCompiledDiagnostics(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, tc := range []struct{ name, stderr string }{
		{"overflow", "runtime error: integer overflow\n"},
		{"depth", "runtime error: word call depth limit of 256 exceeded\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wasmPath := compileFixture(t, tc.name)
			source, err := os.ReadFile(filepath.Join("testdata", tc.name+".zhex"))
			if err != nil {
				t.Fatal(err)
			}
			var interpreted bytes.Buffer
			_ = lang.Run(filepath.Join("testdata", tc.name+".zhex"), string(source), strings.NewReader(""), &interpreted) // failure is expected
			out, stderr, code := runNode(t, wasmPath)
			if code != 1 || stderr != tc.stderr {
				t.Fatalf("code=%d stderr=%q, want code 1 and %q", code, stderr, tc.stderr)
			}
			if out != interpreted.String() {
				t.Fatalf("stdout before failure mismatch:\ninterpreted: %q\ncompiled:    %q", interpreted.String(), out)
			}
		})
	}
}
