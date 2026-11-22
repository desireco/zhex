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
	"time"

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
// its stdout, stderr, and exit status. Extra runner args select the browser
// host's inputs (DESIGN-M3 §1c): --root seeds the virtual FS, --stdin
// supplies the stdin string, --env injects environment pairs.
func runNode(t *testing.T, wasmPath string, runnerArgs ...string) (string, string, int) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node is not installed; skipping")
	}
	args := append([]string{filepath.Join("testdata", "run.mjs"), wasmPath}, runnerArgs...)
	cmd := exec.Command(node, args...)
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

// smokeWat stages the three bytes "hi\n" in linear memory and prints them —
// the minimal module honoring the runner contract (exports memory,
// zhex_main, zhex_last_error; imports env.print). M3 moved the newline into
// the module (DESIGN-M3 Surprises 2: env.print sends raw bytes), so the
// module carries its own trailing newline.
const smokeWat = `(module
  (import "env" "print" (func $env.print (param i32 i32)))
  (memory (export "memory") 1)
  (global $err (mut i32) (i32.const 0))
  (func (export "zhex_main")
    (i32.store8 (i32.const 0) (i32.const 104))
    (i32.store8 (i32.const 1) (i32.const 105))
    (i32.store8 (i32.const 2) (i32.const 10))
    (call $env.print (i32.const 0) (i32.const 3)))
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

// hostContractWat is a hand-written MVP module calling every one of the 15
// env.* imports of the M3 browser host (DESIGN-M3 §1a) — including the errno
// paths — and reporting each result as one line: a tag byte, a signed
// two-digit number, a newline. Content returned through the scratch window
// (offset 0, [u32 len][bytes]) is printed raw after its step's line. Path
// strings live at 1024+, payloads to write at 2048+, print staging at 4096,
// and the stdin buffer at 8192.
const hostContractWat = `(module
  (import "env" "print" (func $print (param i32 i32)))
  (import "env" "stdin_read" (func $stdin_read (param i32 i32) (result i32)))
  (import "env" "fs_read_file" (func $fs_read_file (param i32 i32) (result i32)))
  (import "env" "fs_write_file" (func $fs_write_file (param i32 i32 i32 i32 i32) (result i32)))
  (import "env" "fs_open_read" (func $fs_open_read (param i32 i32) (result i32)))
  (import "env" "fs_read_line" (func $fs_read_line (param i32) (result i32)))
  (import "env" "fs_close" (func $fs_close (param i32) (result i32)))
  (import "env" "fs_stat" (func $fs_stat (param i32 i32) (result i32)))
  (import "env" "fs_list" (func $fs_list (param i32 i32) (result i32)))
  (import "env" "fs_make_folder" (func $fs_make_folder (param i32 i32 i32) (result i32)))
  (import "env" "fs_delete" (func $fs_delete (param i32 i32) (result i32)))
  (import "env" "fs_delete_folder" (func $fs_delete_folder (param i32 i32 i32) (result i32)))
  (import "env" "fs_move" (func $fs_move (param i32 i32 i32 i32 i32) (result i32)))
  (import "env" "env_get" (func $env_get (param i32 i32) (result i32)))
  (import "env" "env_environ" (func $env_environ (result i32)))
  (memory (export "memory") 1)
  (data (i32.const 1024) "a.txt")
  (data (i32.const 1030) "b.txt")
  (data (i32.const 1036) "nope.txt")
  (data (i32.const 1048) "d")
  (data (i32.const 1052) "dir")
  (data (i32.const 1058) "dir/.dot")
  (data (i32.const 1070) "dir/aa")
  (data (i32.const 1080) "dir/zz")
  (data (i32.const 1090) "full")
  (data (i32.const 1096) "full/child")
  (data (i32.const 1110) "moved.txt")
  (data (i32.const 1124) "lines.txt")
  (data (i32.const 1136) "deep/tree/one")
  (data (i32.const 1156) "ZHEX_A")
  (data (i32.const 1166) "MISSING_KEY")
  (data (i32.const 1180) "no.txt")
  (data (i32.const 2048) "one\ntwo\npartial")
  (data (i32.const 2080) "app")
  (data (i32.const 2096) "x\n")
  (data (i32.const 2104) "l1\nl2\npartial")

  ;; $print_num renders a signed value below 100 as three chars — '+' or '-'
  ;; and two digits — at 4097..4099 (scratch area of the print staging).
  (func $print_num (param $v i32)
    (local $n i32)
    (if (i32.lt_s (local.get $v) (i32.const 0))
      (then
        (i32.store8 (i32.const 4097) (i32.const 45))
        (local.set $n (i32.sub (i32.const 0) (local.get $v))))
      (else
        (i32.store8 (i32.const 4097) (i32.const 43))
        (local.set $n (local.get $v))))
    (i32.store8 (i32.const 4098) (i32.add (i32.const 48) (i32.div_u (local.get $n) (i32.const 10))))
    (i32.store8 (i32.const 4099) (i32.add (i32.const 48) (i32.rem_u (local.get $n) (i32.const 10))))
    (call $print (i32.const 4097) (i32.const 3)))

  ;; $report prints one step line: tag byte, number, newline.
  (func $report (param $tag i32) (param $v i32)
    (i32.store8 (i32.const 4096) (local.get $tag))
    (call $print (i32.const 4096) (i32.const 1))
    (call $print_num (local.get $v))
    (i32.store8 (i32.const 4100) (i32.const 10))
    (call $print (i32.const 4100) (i32.const 1)))

  ;; $print_scratch prints the current scratch payload ([u32 len][bytes]).
  (func $print_scratch
    (call $print (i32.const 4) (i32.load (i32.const 0))))

  ;; $print_blobs prints the first $count [u32 len][bytes] blobs of the
  ;; scratch payload after its leading count word, each followed by ';'. The
  ;; same walker covers fs_list entries (count) and env_environ pairs
  ;; (two blobs per pair).
  (func $print_blobs (param $count i32)
    (local $at i32)
    (local $i i32)
    (local $len i32)
    (local.set $at (i32.const 8))
    (loop $again
      (local.set $len (i32.load (local.get $at)))
      (call $print (i32.add (local.get $at) (i32.const 4)) (local.get $len))
      (i32.store8 (i32.const 4101) (i32.const 59))
      (call $print (i32.const 4101) (i32.const 1))
      (local.set $at (i32.add (local.get $at) (i32.add (i32.const 4) (local.get $len))))
      (local.set $i (i32.add (local.get $i) (i32.const 1)))
      (br_if $again (i32.lt_u (local.get $i) (local.get $count)))))

  (func (export "zhex_main")
    (local $h i32)
    (local $n i32)
    (call $report (i32.const 87) (call $fs_write_file (i32.const 1024) (i32.const 5) (i32.const 2048) (i32.const 15) (i32.const 0)))
    (call $report (i32.const 80) (call $fs_write_file (i32.const 1030) (i32.const 5) (i32.const 2080) (i32.const 3) (i32.const 1)))
    (call $report (i32.const 82) (call $fs_read_file (i32.const 1024) (i32.const 5)))
    (call $print_scratch)
    (call $report (i32.const 68) (call $fs_read_file (i32.const 1036) (i32.const 8)))
    (call $report (i32.const 77) (call $fs_make_folder (i32.const 1048) (i32.const 1) (i32.const 0)))
    (call $report (i32.const 73) (call $fs_read_file (i32.const 1048) (i32.const 1)))
    (call $report (i32.const 83) (call $fs_stat (i32.const 1024) (i32.const 5)))
    (call $print_num (i32.load (i32.const 4)))
    (call $print_num (i32.wrap_i64 (i64.load (i32.const 8))))
    (i32.store8 (i32.const 4100) (i32.const 10))
    (call $print (i32.const 4100) (i32.const 1))
    (call $report (i32.const 84) (call $fs_stat (i32.const 1048) (i32.const 1)))
    (call $print_num (i32.load (i32.const 4)))
    (call $print_num (i32.wrap_i64 (i64.load (i32.const 8))))
    (i32.store8 (i32.const 4100) (i32.const 10))
    (call $print (i32.const 4100) (i32.const 1))
    (call $report (i32.const 85) (call $fs_stat (i32.const 1036) (i32.const 8)))
    (call $report (i32.const 71) (call $fs_make_folder (i32.const 1052) (i32.const 3) (i32.const 0)))
    (call $report (i32.const 72) (call $fs_write_file (i32.const 1058) (i32.const 8) (i32.const 2096) (i32.const 2) (i32.const 0)))
    (call $report (i32.const 74) (call $fs_write_file (i32.const 1070) (i32.const 6) (i32.const 2080) (i32.const 3) (i32.const 0)))
    (call $report (i32.const 75) (call $fs_write_file (i32.const 1080) (i32.const 6) (i32.const 2080) (i32.const 3) (i32.const 0)))
    (call $report (i32.const 76) (call $fs_list (i32.const 1052) (i32.const 3)))
    (call $print_blobs (i32.load (i32.const 4)))
    (call $report (i32.const 88) (call $fs_delete (i32.const 1080) (i32.const 6)))
    (call $report (i32.const 89) (call $fs_delete (i32.const 1052) (i32.const 3)))
    (call $report (i32.const 90) (call $fs_delete (i32.const 1036) (i32.const 8)))
    (call $report (i32.const 102) (call $fs_make_folder (i32.const 1090) (i32.const 4) (i32.const 0)))
    (call $report (i32.const 103) (call $fs_write_file (i32.const 1096) (i32.const 10) (i32.const 2080) (i32.const 3) (i32.const 0)))
    (call $report (i32.const 104) (call $fs_delete_folder (i32.const 1090) (i32.const 4) (i32.const 0)))
    (call $report (i32.const 105) (call $fs_delete_folder (i32.const 1090) (i32.const 4) (i32.const 1)))
    (call $report (i32.const 86) (call $fs_move (i32.const 1024) (i32.const 5) (i32.const 1110) (i32.const 9) (i32.const 0)))
    (call $report (i32.const 118) (call $fs_move (i32.const 1030) (i32.const 5) (i32.const 1110) (i32.const 9) (i32.const 0)))
    (call $report (i32.const 119) (call $fs_move (i32.const 1030) (i32.const 5) (i32.const 1110) (i32.const 9) (i32.const 1)))
    (call $report (i32.const 120) (call $fs_move (i32.const 1180) (i32.const 6) (i32.const 1110) (i32.const 9) (i32.const 0)))
    (call $report (i32.const 112) (call $fs_make_folder (i32.const 1136) (i32.const 13) (i32.const 1)))
    (call $report (i32.const 111) (call $fs_write_file (i32.const 1124) (i32.const 9) (i32.const 2104) (i32.const 13) (i32.const 0)))
    (local.set $h (call $fs_open_read (i32.const 1124) (i32.const 9)))
    (call $report (i32.const 79) (local.get $h))
    (call $report (i32.const 113) (call $fs_read_line (local.get $h)))
    (call $print_scratch)
    (call $report (i32.const 113) (call $fs_read_line (local.get $h)))
    (call $print_scratch)
    (call $report (i32.const 113) (call $fs_read_line (local.get $h)))
    (call $print_scratch)
    (call $report (i32.const 113) (call $fs_read_line (local.get $h)))
    (call $report (i32.const 99) (call $fs_close (local.get $h)))
    (call $report (i32.const 99) (call $fs_close (local.get $h)))
    (local.set $h (call $fs_open_read (i32.const 1048) (i32.const 1)))
    (call $report (i32.const 79) (local.get $h))
    (call $report (i32.const 114) (call $fs_read_line (local.get $h)))
    (local.set $n (call $stdin_read (i32.const 8192) (i32.const 4096)))
    (call $report (i32.const 110) (local.get $n))
    (call $print (i32.const 8192) (local.get $n))
    (call $report (i32.const 110) (call $stdin_read (i32.const 8192) (i32.const 4096)))
    (call $report (i32.const 69) (call $env_get (i32.const 1156) (i32.const 6)))
    (call $print_scratch)
    (call $report (i32.const 70) (call $env_get (i32.const 1166) (i32.const 11)))
    (call $report (i32.const 101) (call $env_environ))
    (call $print_blobs (i32.mul (i32.load (i32.const 4)) (i32.const 2))))
  (func (export "zhex_last_error") (result i32) (i32.const 0)))`

// TestRunnerM3HostContract proves the browser host surface of run.mjs
// (DESIGN-M3 §1a, §2) against a hand-written module: all 15 imports, the
// scratch protocol, errno numbers (WASI preview 1), handle semantics
// (idempotent close, EOF, EISDIR on directory reads), virtual-FS mutation
// and listing order, stdin delivery, and the env table — independent of the
// emitter, which lands separately.
func TestRunnerM3HostContract(t *testing.T) {
	requireTool(t, "wasm-tools")
	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "host.wasm")
	if err := assemble(hostContractWat, wasmPath); err != nil {
		t.Fatalf("assemble host contract module: %v", err)
	}
	stdinPath := filepath.Join(dir, "stdin.txt")
	if err := os.WriteFile(stdinPath, []byte("Ada\nGrace\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runNode(t, wasmPath, "--root", root, "--stdin", stdinPath, "--env", "ZHEX_A=runner")
	want := "W+00\n" + // fs_write_file create-or-truncate
		"P+00\n" + // append creates a missing file
		"R+00\n" + "one\ntwo\npartial" + // whole-file read through the scratch window
		"D+44\n" + // read a missing file: ENOENT
		"M+00\n" + // make one folder level
		"I+31\n" + // read a directory: EISDIR at the read stage
		"S+00\n" + "+00+15\n" + // stat a file: kind 0, size 15
		"T+00\n" + "+01+00\n" + // stat a folder: kind 1, size 0
		"U+44\n" + // stat a missing path: ENOENT
		"G+00\n" + // make the listing folder
		"H+00\n" + "J+00\n" + "K+00\n" + // seed a dotfile and two plain files
		"L+00\n" + ".dot;aa;zz;" + // list: bare names, byte-sorted, dotfiles kept
		"X+00\n" + // delete a file
		"Y+55\n" + // delete a non-empty folder: ENOTEMPTY
		"Z+44\n" + // delete a missing path: ENOENT
		"f+00\n" + "g+00\n" + "h+55\n" + "i+00\n" + // delete-folder: refusal then recursive removal
		"V+00\n" + // move a file
		"v+20\n" + // move onto an existing file without overwrite: EEXIST
		"w+00\n" + // move with overwrite replaces
		"x+44\n" + // move a missing source: ENOENT
		"p+00\n" + // make-folder with parents creates ancestors
		"o+00\n" + // write the line-streaming fixture
		"O+00\n" + // open for reading: first handle
		"q+01\n" + "l1\n" + // read a full line, raw bytes through "\n"
		"q+01\n" + "l2\n" +
		"q+01\n" + "partial" + // partial final line without a newline
		"q+00\n" + // clean EOF
		"c+00\n" + "c+00\n" + // close is idempotent
		"O+01\n" + // opening a directory yields a handle
		"r-31\n" + // reading it fails EISDIR
		"n+10\n" + "Ada\nGrace\n" + // stdin delivered in one read
		"n+00\n" + // second read is EOF
		"E+01\n" + "runner" + // env_get of a set key
		"F+00\n" + // env_get of an unset key
		"e+00\n" + "ZHEX_A;runner;" // env_environ lists key and value
	if code != 0 || stderr != "" || out != want {
		t.Fatalf("host contract: code=%d stderr=%q\n got: %q\nwant: %q", code, stderr, out, want)
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

// The staged-message contract (DESIGN-M2 §2c; M3 adds codes 16-18,
// DESIGN-M3 §8): dynamic fail codes stage their message as UTF-8 at memory
// offset 0 and the runner decodes it; fixed codes 13 and 14 stay in the
// table and work without the accessor.
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
			name:    "unhandled-result",
			code:    16,
			message: `cannot open "nope.txt": open nope.txt: no such file or directory`,
			withLen: true,
			stderr:  "runtime error: cannot open \"nope.txt\": open nope.txt: no such file or directory\n",
		},
		{
			name:    "file-stream-open",
			code:    17,
			message: `reading "x" failed: read x: is a directory`,
			withLen: true,
			stderr:  "runtime error: reading \"x\" failed: read x: is a directory\n",
		},
		{
			name:    "file-exists stat failure",
			code:    18,
			message: `cannot check "p": stat p: no such file or directory`,
			withLen: true,
			stderr:  "runtime error: cannot check \"p\": stat p: no such file or directory\n",
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

// runWasmtime executes a WASI module under the wasmtime CLI the way the
// WASI skin is validated (DESIGN-M3 §1c): the fixture root is preopened at
// /, env pairs are passed with --env, and stdin comes from a file. Returns
// stdout, stderr, and the exit status.
func runWasmtime(t *testing.T, wasmPath, rootDir, stdinPath string, env []string) (string, string, int) {
	t.Helper()
	wasmtime, err := exec.LookPath("wasmtime")
	if err != nil {
		t.Skipf("wasmtime is not installed; skipping")
	}
	args := []string{"run", "--dir", rootDir + "::/"}
	for _, pair := range env {
		args = append(args, "--env", pair)
	}
	args = append(args, wasmPath)
	cmd := exec.Command(wasmtime, args...)
	if stdinPath != "" {
		in, err := os.Open(stdinPath)
		if err != nil {
			t.Fatal(err)
		}
		defer in.Close()
		cmd.Stdin = in
	}
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return out.String(), stderr.String(), exit.ExitCode()
		}
		t.Fatalf("wasmtime runner: %v", err)
	}
	return out.String(), stderr.String(), 0
}

// wasiSmokeWat is a minimal WASI preview 1 module writing "wasi ok\n" to
// stdout via fd_write — it proves the wasmtime invocation (preopen, stdin
// redirection, exit status) without needing the WASI skin's emitter.
const wasiSmokeWat = `(module
  (import "wasi_snapshot_preview1" "fd_write" (func $fd_write (param i32 i32 i32 i32) (result i32)))
  (memory (export "memory") 1)
  (data (i32.const 1024) "wasi ok\n")
  (func (export "_start")
    (i32.store (i32.const 8) (i32.const 1024))
    (i32.store (i32.const 12) (i32.const 8))
    (drop (call $fd_write (i32.const 1) (i32.const 8) (i32.const 1) (i32.const 16)))))`

// wasiExitWat writes one line, then exits with status 3 through proc_exit —
// the deliberate-exit path the WASI skin uses for staged failures
// (DESIGN-M3 Surprises 8).
const wasiExitWat = `(module
  (import "wasi_snapshot_preview1" "fd_write" (func $fd_write (param i32 i32 i32 i32) (result i32)))
  (import "wasi_snapshot_preview1" "proc_exit" (func $proc_exit (param i32)))
  (memory (export "memory") 1)
  (data (i32.const 1024) "stopping\n")
  (func (export "_start")
    (i32.store (i32.const 8) (i32.const 1024))
    (i32.store (i32.const 12) (i32.const 9))
    (drop (call $fd_write (i32.const 1) (i32.const 8) (i32.const 1) (i32.const 16)))
    (call $proc_exit (i32.const 3))))`

func TestWasmtimeRunnerContract(t *testing.T) {
	requireTool(t, "wasm-tools")
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	smoke := filepath.Join(dir, "smoke.wasi.wasm")
	if err := assemble(wasiSmokeWat, smoke); err != nil {
		t.Fatalf("assemble wasi smoke module: %v", err)
	}
	out, stderr, code := runWasmtime(t, smoke, root, "", nil)
	if code != 0 || stderr != "" || out != "wasi ok\n" {
		t.Fatalf("wasi smoke: code=%d stderr=%q out=%q", code, stderr, out)
	}

	exit := filepath.Join(dir, "exit.wasi.wasm")
	if err := assemble(wasiExitWat, exit); err != nil {
		t.Fatalf("assemble wasi exit module: %v", err)
	}
	out, stderr, code = runWasmtime(t, exit, root, "", nil)
	if code != 3 || stderr != "" || out != "stopping\n" {
		t.Fatalf("wasi exit: code=%d stderr=%q out=%q", code, stderr, out)
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

// compileFixture compiles one testdata fixture to a temp artifact with the
// given skin (browser by default, DESIGN-M3 §1c).
func compileFixture(t *testing.T, name string, skin lang.WatSkin) string {
	t.Helper()
	path := filepath.Join("testdata", name+".zhex")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), name+".wasm")
	if err := Compile(path, string(source), outPath, nil, skin); err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return outPath
}

// The differential gate (DESIGN.md §9, DESIGN-M2 §7): every fixture runs
// through the interpreter and the compiled module; stdout must match byte
// for byte and the exit status must agree.
func TestDifferential(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, name := range []string{
		"fib", "core", "lists", "choices", "arrays",
		"text", "text-to-int", "maps", "callbacks", "sort", "while", "string-int", "array-select",
		"fold-ref",
	} {
		t.Run(name, func(t *testing.T) {
			wasmPath := compileFixture(t, name, lang.WatSkinBrowser)
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

// growthBound is the wall-clock canary for the growth fixtures (#187):
// amortized appends finish in well under a second under both skins, while
// the grow-by-one appends this gate caught were O(n^2) — 100k appends copy
// ~5e9 elements and blow far past the bound. Generous enough that loaded
// CI machines stay green on the amortized path.
const growthBound = 10 * time.Second

// TestDifferentialGrowth (#187): the list-heavy fixture builds 100k-element
// lists and a 100k-line buffer by append, then digests them. Both skins
// must match the interpreter byte for byte and finish inside growthBound.
func TestDifferentialGrowth(t *testing.T) {
	requireTool(t, "wasm-tools")
	path := filepath.Join("testdata", "grow-lists.zhex")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var interpreted bytes.Buffer
	runErr := lang.Run(path, string(source), strings.NewReader(""), &interpreted)
	wantOut, wantCode := interpreted.String(), exitCode(runErr)

	browser := compileFixture(t, "grow-lists", lang.WatSkinBrowser)
	start := time.Now()
	out, _, code := runNode(t, browser)
	if elapsed := time.Since(start); elapsed > growthBound {
		t.Fatalf("browser skin took %s (bound %s): list growth is quadratic again", elapsed, growthBound)
	}
	if out != wantOut || code != wantCode {
		t.Fatalf("browser skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
	}

	wasi := compileFixture(t, "grow-lists", lang.WatSkinWASI)
	dir := t.TempDir()
	stdinPath := filepath.Join(dir, "stdin.txt")
	if err := os.WriteFile(stdinPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	out, _, code = runWasmtime(t, wasi, t.TempDir(), stdinPath, nil)
	if elapsed := time.Since(start); elapsed > growthBound {
		t.Fatalf("wasi skin took %s (bound %s): list growth is quadratic again", elapsed, growthBound)
	}
	if out != wantOut || code != wantCode {
		t.Fatalf("wasi skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
	}
}

// TestDifferentialStreamIO (#187): 100k stdin lines read through the
// read-to-EOF loop, each echoed with a prefix — the streaming shape whose
// per-line host crossings this change buffers. Correctness gate only (the
// win is a small constant factor; the #6 benchmark carries the numbers).
func TestDifferentialStreamIO(t *testing.T) {
	requireTool(t, "wasm-tools")
	var stdin bytes.Buffer
	for i := 0; i < 100000; i++ {
		fmt.Fprintf(&stdin, "row-%d\n", i%9973)
	}
	path := filepath.Join("testdata", "grow-stream.zhex")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var interpreted bytes.Buffer
	runErr := lang.Run(path, string(source), bytes.NewReader(stdin.Bytes()), &interpreted)
	wantOut, wantCode := interpreted.String(), exitCode(runErr)

	dir := t.TempDir()
	stdinPath := filepath.Join(dir, "stdin.txt")
	if err := os.WriteFile(stdinPath, stdin.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	browser := compileFixture(t, "grow-stream", lang.WatSkinBrowser)
	out, _, code := runNode(t, browser, "--stdin", stdinPath)
	if out != wantOut || code != wantCode {
		t.Fatalf("browser skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
	}

	wasi := compileFixture(t, "grow-stream", lang.WatSkinWASI)
	out, _, code = runWasmtime(t, wasi, t.TempDir(), stdinPath, nil)
	if out != wantOut || code != wantCode {
		t.Fatalf("wasi skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
	}
}

// sortBound is the wall-clock canary for the sort fixture (#188): a merge
// sort over the fixture's lists finishes in well under a second under both
// skins, while the insertion sort this gate caught was O(n^2) — the four
// heavy sorts compare and shift ~2e9 times and blow far past the bound.
const sortBound = 10 * time.Second

// TestDifferentialSort (#188): the fixture sorts 60k Int and 30k String
// lists with heavily duplicated keys — list.sort plus stable list.sort-by
// runs whose equal-key order must match the interpreter byte for byte —
// and must finish inside sortBound under both skins.
func TestDifferentialSort(t *testing.T) {
	requireTool(t, "wasm-tools")
	path := filepath.Join("testdata", "sort-lists.zhex")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var interpreted bytes.Buffer
	runErr := lang.Run(path, string(source), strings.NewReader(""), &interpreted)
	wantOut, wantCode := interpreted.String(), exitCode(runErr)

	browser := compileFixture(t, "sort-lists", lang.WatSkinBrowser)
	start := time.Now()
	out, _, code := runNode(t, browser)
	if elapsed := time.Since(start); elapsed > sortBound {
		t.Fatalf("browser skin took %s (bound %s): list sorting is quadratic again", elapsed, sortBound)
	}
	if out != wantOut || code != wantCode {
		t.Fatalf("browser skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
	}

	wasi := compileFixture(t, "sort-lists", lang.WatSkinWASI)
	dir := t.TempDir()
	stdinPath := filepath.Join(dir, "stdin.txt")
	if err := os.WriteFile(stdinPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	out, _, code = runWasmtime(t, wasi, t.TempDir(), stdinPath, nil)
	if elapsed := time.Since(start); elapsed > sortBound {
		t.Fatalf("wasi skin took %s (bound %s): list sorting is quadratic again", elapsed, sortBound)
	}
	if out != wantOut || code != wantCode {
		t.Fatalf("wasi skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
	}
}

// mapBound is the wall-clock canary for the map fixture (#188): building
// the fixture's 100k-entry maps with amortized storage finishes in about a
// second under both skins, while the grow-by-one-entry puts this gate
// caught copied ~1e10 words and blew far past the bound.
const mapBound = 10 * time.Second

// TestDifferentialMaps (#188): the fixture builds 100k-entry String- and
// Int-keyed maps by put, then exercises remove, re-put, into, and
// insertion-order iteration. Both skins must match the interpreter byte
// for byte and finish inside mapBound.
func TestDifferentialMaps(t *testing.T) {
	requireTool(t, "wasm-tools")
	path := filepath.Join("testdata", "grow-maps.zhex")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var interpreted bytes.Buffer
	runErr := lang.Run(path, string(source), strings.NewReader(""), &interpreted)
	wantOut, wantCode := interpreted.String(), exitCode(runErr)

	browser := compileFixture(t, "grow-maps", lang.WatSkinBrowser)
	start := time.Now()
	out, _, code := runNode(t, browser)
	if elapsed := time.Since(start); elapsed > mapBound {
		t.Fatalf("browser skin took %s (bound %s): map growth is quadratic again", elapsed, mapBound)
	}
	if out != wantOut || code != wantCode {
		t.Fatalf("browser skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
	}

	wasi := compileFixture(t, "grow-maps", lang.WatSkinWASI)
	dir := t.TempDir()
	stdinPath := filepath.Join(dir, "stdin.txt")
	if err := os.WriteFile(stdinPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	start = time.Now()
	out, _, code = runWasmtime(t, wasi, t.TempDir(), stdinPath, nil)
	if elapsed := time.Since(start); elapsed > mapBound {
		t.Fatalf("wasi skin took %s (bound %s): map growth is quadratic again", elapsed, mapBound)
	}
	if out != wantOut || code != wantCode {
		t.Fatalf("wasi skin: code=%d (want %d)\ninterpreted: %q\ncompiled:    %q", code, wantCode, wantOut, out)
	}
}

func TestWASIOutputScratchIsolation(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, tc := range []struct {
		name   string
		source string
		want   string
	}{
		{"long-print", fmt.Sprintf("print(%q)\n", strings.Repeat("a", 9000)), strings.Repeat("a", 9000) + "\n"},
		{"multiple-flushes", fmt.Sprintf("print(\"before\")\nprint(%q)\nprint(\"after\")\n", strings.Repeat("abé", 30000)), "before\n" + strings.Repeat("abé", 30000) + "\nafter\n"},
		{"file-scratch", "print(\"before\")\nlet contents = file.read(\"large.txt\") try\nprint(text.length(contents))\nprint(\"after\")\n", "before\n9000\nafter\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "large.txt"), []byte(strings.Repeat("x", 9000)), 0o600); err != nil {
				t.Fatal(err)
			}
			outPath := filepath.Join(t.TempDir(), "out.wasm")
			if err := Compile("output.zhex", tc.source, outPath, nil, lang.WatSkinWASI); err != nil {
				t.Fatal(err)
			}
			out, stderr, code := runWasmtime(t, outPath, dir, "", nil)
			if code != 0 || out != tc.want {
				t.Fatalf("code=%d, output bytes=%d (want %d), stderr=%s", code, len(out), len(tc.want), stderr)
			}
		})
	}
}

func TestBufferedStdinScratchIsolation(t *testing.T) {
	requireTool(t, "wasm-tools")
	source := `let first = read-line("") try
