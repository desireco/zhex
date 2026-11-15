// Command dogfood runs the #65 prototype: a Zhex-written REPL front end
// driving the Go session engine through the per-run repl.* seam. Test-harness
// only — the production `zhex repl` entry point is untouched.
package main

import (
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"

	"zhex/internal/lang"
)

//go:embed repl.zhex
var driverSource string

func main() { os.Exit(run(os.Stdin, os.Stdout, os.Stderr)) }

// run mirrors production's wiring: one shared buffered reader for prompts,
// entries, and program read-line; echo and prompts on output; engine
// diagnostics (console.fail) on diagnostics; exit status propagated.
func run(input io.Reader, output, diagnostics io.Writer) int {
	session := lang.NewSession(input, output, nil, func(name string) (lang.ModuleSource, bool) {
		path := name + ".zhex"
		source, err := os.ReadFile(path)
		if err != nil {
			return lang.ModuleSource{}, false
		}
		return lang.ModuleSource{Path: path, Source: string(source)}, true
	})
	session.SetDiagnostics(diagnostics)
	if _, err := session.Enter("word bye() -> Unit\n    console.exit(0)\n"); err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	session.RegisterReplSeam()
	// Load-bearing invariant: the driver always ends through console.exit (the
	// eof branch), so Enter takes the error path here and never adopts the
	// driver's words — that is what keeps repl.words() and exports clean. A
	// driver that returned normally would silently pollute every session.
	if _, err := session.Enter(driverSource); err != nil {
		var exit *lang.ExitError
		if errors.As(err, &exit) {
			return exit.Status
		}
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	return 0
}
