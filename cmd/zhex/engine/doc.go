// Command engine is the Zhex interpreter as a browser-ready WebAssembly
// module (#91, Track A of docs/wasm-plan.md): one shared engine that the
// site's playground (#88) and the read-line spike harness load through
// loader.js — never a copy. Built with GOOS=js GOARCH=wasm (see the justfile's
// engine recipe), it exposes the engine API on globalThis.zhex:
//
//	runSource(source, stdin) -> {output, diagnostics, report, trace, exit}
//	runInteractive(source, onOutput, onDone) -> {deliver(line), close()}
//	newSession(stdin) -> {enter, words, takeOutput, takeDiagnostics}
//	isIncomplete(source) -> boolean
//	delimiterIndent(source) -> number | null
//	lastStatementIndent(source) -> number | null
//	setResolve(fn)  // fn(name) -> {path, source} or null, the `use` supplier
//
// runSource is one-shot: output is program stdout, diagnostics the
// console.fail channel, report the --report=json envelope of a failed run
// (null otherwise), trace the --trace JSONL events ("" when none), and exit
// the status zhex run would exit with. runInteractive (#122) is the async
// stdin seam over the same one-shot run: the program runs on its own
// goroutine, read-line parks on host-delivered input, onOutput(text)
// receives every write as it happens (prompts included, so they land before
// the read parks), and onDone(result) receives runSource's result shape once
// the program finishes; the returned handle's deliver(line) queues one line
// (a missing trailing newline is added) and close() ends the stream — a read
// past close fails with the coded end-of-file failure, exactly like
// runSource's supplied stdin running out. Bulk-delivering more than 16
// lines before the program reads parks the deliver callback until it drains,
// but never past close or the run's end. A caller abandoning a run must call
// close(): it unparks a blocked read as end-of-file, lets the run goroutine
// finish, and Releases the handle's deliver/close wrappers (the run's exit
// Releases them too, whichever comes first).
// newSession carries the REPL law —
// words, names, and adopted modules survive entries; enter(source) ->
// {echo, error, report, exit} with the redefined note and print output in the
// buffer takeOutput drains, console.fail output in takeDiagnostics.
//
// The capability boundary is coded: the filesystem words, file.stream,
// console.arg, and env.load fail with not-available-in-build diagnostics
// (internal/lang/boundary.go); the lexical path words, read-line against the
// supplied stdin or the delivered interactive stream, console.args-count,
// and env.get keep working. timezone.load is not fenced either — the
// interpreter embeds the time database (internal/lang/temporal.go imports
// time/tzdata), so zone names resolve without the operating system and an
// unknown name fails inside its own Result with timezone-load.
package main