var padding = "x"
for i in range(0, 24)
    padding = padding + padding
print(padding)
let second = read-line("") try
print(second)
`
	dir := t.TempDir()
	stdinPath := filepath.Join(dir, "stdin.txt")
	if err := os.WriteFile(stdinPath, []byte("a\nb\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("x", 1<<24) + "\nb\n"
	for _, skin := range []lang.WatSkin{lang.WatSkinBrowser, lang.WatSkinWASI} {
		t.Run(fmt.Sprint(skin), func(t *testing.T) {
			outPath := filepath.Join(t.TempDir(), "out.wasm")
			if err := Compile("input.zhex", source, outPath, nil, skin); err != nil {
				t.Fatal(err)
			}
			var out, stderr string
			var code int
			if skin == lang.WatSkinBrowser {
				out, stderr, code = runNode(t, outPath, "--stdin", stdinPath)
			} else {
				out, stderr, code = runWasmtime(t, outPath, dir, stdinPath, nil)
			}
			if code != 0 || out != want {
				t.Fatalf("code=%d, output bytes=%d (want %d), stderr=%s", code, len(out), len(want), stderr)
			}
		})
	}
}

func TestWASIFileBufferScratchIsolation(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, tc := range []struct {
		name    string
		padding int
		first   string
	}{
		{"handle-state", 14, "one"},
		{"pending-bytes", 21, "one"},
		{"long-line", 0, strings.Repeat("a", 1100000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "lines.txt"), []byte(tc.first+"\ntwo\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			source := fmt.Sprintf(`let input = file.open("lines.txt") try
