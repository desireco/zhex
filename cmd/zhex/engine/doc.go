// Command engine is the Zhex interpreter as a browser-ready WebAssembly
// module (#91, Track A of docs/wasm-plan.md): one shared engine that the
// site's playground (#88) and the read-line spike harness load through
// loader.js — never a copy. Built with GOOS=js GOARCH=wasm (see the justfile's
// engine recipe), it exposes the engine API on globalThis.zhex:
//
//   runSource(source, stdin) -> {output, diagnostics, report, trace, exit}
//   newSession(stdin) -> {enter, words, takeOutput, takeDiagnostics}
//   isIncomplete(source) -> boolean
//   delimiterIndent(source) -> number | null
//   lastStatementIndent(source) -> number | null
//   setResolve(fn)  // fn(name) -> {path, source} or null, the `use` supplier
//
// runSource is one-shot: output is program stdout, diagnostics the
// console.fail channel, report the --report=json envelope of a failed run
// (null otherwise), trace the --trace JSONL events ("" when none), and exit
// the status zhex run would exit with. newSession carries the REPL law —
// words, names, and adopted modules survive entries; enter(source) ->
// {echo, error, report, exit} with the redefined note and print output in the
// buffer takeOutput drains, console.fail output in takeDiagnostics.
//
// The capability boundary is coded: the filesystem words, file.stream,
// console.arg, and env.load fail with not-available-in-build diagnostics
// (internal/lang/boundary.go); the lexical path words, read-line against the
// supplied stdin, console.args-count, and env.get keep working.
package main
