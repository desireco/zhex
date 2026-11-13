# Zhex agent guide

Zhex is a Forth-inspired language with named values and checked composition. A Go interpreter executes a checked syntax tree; there is no native backend yet. Write small words, name important values, and let the checker catch mistakes before anything runs.

## Repository map

- `internal/lang` — the language implementation: `syntax.go`, `lexer.go`, `parser.go`, `check.go`, `eval.go`, `host.go`, `session.go`. Tests in that directory are the executable specification (see `docs/tests.md`).
- `cmd/zhex` — the CLI: `go run ./cmd/zhex run <file>`.
- `examples/*.zhex` — validated example corpus (hello, fizzbuzz, fibonacci, gcd, times-table, factorial, palindrome, composition). The website executes every example at build time; a broken example fails the build.
- `docs/` — the language contract (`initial-language.md`), getting-started guide, and research notes.
- `site/` + `tools/build-site` — website templates and the Go generator; `site/design/` ships the design system verbatim.
- `editors/` — syntax highlighting for Vim/Neovim, VS Code, and Zed.
- `ROADMAP.md` and the public project board (github.com/users/desireco/projects/8) track the same work; keep them in sync.

## Writing correct Zhex

- Blocks use four spaces per indentation level; tabs outside strings are rejected. `#` starts a comment. Statements end at a newline; parentheses do not continue a line.
- Bindings: `let name = expr` is immutable; `var name = expr` allows later `name = expr`. Types are `Int` (signed 64-bit), `Bool`, `String`, `Unit`, plus the `Buffer` and `File` resource types. There are no implicit conversions.
- Words: `word name(param: Type, ...) -> Type` followed by an indented body. The final statement supplies the result. Call with `name(args)` or through `flow`. Words may be recursive (runtime depth limit 256).
- Arithmetic: `+ - * / %` on integers; `+` also concatenates two strings. Division truncates toward zero; division or remainder by zero and integer overflow are language errors.
- Comparisons: `< <= > >=` require integers; `==` and `!=` compare matching `Int`, `Bool`, or `String` values. `and`/`or` short-circuit. Precedence, low to high: `or`, `and`, `== !=`, `< <= > >=`, `+ -`, `* / %`, unary `-`/`not`, calls, postfix `try`.
- Branching: `if`, optional `elif`, optional `else`. Conditions must be `Bool`; branch result types must match.
- Loops: `for n in range(start, end)` — exclusive end, empty when `start >= end`; `for line in file.lines(path)` streams a file's chomped lines and always closes the file. Both exist only in loop syntax.
- Flows: `flow <expression>` evaluates the expression, then passes its value through one single-argument word per line; each word receives the previous result, and the last result is the flow's result. The seed can be any expression — `flow gcd(a, b)` starts from a computed value.
- Input: `read-line(prompt)` returns `Result<String, InputError>` and `text.to-int(text)` returns `Result<Int, ParseError>`. Postfix `try` unwraps success and propagates failure — input failure exits nonzero with a diagnostic. Program arguments come from `console.arg(i)` (a `Result`) and `console.args-count()`, forwarded by `zhex run <file> [args...]`.
- Standard operations: `print`, `range`, `int.to-string`, `read-line`; `text.` words — `to-int`, `reverse`, `length`, `line-count`, `word-count`, `upper`, `lower`, `trim`, `contains`, `index-of`, `slice`, `replace`; `buffer.` words — `new`, `add-line`, `line-count`, `line`, `set-line`, `to-text`; `file.` words — `read-all`, `read-lines`, `open`, `read-line`, `close`, `lines` (loops only); `console.arg`, `console.args-count`.
- Names: ASCII letters, digits, underscores, plus hyphenated segments (`is-palindrome`). Dots qualify operation names (`int.to-string`); a dot says which type owns the verb — nothing else is dotted. Gotcha: `a-b` is a single name — write `a - b` for subtraction.
- Buffers and files: `let` binds a `Buffer` or `File` name, not a copy — two names observe one buffer. Both are excluded from `==`/`!=` and rejected by `print` (turn a buffer into text with `buffer.to-text`); the buffer, file, and argument words that can fail return `Result`s unwrapped with `try`.

