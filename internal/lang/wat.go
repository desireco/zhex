package lang

import "errors"

// CompileWat checks the program exactly like CompileModules (checker errors
// return unchanged) and emits the full WebAssembly text: the runtime prelude,
// program-specific types, one wasm function per word, zhex_main for the
// top-level statements, and the exports (internal/wasm/DESIGN.md §1, §7).
// Constructs outside the wasm milestone-1 subset fail with a positioned
// not-available-in-build diagnostic.
func CompileWat(entryPath, entrySource string, resolve func(string) (ModuleSource, bool)) (string, error) {
	return "", errors.New("wat emitter: not implemented")
}
