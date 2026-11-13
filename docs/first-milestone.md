# First milestone Readable checked composition

## Goal and status

Build a small Go interpreter that runs readable Zhex programs with typed words, inferred locals, composition, decisions, loops, and console input/output. The first examples are a greeting, FizzBuzz, Fibonacci, factorial, and palindrome checking. Invalid programs and invalid input should produce useful source-level errors.

Implemented: the milestone's six programs run through the CLI and the acceptance tests cover their output and failure cases. The example corpus has since grown to ten; [syntax-examples.md](syntax-examples.md) records it.

This establishes an executable language foundation: a small syntax, defined behavior, a checker, and an evaluator. It lets us judge readability using working code before extending the language.

## Smallest target program

```text
word square(n: Int) -> Int
    let result = n * n
    result

word double(n: Int) -> Int
    n + n

flow 4
    square
    double
    print
```

Run the program with:

```sh
go run ./cmd/zhex run examples/composition.zhex
```

Expected standard output:

```text
32
```

The interpreter and command are implemented. The [initial language contract](initial-language.md) records the grammar and semantics; the [test catalog](tests.md) identifies executable acceptance cases.

The [example corpus](syntax-examples.md) extends this smallest target with runnable `.zhex` source files, input/output contracts, expected results, and edge cases. Completing this milestone means running the whole corpus, including console input.

## Required behavior

- Define reusable words with explicit parameter and result types, and check their bodies against those signatures.
- Support integers, booleans, strings, arithmetic, comparisons, named parameters, and immutable lexical locals. Infer local expression types rather than requiring repeated annotations. Explicit `var` bindings allow type-preserving reassignment for iterative algorithms.
- Execute a flow in source order. Its intermediate values have checked types; each step receives the previous result. `print` has built-in overloads for `Int`, `String`, and `Bool`, writes a representation followed by a newline, and returns `Unit`.
- Support `if`/`elif`/`else` and one bounded `for` loop over a half-open integer range. Provide integer-to-text conversion and exact code-point string reversal through built-in operations.
- Read text and integers from standard input with a prompt. Handle redirected input, empty lines, malformed integers, and end-of-input according to the [example contracts](syntax-examples.md). Propagate input errors through postfix `try`.
- Resolve names and check the whole file before evaluation. Unknown names, incorrect argument counts, and incompatible types prevent program execution.
- Report malformed source and arithmetic failures with the filename, line, and column, and return a nonzero exit status. Arithmetic failures remain runtime errors unless established during checking.

Named expressions and flows use the same numeric rules: signed 64-bit integers, checked overflow, and errors for division by zero. The initial language contract defines the supported operators, precedence, literal bounds, and failure behavior.

## Foundation for later work

Keep parsing, checking, and evaluation separate. Carry source locations, resolved word references, and type information into the checked program so a later compiler can reuse the frontend. Named values and implicit flow values should describe the same kind of typed computation.

The performance direction is to let future compiled code represent suitable values directly, eliminate unnecessary intermediate movement, and optimize across small words. A local name should not itself require heap allocation or dynamic dispatch. This milestone demonstrates semantics and architecture; native speed and allocation behavior need later backend measurements.

## Completion criteria

1. The target program runs from a file and prints exactly `32` followed by a newline.
2. Equivalent direct word calls and flows produce the same results; local names preserve lexical scope and inferred types.
3. A deliberate unknown word, wrong argument count, and incompatible type produce specific diagnostics before any output occurs.
4. Overflow and division by zero produce controlled language errors rather than a Go panic or silent wrapping.
5. Program-level tests cover these cases, and `go test ./...` and `go build ./cmd/zhex` succeed.
6. A reader can trace `4 → 16 → 32 → output` from the source without manually tracking stack permutations.
7. The greeting, FizzBuzz, Fibonacci, factorial, and palindrome programs accept the documented inputs and produce the expected output, including boundary cases.
8. Malformed numeric input and premature EOF produce controlled errors with nonzero exit status; valid redirected input behaves the same as terminal input.

## Scope

Tasks 1–5 are complete. A REPL, user-written modules, rich domain types, file resources, native compilation, and application frameworks follow in later milestones. Their extension points should remain clear, but their implementations are not needed to finish this goal.

The next planned work is the REPL and explicit module support. The current interpreter uses separate parser, checker, evaluator, and console-operation files in `internal/lang` so these can grow without adding a package boundary for every component.