print(text.length(file.read-line(input) try))
var padding = "x"
for i in range(0, %d)
    padding = padding + padding
print(padding)
print(file.read-line(input) try)
file.close(input)
`, tc.padding)
			outPath := filepath.Join(t.TempDir(), "out.wasm")
			if err := Compile("input.zhex", source, outPath, nil, lang.WatSkinWASI); err != nil {
				t.Fatal(err)
			}
			out, stderr, code := runWasmtime(t, outPath, dir, "", nil)
			want := fmt.Sprintf("%d\n", len(tc.first)) + strings.Repeat("x", 1<<tc.padding) + "\ntwo\n"
			if code != 0 || out != want {
				t.Fatalf("code=%d, output bytes=%d (want %d), stderr=%s", code, len(out), len(want), stderr)
			}
		})
	}
}

func TestCompiledZeroDivisorPreservesOutput(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, op := range []string{"/", "%"} {
		t.Run(op, func(t *testing.T) {
			source := "print(\"before\")\nprint(1 " + op + " 0)\n"
			for _, skin := range []lang.WatSkin{lang.WatSkinBrowser, lang.WatSkinWASI} {
				outPath := filepath.Join(t.TempDir(), "zero.wasm")
				if err := Compile("zero.zhex", source, outPath, nil, skin); err != nil {
					t.Fatal(err)
				}
				var out, stderr string
				var code int
				if skin == lang.WatSkinBrowser {
					out, stderr, code = runNode(t, outPath)
				} else {
					out, stderr, code = runWasmtime(t, outPath, t.TempDir(), "", nil)
				}
				if code != 1 || out != "before\n" || stderr != "runtime error: division or remainder by zero\n" {
					t.Fatalf("skin %v: code=%d, stdout=%q, stderr=%s", skin, code, out, stderr)
				}
			}
		})
	}
}

// m3Case is one milestone-3 differential fixture (DESIGN-M3 §9): a program
// plus the working tree it observes — seed files, a stdin string, and
// injected environment pairs. The same tree backs all three runs: the
// interpreter (cwd), the browser skin (the runner's virtual FS root), and
// the WASI skin (the wasmtime preopen). Portable-path discipline keeps the
// programs on relative "/"-separated paths only (§2).
type m3Case struct {
	name  string
	seeds map[string]string // relative path -> content (directories implied)
	stdin string
	env   [][2]string
}

// m3DifferentialCases is the §9 fixture list that must exit 0 with matching
// stdout: path words, file operations, streaming, read-line, buffers,
// durations, json trees, env, and the style-adapted acceptance pair.
func m3DifferentialCases() []m3Case {
	sample, err := os.ReadFile(filepath.Join("testdata", "sample.txt"))
	if err != nil {
		panic(err)
	}
	return []m3Case{
		{name: "paths"},
		{name: "files"},
		{name: "stream"},
		{name: "readline", stdin: "Ada\nGrace\n"},
		{name: "readline-blank", stdin: "Ada\n\nGrace\n"},
		{name: "readline-detail", stdin: "Ada\n\nGrace\n"},
		{name: "grow-stream", stdin: "\r\n" + strings.Repeat("é", 5000) + "\r\npartial\r"},
		{name: "buffers"},
		{name: "duration"},
		{name: "json"},
		{name: "env", env: [][2]string{{"ZHEX_TEST_INJECTED", "from-runner"}}},
		{name: "text-stats-wasm", seeds: map[string]string{"sample.txt": string(sample)}},
		{name: "file-work-wasm"},
	}
}

func TestSiteArrayRankDiagnostic(t *testing.T) {
	requireTool(t, "wasm-tools")
	requireTool(t, "node")
	outPath := filepath.Join(t.TempDir(), "rank.wasm")
	source := "let shape: List<Int> = []\nlet values = array.new(shape, 0)\n"
	if err := Compile("rank.zhex", source, outPath, nil, lang.WatSkinBrowser); err != nil {
		t.Fatal(err)
	}
	script := `import { readFileSync } from "node:fs";