REPL sessions: `zhex repl [--trace]` keeps words and values across entries. A blank line ends a multi-line entry — only a missing indented block continues, so statements never span lines. Redefining a word replaces it and prints `redefined <name>`; `let` shadows silently. `words` lists signatures, and bare expressions echo their value (`ok("...")`/`err("...")` for Results). Errors print the annotated diagnostic and the session survives; a runtime error keeps the entry's earlier effects. Ctrl-D exits 0, a final line without its newline still runs, and the `words` command shadows a user binding of that name.

Canonical patterns live in `examples/` — `gcd` shows recursion and a flow seeded with a computed call, `times-table` shows bounded loops and string building — and on the website's docs page (desireco.github.io/zhex/docs.html).

## Reading checker errors

The checker validates the whole program before anything runs and reports `file:line:column` diagnostics, so nothing partial executes. Each diagnostic prints the source line with a caret — the caret underlines the whole failing expression, not one column — and, where a known fix exists, a `hint:` line; the first line stays stable for scripts and agents. Type mismatches narrate where a value came from, e.g. `...found Result<Int, ParseError>; m comes from text.to-int`. The mistakes the compiler now hints itself:

- `unknown name "a-b"` — hyphenated names are single names; write `a - b` with spaces for subtraction.
- `... does not support type Result<String, InputError>` (or any `Result` in a plain value position) — a read result needs postfix `try` to unwrap it (`let line = read-line("N: ") try`).
- `print does not support type Buffer` — a buffer is lines, not text; convert it first (`print(buffer.to-text(b))`).

Typos get did-you-mean suggestions drawn from the visible names and words. Every diagnostic carries a stable code, cataloged on the site's errors page; `zhex run <file> --report=json` emits the machine-readable report `{code, severity, file, line, column, message, hint}` for agents and tools, plus optional additive `span_start_column`/`span_end_column` — present only for checker diagnostics with a span, never for runtime ones. The diagnostic contract and phased plan (spans, provenance, tracing, replay) live in `docs/diagnostics-plan.md`, which supersedes the channel sketches in `docs/error-reporting-research.md` and older tickets.

`zhex run <file> --trace` watches a run instead of its failures: it streams JSONL step events to stderr — word calls with rendered arguments, binding writes with values, flow steps, and word returns — before any diagnostic, and combines with `--report=json` (events, then the report). Agents: use it to see what a program did; the event vocabulary is documented in the site's llms.txt, and a binding holding a `Result` renders `ok("inner")` or `err("message")`. Tracing never fails a run, and output without the flag is unchanged.

To validate a program quickly, run `go run ./cmd/zhex run <file>` — checking happens before execution, so a clean run means types and names resolved.

## Ticket workflow

File every new ticket into the board's **Inbox** bucket with no status — never straight into Todo; triage promotes Inbox cards. Work on tracked tickets (issues on the project board) in this order:

1. **Analyze first.** Read the ticket and study the relevant code or docs. If the ticket is unclear or incomplete, improve it: update its title and description so it states the goal, scope, and acceptance criteria accurately.
2. **Claim before working.** Post your plan as a comment, then move the card to In Progress. A ticket must be In Progress while anyone is working on it, so nobody else starts the same work; if nobody has claimed it, claim it before you start.
3. **Follow the plan** until every part of it is done; keep scope to what the ticket describes.
4. **Return unfinished tickets.** If you cannot complete the ticket, stop, leave a comment with what is done and what remains, and move the card back to Todo. Never leave a ticket In Progress unattended.
5. **Verify and record.** Make the functionality work (tests, build, deploy checks), then update everything that records truth wherever it lives: website docs, repository docs, AGENTS.md, ROADMAP.md, and the board.
6. **Close last.** Commit and push, let the deploy go green, then move the card to Done and close the issue with a short ship note.

## Verifying work

- `just test` must pass for any change — it covers the interpreter, examples, and website build; `go vet ./...` must be clean.
- Tests are the language definition: extend them test-first when changing behavior, and keep `ROADMAP.md`'s Done/Next/Future shape (the website generator validates it).
- Run a program with `go run ./cmd/zhex run examples/hello.zhex`; build the website with `just site`. User-facing surfaces (website pages, llms.txt) show the canonical `zhex run examples/<file>.zhex` form instead — the interpreter binary is built once with `just build`, and the Go toolchain is an implementation detail, not part of the language's public story.
- Commits are focused and conventional (`feat:`, `fix:`, `docs:`, `chore:`), made on `main`.
