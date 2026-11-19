package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{"compile"},
		{"compile", "-o"},
		{"compile", "a.zhex", "--bogus"},
		{"compile", "-o", "out.wasm"},
		{"compile", "a.zhex", "b.zhex"},
		{"compile", "a.zhex", "-o", "x.wasm", "-o", "y.wasm"},
		{"compile", "a.zhex", "--target=wasm3"},
		{"compile", "a.zhex", "--target"},
	} {
		var out, diagnostics bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &diagnostics); code != 2 || !strings.Contains(diagnostics.String(), "usage:") {
			t.Fatalf("args %v: code %d stderr %q", args, code, diagnostics.String())
		}
	}
}

// The native target (#185): a directory of Go sources plus a built
// binary whose run matches zhex run byte-for-byte — the CLI seam of the
// M1 smoke.
func TestCompileNativeTargetBuildsAndRuns(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not installed; skipping")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "program.zhex")
	if err := os.WriteFile(path, []byte("print(\"native hi\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outDir := filepath.Join(dir, "program")
	var out, diagnostics bytes.Buffer
	code := run([]string{"compile", path, "--target=native", "-o", outDir}, strings.NewReader(""), &out, &diagnostics)
	if code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("code %d stderr %q", code, diagnostics.String())
	}
	for _, name := range []string{"main.go", "go.mod", filepath.Join("zhexrt", "int.go")} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	binary := filepath.Join(outDir, "program")
	var runOut bytes.Buffer
	cmd := exec.Command(binary)
	cmd.Stdout = &runOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("run built binary: %v", err)
	}
	if runOut.String() != "native hi\n" {
		t.Fatalf("built binary output %q", runOut.String())
	}
}

// Checker errors print through the existing renderer — position, excerpt,
// caret, hint — and no artifact is written. This path needs no wasm-tools.
func TestCompileCheckerErrorRendersPositionedDiagnostic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.zhex")
	if err := os.WriteFile(path, []byte("print(a-b)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "out.wasm")
	var out, diagnostics bytes.Buffer
	code := run([]string{"compile", path, "-o", outPath}, strings.NewReader(""), &out, &diagnostics)
	if code != 1 || !strings.Contains(diagnostics.String(), path+":1:7: unknown name \"a-b\"") ||
		!strings.Contains(diagnostics.String(), "hint:") {
		t.Fatalf("code %d stderr %q", code, diagnostics.String())
	}
	if _, err := os.Stat(outPath); !os.IsNotExist(err) {
		t.Fatalf("artifact written despite checker error: %v", err)
	}
}

// A clean program without wasm-tools on PATH prints exactly the pinned
// missing-tool diagnostic and writes nothing (DESIGN.md §8).
func TestCompileMissingToolDiagnostic(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	path := filepath.Join(t.TempDir(), "program.zhex")
	if err := os.WriteFile(path, []byte("print(1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	code := run([]string{"compile", path}, strings.NewReader(""), &out, &diagnostics)
	want := "wasm-tools is not installed; zhex compile needs it to assemble the WebAssembly module (code: missing-tool)\n" +
		"hint: install it — cargo install wasm-tools (rustup), pacman -S wasm-tools (Arch), or brew install wasm-tools (macOS)\n"
	if code != 1 || diagnostics.String() != want {
		t.Fatalf("code %d stderr %q", code, diagnostics.String())
	}
	if _, err := os.Stat(strings.TrimSuffix(path, ".zhex") + ".wasm"); !os.IsNotExist(err) {
		t.Fatalf("artifact written despite missing tool: %v", err)
	}
}

func TestCompileWritesArtifact(t *testing.T) {
	if _, err := exec.LookPath("wasm-tools"); err != nil {
		t.Skip("wasm-tools is not installed; skipping")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "program.zhex")
	if err := os.WriteFile(path, []byte("print(\"hi\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	outPath := filepath.Join(dir, "out.wasm")
	code := run([]string{"compile", path, "-o", outPath}, strings.NewReader(""), &out, &diagnostics)
	if code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("code %d stderr %q", code, diagnostics.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil || !bytes.HasPrefix(data, []byte("\x00asm")) {
		t.Fatalf("artifact: %v (%d bytes)", err, len(data))
	}

	// Without -o the output derives from the input name.
	derived := filepath.Join(dir, "derived")
	if err := os.WriteFile(derived+".zhex", []byte("print(\"hi\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diagnostics.Reset()
	code = run([]string{"compile", derived + ".zhex"}, strings.NewReader(""), &out, &diagnostics)
	if code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("default output: code %d stderr %q", code, diagnostics.String())
	}
	if data, err := os.ReadFile(derived + ".wasm"); err != nil || !bytes.HasPrefix(data, []byte("\x00asm")) {
		t.Fatalf("default artifact: %v (%d bytes)", err, len(data))
	}

	// A file without the .zhex suffix appends .wasm instead of replacing.
	plain := filepath.Join(dir, "plain")
	if err := os.WriteFile(plain, []byte("print(1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	diagnostics.Reset()
	code = run([]string{"compile", plain}, strings.NewReader(""), &out, &diagnostics)
	if code != 0 || diagnostics.Len() != 0 {
		t.Fatalf("append output: code %d stderr %q", code, diagnostics.String())
	}
	if _, err := os.Stat(plain + ".wasm"); err != nil {
		t.Fatalf("appended artifact: %v", err)
	}
}