import { runZhex } from "../../site/src/scripts/zhex-run.mjs";
console.log(runZhex(readFileSync(process.argv[1])).error);`
	out, err := exec.Command("node", "--input-type=module", "-e", script, outPath).CombinedOutput()
	if err != nil {
		t.Fatalf("site runner: %v: %s", err, out)
	}
	if want := "runtime error: array rank must be between 1 and 32\n"; string(out) != want {
		t.Fatalf("site runner error = %q, want %q", out, want)
	}
}

// m3DiagnosticsCases are the §9 failure fixtures: unhandled Results whose
// staged message (codes 16-17) the runner decodes after the M1 convention —
// `runtime error: ` plus the interpreter's first line minus its position.
func m3DiagnosticsCases() []m3Case {
	return []m3Case{
		{name: "files-fail-read"},
		{name: "files-fail-write"},
		{name: "files-fail-delete", seeds: map[string]string{"folder/keep.txt": "x\n"}},
		{name: "files-fail-list", seeds: map[string]string{"notes.txt": "hi\n"}},
		{name: "files-fail-dir", seeds: map[string]string{"folder/keep.txt": ""}},
		{name: "stream-fail"},
		{name: "readline-eof", stdin: ""},
		{name: "buffer-fail"},
		{name: "json-fail"},
		{name: "env-fail-parse"},
		{name: "env-fail-missing"},
	}
}

// m3ExpectedStderr pins each diagnostics case's exact stderr line,
// derived from the interpreter's message on this machine.
func m3ExpectedStderr(name string) string {
	switch name {
	case "files-fail-read":
		return "runtime error: cannot open \"nope.txt\": open nope.txt: no such file or directory\n"
	case "files-fail-write":
		return "runtime error: cannot open \"missing/file.txt\" for writing: open missing/file.txt: no such file or directory\n"
	case "files-fail-delete":
		return "runtime error: cannot delete \"folder\": remove folder: directory not empty\n"
	case "files-fail-list":
		return "runtime error: cannot list \"notes.txt\": open notes.txt: not a directory\n"
	case "files-fail-dir":
		return "runtime error: cannot open \"folder\": read folder: is a directory\n"
	case "stream-fail":
		return "runtime error: cannot open \"missing.txt\": open missing.txt: no such file or directory\n"
	case "readline-eof":
		return "runtime error: end of input\n"
	case "buffer-fail":
		return "runtime error: buffer index 5 is outside 0..2\n"
	case "json-fail":
		return "runtime error: invalid JSON: invalid character 'x' looking for beginning of value at \"\"\n"
	case "env-fail-parse":
		return "runtime error: line 3 of \"bad.env\" is not a KEY=VALUE assignment: \"BAD LINE\"\n"
	case "env-fail-missing":
		return "runtime error: cannot read \".env\": open .env: no such file or directory\n"
	}
	panic("unknown diagnostics fixture " + name)
}

// prepareM3Root builds the case's working tree: seed files under a fresh
// temp root, a stdin file, and env pairs applied to this process for the
// interpreter run. Returns the root and the stdin file path.
func prepareM3Root(t *testing.T, c m3Case) (string, string) {
	t.Helper()
	root := t.TempDir()
	for path, content := range c.seeds {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stdinPath := filepath.Join(root, "..", "stdin.txt")
	if err := os.WriteFile(stdinPath, []byte(c.stdin), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, pair := range c.env {
		t.Setenv(pair[0], pair[1])
	}
	return root, stdinPath
}

// interpretFixture runs one fixture through the interpreter with the
// process cwd inside the case's working tree, returning stdout and the exit
// status the CLI would produce.
func interpretFixture(t *testing.T, c m3Case, root string) (string, int) {
	t.Helper()
	path := filepath.Join("testdata", c.name+".zhex")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer func() { os.Chdir(previous) }()
	var interpreted bytes.Buffer
	runErr := lang.Run(path, string(source), strings.NewReader(c.stdin), &interpreted)
	return interpreted.String(), exitCode(runErr)
}

// TestDifferentialM3 runs every milestone-3 fixture under both skins
// (DESIGN-M3 §9): the browser module executes under node run.mjs with the
// virtual FS seeded from the working tree; the WASI module executes under
// wasmtime with the tree preopened at /. Both must match the interpreter's
// stdout byte for byte and its exit status.
func TestDifferentialM3(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, c := range m3DifferentialCases() {
		t.Run(c.name, func(t *testing.T) {
			// each execution gets a fresh root: the interpreter's own file
			// effects would otherwise leak into the compiled runs
			root, _ := prepareM3Root(t, c)
			wantOut, wantCode := interpretFixture(t, c, root)

			browserRoot, browserStdin := prepareM3Root(t, c)
			browser := compileFixture(t, c.name, lang.WatSkinBrowser)
			args := []string{"--root", browserRoot, "--stdin", browserStdin}
			for _, pair := range c.env {
				args = append(args, "--env", pair[0]+"="+pair[1])
			}
			out, stderr, code := runNode(t, browser, args...)
			if code != wantCode || stderr != "" || out != wantOut {
				t.Fatalf("browser skin: code=%d (want %d) stderr=%q\nstdout mismatch:\ninterpreted: %q\ncompiled:    %q", code, wantCode, stderr, wantOut, out)
			}

			wasiRoot, wasiStdin := prepareM3Root(t, c)
			wasi := compileFixture(t, c.name, lang.WatSkinWASI)
			var env []string
			for _, pair := range c.env {
				env = append(env, pair[0]+"="+pair[1])
			}
			out, stderr, code = runWasmtime(t, wasi, wasiRoot, wasiStdin, env)
			if code != wantCode || stderr != "" || out != wantOut {
				t.Fatalf("wasi skin: code=%d (want %d) stderr=%q\nstdout mismatch:\ninterpreted: %q\ncompiled:    %q", code, wantCode, stderr, wantOut, out)
			}
		})
	}
}

// TestCompiledDiagnosticsM3 pins the exact stderr of each milestone-3
// failure fixture under both skins: the staged message renders after the M1
// convention (runtime error: plus the interpreter's first line minus its
// position), stdout produced before the failure still matches, and the exit
// status is 1.
func TestCompiledDiagnosticsM3(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, c := range m3DiagnosticsCases() {
		t.Run(c.name, func(t *testing.T) {
			root, stdinPath := prepareM3Root(t, c)
			wantOut, _ := interpretFixture(t, c, root)
			wantStderr := m3ExpectedStderr(c.name)

			browser := compileFixture(t, c.name, lang.WatSkinBrowser)
			args := []string{"--root", root, "--stdin", stdinPath}
			for _, pair := range c.env {
				args = append(args, "--env", pair[0]+"="+pair[1])
			}
			out, stderr, code := runNode(t, browser, args...)
			if code != 1 || stderr != wantStderr || out != wantOut {
				t.Fatalf("browser skin: code=%d stderr=%q\nwant stderr %q\nstdout mismatch:\ninterpreted: %q\ncompiled:    %q", code, stderr, wantStderr, wantOut, out)
			}

			wasi := compileFixture(t, c.name, lang.WatSkinWASI)
			var env []string
			for _, pair := range c.env {
				env = append(env, pair[0]+"="+pair[1])
			}
			out, stderr, code = runWasmtime(t, wasi, root, stdinPath, env)
			if code != 1 || stderr != wantStderr || out != wantOut {
				t.Fatalf("wasi skin: code=%d stderr=%q\nwant stderr %q\nstdout mismatch:\ninterpreted: %q\ncompiled:    %q", code, stderr, wantStderr, wantOut, out)
			}
		})
	}
}

// The milestone-3 fence inventory (DESIGN-M3 §8/§9.11): these constructs
// fail emission with the not-available-in-build diagnostic before any tool
// runs — the fence message and milestone-neutral hint are asserted verbatim.
func TestM3Fences(t *testing.T) {
	for _, tc := range []struct{ name, source, construct string }{
		{"free-space", "let free = file.free-space(\".\") try\nprint(int.to-string(free))\n", "file.free-space"},
		// xml-decode unfenced in M4 (DESIGN-M4 2) — B1's sanctioned touch
		{"date-new", "let d = date.new(2026, 1, 15)\nprint(date.to-string(d))\n", "date.new"},
		{"console-arg", "let first = console.arg(0) try\nprint(first)\n", "console.arg"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Compile(tc.name+".zhex", tc.source, filepath.Join(t.TempDir(), "out.wasm"), nil, lang.WatSkinBrowser)
			if err == nil || !strings.Contains(err.Error(), tc.construct+" is not available in this build") {
				t.Fatalf("fence %s: got %v", tc.construct, err)
			}
			if !strings.Contains(err.Error(), "this construct is not compiled by zhex compile yet; run it with zhex run instead") {
				t.Fatalf("fence %s: hint missing in %v", tc.construct, err)
			}
		})
	}
}

// Runtime diagnostics in the artifact print the pinned §6a strings without
// positions; stdout produced before the failure still matches the
// interpreter byte for byte. The milestone-2 rows add the staged-message
// codes 12 and 15 (DESIGN-M2 §2c, §7).
// TestCompiledIntWidthFence pins the #167 fence: compiled wasm carries
// Ints at the signed 64-bit width while the interpreter computes 128-bit,
// so a product the interpreter prints traps as integer overflow in the
// compiled build. Wide literals refuse at emission (TestWatFences in
// internal/lang); this is the runtime half of the same fence.
func TestCompiledIntWidthFence(t *testing.T) {
	requireTool(t, "wasm-tools")
	source := "print(3037000500 * 3037000500)\n"
	var interpreted bytes.Buffer
	if err := lang.Run("fence.zhex", source, strings.NewReader(""), &interpreted); err != nil || interpreted.String() != "9223372037000250000\n" {
		t.Fatalf("interpreter run: %q %v; want the 128-bit product", interpreted.String(), err)
	}
	outPath := filepath.Join(t.TempDir(), "fence.wasm")
	if err := Compile("fence.zhex", source, outPath, nil, lang.WatSkinBrowser); err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, stderr, code := runNode(t, outPath)
	if code != 1 || out != "" || stderr != "runtime error: integer overflow\n" {
		t.Fatalf("compiled run: code=%d stdout=%q stderr=%q; want the build's 64-bit overflow fence", code, out, stderr)
	}
}

func TestCompiledDiagnostics(t *testing.T) {
	requireTool(t, "wasm-tools")
	for _, tc := range []struct{ name, stderr string }{
		{"overflow", "runtime error: integer overflow\n"},
		{"depth", "runtime error: word call depth limit of 256 exceeded\n"},
		{"text-to-int-fail", "runtime error: \"x\" is not a signed 64-bit decimal integer\n"},
		{"maps-iterate-fail", "runtime error: collection is being iterated\n"},
		{"array-select-rank", "runtime error: array.select requires a rank-2 array, found rank 3\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wasmPath := compileFixture(t, tc.name, lang.WatSkinBrowser)
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
