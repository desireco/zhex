# Initial Zhex examples

These source files run in the Go interpreter and are validated by its test suite. They exercise readable composition, named values, conditions, loops, and console input/output. The [initial language contract](initial-language.md) defines their syntax and semantics.

## Example programs

| File | Input | Result after the prompt | Purpose |
| --- | --- | --- | --- |
| [composition.zhex](../examples/composition.zhex) | None | `32` | Typed words, inferred locals, flow composition. |
| [hello.zhex](../examples/hello.zhex) | `Ada` | `Hello, Ada!` | Read a whole line, build a string, print it. |
| [fizzbuzz.zhex](../examples/fizzbuzz.zhex) | `15` | The sequence below | Remainders, ordered conditions, bounded iteration, formatting. |
| [fibonacci.zhex](../examples/fibonacci.zhex) | `10` | `55` | Explicit local mutation and iterative computation. |
| [factorial.zhex](../examples/factorial.zhex) | `5` | `120` | Accumulation, empty loops, integer limits. |
| [palindrome.zhex](../examples/palindrome.zhex) | `racecar` | `Palindrome` | String input, comparison, and a text-library operation. |

Each result line ends with a newline. The interactive programs print a prompt before reading. For FizzBuzz with input `15`, the result lines are:

```text
1
2
Fizz
4
Buzz
Fizz
7
8
Fizz
Buzz
11
Fizz
13
14
FizzBuzz
```

## Input and output

| Operation | Contract |
| --- | --- |
| `console.read-line(prompt)` | Write and flush the prompt, then return `Result<String, InputError>` from one line of standard input. |
| `console.read-int(prompt)` | Read a line, trim surrounding whitespace, and parse an optional sign followed by decimal digits into a checked 64-bit `Int`; return `Result<Int, InputError>`. |
| `print(value)` | Write an `Int`, `String`, or `Bool` to standard output followed by a newline; return `Unit`. |

`read-line` removes one trailing LF and an immediately preceding CR, preserving the remaining spaces and text. An empty line is a valid empty string. A nonempty final line without a newline is accepted. EOF before any line content, invalid UTF-8, and I/O failures produce input errors. `read-int` additionally rejects empty/whitespace-only input, malformed integers, and integers outside the supported range.

In these examples, postfix `try` unwraps a successful input result. Failure propagates to the runner, which writes a source-located diagnostic to standard error and exits nonzero. Earlier runtime output, including a prompt, can remain. This is a narrow initial use of `Result` and propagation; user-defined generic types and recovery syntax are later design work.

All names and types are checked before the program runs, so an unknown word or incompatible flow produces no prompt and consumes no input. Output failures should likewise produce controlled runtime errors. Input comes from standard input in both interactive terminals and redirected streams; file execution and the REPL must share that input stream without losing buffered data.

Run the examples:

```sh
go run ./cmd/zhex run examples/hello.zhex
go run ./cmd/zhex run examples/fizzbuzz.zhex
go run ./cmd/zhex run examples/fibonacci.zhex
go run ./cmd/zhex run examples/factorial.zhex
go run ./cmd/zhex run examples/palindrome.zhex
```

## Syntax exercised by the examples

- Named word parameters and result types are explicit; local types and flow intermediate types are inferred.
- `let` binds an immutable local. `var` explicitly permits reassignment within lexical scope; assignment preserves its type. Naming a value does not dictate its eventual machine representation.
- Blocks use indentation. A word's final expression supplies its result; an `if` used as a value has compatible branch result types. Statement branches in these top-level programs return `Unit`.
- Calls use parentheses. Qualified names such as `text.reverse` identify module operations; hyphenated words such as `is-palindrome` are identifiers. Use spaces around subtraction between names: `a-b` is a name, while `a - b` subtracts.
- `+` supports integer addition or string concatenation with matching operand types; no automatic integer-to-string conversion occurs. `%` is integer remainder. `==` compares same-typed values by value. `or` combines booleans with short-circuit evaluation.
- `range(start, end)` is an ascending integer iteration with an exclusive end. When `start >= end`, it performs no iterations. It should not require materializing a list of all integers.
- `int.to-string` returns decimal text. `text.reverse` reverses a valid string by Unicode code points. These standard operations are predeclared for this milestone; user-written modules follow later.

`print` uses statically selected built-in overloads rather than requiring a general formatting protocol. Boolean output is `true` or `false`. Broader traits, collections, and polymorphism are not needed for these examples.

## Boundary and error examples

| Program | Case | Expected behavior |
| --- | --- | --- |
| Hello | Empty line | Print `Hello, !`. |
| FizzBuzz | `1` | Print `1`. |
| FizzBuzz | `0`, negative, or above `10000` | Print the supported-range message and skip the calculation. |
| Fibonacci | `0`, `1`, `2` | Print `0`, `1`, `1`, respectively. |
| Fibonacci | `92` | Print `7540113804746346429` without computing the overflowing next Fibonacci number. |
| Fibonacci | Negative or above `92` | Print the supported-range message and skip the calculation. |
| Factorial | `0`, `1` | Print `1`. |
| Factorial | `20` | Print `2432902008176640000`. |
| Factorial | Negative or above `20` | Print the supported-range message and skip the calculation. |
| Palindrome | `racecar`, `abba`, `été`, or an empty line | Print `Palindrome`. |
| Palindrome | `hello` or `Racecar` | Print `Not a palindrome`. |
| Numeric input | `abc`, `1.5`, empty line, or out-of-range integer | Report an input error and exit nonzero. |
| Any input | EOF before any content | Report an input error and exit nonzero; never silently substitute zero or an empty string. |

Supported-range messages are successful program branches and exit normally; malformed input is a runtime error. Fibonacci uses `F(0) = 0` and `F(1) = 1`. The example runner bounds avoid signed 64-bit overflow; the arithmetic operations must still detect overflow independently. The mathematical helper words assume the documented input domain and are guarded by their callers here.

Palindrome comparison is exact, including case, punctuation, spaces, and Unicode normalization. Code-point reversal does not group combining sequences into visual characters. Case folding, normalization, and grapheme-aware operations can be added as explicit text-library features later.

## Later algorithm examples

GCD would exercise Euclid's algorithm and a condition-controlled loop; primality testing would exercise early exits; exponentiation by squaring would exercise logarithmic iteration. Binary search would exercise arrays and indexing. Normalized palindrome checks and word-frequency counting would exercise richer Unicode/text operations and maps. Add these as the corresponding features become concrete.
