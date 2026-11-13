# Executable language tests

The test suite is part of Zhex's language definition. Program tests run the real parser, checker, and evaluator with supplied console streams, then compare output or diagnostics. The example source files are the readable starting point; the tests add boundary cases and deliberately invalid programs.

## Run the suite

```sh
go test ./...
go test -race ./...
go vet ./...
```

Run an individual area with `go test ./internal/lang -run TestExamples` or `go test ./internal/lang -run TestRejectedPrograms -v`. Tests use only the Go standard library. No network or external service is needed. The CLI process test builds a temporary executable with the installed Go toolchain.

## Coverage matrix

Where each shipped feature family is documented, and which tests pin it. The catalog below names the individual tests.

| Feature | Documented on | Pinned by |
| --- | --- | --- |
| Diagnostics — excerpt, caret, hints, suggestions | [AGENTS.md](../AGENTS.md), [diagnostics-plan.md](diagnostics-plan.md), [catalog.md](../internal/lang/catalog.md), [llms.txt](../site/llms.txt), the [site errors page](../site/errors.tmpl) | [diagnostics_test.go](../internal/lang/diagnostics_test.go) |
| Codes and the JSON report | [AGENTS.md](../AGENTS.md), [diagnostics-plan.md](diagnostics-plan.md), [catalog.md](../internal/lang/catalog.md), [llms.txt](../site/llms.txt), the [site errors page](../site/errors.tmpl) | [codes_test.go](../internal/lang/codes_test.go) |
| Spans and provenance | [AGENTS.md](../AGENTS.md), [diagnostics-plan.md](diagnostics-plan.md) | [spans_test.go](../internal/lang/spans_test.go), [diagnostics_test.go](../internal/lang/diagnostics_test.go) |
| Trace events | [AGENTS.md](../AGENTS.md), [diagnostics-plan.md](diagnostics-plan.md), [llms.txt](../site/llms.txt) | [trace_test.go](../internal/lang/trace_test.go) and `TestReplTrace` in [main_test.go](../cmd/zhex/main_test.go) |
| REPL | [AGENTS.md](../AGENTS.md), [getting-started.md](getting-started.md), [initial-language.md](initial-language.md), the [site docs page](../site/docs.tmpl), [llms.txt](../site/llms.txt), [README.md](../README.md) | [session_test.go](../internal/lang/session_test.go), [main_test.go](../cmd/zhex/main_test.go) |
| Vocabulary — text, buffer, file, arguments | [AGENTS.md](../AGENTS.md), [initial-language.md](initial-language.md), the [site docs page](../site/docs.tmpl), [llms.txt](../site/llms.txt) | [text_test.go](../internal/lang/text_test.go), [buffer_test.go](../internal/lang/buffer_test.go), [file_test.go](../internal/lang/file_test.go), [args_test.go](../internal/lang/args_test.go) |
| Modules — use, qualified calls, underscore privacy | [AGENTS.md](../AGENTS.md), [initial-language.md](initial-language.md), the [site docs page](../site/docs.tmpl), [llms.txt](../site/llms.txt), [README.md](../README.md) | [modules_test.go](../internal/lang/modules_test.go), the module tests in [session_test.go](../internal/lang/session_test.go), `TestRunSiblingModules` and `TestReplUseSessionScript` in [main_test.go](../cmd/zhex/main_test.go) |
| Example corpus and text-stats | [README.md](../README.md), [getting-started.md](getting-started.md), [syntax-examples.md](syntax-examples.md), the [site examples page](../site/examples.tmpl), [llms.txt](../site/llms.txt) | `TestExamples` in [examples_test.go](../internal/lang/examples_test.go) |
| Benchmarks | [AGENTS.md](../AGENTS.md), [measurements.md](measurements.md), this catalog | [bench_test.go](../internal/lang/bench_test.go) |

## Language behavior catalog

