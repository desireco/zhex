# Zhex roadmap

Build a readable language from small, composable words. The Go interpreter lets us try the syntax and settle behavior before choosing a compiled implementation.

## Done

The first language milestone runs real programs with checked types and console input and output.

- Interpreter: typed words, inferred locals, conditions, bounded loops, and checked flows.
- Working examples: composition, console input, FizzBuzz, Fibonacci, factorial, and palindrome checking.
- Executable specification: tests cover syntax, types, scope, arithmetic boundaries, Unicode, and failure paths.
- Getting started: documentation, a public examples website, and local highlighting packages for Vim/Neovim, VS Code, and Zed.

## Next

Make everyday experiments quicker, then build a useful text utility. This is the proposed order; each step needs working examples and tests.

- REPL: enter multi-line words, keep definitions between entries, inspect values and signatures, and recover from errors.
- Modules: explicit imports, public words, qualified names, and useful errors across files.
- Text utility: accept command-line arguments, stream a file, and close resources on success and failure.
- Measurements: record execution time and allocations for that utility, then review readability and choose the next application.

## Future

These are directions to explore. Their order and implementation remain open, and there are no delivery dates yet.

- Rich values: collections, records, dates, times, durations, and explicit error types with readable operations and formatting.
- Compiled execution: evaluate a native backend, including C as a possible target, and measure composition, text processing, and allocation behavior.
- WebAssembly code generation: compile Zhex programs to WASM, starting with browser execution and a small host interface for input and output.
- Application libraries: grow through small CLI, HTML-processing, web-server, game, and database experiments.
- Development tools: formatting, language-server support, and published editor packages as the syntax settles.
