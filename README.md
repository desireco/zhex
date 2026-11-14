# Zhex

**Small words. Readable programs.**

Zhex is an experimental **Forth-inspired language** for composing small operations into larger programs. It keeps the idea of passing values through a sequence of words, while adding named inputs, local values, familiar expressions, and clear control flow.

The first interpreter is written in Go. It runs real `.zhex` files with console input and output. The language is still taking shape; syntax and APIs may change.

[Website and examples](https://desireco.github.io/zhex/) · [Manifesto](docs/manifesto.md) · [Getting started](docs/getting-started.md) · [Language reference](docs/initial-language.md) · [Roadmap](ROADMAP.md) · [Project board](https://github.com/users/desireco/projects/8)

## What it looks like

```text
word square(n: Int) -> Int
    let result = n * n
    result

let line = read-line("Number: ") try
let number = text.to-int(line) try

flow number
    square
    print
```

Enter `7` and it prints `49`. A word is a function. A `flow` passes each result to the next word. The checker infers local types and verifies that the operations fit together before the program runs.

## Try it

Install Go 1.23 or newer, then clone the public repository:

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
go run ./cmd/zhex run examples/gcd.zhex
go run ./cmd/zhex run examples/times-table.zhex
go run ./cmd/zhex run examples/text-stats.zhex examples/text-stats-sample.txt
```

Build an executable when you want to use it directly:

```sh
go build -o bin/zhex ./cmd/zhex
./bin/zhex run examples/fibonacci.zhex
```

[Start with the short guide](docs/getting-started.md) for input, output, and the main language constructs.

[Editor support](docs/editors.md) provides syntax highlighting for Vim/Neovim, VS Code, and Zed.

## Learn through examples

| Example | What it demonstrates |
| --- | --- |
| [Composition](examples/composition.zhex) | Typed words, inferred locals, and a flow that prints `32`. |
| [Records](examples/records.zhex) | A nominal named type, positional constructor, and immutable fields. |
| [Lists and arrays](examples/collections.zhex) | Shared list storage, checked indexing, dual-binding iteration, row-major arrays, and overlap-preserving resize. |
| [Greeting](examples/hello.zhex) | Read a line, build a string, and print it. |
| [FizzBuzz](examples/fizzbuzz.zhex) | Decisions, remainders, and a bounded loop. |
| [Fibonacci](examples/fibonacci.zhex) | Explicitly mutable locals and iterative calculation. |
| [Factorial](examples/factorial.zhex) | Accumulation and integer boundaries. |
| [Palindrome](examples/palindrome.zhex) | Text input, comparison, and Unicode code-point reversal. |
| [GCD](examples/gcd.zhex) | Euclid's algorithm by recursion, seeded into a flow. |
| [Times table](examples/times-table.zhex) | A bounded loop building strings from integers. |
| [Text stats](examples/text-stats.zhex) | Program arguments, whole-file counts, and streamed uppercased lines. |
| [Error recovery](examples/recovery.zhex) | Catch invalid numeric text, show a short explanation, and continue with a fallback value. |
| [Exit status](examples/exit-status.zhex) | Validate a requested status and end deliberately with `console.exit`. |
| [Math modules](examples/modules/main.zhex) | A two-file program: `use math` imports a sibling library, calls its words qualified, and keeps one helper underscore-private. |

The [example guide](docs/syntax-examples.md) records expected output and edge cases. The [language contract](docs/initial-language.md) defines the current rules.

## What works today

The interpreter supports typed word definitions, named parameters, inferred or explicitly annotated local types, immutable `let` bindings, mutable `var` bindings, nominal records, shared mutable lists and arrays, integers, booleans, strings, conditions, bounded loops, and checked flows. Console reads, program arguments, printing, integer formatting, a dozen text words, line buffers, and streamed file reads are built in. Programs split across files: `use math` imports the sibling `math.zhex`, its words and types are callable qualified (`math.gcd`, `model.Customer`), and a leading underscore keeps a word or type private to its file. An interactive REPL, `zhex repl`, checks and runs each entry as you type it, keeping words, values, types, and adopted modules across the session.

The whole file is checked before execution, including unused word bodies. Static errors do not prompt or consume input. Runtime errors report a source location; integer overflow, division by zero, invalid input, and end-of-input are handled explicitly.

## Where we are heading

We want a readable, modular language for command-line tools, games, web applications and servers, HTML/text processing, and database software. High performance is a goal to establish through implementation and measurement.

The current interpreter defines and tests behavior. Additional named-type forms such as choices and aliases, maps, temporal values, resource scopes, and native compilation are future work. Zhex is inspired by Forth; it does not aim for Forth compatibility. Mirth, Factor, and 8th also inform the [design research](docs/forth-implementation-research.md). Further [language presentation research](docs/language-presentation-research.md) explores how to explain the language through readable examples.

## Tests are part of the language

Tests describe source programs, supplied input, expected output, and expected errors. They validate the examples as well as scope, evaluation order, type checking, arithmetic boundaries, Unicode, and input/output failures.

```sh
go test ./...
go test -race ./...
go vet ./...
```

See the [test catalog](docs/tests.md) to find the executable specification for each feature.

## Development commands

With [just](https://just.systems/man/en/) installed, run `just` to list the project's commands:

```sh
just test
just check
just build
just run fibonacci
just serve
```

`just run` starts the greeting example and keeps console input available. Install site dependencies with `npm ci --prefix site` (Node 22.23.4 or newer (or Node 24)). `just site` builds the website, and `just serve` rebuilds it and serves it at [127.0.0.1:8000/zhex/](http://127.0.0.1:8000/zhex/). Pass a different port with `just serve 8080`. `just test-race` runs the race checks. The underlying Go commands remain available directly.

## Project documents

- [Manifesto](docs/manifesto.md): the intentions behind the language
- [Getting started](docs/getting-started.md)
- [Agent guide](AGENTS.md): writing correct Zhex and verifying changes with the test suite
- [Language goals](docs/language-goals.md) and [first milestone](docs/first-milestone.md)
- [Earlier design notes](docs/design-notes.md) and [reference material](docs/reference/)

## License

MIT — see [LICENSE](LICENSE). Copyright © 2026 Dakic OnLine LLC (Zeljko Dakic).