| Test | Executable specification |
| --- | --- |
| `TestExamples` in [examples_test.go](../internal/lang/examples_test.go) | All ten programs — nine single files and the two-file modules program, its module resolved as a sibling — console prompts, ordinary inputs, empty lines, final lines without newlines, numeric boundaries, exact palindrome comparison, GCD recursion with its nonnegative guard, the times-table bounds, text-stats counts, uppercased streaming, failure exits, and the modules program's positive-integer guard. |
| `TestLanguageFeatures` in [features_test.go](../internal/lang/features_test.go) | Operator precedence, booleans, short-circuit evaluation, minimum integers, strings, mutable loops, empty ranges, lexical shadowing, forward calls, multiple arguments, and input results. |
| `TestRejectedPrograms` in [features_test.go](../internal/lang/features_test.go) | Unknown names, wrong arity/types, immutable assignment, scope violations, duplicate definitions, incompatible branches/returns, invalid flows, missing `try`, malformed source, and unsupported types. |
| `TestRuntimeErrors` in [features_test.go](../internal/lang/features_test.go) | Checked arithmetic failures produce language diagnostics instead of silent wrapping or host panics. |
| `TestInputFailures` in [examples_test.go](../internal/lang/examples_test.go) | EOF, empty or malformed numeric input, out-of-range integers, and invalid UTF-8. |
| Tests in [io_test.go](../internal/lang/io_test.go) | No input/output before static checking succeeds, shared buffered input, prompt flushing, long lines, line-ending handling, disconnected streams, and short writes. |
| Tests in [limits_test.go](../internal/lang/limits_test.go) | Syntax/call depth limits and fresh execution state for a compiled program. |
| `TestCheckedArithmeticAgainstIntegerMath` in [arithmetic_test.go](../internal/lang/arithmetic_test.go) | Checked 64-bit arithmetic agrees with arbitrary-precision reference mathematics across boundary and deterministic randomized inputs. |
| `FuzzCompile` in [fuzz_test.go](../internal/lang/fuzz_test.go) | Malformed source produces diagnostics rather than unexpected parser/checker panics. Default tests run the seed corpus; a timed fuzz run explores mutations. |
| Tests in [diagnostics_test.go](../internal/lang/diagnostics_test.go) | Diagnostics carry a source excerpt, a caret, and the hyphen-name hint; typos get did-you-mean suggestions; every `Result` type gets a `try` hint and a rendering (the result table); carets span whole failing expressions; type mismatches narrate provenance; and runtime diagnostics show excerpts too. |
| Tests in [codes_test.go](../internal/lang/codes_test.go) | The code catalog stays a bijection with the interpreter's own definitions, sorted and unique; every raise site pairs with its code; and `--report=json` keeps its pinned shape, with span fields added only when a checker diagnostic has a span. |
| `TestExpressionsCarryEndPositions` in [spans_test.go](../internal/lang/spans_test.go) | Parsed expressions carry end positions, so a caret can underline the whole failing expression instead of one column. |
| `TestTrace*` in [trace_test.go](../internal/lang/trace_test.go) | Golden JSONL event streams for composition and GCD; loop, assignment, `Result`, and Unit bindings; Buffer and File renderings (`TestTraceResourceRenderings`) — and the REPL's `"file":"repl"` events, pinned from the CLI by `TestReplTrace` in [main_test.go](../cmd/zhex/main_test.go). |
| `Benchmark*` with `TestBenchmarksRunRealPrograms` in [bench_test.go](../internal/lang/bench_test.go) | The four benchmarks — text-stats counts, text-stats upper, compiling text-stats, and Fibonacci — time real programs; the guard test fails if a benchmarked program stops running. `just bench` records results in [measurements.md](measurements.md). |
| Tests in [session_test.go](../internal/lang/session_test.go) | REPL entry boundaries (only a missing indented block continues), persistence of words and values across entries, word redefinition with a note, rollback after a check error, effects kept after a runtime error, the shared buffered reader, and module adoption: `use` resolving against the working directory, repeats as a no-op, qualified `words` listings without module-private words, dotted redefinition replacing an adopted word with a note, adoption checking module bodies and rolling back on failure, nested and transitive uses staying private, and module runtime errors annotated from the module's source. |
| Tests in [text_test.go](../internal/lang/text_test.go) | The `text` words' semantics — code-point length, indexing, and slicing with clamped bounds, line and word counts, Unicode case and trim, literal all-occurrences `replace` — and the `replace-empty` error with text and JSON agreeing. |
| Tests in [buffer_test.go](../internal/lang/buffer_test.go) | Buffer round trip, aliasing through `let` (two names, one buffer), `buffer-range` failures inside the `Result`, rejection by `print` (with a `buffer.to-text` hint) and by `==`, and `Buffer` in word signatures. |
| Tests in [file_test.go](../internal/lang/file_test.go) | Whole-file and streaming reads, chomped-line parity across all three readers (including a final bare CR), idempotent close, read-after-close and end-of-file failures, the `for line in file.lines` loop (empty files, `String` binding composing with `text` words), its loop-only context rule, and the guaranteed close across success, mid-stream, and loop-body failures. |
| Tests in [args_test.go](../internal/lang/args_test.go) | `console.arg` and `console.args-count` argument forwarding, the `argument-missing` diagnostic naming index and count, and the `try` hint for a bare `console.arg` result. |
| Tests in [modules_test.go](../internal/lang/modules_test.go) | `use` parsing and its top-level-only rule, sibling resolution with duplicate-use idempotence, `use-missing` naming the searched path, cycle and reserved-name errors spelling files (including cycles through the entry and `./` path spellings), repeated imports within a single REPL entry remaining silent, `module-body` rejection, qualified and bare intra-module calls, underscore privacy (`private-use` outside, never suggested cross-module), per-file duplicates and entry collisions, same-named words across modules, transitive imports staying private, module bodies checking before the entry file, module syntax and runtime errors annotating the module's source, qualified trace names (module-private words included) and span-free runtime reports, multi-file/single-file output parity, and `use` hoisting below the use site. |
| Tests in [main_test.go](../cmd/zhex/main_test.go) | Actual CLI behavior, usage errors, missing files, stdout/stderr separation, executable invocation, exit codes, `zhex repl` sessions (scripted entries, `words` — including its shadowing of a user binding, `TestReplWordsShadowsBinding` — `--trace` events, EOF and final-line handling), sibling-module runs loading `math.zhex` beside the entry with `use-missing` naming the searched path (`TestRunSiblingModules`), and REPL sessions that adopt a module with qualified calls and the private-use diagnostic (`TestReplUseSessionScript`). |

`TestArithmetic` and `TestInvalidProgramHasNoEffects` in [run_test.go](../internal/lang/run_test.go) preserve the first execution slice and the check-before-effects rule.

For a short parser fuzz run, use `go test ./internal/lang -run '^$' -fuzz FuzzCompile -fuzztime=3s -parallel=2`.

## Adding a language feature

Add a small source example with its expected result or diagnostic first. Run the focused test and confirm it exposes the missing behavior. Implement the feature, update the [language contract](initial-language.md), then run the full suite. Preserve invalid-program tests as well as successful examples: preventing an incorrect execution is part of the language's behavior.

These tests validate the Go interpreter. They do not establish the performance of a future native backend. That backend should reuse the semantic examples and add its own representation and performance checks.
