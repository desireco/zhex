# Initial Zhex language contract

This contract defines the first Go interpreter and the executable examples used to validate it. Zhex is a Forth-inspired language with named values and checked composition. The initial implementation executes a checked syntax tree; it does not generate native code.

## Source and blocks

Source is valid UTF-8. Blocks use four spaces per indentation level; tabs outside strings and comments are rejected. Blank lines and `#` comments are ignored. Statements end at a newline; parentheses do not introduce multiline continuation in this version. Syntax nesting and expression-tree depth are limited to 256 to report overly deep source as a language error.

Names use ASCII letters, digits, and underscores, starting with a letter or underscore. Hyphen-separated name segments beginning with a letter or underscore are allowed, as in `is-palindrome`. Dots qualify names: a dot names the type or module that owns the verb, and a leading underscore marks a word module-private (see Modules). Consequently `a-b` is a name, while `a - b` is subtraction: use spaces around subtraction between names. Keywords cannot be binding or word names; `use` is one of them. Strings use double quotes and Go-style escapes, and must decode to valid UTF-8.

## Values and expressions

The value types are `Int`, `Bool`, `String`, and `Unit`, plus the `Buffer` and `File` resource types described with the standard operations. `Int` is a signed 64-bit integer. Literals include decimal integers, `true`, `false`, and strings. Negative integer literals include the full signed range. Overflow is a language error; division and remainder by zero are errors. Division truncates toward zero; remainder has the dividend's sign. The minimum integer divided by `-1` overflows; its remainder modulo `-1` is zero.

Precedence, from lowest to highest: `or`, `and`, `== !=`, `< <= > >=`, `+ -`, `* / %`, unary `-`/`not`, calls and postfix `try`. Binary operators associate left. Operands and call arguments evaluate left to right; `and` and `or` short-circuit. Arithmetic requires integers, except `+` also concatenates two strings. Ordering requires integers. Equality compares matching `Int`, `Bool`, or `String` values. There are no implicit conversions.

`let name = expression` creates an immutable binding. `var name = expression` allows assignment with `name = expression`, preserving its inferred type. Bindings have lexical block scope; duplicates in one scope are errors. Inner scopes can shadow outer names. Parameters are immutable.

## Words and composition

`word name(parameter: Type, ...) -> Type` introduces a top-level definition followed by an indented body. Parameter and result types are explicit. Definitions may call other definitions, including forward references. The final statement supplies the word result; bindings, assignments, loops, and empty blocks have type `Unit`. Words do not capture top-level local bindings. Recursive calls have a runtime depth limit of 256.

Call a word with `name(arguments)`. A `flow expression` block contains one word name per line; each word receives the previous value as its single argument. Its final result is the flow's result. Calls are resolved and their input/output types checked before execution. Named calls and flow steps use the same operations.

## Modules

A program splits across files. `use math` at the top level of a file imports the module named `math` by loading the sibling `math.zhex` next to the importing file — each file's own imports resolve beside it. A module that cannot be loaded is a `use-missing` error naming the searched path. `use` is a keyword and is allowed only at a file's top level (`use-location`); a repeated `use` of the same module is a no-op.

An imported word is called qualified, as in `math.gcd(a, b)` or a flow step. Inside its own file a module's words call each other bare, and they cannot see the importing file's words. A word whose name starts with an underscore is private to its module: callable only inside its own file, never suggested outside it, and reached from another file only as a `private-use` error. A module's own imports stay private to it — naming a module directly is the only way to see its words — and two modules never collide, even when their words share names. Duplicate word names are detected per file, and an entry file cannot redefine an adopted qualified name.

Module names `text`, `int`, `buffer`, `file`, and `console` are reserved (`use-reserved`); those prefixes belong to the standard operations. An imported file declares words only — top-level statements are a `module-body` error — and import cycles are `use-cycle` errors spelling the files in the cycle. Every loaded module checks before the file that imports it, so dependencies report first, and module errors annotate the module's own source at check time and at runtime.

The REPL resolves `use` against the working directory and adopts each named module for the rest of the session; its qualified words then persist across entries and list under `words`. The two-file [modules example](../examples/modules/main.zhex) shows the shape: `main.zhex` runs the program and `math.zhex` beside it supplies the words, one of them underscore-private.

## Decisions and iteration

`if condition`, optional `elif condition` blocks, and optional `else` select a branch. Conditions must be `Bool`. Branch result types must match; a missing `else` supplies `Unit`. Each branch has its own lexical scope.

`for name in range(start, end)` iterates upward over integers with an exclusive end, without building a list. Bounds evaluate once. If `start >= end`, the loop is empty. The iteration binding is immutable and each iteration has its own block scope. The loop result is `Unit`.

