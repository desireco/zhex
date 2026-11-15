// Package wasm orchestrates Zhex → WebAssembly compilation: WAT emission
// lives in internal/lang (CompileWat) and this package assembles the text
// into a binary artifact with the external wasm-tools assembler.
package wasm

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"zhex/internal/lang"
)

// missingToolMessage is the D6 missing-tool diagnostic, pinned verbatim in
// internal/wasm/DESIGN.md §8: a factual first line naming the tool and its
// code, then a one-line install hint per platform.
const missingToolMessage = "wasm-tools is not installed; zhex compile needs it to assemble the WebAssembly module (code: missing-tool)\n" +
	"hint: install it — cargo install wasm-tools (rustup), pacman -S wasm-tools (Arch), or brew install wasm-tools (macOS)"

// MissingToolError reports that wasm-tools is not on PATH.
type MissingToolError struct{}

func (e *MissingToolError) Error() string { return missingToolMessage }

// Compile checks the source, emits WAT (lang.CompileWat), and assembles it
// into outPath; a nil return means the artifact is written. Checker errors
// pass through unchanged so the CLI renders them exactly as run does.
func Compile(path, source, outPath string, resolve func(string) (lang.ModuleSource, bool)) error {
	wat, err := lang.CompileWat(path, source, resolve)
	if err != nil {
		return err
	}
	return assemble(wat, outPath)
}

// assemble writes wat to a temporary file and assembles it with
// `wasm-tools parse <in.wat> -o <out.wasm>`, then validates the result —
// parse assembles without checking types (DESIGN.md, Surprises 1), so the
// validate step is what catches emitter bugs. wabt's wat2wasm cannot
// assemble WasmGC instructions, so wasm-tools is the only supported
// assembler.
func assemble(wat, outPath string) error {
	tool, err := exec.LookPath("wasm-tools")
	if err != nil {
		return &MissingToolError{}
	}
	tmp, err := os.CreateTemp("", "zhex-*.wat")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(wat); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd := exec.Command(tool, "parse", tmp.Name(), "-o", outPath)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return toolFailure(stderr.String())
	}
	stderr.Reset()
	validate := exec.Command(tool, "validate", outPath)
	validate.Stderr = &stderr
	if err := validate.Run(); err != nil {
		os.Remove(outPath)
		return toolFailure(stderr.String())
	}
	return nil
}

// toolFailure renders a failed wasm-tools invocation; the tool's own stderr
// is the only detail available, so it is always included.
func toolFailure(stderr string) error {
	detail := strings.TrimSpace(stderr)
	if detail == "" {
		detail = "(no output)"
	}
	return fmt.Errorf("wasm-tools failed: %s", detail)
}
