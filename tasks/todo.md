# Initial Go interpreter tasks

Tasks 1–5 are implemented and verified; tasks 6–8 remain pending. This checklist accompanies the [plan](plan.md) and [language goals](../docs/language-goals.md). The [example corpus](../docs/syntax-examples.md) runs in the Go interpreter. Implemented components share `internal/lang` with separate files for each stage.

Tasks 1–5 deliver the [first milestone](../docs/first-milestone.md), including console input. Tasks 6–8 extend that foundation toward the initial practical CLI.

## Task 1 Establish the initial language contract

Describe the smallest grammar and semantics sufficient for the example corpus, including indentation, evaluation order, values, calls, branches, flows, and errors.

- [x] Specify the proposed defaults in the plan and explicitly defer unsupported features.
- [x] Settle the existing examples' calls, locals, decisions, loops, text operations, and input/`try` semantics; defer module syntax to task 7.
- [x] Give representative invalid examples with expected diagnostic locations.

Verification: manually trace each example against the contract; check consistency with the goals.

Dependencies: none. Likely files: `docs/initial-language.md`, `docs/syntax-examples.md`. Scope: small.

## Task 2 Run arithmetic from a file

Establish the Go executable and the shortest complete source-to-result path.

- [x] Run an integer arithmetic expression and `print` from a file through parsing, checking, and evaluation.
- [x] Report malformed input, wrong operand types, overflow, and division by zero with source locations and a nonzero exit status.
- [x] Add valid and invalid executable examples to a program-level test harness using injected console streams.

Verification: `go test ./...`, `go build ./cmd/zhex`, and run the arithmetic example through the CLI.

Dependencies: 1. Likely areas: `go.mod`, `cmd/zhex/`, `internal/`, `testdata/`. Scope: medium; keep package scaffolding minimal, splitting implementation further if it grows beyond one focused session.

## Task 3 Compose named words and flows

Make reusable operations readable without stack rearrangement.

- [x] Define typed words with named inputs and lexical locals; resolve calls before execution.
- [x] Execute a flow with compatible adjacent input/output types and a final optional `Unit` result.
- [x] Reject unknown names, incorrect argument counts, and incompatible flow steps before host effects run.

Verification: run the composition example and compare equivalent named-call and flow results; run focused checker/evaluator tests and `go test ./...`.

Dependencies: 2. Likely areas: syntax, checker, evaluator, examples/tests. Scope: medium.

## Checkpoint after task 3

- [x] Run `examples/composition.zhex` from the first milestone and verify exact output `32` plus a newline and the specified error cases.
- [x] Arithmetic, named locals, calls, and flows have consistent rules and diagnostics.
- [x] Review whether the examples communicate data flow without manual stack simulation.
- [x] Update the language contract to match intentional decisions before adding constructs.

## Task 4 Add decisions and bounded iteration

Support meaningful application logic and the operations needed to process text.

- [x] Execute conditions with compatible branch results and defined boolean behavior.
- [x] Support a half-open integer-range loop and explicit type-preserving mutable locals; provide integer formatting, string concatenation/comparison, and code-point reversal.
- [x] Handle empty inputs, loop boundaries, and invalid conditions with defined results or useful errors.

Verification: call the FizzBuzz, Fibonacci, factorial, and palindrome words with literal arguments; check the corpus's numeric/text boundary cases plus branch, scope, and iteration behavior through `go test ./...`.

Dependencies: 3. Likely areas: syntax, checker, evaluator, host operations, examples/tests. Scope: medium.

## Task 5 Add console input and complete the examples

Make the initial examples accept user input and print their results.

- [x] Implement prompted `console.read-line` and `console.read-int`, the built-in input results, and postfix `try` with injected input/output streams.
- [x] Handle empty lines, a final line without a newline, invalid UTF-8, malformed/out-of-range integers, EOF, and I/O failures according to the corpus contracts.
- [x] Run all six example files with supplied standard input and verify exact output, diagnostics, and exit status; type errors must occur before any prompt or read.

Verification: use scripted input streams for the full corpus and a terminal smoke check; run `go test ./...` and `go build ./cmd/zhex`. Distinguish terminal input echo from the program's own output.

Dependencies: 4. Likely areas: host console operations, checker signatures, evaluator propagation, CLI integration tests. Scope: medium.

## Checkpoint after task 5

- [x] All first-milestone examples and their boundary/error cases work.
- [x] Console input/output and checked composition are demonstrated end to end.
- [x] Review the syntax using both algorithmic and text examples before adding more language features.

## Task 6 Add interactive execution

Make language experiments quick using the existing execution path.

- [ ] Enter multi-line definitions and expressions in a REPL with a persistent definition environment.
- [ ] Preserve the last valid environment after an invalid entry and display useful diagnostics.
- [ ] Inspect values and word signatures; ensure file and interactive execution agree.

Verification: run a scripted REPL session with a definition, successful call, failed edit, and subsequent successful call; run `go test ./...`.

Dependencies: 5. Likely areas: CLI/REPL, checker environment, diagnostics, session tests. Scope: medium.

## Task 7 Compose explicit modules

Split a working program into independently understandable modules.

- [ ] Import public words using the contract's explicit module and qualification rules.
- [ ] Reject private-name access, duplicate definitions, missing imports, and cyclic imports in the initial module model.
- [ ] Report a cross-module error with the originating file and source location.

Verification: run a two-module example plus invalid import cases and `go test ./...`.

Dependencies: 4. Likely areas: module loading, checker, CLI, fixtures. Scope: medium.

## Checkpoint after task 7

- [ ] File execution, REPL behavior, and modules use the same semantics.
- [ ] Review validation and text-processing examples for readability.
- [ ] Run the complete test suite and build; settle cleanup semantics before file-resource integration.

## Task 8 Deliver a useful text utility

Build a small CLI that streams text, computes a result, and demonstrates cleanup.

- [ ] Read a path from arguments, process text incrementally, and print a useful result from modular Zhex code.
- [ ] Close file resources on success and failure; reject operations on closed handles with a defined error.
- [ ] Document and measure one representative workload, including input size, elapsed time, and Go allocation behavior.

Verification: run normal, empty, unreadable, and deliberately failing inputs; verify cleanup and bounded buffering on a large fixture; run `go test ./...` and `go build ./cmd/zhex`.

Dependencies: 6, 7. Likely areas: host files/resources, CLI arguments, example modules, integration tests, benchmark notes. Scope: medium; keep the utility to one operation and split resource support if needed.

## Initial interpreter completion

- [ ] A new user can build the interpreter, run an example, and try it interactively using documented commands.
- [ ] Representative logic and error examples match the written contract.
- [ ] The text utility works with modules, clear errors, and predictable cleanup.
- [ ] Record what became easier to read, what remained awkward, and which application should drive the next slice.
