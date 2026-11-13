# Initial Zhex language contract

This contract defines the first Go interpreter and the executable examples used to validate it. Zhex is a Forth-inspired language with named values and checked composition. The initial implementation executes a checked syntax tree; it does not generate native code.

## Source and blocks

Source is valid UTF-8. Blocks use four spaces per indentation level; tabs outside strings and comments are rejected. Blank lines and `#` comments are ignored. Statements end at a newline; parentheses do not introduce multiline continuation in this version. Syntax nesting and expression-tree depth are limited to 256 to report overly deep source as a language error.

Names use ASCII letters, digits, and underscores, starting with a letter or underscore. Hyphen-separated name segments beginning with a letter or underscore are allowed, as in `is-palindrome`. Dots qualify operation names. Consequently `a-b` is a name, while `a - b` is subtraction: use spaces around subtraction between names. Keywords cannot be binding or word names. Strings use double quotes and Go-style escapes, and must decode to valid UTF-8.

## Values and expressions

The value types are `Int`, `Bool`, `String`, and `Unit`. `Int` is a signed 64-bit integer. Literals include decimal integers, `true`, `false`, and strings. Negative integer literals include the full signed range. Overflow is a language error; division and remainder by zero are errors. Division truncates toward zero; remainder has the dividend's sign. The minimum integer divided by `-1` overflows; its remainder modulo `-1` is zero.

Precedence, from lowest to highest: `or`, `and`, `== !=`, `< <= > >=`, `+ -`, `* / %`, unary `-`/`not`, calls and postfix `try`. Binary operators associate left. Operands and call arguments evaluate left to right; `and` and `or` short-circuit. Arithmetic requires integers, except `+` also concatenates two strings. Ordering requires integers. Equality compares matching `Int`, `Bool`, or `String` values. There are no implicit conversions.

`let name = expression` creates an immutable binding. `var name = expression` allows assignment with `name = expression`, preserving its inferred type. Bindings have lexical block scope; duplicates in one scope are errors. Inner scopes can shadow outer names. Parameters are immutable.

## Words and composition

`word name(parameter: Type, ...) -> Type` introduces a top-level definition followed by an indented body. Parameter and result types are explicit. Definitions may call other definitions, including forward references. The final statement supplies the word result; bindings, assignments, loops, and empty blocks have type `Unit`. Words do not capture top-level local bindings. Recursive calls have a runtime depth limit of 256.

Call a word with `name(arguments)`. A `flow expression` block contains one word name per line; each word receives the previous value as its single argument. Its final result is the flow's result. Calls are resolved and their input/output types checked before execution. Named calls and flow steps use the same operations.

## Decisions and iteration

`if condition`, optional `elif condition` blocks, and optional `else` select a branch. Conditions must be `Bool`. Branch result types must match; a missing `else` supplies `Unit`. Each branch has its own lexical scope.

`for name in range(start, end)` iterates upward over integers with an exclusive end, without building a list. Bounds evaluate once. If `start >= end`, the loop is empty. The iteration binding is immutable and each iteration has its own block scope. The loop result is `Unit`. `range` is only available in this loop syntax initially.

## Standard operations and console input

The initial predeclared operations are `print`, `int.to-string`, `text.reverse`, `read-line`, and `text.to-int`. `print` accepts one `Int`, `Bool`, or `String`, writes it followed by a newline, and returns `Unit`. Boolean text is `true` or `false`. `int.to-string` takes an integer. `text.reverse` takes a string and reverses Unicode code points without normalization. The naming rule is: a dot says which type owns the verb — nothing else is dotted; the core operations are bare words.

`read-line(prompt)` takes a string prompt and returns `Result<String, InputError>` from one line of standard input. `text.to-int(text)` is the inverse of `int.to-string`: it parses a signed 64-bit decimal, trimming surrounding whitespace, and returns `Result<Int, ParseError>`. Postfix `try` unwraps success and propagates failure. Results cannot be printed, compared, or used as plain values. General user-defined result types and recovery are deferred.

The [example guide](syntax-examples.md) defines line endings, integer parsing, EOF, invalid UTF-8, prompts, and supported input domains. Prompts are written and flushed before reading. Redirected input is supported. Runtime input/output errors include source locations and exit nonzero; previous output remains visible. The interpreter checks the whole program, including unused word bodies, before any input or output occurs.

## Commands and validation

Run `go run ./cmd/zhex run examples/fibonacci.zhex`, or build with `go build -o bin/zhex ./cmd/zhex` and run `./bin/zhex run examples/fibonacci.zhex`. The runner requires a source file; `repl` is later work.

Run `go test ./...` for the executable language cases and CLI tests. Run `go vet ./...` for Go checks. The [test catalog](tests.md) maps syntax and behavior to those tests. Modules, collections, rich domain types, resource scopes, and native compilation remain later work.
