package wasm

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

func TestAssembleRejectsInvalidWat(t *testing.T) {
	requireTool(t, "wasm-tools")
	err := assemble("(module\n  (bogus", filepath.Join(t.TempDir(), "bad.wasm"))
	if err == nil || !strings.HasPrefix(err.Error(), "wasm-tools failed:") {
		t.Fatalf("invalid WAT: got %v, want a wasm-tools failed error", err)
	}
}
