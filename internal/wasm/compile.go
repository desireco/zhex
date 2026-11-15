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
)

// missingToolMessage is the D6 missing-tool diagnostic, pinned verbatim in
// internal/wasm/DESIGN.md §8: a factual first line naming the tool and its
// code, then a one-line install hint per platform.
const missingToolMessage = "wasm-tools is not installed; zhex compile needs it to assemble the WebAssembly module (code: missing-tool)\n" +
	"hint: install it — cargo install wasm-tools (rustup), pacman -S wasm-tools (Arch), or brew install wasm-tools (macOS)"

// MissingToolError reports that wasm-tools is not on PATH.
type MissingToolError struct{}

func (e *MissingToolError) Error() string { return missingToolMessage }

// assemble writes wat to a temporary file and assembles it with
// `wasm-tools parse <in.wat> -o <out.wasm>`. wabt's wat2wasm cannot
// assemble WasmGC instructions (DESIGN.md, Surprises 1), so wasm-tools is
// the only supported assembler.
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
		return fmt.Errorf("wasm-tools failed: %s", lastLine(stderr.String()))
	}
	return nil
}

// lastLine trims an error stream to its final non-empty line — wasm-tools
// ends its diagnostics with the failing token's summary.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return lines[len(lines)-1]
}
