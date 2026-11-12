# Zhex

Zhex is an experimental **Forth-inspired programming language** built around a simple idea: small, readable operations should compose into useful programs.

We want code that people can understand without mentally tracking a stack. Named inputs, local values, familiar arithmetic, and clear control flow sit alongside explicit composition. The same clarity should make AI-generated code easier to review and correct.

The first interpreter is written in Go. **You can run the examples today**, including console input and output. Source files use `.zhex`.

## A small example

```text
word square(n: Int) -> Int
    let result = n * n
    result

let number = console.read-int("Number: ") try

flow number
    square
    print
```

Enter `7` and the program prints `49`. The checker infers the local types and verifies that the steps fit together. `try` propagates an input error instead of silently substituting a value.

## Try it

You need Go 1.23 or newer. There are no third-party Go dependencies.

```sh
go run ./cmd/zhex run examples/hello.zhex
go run ./cmd/zhex run examples/fizzbuzz.zhex
go run ./cmd/zhex run examples/fibonacci.zhex
go run ./cmd/zhex run examples/factorial.zhex
go run ./cmd/zhex run examples/palindrome.zhex
```

Each of these programs prompts for input. To build a local executable:

```sh
go build -o bin/zhex ./cmd/zhex
./bin/zhex run examples/fibonacci.zhex
```

Input can also come from a pipe. For example, supplying `10` to Fibonacci prints `55` after its prompt. [Simple composition](examples/composition.zhex) runs without input and prints `32`.

## What works now

- Typed word definitions, named parameters, inferred local types, and forward calls.
- Immutable `let` values and explicitly mutable `var` values.
- Integers, booleans, strings, arithmetic, comparisons, and short-circuit logic.
- `if` / `elif` / `else`, bounded integer loops, and checked `flow` composition.
- Prompted text and integer input, printing, integer formatting, and Unicode code-point reversal.
- Source-located syntax, type, arithmetic, and input/output errors.

The whole file is checked before it runs, including unused word bodies. Invalid programs do not prompt or consume input. Integer arithmetic checks for overflow.

## Where we are heading

The long-term goal is a readable, modular language for command-line tools, games, web applications and servers, HTML/text processing, and database software.

High performance is part of that goal. Named values and rich types should allow efficient representations, while small operations should be easy to optimize together. The current interpreter establishes behavior; native compilation and its performance measurements are future work.

Modules, rich domain types such as dates and durations, resource scopes, a REPL, and native compilation are still ahead. The language is experimental and its syntax can change. Zhex is inspired by Forth; it does not aim for Forth compatibility.

## Tests are language examples

Tests describe source programs, supplied input, expected output, and expected errors. They check the six example files as well as scope, type checking, evaluation order, overflow, input failures, and Unicode behavior.

```sh
go test ./...
go test -race ./...
go vet ./...
```

Start with the [test catalog](docs/tests.md), [example guide](docs/syntax-examples.md), or [initial language contract](docs/initial-language.md).

## Project documents

- [Language goals](docs/language-goals.md)
- [First milestone](docs/first-milestone.md)
- [Implementation plan](tasks/plan.md) and [task checklist](tasks/todo.md)
- [Forth implementation research](docs/forth-implementation-research.md), including Mirth, Factor, and 8th
- [Earlier design notes](docs/design-notes.md)

The original exploration was the Codex chat **Make Forth Great Again**. Its comparison documents are preserved in [docs/reference](docs/reference/).
