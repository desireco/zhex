// Package native orchestrates Zhex → Go-native compilation: Go emission
// lives in internal/lang (CompileNativeSource) and this package writes the
// self-contained program directory (main.go, go.mod, the zhexrt runtime
// sources) and builds it with the Go toolchain (#185, the Option 1
// report's M1 seam).
package native

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"zhex/internal/lang"
	"zhex/internal/zhexrt"
)

// generatedModule names every generated program's Go module. One shared
// name keeps the Go build cache warm across programs: the zhexrt sources
// compile once.
const generatedModule = "zhexprog"

// MissingToolError reports that the Go toolchain is absent: the program
// sources and go.mod are written, and the message names the build.
type MissingToolError struct{ Dir string }

func (e *MissingToolError) Error() string {
	return fmt.Sprintf("go is not installed; the program source is in %s — build it with: (cd %s && go build)", e.Dir, e.Dir)
}

// Compile checks the source, emits the Go program under outDir, and
// builds the binary named after outDir with `go build`. Checker errors
// pass through unchanged so the CLI renders them exactly as run does;
// without go on PATH the sources are still written and the returned
// MissingToolError names the build. Cross-compilation follows
// GOOS/GOARCH, which the toolchain reads from the environment.
func Compile(path, source, outDir string, resolve func(string) (lang.ModuleSource, bool)) error {
	mainGo, err := lang.CompileNativeSource(path, source, resolve)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(outDir, "zhexrt"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "main.go"), []byte(mainGo), 0o644); err != nil {
		return err
	}
	goMod := fmt.Sprintf("module %s\n\ngo 1.23\n", generatedModule)
	if err := os.WriteFile(filepath.Join(outDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		return err
	}
	for name, text := range zhexrt.Files() {
		if err := os.WriteFile(filepath.Join(outDir, "zhexrt", name), []byte(text), 0o644); err != nil {
			return err
		}
	}
	tool, err := exec.LookPath("go")
	if err != nil {
		return &MissingToolError{Dir: outDir}
	}
	binary := filepath.Join(outDir, filepath.Base(filepath.Clean(outDir)))
	build := exec.Command(tool, "build", "-o", binary, ".")
	build.Dir = outDir
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("go build failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
