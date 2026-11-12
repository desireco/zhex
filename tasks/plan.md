# Initial Go interpreter plan

## Purpose

Build a small executable model of Zhex that makes syntax and semantics easy to try while establishing a reusable frontend for a larger language. The first useful application is a text-processing CLI. This plan implements the direction in [language goals](../docs/language-goals.md); tasks 1–5 are implemented and tested; tasks 6–8 remain pending.

The [first milestone](../docs/first-milestone.md) completes tasks 1–5: run typed composition and the [interactive example corpus](../docs/syntax-examples.md), including FizzBuzz, Fibonacci, factorial, palindrome checking, and a greeting. Console input is part of this milestone. A REPL, user-written modules, and the text CLI follow after that foundation works.

## Initial implementation decisions

**Chosen:** Go is the first implementation language. Readability, small composable words, explicit modules, and a path to performant applications are language goals.

**Implemented in the first interpreter:** indentation-based blocks, named word parameters, lexical locals, familiar arithmetic/comparisons, and explicit `flow` blocks. Start with `Int`, `Bool`, `String`, and `Unit`, plus the built-in input result type needed for `try`. Support explicit mutable `var` locals for iterative algorithms, and infer local types. Add collections as the later text utility needs them. Keep one return value per word initially. Go's garbage collector manages interpreter storage; external resources still have explicit language-level cleanup.

The [language contract](../docs/initial-language.md) specifies signed 64-bit integers, checked overflow, and errors for division by zero; integer division truncates toward zero. Tests verify these rules independently of Go's default behavior. Floating-point arithmetic can follow when needed by a concrete example.

A supported example is:

```text
word square(n: Int) -> Int
    n * n

word describe(n: Int) -> String
    if n > 10
        "large"
    else
        "small"

flow 4
    square
    describe
    print
```

Expected output is `large` followed by a newline. This syntax is implemented and defined by the initial language contract. Initially a flow supplies one value to each one-argument word; its final step may return `Unit`. Broader stack effects, quotations, and higher-order composition are later extensions.

## Execution architecture

```text
source → tokenizer/parser → syntax tree with source locations
       → name and type checking → checked program → evaluator
                                                    ↓
                                              host operations
```

The checker resolves calls and verifies word signatures, expressions, branches, and flow compatibility before a file starts executing. A checked program can initially be a resolved syntax tree with type information; avoid building both a bytecode VM and a tree evaluator. A later compiler can lower this representation into a suitable intermediate form and generate C without reusing evaluator internals.

The evaluator owns explicit Zhex values, lexical call frames, and flow values. Keep Zhex calls in an execution context so error traces and execution limits can eventually be supported. A flow may carry its value through an explicit stack internally, but user-visible stack operations are not required for the first syntax.

Host operations expose signatures and receive a context with input, output, and resource access. The initial host operations include prompted line/integer input, printing, integer formatting, and string reversal. Specify their contracts and input failures in the example corpus. Share buffered input between any future REPL and program reads. Keep filesystem and terminal details out of parsing and type checking. Begin with a concrete implementation and small boundaries; do not design a general plugin framework.

## Repository structure

```text
cmd/zhex/            CLI entry point and command tests
internal/lang/       Separate syntax, lexer, parser, checker, evaluator, and host files
                     Program-level tests live beside the implementation
examples/            Small runnable Zhex programs
docs/                Goals, language contract, examples, design rationale
tasks/               This plan and its implementation checklist
```

Introduce directories as needed by each slice. Use standard Go formatting and tests. Start with the standard library; assess a dependency only when a specific feature requires it. Available commands are `go test ./...`, `go build ./cmd/zhex`, `go run ./cmd/zhex run examples/composition.zhex`. The REPL command remains planned.

## Ordered implementation slices

| Task | Working outcome | Depends on |
| --- | --- | --- |
| 1 | A compact language contract and representative syntax examples | — |
| 2 | Run checked arithmetic and print a result from a source file | 1 |
| 3 | Define and call named words, bind locals, and execute checked flows | 2 |
| 4 | Run algorithm bodies with decisions, bounded loops, mutable locals, and minimal text operations | 3 |
| 5 | Accept console input and run the complete initial example corpus | 4 |
| 6 | Experiment in a REPL using the same checker and evaluator | 5 |
| 7 | Import a small module through an explicit public interface | 4 |
| 8 | Run a useful streaming text CLI with scoped file cleanup and errors | 6, 7 |

Acceptance criteria, focused verification, and likely files are tracked in [todo.md](todo.md). At the checkpoints after tasks 3, 5, and 7, review the examples for readability and revise the language contract before expanding the implementation.

## Validation

Each slice adds meaningful program-level cases: valid output, invalid names/types, source positions, and the behavior newly introduced by that slice. Test the same language behavior through files and the REPL. Avoid tests that merely duplicate a parser's internal node structure.

Use the input/output and boundary matrix in the example corpus as the first executable acceptance suite. Include Fibonacci at index 92, factorial at 20, exact palindrome comparisons, malformed input, EOF, and redirected input. Capture prompts separately from terminal input echo when checking standard output.

The first practical CLI should process an input larger than its working buffer without reading the whole file, produce a checked result, and return a useful error and nonzero exit status for an unreadable file. Test cleanup on completion and failure. Record elapsed time and allocations on a declared workload to establish a baseline, without presenting interpreter measurements as native-language performance.

## Extension sequence

After the initial CLI works, choose the next slice using an actual application:

1. Structured values and a SQLite module exercise records, result handling, transactions, and resource scopes.
2. HTML parsing/transformation or a small HTTP endpoint exercises text representation, host bindings, and errors. Choose a real HTML parser; token scanning alone is not a general HTML parser.
3. A small 2D game exercises floating-point math, mutable state, host callbacks, and allocation behavior.
4. Native compilation through C tests whether the same checked programs can run efficiently with an explicit runtime. Its order may move earlier if interpreter overhead blocks useful experiments.

Browser execution, actors, advanced stack polymorphism, copy-on-write arrays, and database-engine infrastructure each need their own concrete requirements. Keep their semantics open rather than approximating them with Go behavior.

## Risks and design checks

| Risk | Response |
| --- | --- |
| Syntax expands before it proves readable | Maintain a small example corpus and one spelling per initial construct. |
| Host-language behavior leaks into Zhex | Specify numbers, evaluation order, types, errors, and resource ownership; verify those rules explicitly. |
| Runtime and parser become inseparable | Keep a checked-program boundary with no host I/O during parsing/checking. |
| A future backend demands a rewrite | Preserve source locations and resolved calls; share semantic examples across execution paths. |
| Broad application goals overwhelm the first version | Finish the text CLI before adding a framework or new runtime subsystem. |

## Open design questions

- Exact word-call, local-binding, module, and cleanup syntax; how much inference to allow at public interfaces.
- String versus byte-buffer behavior and iteration, including Unicode handling.
- Error values and propagation syntax, and whether early versions allow recursion and impose execution limits.
- How general stack composition and quotations should interact with named expressions.
- Long-term data memory policy, concurrency model, and native/browser targets.

Task 1 resolves only the subset needed for the initial slices. Keep later decisions explicit and open.
