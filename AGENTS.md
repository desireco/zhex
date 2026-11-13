# Zhex agent guide

Zhex is a Forth-inspired language with named values and checked composition. A Go interpreter executes a checked syntax tree; there is no native backend yet. Write small words, name important values, and let the checker catch mistakes before anything runs.

## Repository map

- `internal/lang` — the language implementation: `syntax.go`, `lexer.go`, `parser.go`, `check.go`, `eval.go`, `host.go`. Tests in that directory are the executable specification (see `docs/tests.md`).
- `cmd/zhex` — the CLI: `go run ./cmd/zhex run <file>`.
- `examples/*.zhex` — validated example corpus (hello, fizzbuzz, fibonacci, factorial, palindrome, composition). The website executes every example at build time; a broken example fails the build.
- `docs/` — the language contract (`initial-language.md`), getting-started guide, and research notes.
- `site/` + `tools/build-site` — website templates and the Go generator; `site/design/` ships the design system verbatim.
- `editors/` — syntax highlighting for Vim/Neovim, VS Code, and Zed.
- `ROADMAP.md` and the public project board (github.com/users/desireco/projects/8) track the same work; keep them in sync.

## Writing correct Zhex

- Blocks use four spaces per indentation level; tabs outside strings are rejected. `#` starts a comment. Statements end at a newline; parentheses do not continue a line.
- Bindings: `let name = expr` is immutable; `var name = expr` allows later `name = expr`. Types are `Int` (signed 64-bit), `Bool`, `String`, and `Unit`. There are no implicit conversions.
- Words: `word name(param: Type, ...) -> Type` followed by an indented body. The final statement supplies the result. Call with `name(args)` or through `flow`.
- Arithmetic: `+ - * / %` on integers; `+` also concatenates two strings. Division truncates toward zero; division or remainder by zero and integer overflow are language errors.
- Comparisons: `< <= > >=` require integers; `==` and `!=` compare matching `Int`, `Bool`, or `String` values. `and`/`or` short-circuit. Precedence, low to high: `or`, `and`, `== !=`, `< <= > >=`, `+ -`, `* / %`, unary `-`/`not`, calls, postfix `try`.
- Branching: `if`, optional `elif`, optional `else`. Conditions must be `Bool`; branch result types must match.
- Loops: `for n in range(start, end)` — exclusive end, empty when `start >= end`; `range` exists only in this loop syntax.
- Flows: `flow value` followed by one single-argument word per line; each word receives the previous result, and the last result is the flow's result.
- Input: `console.read-line(prompt)` and `console.read-int(prompt)` return results that can fail. Postfix `try` unwraps success and propagates failure — input failure exits nonzero with a diagnostic.
- Standard operations: `print`, `range`, `int.to-string`, `text.reverse`, `console.read-line`, `console.read-int`.
- Names: ASCII letters, digits, underscores, plus hyphenated segments (`is-palindrome`). Dots qualify operation names (`int.to-string`). Gotcha: `a-b` is a single name — write `a - b` for subtraction.

Canonical patterns live in `examples/` and on the website's docs page (desireco.github.io/zhex/docs.html).

## Verifying work

- `just test` must pass for any change — it covers the interpreter, examples, and website build; `go vet ./...` must be clean.
- Tests are the language definition: extend them test-first when changing behavior, and keep `ROADMAP.md`'s Done/Next/Future shape (the website generator validates it).
- Run a program with `go run ./cmd/zhex run examples/hello.zhex`; build the website with `just site`.
- Commits are focused and conventional (`feat:`, `fix:`, `docs:`, `chore:`), made on `main`.
