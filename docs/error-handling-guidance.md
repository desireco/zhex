# Error handling without unnecessary propagation

Design guidance, November 13, 2026. This is the policy for reviewing new APIs and
simplifying existing ones. It supersedes the assumption that every possible
runtime failure requires a Result. It is not a claim that every migration below
has shipped; executable tests and the language contract describe current syntax.

## Keep ordinary computation ordinary

Return a plain value when an operation performs ordinary computation on values
the program controls. Keep automatic safety checks. Invalid indexing, invalid
internal dimensions, integer overflow and mutation during traversal are usually
programming faults: report a precise diagnostic instead of making every caller
carry an error result.

Return a Result when failure is an expected outcome that the application can
meaningfully handle: reading an unavailable file, parsing external text, decoding
untrusted data, or resolving an unknown timezone name. A Result is a contract
with the caller, not a list of everything that might go wrong inside a machine.

Use Option when absence is an ordinary answer, as with a missing map key. Do not
turn absence into an error just to reuse error-handling machinery.

| Operation | Normal contract | Failure or absence |
| --- | --- | --- |
| List/array indexed read | Element value | Automatic bounds/rank diagnostic |
| List/array indexed write | Unit | Automatic validation before mutation |
| Numeric array construction, append, resize | Array or Unit | Invalid shape, traversal or supported-size diagnostic |
| Map lookup | Option of the element | None for an absent key |
| Arithmetic on typed values | Computed value | Overflow or invalid arithmetic diagnostic |
| Numeric date/time construction and typed arithmetic | Temporal value | Invalid fields or representational overflow diagnostic |
| External text/date parsing, timezone-name lookup | Result | Caller handles or propagates expected invalid input |
| File/input operations | Result | Caller handles or propagates operational failure |

Collection reads and construction/growth in this table are approved migration targets.
Temporal operations are design guidance for their implementation. Keep existing
checked spellings working where the approved migration allows them. Do not
silently discard Results from ordinary fallible calls.

## Checking is not error handling

The checker should reject an invalid ordinary operation when it can establish
the relevant facts. It may prove an access safe and remove a redundant runtime
check. Otherwise the interpreter checks bounds automatically before access.
Neither case requires a source-level `try`.

Static bounds analysis is an improvement, not a prerequisite for plain indexing.
`Array<Int>` alone does not prove a particular shape. Dynamic indices, aliases,
resizing and calls can invalidate facts; retain runtime checks unless a proof is
sound. A failed check must never become an unchecked Go panic, invalid memory
access, or silent default value.

An explicitly checked operation still permits recovery when its failing input is
known statically; optimization must preserve its documented `catch` behavior.

## Use try to propagate, catch to decide

`try` unwraps a successful Result and passes a failure to the caller. It does not
recover. `catch` supplies the application's response to a failed Result. Put
recovery where the program knows what a failure means: retry input, choose a
documented default, report the problem, or stop deliberately.

A parsing helper can legitimately propagate a parsing failure through another
helper to the caller that decides what to do. Each genuinely fallible call still
uses the language's explicit `try`/`catch` rules. Do not add dummy catches merely
to avoid propagation, and do not make every internal helper fallible because its
implementation indexes a collection or allocates ordinary storage.

Declare user words with their success type. Let escaping `try` infer the error
signature and wrap successful returns once. Keep explicit `Result`, `ok` and
`err` available for contracts that need them; do not teach them as routine
scaffolding around every transformation. No new general exception system or
implicit propagation is needed to simplify ordinary code.

## Make deliberate recovery narrow

An explicit checked form of a selected intrinsic is useful when its usual
programming precondition is instead uncertain application data. Ordinary
indexing returns a value; applying `try` or `catch` directly to the indexed read
selects its recoverable form. Existing parenthesized checked writes provide the
same distinction for mutation.

The approved collection set is indexed reads, `array.new`, `list.append` and
`array.resize`, alongside existing checked writes. Direct `try`/`catch` and
guarded flow steps select checked mode; a plain intrinsic call does not return
an unhandled Result. Merely assigning its result to a name does not select
checked mode. Ordinary user-defined fallible calls retain their Result contract.

This is a small, documented intrinsic rule, not permission to apply `catch` to
arbitrary runtime faults. Evaluate receiver, indices and arguments once, in the
normal order. Only the selected operation's documented validation failures
become its Result. Failures while evaluating a receiver or index, propagated
argument errors, and faults in the recovery body retain their own behavior.
An outer checked access must not silently guard an inner plain access.

Checked construction/growth should follow the same rule if supplied: recover
from documented preflight failures such as invalid dimensions or a supported
size limit. Keep mutation atomic on failure. Do not promise recovery from Go
process out-of-memory termination, and do not introduce arbitrary capacity
limits merely to justify returning a Result. Allocation alone is not a reason
to mark all value composition fallible.

A fallback must be correct for every failure it handles. If only an out-of-range
coordinate means zero, do not also turn a wrong-rank failure into zero. Validate
the relevant domain once, branch on the intended boundary condition, or inspect
the error code and preserve other failures. Do not broaden error handling just
to shorten an example.

## Apply this to Game of Life

The example owns a fixed 7 by 7 board. `show` reads valid coordinates; `advance`
creates another board of that shape and writes valid coordinates. These words
should have plain signatures. Their internal reads and construction, calls to
`next-cell`, and the final `advance`/`show` flow steps should require no `try`.

The neighbor rule is different: the algorithm deliberately treats positions
outside the finite board as dead. The simplest example can skip those positions
before reading, using the board's known bounds. A checked read with recovery is
also justified if it handles that boundary condition specifically. A broad
`catch` returning zero must not conceal a malformed board or unrelated fault.

After the read and construction migrations, the composition should be:

```text
flow start
    advance
    advance
    show
```

Passing a wrongly shaped board is still diagnosed. Removing propagation markers
does not claim that the compiler proves every board shape or remove its checks.

## Review and migration checklist

- Name the expected failure and meaningful caller response before introducing a
  Result. If the only explanation is "this could fail somehow," use a plain
  value and appropriate safety checks.
- For each `try` in an example, identify the recoverable operation it carries.
  Remove propagation caused solely by ordinary indexing/construction when that
  API migrates. Keep genuine input, parsing and I/O handling.
- Test both plain and explicitly checked failures: correct value/effective
  signature, precise diagnostics, exactly-once evaluation, nested failures that
  are not caught accidentally, and unchanged state after failed mutation.
- Keep tests rejecting discarded ordinary Results and tests for real recovery
  and propagation. Safety and deliberate recovery must survive simplification.
- Update canonical examples, public docs, llms.txt, AGENTS.md and historical plan
  amendments with each shipped API change. Clearly distinguish automatic bounds
  checking from optional application recovery; never say all built-ins return
  Results or all potentially failing computation needs `try`.
- Run `just test` and `go vet ./...` for the integrated change, including the
  website's executable examples. Keep the board and ROADMAP aligned with what
  actually ships.
