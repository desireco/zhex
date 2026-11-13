# Executable language tests

The test suite is part of Zhex's language definition. Program tests run the real parser, checker, and evaluator with supplied console streams, then compare output or diagnostics. The example source files are the readable starting point; the tests add boundary cases and deliberately invalid programs.

## Run the suite

```sh
go test ./...
go test -race ./...
go vet ./...
```

Run an individual area with `go test ./internal/lang -run TestExamples` or `go test ./internal/lang -run TestRejectedPrograms -v`. Tests use only the Go standard library. No network or external service is needed. The CLI process test builds a temporary executable with the installed Go toolchain.

## Language behavior catalog

| Test | Executable specification |
| --- | --- |
| `TestExamples` in [examples_test.go](../internal/lang/examples_test.go) | All six `.zhex` files, console prompts, ordinary inputs, empty lines, final lines without newlines, numeric boundaries, and exact palindrome comparison. |
| `TestLanguageFeatures` in [features_test.go](../internal/lang/features_test.go) | Operator precedence, booleans, short-circuit evaluation, minimum integers, strings, mutable loops, empty ranges, lexical shadowing, forward calls, multiple arguments, and input results. |
| `TestRejectedPrograms` in [features_test.go](../internal/lang/features_test.go) | Unknown names, wrong arity/types, immutable assignment, scope violations, duplicate definitions, incompatible branches/returns, invalid flows, missing `try`, malformed source, and unsupported types. |
| `TestRuntimeErrors` in [features_test.go](../internal/lang/features_test.go) | Checked arithmetic failures produce language diagnostics instead of silent wrapping or host panics. |
| `TestInputFailures` in [examples_test.go](../internal/lang/examples_test.go) | EOF, empty or malformed numeric input, out-of-range integers, and invalid UTF-8. |
| Tests in [io_test.go](../internal/lang/io_test.go) | No input/output before static checking succeeds, shared buffered input, prompt flushing, long lines, line-ending handling, disconnected streams, and short writes. |
| Tests in [limits_test.go](../internal/lang/limits_test.go) | Syntax/call depth limits and fresh execution state for a compiled program. |
| `TestCheckedArithmeticAgainstIntegerMath` in [arithmetic_test.go](../internal/lang/arithmetic_test.go) | Checked 64-bit arithmetic agrees with arbitrary-precision reference mathematics across boundary and deterministic randomized inputs. |
| `FuzzCompile` in [fuzz_test.go](../internal/lang/fuzz_test.go) | Malformed source produces diagnostics rather than unexpected parser/checker panics. Default tests run the seed corpus; a timed fuzz run explores mutations. |
| Tests in [session_test.go](../internal/lang/session_test.go) | REPL entry boundaries (only a missing indented block continues), persistence of words and values across entries, word redefinition with a note, rollback after a check error, effects kept after a runtime error, and the shared buffered reader. |
| Tests in [text_test.go](../internal/lang/text_test.go) | The `text` words' semantics — code-point length, indexing, and slicing with clamped bounds, line and word counts, Unicode case and trim, literal all-occurrences `replace` — and the `replace-empty` error with text and JSON agreeing. |
| Tests in [buffer_test.go](../internal/lang/buffer_test.go) | Buffer round trip, aliasing through `let` (two names, one buffer), `buffer-range` failures inside the `Result`, rejection by `print` (with a `buffer.to-text` hint) and by `==`, and `Buffer` in word signatures. |
| Tests in [file_test.go](../internal/lang/file_test.go) | Whole-file and streaming reads, chomped lines, idempotent close, read-after-close and end-of-file failures, the `for line in file.lines` loop (empty files, `String` binding composing with `text` words), its loop-only context rule, and the guaranteed close across success, mid-stream, and loop-body failures. |
| Tests in [args_test.go](../internal/lang/args_test.go) | `console.arg` and `console.args-count` argument forwarding, the `argument-missing` diagnostic naming index and count, and the `try` hint for a bare `console.arg` result. |
| Tests in [main_test.go](../cmd/zhex/main_test.go) | Actual CLI behavior, usage errors, missing files, stdout/stderr separation, executable invocation, exit codes, and `zhex repl` sessions (scripted entries, `words`, `--trace` events, EOF and final-line handling). |

`TestArithmetic` and `TestInvalidProgramHasNoEffects` in [run_test.go](../internal/lang/run_test.go) preserve the first execution slice and the check-before-effects rule.

For a short parser fuzz run, use `go test ./internal/lang -run '^$' -fuzz FuzzCompile -fuzztime=3s -parallel=2`.

## Adding a language feature

Add a small source example with its expected result or diagnostic first. Run the focused test and confirm it exposes the missing behavior. Implement the feature, update the [language contract](initial-language.md), then run the full suite. Preserve invalid-program tests as well as successful examples: preventing an incorrect execution is part of the language's behavior.

These tests validate the Go interpreter. They do not establish the performance of a future native backend. That backend should reuse the semantic examples and add its own representation and performance checks.