`for line in file.lines(path)` iterates over the lines of a UTF-8 text file without holding the whole file. The path is a `String` evaluated once; each line is chomped like `read-line` (one trailing LF and an immediately preceding CR removed) and bound as an immutable `String` with its own block scope. An empty file runs no iterations; a final line without its newline still runs. The file is closed when the loop finishes and on every exit, including a failure in the loop body. A path that cannot be opened is a `file-open` error; content that is not valid UTF-8 is a `file-utf8` error. `range` and `file.lines` exist only in these loop syntaxes.

## Standard operations and console input

The initial predeclared operations are `print`, `int.to-string`, `text.reverse`, `read-line`, and `text.to-int`. `print` accepts one `Int`, `Bool`, or `String`, writes it followed by a newline, and returns `Unit`. Boolean text is `true` or `false`. `int.to-string` takes an integer. `text.reverse` takes a string and reverses Unicode code points without normalization. The naming rule is: a dot names the type or module that owns the verb — nothing else is dotted; the core operations are bare words.

The `text` words operate on `String` values, Unicode-safe by code point. `text.length` counts code points. `text.line-count` counts lines — empty text is `0`, and a trailing newline does not add a line. `text.word-count` splits on Unicode whitespace. `text.upper` and `text.lower` map case; `text.trim` removes surrounding Unicode whitespace. `text.contains(text, part)` reports whether `part` occurs. `text.index-of(text, part)` returns the code-point position of the first occurrence, or `-1` when absent. `text.slice(text, start, end)` returns the code points from `start` up to `end`; bounds below zero and past the length clamp, and `start >= end` yields the empty string. `text.replace(text, old, new)` rewrites every occurrence of `old` with `new` — literally, not as a pattern; empty `old` is a `replace-empty` error, a bug signal rather than a recoverable case.

`Buffer` is a growable list of `String` lines for building text incrementally. `buffer.new()` creates an empty buffer; `buffer.add-line(b, line)` appends a line and returns `Unit`; `buffer.line-count(b)` returns the number of lines; `buffer.line(b, i)` returns `Result<String, BufferError>`, failing `buffer-range` on an index outside the lines; `buffer.set-line(b, i, line)` returns `Result<Unit, BufferError>` under the same rule; `buffer.to-text(b)` joins the lines with newlines. `let` binds the name, not a copy, so a second name observes the same buffer. Buffers are excluded from `==` and `!=`, and `print` rejects a `Buffer` with a hint to convert through `buffer.to-text`.

Files read through two layers. The whole-file layer returns everything at once: `file.read-all(path)` yields `Result<String, FileError>`, and `file.read-lines(path)` yields `Result<Buffer, FileError>` with every line chomped. The streaming layer holds an open handle: `file.open(path)` yields `Result<File, FileError>`; `file.read-line(f)` yields the next chomped line as `Result<String, FileError>`, and reaching the end of file fails inside the `Result`; `file.close(f)` releases the handle, returns `Unit`, and is safe to call twice, while reading from a closed file fails. The `for line in file.lines(path)` loop covers the streaming case with the close guaranteed on every exit. Files are read as UTF-8 and validated at the boundary; a path that cannot be opened fails `file-open`, naming the path and the operating system's reason. A `File` is excluded from `==` and `!=`, and `print` rejects it.

`read-line(prompt)` takes a string prompt and returns `Result<String, InputError>` from one line of standard input. `text.to-int(text)` is the inverse of `int.to-string`: it parses a signed 64-bit decimal, trimming surrounding whitespace, and returns `Result<Int, ParseError>`. Program arguments arrive the same way: `zhex run <file> [args...]` forwards non-option arguments after the source file to the program; `--trace` and `--report=text|json` remain interpreter options wherever they occur, and other `--` options are rejected, `console.args-count()` returns how many there are, and `console.arg(i)` returns `Result<String, InputError>`, failing `argument-missing` on an index outside the arguments. Postfix `try` unwraps success and propagates failure. Results cannot be printed, compared, or used as plain values. General user-defined result types and recovery are deferred.

The [example guide](syntax-examples.md) defines line endings, integer parsing, EOF, invalid UTF-8, prompts, and supported input domains. Prompts are written and flushed before reading. Redirected input is supported. Runtime input/output errors include source locations and exit nonzero; previous output remains visible. The interpreter checks the whole program, including unused word bodies, before any input or output occurs.

## Commands and validation

Run `go run ./cmd/zhex run examples/fibonacci.zhex`, or build with `go build -o bin/zhex ./cmd/zhex` and run `./bin/zhex run examples/fibonacci.zhex`. The runner requires a source file; `zhex repl [--trace]` instead starts an interactive session whose words and values persist across entries — a blank line ends a multi-line entry, bare expressions echo their value, errors print a diagnostic without ending the session, and `words` lists signatures.

Run `go test ./...` for the executable language cases and CLI tests. Run `go vet ./...` for Go checks. The [test catalog](tests.md) maps syntax and behavior to those tests. Collections, rich domain types, resource scopes, and native compilation remain later work.
