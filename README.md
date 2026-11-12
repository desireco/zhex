# Zhex

**Small words. Readable programs.**

Zhex is an experimental **Forth-inspired language** for composing small operations into larger programs. It keeps the idea of passing values through a sequence of words, while adding named inputs, local values, familiar expressions, and clear control flow.

The first interpreter is written in Go. It runs real `.zhex` files with console input and output. The language is still taking shape; syntax and APIs may change.

[Website and examples](https://desireco.github.io/zhex/) · [Getting started](docs/getting-started.md) · [Language reference](docs/initial-language.md)

## What it looks like

```text
word square(n: Int) -> Int
    let result = n * n
    result

let number = console.read-int("Number: ") try

flow number
    square
    print
```

Enter `7` and it prints `49`. A word is a function. A `flow` passes each result to the next word. The checker infers local types and verifies that the operations fit together before the program runs.

## Try it

Install Go 1.23 or newer, then clone the repository. The source repository is currently private, so cloning requires access:

```sh
git clone https://github.com/desireco/zhex.git
cd zhex
go run ./cmd/zhex run examples/hello.zhex
```

The program asks for your name and prints a greeting. No third-party Go dependencies are required.

For more examples:

```sh
go run ./cmd/zhex run examples/fizzbuzz.zhex
go run ./cmd/zhex run examples/fibonacci.zhex
go run ./cmd/zhex run examples/factorial.zhex
go run ./cmd/zhex run examples/palindrome.zhex
```

Build an executable when you want to use it directly:

```sh
go build -o bin/zhex ./cmd/zhex
./bin/zhex run examples/fibonacci.zhex
```

[Start with the short guide](docs/getting-started.md) for input, output, and the main language constructs.

## Learn through examples

| Example | What it demonstrates |
| --- | --- |
| [Composition](examples/composition.zhex) | Typed words, inferred locals, and a flow that prints `32`. |
| [Greeting](examples/hello.zhex) | Read a line, build a string, and print it. |
| [FizzBuzz](examples/fizzbuzz.zhex) | Decisions, remainders, and a bounded loop. |
| [Fibonacci](examples/fibonacci.zhex) | Explicitly mutable locals and iterative calculation. |
| [Factorial](examples/factorial.zhex) | Accumulation and integer boundaries. |
| [Palindrome](examples/palindrome.zhex) | Text input, comparison, and Unicode code-point reversal. |

The [example guide](docs/syntax-examples.md) records expected output and edge cases. The [language contract](docs/initial-language.md) defines the current rules.

## What works today

The interpreter supports typed word definitions, named parameters, inferred local types, immutable `let` bindings, mutable `var` bindings, integers, booleans, strings, conditions, bounded loops, and checked flows. Console reads, printing, integer formatting, and text reversal are built in.

The whole file is checked before execution, including unused word bodies. Static errors do not prompt or consume input. Runtime errors report a source location; integer overflow, division by zero, invalid input, and end-of-input are handled explicitly.

## Where we are heading

We want a readable, modular language for command-line tools, games, web applications and servers, HTML/text processing, and database software. High performance is a goal to establish through implementation and measurement.

The current interpreter defines and tests behavior. A REPL, user-written modules, rich domain types, resource scopes, and native compilation are future work. Zhex is inspired by Forth; it does not aim for Forth compatibility. Mirth, Factor, and 8th also inform the [design research](docs/forth-implementation-research.md).

## Tests are part of the language

Tests describe source programs, supplied input, expected output, and expected errors. They validate the examples as well as scope, evaluation order, type checking, arithmetic boundaries, Unicode, and input/output failures.

```sh
go test ./...
go test -race ./...
go vet ./...
```

See the [test catalog](docs/tests.md) to find the executable specification for each feature.

## Project documents

- [Getting started](docs/getting-started.md)
- [Language goals](docs/language-goals.md) and [first milestone](docs/first-milestone.md)
- [Implementation plan](tasks/plan.md) and [task checklist](tasks/todo.md)
- [Earlier design notes](docs/design-notes.md) and [reference material](docs/reference/)
