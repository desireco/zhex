# Program-level error handling: research and examples

Approved implementation contract for [#23](https://github.com/desireco/zhex/issues/23), November 13, 2026. The owner approved implementation after the research conversation. The contract immediately below supersedes the historical brainstorm later in this file. Implementation, verification, and deployment are tracked on the issue; approval of the design does not itself assert that the feature has shipped.

## Approved implementation contract

### Fallible words and explicit propagation

Words that can return a recoverable failure declare `Result<T, E>`. T is one supported value type; E is InputError, ParseError, FileError, BufferError, or the common Error family. This is a dedicated Result return annotation, not general user-defined generics, error unions, or custom records. Ordinary words keep ordinary return types. Error is a supported parameter/value type for inspection and reporting; Result parameters are not part of this slice.

`ok(value)` and `err(problem)` construct the two alternatives, checked against the enclosing fallible word's declared result. They require that context rather than inventing an unconstrained error type. A Result carries a value or an error, never both. A fallible word's final expression must construct the declared Result; there is no silent lifting of T into Result. `return` permits an explicit early Result return from a fallible word, including from a recovery block.

Postfix `try` unwraps success. On failure inside a declared fallible word, it immediately returns the error to that word's caller and skips its remaining statements. The error family must match; concrete families do not silently widen to Error. A non-fallible word cannot propagate a Result failure and must handle it. Top-level `try` preserves the runner's source-located diagnostic and exit status 1. Runtime faults remain outside Result recovery.

Every fallible call in a chain must explicitly propagate or handle failure. A fallible call cannot be used as a naked final Result expression to bypass the explicit call-site rule. Five call layers therefore show five propagation markers and a boundary handler, rather than one invisible failure path through ordinary signatures.

### Local recovery

`catch` is a postfix block expression with an optional immutable Error binding. The header ends the source line and introduces an indented body. This slice has no one-line catch shorthand. The left operand must be a Result. Its operation executes once; only a failed Result runs the handler. Success and the handler's continuing result both have type T. A handler can instead `return err(...)` from its enclosing fallible word.

Contract example, executable once #23 is implemented:

```text
let count = text.to-int("abc") catch problem
    print(error.code(problem))
    3
print(count)
```

The error binding is visible only inside the handler. Catch does not catch a fault or a propagated failure raised while evaluating its operand. For example, parsing must not silently catch a failed nested `read-line(...) try`. No rollback of prior output or mutation is implied.

### Stored Results and discarded failures

Reject naked discarded Results, including bare REPL Result expressions. The single stored-Result allowance is deliberately conservative:

1. Use immutable `let pending = fallible-call(...)`.
2. The immediately following statement in that same lexical block and REPL entry must have **the entire expression** `pending try` or `pending catch ...`.
3. That statement may be a standalone expression or the right-hand side of a new `let` binding. It cannot be assignment to an existing mutable name.
4. Reject Result aliases, mutation, passing to another word, raw returns, repeated handling, and handling only within a branch, loop, or short-circuit operand.

Thus `let pending = text.to-int("3")` followed by `let n = pending try` is valid; `print(pending try)`, a conditional use, or an intervening statement is invalid. This immediate barrier guarantees handling without introducing a general ownership or all-path dataflow system. Result values do not persist unresolved between REPL entries. Existing Result-rendering internals may still serve tracing; they no longer authorize a bare Result echo.

### Flows and explicit returns

A flow step naming a fallible word must include postfix `try`, as in `parse-number try`; successful output becomes the next step's input. A bare fallible step is rejected. The enclosing fallible word or top-level runner receives failure under the same rules as a direct call. A Result seed also requires explicit unwrapping or recovery. There is no automatic Result lifting across flows.

`return err(...)` makes conversion and propagation from a catch block possible. It returns from the enclosing word, not just from that block. Returning outside a word, returning a mismatched type, or leaking an undeclared error is rejected. An early return satisfies branch result checking as a control-flow exit; it does not manufacture a value for the abandoned continuation.

### Structured Error interface

The nine existing Result-producing built-ins and four concrete error families retain their identities and diagnostic codes. Error is their common inspection view, not an implicit conversion between Result error families. The immutable core contains stable code, message, failing Zhex operation, original Zhex source location, documented details, and an optional cause. Plain propagation preserves origin/cause and does not wrap at every layer.

| Operation | Result |
| --- | --- |
| `error.code(problem)`, `error.message(problem)`, `error.operation(problem)`, `error.file(problem)` | String |
| `error.line(problem)`, `error.column(problem)` | Int |
| `error.detail(problem, key)` | String; empty when the documented detail is absent |
| `error.has-cause(problem)` | Bool |
| `error.cause(problem)` | `Result<Error, Error>`; absent cause fails with stable code `error-no-cause` |
| `error.new(code, message)` | Error; originate a cause-less application failure at this Zhex call site |
| `error.wrap(code, message, cause)` | Error; explicit common-family conversion retaining cause and original origin |

These words accept the common Error view. Error is not a printable, comparable, mutable record; use its accessors. Details are represented through documented string keys in this initial interface. File failures expose path and normalized reason; bounds/argument failures expose their indices and counts. Numeric detail text is explicit metadata formatting, not an implicit conversion in ordinary expressions. Raw input is not automatically added to details.

Error bindings are immutable in this slice. Aliasing a caught error preserves its concrete family; putting it through an Error-valued return, success payload, or fallback must not erase that family implicitly. Ordinary inspection helpers returning String or Unit can accept a concrete caught error. Use explicit wrapping before a helper returns it as common Error. Error-valued Result constructors use direct calls rather than flow steps; Error-returning flow steps are conservatively excluded. The `error` module prefix is reserved for these standard operations.

A catch converting FileError to a word's declared `Result<Int, Error>` uses `return err(error.wrap("settings-read", "Could not read settings.", problem))`. Result-family conversion is explicit. New domain error types and richer structured values remain separate #5 work. The origin denotes the failing Zhex operation's call site, never a Go implementation location. Cause information is preserved as data rather than repeated inside every outer message. Existing text/JSON diagnostic contracts remain stable.

`error.new` enables validation failures that have no earlier cause, such as `return err(error.new("invalid-count", "Count must be positive."))` from a `Result<Int, Error>` word. This creates a value of the built-in common Error family; it does not declare a new error type. Its origin is the Zhex `error.new` call. Application-provided codes and messages are application data; built-in code/catalog stability remains a separate guarantee.

### Optional application reporter

The hook ships in #23 and is usable from Zhex applications. `error.set-reporter(report-word)` is a special checked call whose argument is a named word with signature `(problem: Error) -> Unit`; it does not introduce general first-class functions. Registration is top-level application configuration. Qualified public module words may be used under normal visibility rules. A REPL can replace its reporter explicitly.

`Program.RunWithOptions` additionally accepts a `RunOptions.Reporter` host callback. The language reporter receives the core Error; the host receives a separate reporting envelope with schema version, timestamp, outcome, occurrence identity, original origin, terminal boundary, and optional application correlation. The local language hook may deliberately inspect full error data. Host snapshots omit message, details and source filenames by default; `IncludeSensitive` explicitly opts into those fields. Cause snapshots contain at most 32 error nodes, with `cause_truncated` indicating omitted continuation; the original core cause chain is preserved. No networking or external destination is installed by default.

- A failed Result being produced or crossing `try` does not invoke a reporter.
- `error.report(problem)` explicitly reports a selected handled error synchronously at that call site. It returns Unit and does not alter recovery.
- Failure escaping the application boundary is reported once before its final diagnostic. An already reported occurrence is deduplicated rather than reported again at every layer or on later rethrow.
- If language and host reporters are installed, the language reporter runs first, then the host callback. Mark the occurrence before invocation so recursion cannot create duplicate notifications.
- Reporter reentrancy is suppressed. Callback failures/panics do not replace the original error, change return values, or override exit status. A hook does not install retry/queue/network infrastructure.
- Origin is retained. This implementation provides origin and boundary, not full propagation-frame capture; the existing trace stream supplies word-call events when requested. The reporting envelope belongs to the host/debugging surface rather than the everyday Result type, and never automatically captures arguments, locals or input.

The explicit report call itself is the application's reporting decision; the runtime does not defer it until a surrounding handler finishes. A handler that reports then fails can therefore produce a separate later failure occurrence. This precise rule supersedes the historical suggestion below to wait until recovery succeeds.

### Deliberate program exit

`console.exit(status: Int)` never returns. Status must be between 0 and 255 inclusive: 0 deliberately ends successfully; other values request that process status. It is valid as the terminating branch of any catch fallback and in ordinary or fallible words. The internal no-return type is not a new user annotation.

Valid exit unwinds the interpreter rather than invoking `os.Exit` inside it, so guaranteed-close `file.lines` loops release their handles. It produces neither a language diagnostic nor an automatic error-report notification. An explicit `error.report(problem)` earlier in a handler still runs; a subsequent valid exit does not report that occurrence again. An exit attempted by a reporter is suppressed with that reporter's other failures and cannot override the original program outcome.

An out-of-range status is a source-located `console-exit-status` runtime failure, eligible for normal unhandled reporting. It is not clamped or wrapped into the portable range. In the REPL, a valid exit ends the whole session with the requested status, including a final entry without a newline.

The host receives a typed `ExitError` carrying Status, allowing an embedder to apply termination policy without killing its process. The CLI translates it to the requested status without text or JSON diagnostics. This deliberate control outcome is distinct from both a failed Result and a runtime fault.

`console.fail(message: String)` is the message-bearing form of deliberate termination: it writes the message with one appended newline to the configured diagnostics writer, then requests status 1. It never returns, works as any typed catch fallback, uses the same interpreter unwinding/cleanup as exit, and does not automatically invoke an error reporter. It is a built-in operation, not a macro.

Diagnostics are passed through `RunOptions.Diagnostics` and corresponding CLI/REPL setup; the interpreter does not write directly to the process's stderr. An absent host diagnostics writer discards the message. The CLI supplies its configured stderr writer. A reporter invoking fail cannot override the original outcome. An explicit report before fail remains an explicit application decision.

### Verification and incremental delivery

Terra owns focused incremental implementation commits on main and all staging/pushing. Luna owns public/repository docs and runnable examples. Sol owns this contract and independent semantic review. The project card remains In Progress throughout active implementation.

Meaningful test-first cases cover five-layer propagation, successful and failing fallback paths, early returns, same/different error families, explicit fallible flows, discarded/stored Result rejection, modules/REPL, structured origin/details/cause, runtime faults bypassing catch, and user/host reporter lifecycle, deduplication, recursion and failure isolation. New diagnostics receive stable codes/catalog entries. Existing examples affected by formerly undeclared word failures must acquire accurate fallible signatures rather than escape checker enforcement.

Run `just test` and `go vet ./...`, including corpus execution and Astro link/build checks. Then commit/push, observe green Pages deployment, record evidence, and only then mark Done/close #23. Approval does not justify postponing the application hook or weakening checked propagation.

## Historical research and sketches

The following records the earlier conversation and alternatives. Its hypothetical sketches and statements about the old interpreter are preserved as research, not current instructions. Where they differ, the approved implementation contract above governs. They deliberately use `text` fences.

## Behavior before #23

The [language contract](initial-language.md) and executable tests remain authoritative:

- Nine operations produce fixed Result families: `read-line`, `text.to-int`, `console.arg`, `file.read-all`, `file.read-lines`, `file.open`, `file.read-line`, `buffer.line`, and `buffer.set-line`. Their error families are InputError, ParseError, FileError, and BufferError.
- A Result can be bound to a name. Postfix `try` unwraps success; failure unwinds the current run to the runner, even inside a word. The REPL reports the error and survives the entry.
- Word signatures accept Int, Bool, String, Unit, Buffer, and File. Programs cannot declare Result-returning words, construct success/error variants, branch on a Result, or inspect its error.
- The checker rejects a Result used as an ordinary value. It does not currently enforce the proposed rule that every Result must be handled or propagated: a standalone Result-producing expression can be discarded.
- Iteration consists of bounded ranges and streamed file lines. There is no while, break, continue, or explicit early return. Recursive calls have a depth limit of 256.

The implementation points are [Result representation and types](../internal/lang/syntax.go), [try evaluation](../internal/lang/eval.go), [checking](../internal/lang/check.go), and [signature parsing](../internal/lang/parser.go). Behavior is pinned in [feature tests](../internal/lang/features_test.go), [input tests](../internal/lang/examples_test.go), [resource tests](../internal/lang/file_test.go), and [session tests](../internal/lang/session_test.go).

## Elm and Gleam: the useful comparison

| Concern | Elm | Gleam | Implication for Zhex |
| --- | --- | --- | --- |
| Failure value | `Result error value`, with `Ok` and `Err` | `Result(value, error)`, with `Ok` and `Error` | The same two-outcome idea fits Zhex's single return value. Parameter order and constructor spelling are separate choices. |
| Optional value | `Maybe a`: `Just a` or `Nothing` | `Option(a)`: `Some(a)` or `None` | Absence without a reason should not replace existing detailed failures. |
| Local handling | `case` distinguishes success from failure | `case` distinguishes success from failure | A handler can inspect an error and choose another action; a fixed default covers only one case. |
| Successful transformation | `Result.map` | `result.map` | Change the success value while preserving the error. |
| Fallible chaining | `Result.andThen` | `result.try` | Run the next computation only on success; preserve the existing failure otherwise. |
| Error conversion | `Result.mapError` | `result.map_error` | Change the error type explicitly while retaining the success type. |

The [Elm guide](https://guide.elm-lang.org/error_handling/result.html) uses Result to preserve reasons and choose recovery based on the failure. [Maybe](https://guide.elm-lang.org/error_handling/maybe) carries no failure reason. The [Elm Result API](https://package.elm-lang.org/packages/elm/core/latest/Result), with its [official source](https://github.com/elm/core/blob/1.0.5/src/Result.elm), exposes defaults, mapping, chaining, and error conversion. Its `andThen` requires the same error type on the incoming and outgoing Results.

[Gleam's Result tour](https://tour.gleam.run/data-types/results/) presents errors as typed values handled with `case`. Its [Result API](https://gleam-stdlib.hexdocs.pm/gleam/result.html) includes ordinary library functions for chaining and recovery. `result.try` takes a callback; it is not an exception-catching keyword. `try_recover` runs a callback on failure, keeps the success type, and can change the error type. `map_error` converts only the error payload.

Gleam's [`use` expression](https://gleam.run/book/tour/use.html) turns the rest of a block into a callback. It works with functions other than Result helpers. The official [Gleam-for-Elm comparison](https://gleam.run/cheatsheets/gleam-for-elm-users/) directly relates `result.try` to Elm's `Result.andThen`. Zhex has no first-class callbacks, so copying the `use` mechanism would introduce more than error handling. Zhex already reserves `use` for modules.

**Design inference:** borrow the explicit value model and checked branches. Keep propagation short without importing the full callback vocabulary. An error returned as a value is different from a runtime fault or an exception escaping an arbitrary call.

## Direction established in the conversation

The owner wants each call that can fail to show that possibility at its call site. A chain of five calls should propagate explicitly at each level and handle the failure once at the boundary. A naked fallible call must be a checker error.

The smallest model worth testing against complete examples is:

1. Explicit fallible word signatures.
2. `try` at every propagating call site; in a fallible word, failure returns to that word's caller as a Result.
3. One local recovery construct that evaluates only on failure and can bind the error.
4. A checker rule preventing silent discard of a Result.

This would **change** the current `try` behavior. Today a failure inside an Int-returning word exits the run. A future caller cannot recover that failure merely by placing a handler around the call; the word must first expose fallibility in its signature.

The examples below use `Result<T, E>`, `ok(value)`, and `catch problem` as **placeholder spellings only**. None is available as shown in current Zhex. They do not choose a general generic-type system, custom error declarations, or final constructor syntax.

## Example 1: a local replacement value

Hypothetical, non-executable Zhex:

```text
let retries = text.to-int("not a number") catch
    3

print(retries)
```

Success supplies the parsed Int. Failure evaluates the handler and supplies `3`. Both paths have type Int. A String fallback or a print-only handler would be a type error. The proposed handler is lazy: successful parsing must not execute its side effects.

A one-line default might eventually abbreviate this block, but choosing both spellings now is unnecessary.

## Example 2: bind and inspect a failure

Hypothetical, non-executable Zhex:

```text
let contents = file.read-all("settings.txt") catch problem
    # problem is available only within this handler.
    # Inspect its structured reason here once that interface is defined.
    print("Could not read settings; using the default.")
    "retries=3"
```

This proposes an error binding, **not an existing error accessor**. Zhex currently has no `error.message`, `problem.code`, error pattern matching, or way to print an error value. The proposed error-data contract below makes inspection concrete; its language accessor syntax remains open.

The desired behavior can be specified without selecting an accessor:

| Caught reason | Application choice |
| --- | --- |
| Missing optional settings file | Use built-in defaults. |
| Invalid UTF-8 settings | Explain that the file needs correction; do not silently substitute defaults. |
| Permission failure | Explain the access problem; preserve the cause for the boundary. |

The existing `file-open` diagnostic can include several operating-system reasons. It is not enough by itself to distinguish missing files from permission failures. Typed reasons or structured details need an explicit decision; parsing diagnostic prose is unsuitable.

## Example 3: propagate through five calls, handle once

Hypothetical, non-executable Zhex. The first three words form a parsing/settings subsystem; the final two show application orchestration. These thin wrappers make the call boundaries visible; they are not a recommendation to split real code this finely.

```text
word parse-retries(raw: String) -> Result<Int, ParseError>
    let retries = text.to-int(raw) try
    ok(retries)

word decode-settings(raw: String) -> Result<Int, ParseError>
    let retries = parse-retries(text.trim(raw)) try
    ok(retries)

word configure-job(raw: String) -> Result<Int, ParseError>
    let retries = decode-settings(raw) try
    ok(retries)

word prepare-job(raw: String) -> Result<Int, ParseError>
    let retries = configure-job(raw) try
    ok(retries)

word run-job(raw: String) -> Result<Int, ParseError>
    let retries = prepare-job(raw) try
    ok(retries)

let retries = run-job("abc") catch problem
    print("The retry count must be a decimal integer; using 3.")
    3

print(retries)
```

Each of the five inner fallible calls carries `try`; the application call carries the handler. A failure crosses each declared Result boundary unchanged and skips the remaining body of that word. Only the outer handler chooses a replacement and prints the application message. Successful work should not be repeated or rolled back implicitly.

This example keeps ParseError throughout. A word declared to return an ordinary Int must not use `try` to leak an undeclared recoverable failure. At the program boundary, an explicitly unhandled failure can still be reported by the runner, but the spelling and exit behavior must be settled in the final plan.

### Error types must agree or be converted deliberately

Both Elm `andThen` and Gleam `result.try` preserve one error type along a chain. A file read produces FileError and parsing produces ParseError; they do not magically become the same type.

For a future `load-settings` returning `Result<Int, SettingsError>`, the design must explicitly convert both FileError and ParseError into SettingsError, or explicitly declare both possible failures. The first option also needs a way to construct that domain error and retain its cause. Neither facility exists today.

Elm's `mapError` and Gleam's `map_error` demonstrate conversion with the shape `Result<T, E> -> Result<T, F>`. This is useful evidence for a later mapping operation, not a reason to invent one in the first implementation. Initially, same-error propagation can remain the simple rule. Do not erase all errors to strings solely to make examples type-check.

## Example 4: alternate work, reporting, and stopping

A handler for `Result<T, E>` must produce T if execution continues with a T binding. It may print an explanation and compute a replacement. It cannot merely print and pretend that an Int exists.

Handling a `Result<Unit, E>` at the application boundary can select an alternate Unit-producing action such as reporting a friendly message. However, printing a message does not define a nonzero process exit status. The design still needs to settle how the boundary explains and stops, or re-propagates the original error. No `exit`, `raise`, early-return syntax, or never-returning type is assumed here.

General Result branching becomes attractive when success and failure trigger different application actions rather than produce interchangeable values. An Elm-like two-arm construct would express that directly. Compare this complete-use-case need against a failure-only handler before selecting syntax.

## Example 5: retry without pretending loops already exist

Hypothetical, non-executable Zhex:

```text
word ask-number() -> Result<Int, InputError>
    let line = read-line("N: ") try
    let number = text.to-int(line) catch
        print("Enter a decimal integer.")
        ask-number() try
    ok(number)
```

ParseError is handled locally. InputError, including end of input, propagates to the caller. This avoids retrying forever on EOF. The recursive retry's `try` is explicit, and its unwrapped Int matches the parse operation's success type.

This remains a limited sketch: recursive attempts consume the existing call-depth budget. A reliable indefinite prompt needs a separate loop/control-flow decision, or a deliberate retry construct. A bounded `for` plus state can support a fixed attempt count, but should not be presented as an unlimited loop. Error handling alone does not supply while/break/continue.

## No silent discarded failures

Under the owner's proposed rule, a bare call to `text.to-int("abc")` is a checker error even when its value is not otherwise needed. A propagating call must say `try`; a handled call must name its handler.

Allowing stored Results while requiring eventual handling is a stronger rule than unused-variable detection. For example, handling a stored Result in only one branch leaves another path unchecked; assigning an alias or replacing a mutable binding can hide a discard. Loop bodies add a zero-iteration path.

Two implementation choices remain open:

- Require immediate handling or propagation in the first slice, avoiding deferred Result obligations.
- Permit stored Results and require conservative checking that every path handles or propagates them before scope exit. Define how aliases, assignments, branch joins, and transfer to another word work. Merely reading the variable is insufficient.

The second is more flexible; it should not be described as a trivial extension. Do not claim that Elm or Gleam implements this exact proposed Zhex obligation rule. Gleam documents an [unused-Result warning](https://gleam.run/news/convenient-code-actions/#result-discarding-code-action), which is different from a checker error with all-path tracking.

## Result failures and runtime faults

The proposed recovery construct examines a returned Result. It must not become a blanket handler for division by zero, integer overflow, the runtime call-depth limit, or the direct `replace-empty` error. Those operations currently fail outside Result. Selected buffer bounds failures are Results today, so their recoverability follows their API rather than a blanket assertion that every bounds error is fatal.

Failure during evaluation of a handler's operand is also different from receiving a failed Result. A handler around parsing should not accidentally catch a failed `read-line(...) try` nested in the parse argument. The user should handle input and parsing at the intended boundaries.

## A concrete error shape

The requested model is a tagged choice: a Result contains **either a success value or an error, never both**. Only words that can produce a recoverable failure declare a Result return. Ordinary words keep ordinary return types.

Propose one immutable common error view with the following data. This is a **draft data contract**, not record syntax, an accessor API, or implemented JSON output:

| Field | Proposed meaning |
| --- | --- |
| `code` | Stable machine-readable identifier, reusing existing codes such as `file-open` and `text-to-int`. Branch on this or documented typed reasons, never on message wording. |
| `message` | Concise human explanation of this error. It need not be stable enough for program logic. Do not concatenate a new copy of the same error at each call layer. |
| `operation` | Zhex operation that originally failed, such as `file.read-all` or a user word that deliberately creates an error. |
| `origin` | Original Zhex call-site source file, line, and column. This is the user's failing operation, not the Go implementation's file and line. Plain propagation preserves it. |
| `details` | Optional, documented data specific to the error code. For file-open: the requested path and a normalized reason such as not-found, permission-denied, or other. Buffer errors can supply index and line-count. Missing arguments can supply index and argument-count. Do not include raw input text by default. |
| `cause` | Optional reference to one earlier error when an explicit conversion adds domain context. No cause for ordinary propagation. Preserve a chain; do not flatten the original into a string. |

There is no extra `kind` field duplicating `code`. The static error family E already distinguishes ParseError from FileError; a reporting adapter can derive a family label where needed. `details` is a small documented schema per code, not an arbitrary payload bag that application logic must guess at.

Illustrative serialization of the **proposed data**, not actual runner output:

```json
{
  "code": "file-open",
  "message": "Could not open the settings file.",
  "operation": "file.read-all",
  "origin": {"file": "app.zhex", "line": 12, "column": 16},
  "details": {"path": "settings.txt", "reason": "not-found"},
  "cause": null
}
```

The normalized file reason would be new structured information: today's error mainly embeds the host's reason in prose. Keep existing diagnostic codes stable and add details rather than renaming `file-open` for every operating-system condition. Boundary rendering can retain the existing first diagnostic line and JSON contract; this note does not silently replace either.

An explicit conversion to a future SettingsError would create a new domain code/message and preserve the FileError as its cause. Preserve the original failing source location through that conversion; the conversion site can be reporting context. Do not create a wrapper for each `try`. Cause chains describe **why** an error happened; propagation frames describe **where it travelled**.

[Rust's Error trait](https://doc.rust-lang.org/std/error/trait.Error.html#method.source) separates an error's display from an optional lower-level source. [Go's wrapping model](https://go.dev/blog/go1.13-errors) preserves causes for inspection and warns that exposing a wrapped error makes it part of the API. These support preserving structured causes while deliberately documenting what callers may inspect. Render each cause once; do not repeat its full text inside every outer message.

## Optional application reporting

Error handling must work with no logger, network, account, tracker, or hook. An advanced application may install one reporting adapter at setup, through future runner/host configuration. Registration syntax is intentionally unresolved; it does not belong in every word or `try`.

The reporter receives a separate **reporting envelope**, not extra fields required in every Result:

| Envelope data | Purpose |
| --- | --- |
| `schema-version` | Let adapters evolve without guessing the event shape. |
| `occurrence-id`, timestamp | Correlate and deduplicate this failure occurrence; not semantic error identity. |
| outcome: handled or unhandled | Distinguish explicit recovery from failure escaping the application boundary. |
| error snapshot | The code, operation, origin, and permitted message/details/cause chain. |
| boundary | Where the application chose recovery or let failure escape. |
| optional propagation frames | Bounded list of qualified word and Zhex call-site locations collected during explicit propagation, with a truncation marker. No argument/local values. |
| optional application correlation | Request/run/trace context supplied by the application, not built into error equality or the language's Result type. |

Origin is always useful; collecting a full propagation path is optional. When reporting or development tracing requests it, the runtime can collect call-site metadata at each `try` without invoking the reporter there. Keep this path outside the immutable semantic error, so aliases or repeated use do not mutate one shared error's history. Exact bounds belong in the final implementation plan. A native Go stacktrace is not a substitute for the Zhex path.

### When the hook fires

Recommend terminal boundary reporting, **not every produced error**:

| Stage | Proposed reporting behavior |
| --- | --- |
| A built-in/user word produces a failed Result | No hook. The caller may recover normally. |
| `try` propagates failure | No hook. Optionally collect a frame only. |
| A local handler recovers | No automatic hook. Expected invalid input should not become tracker noise. |
| An application explicitly reports a selected recovered outcome | Invoke once with handled outcome after recovery succeeds. |
| Failure escapes the application boundary | Invoke once with unhandled outcome before the runner reports it. |

Reporting every produced error would expose retries and temporarily failed work, but also generate noise and capture information before the program decides what matters. The optional adapter should therefore see unhandled terminal failures automatically, plus handled failures only when the application explicitly chooses to report them. Development tracing can cover intermediate stages separately.

Re-propagating or wrapping the same occurrence keeps its occurrence identity; none of the five `try` layers generates a second notification. A fresh retry attempt is a new occurrence. Adapters must deduplicate a repeated terminal notification of the same occurrence; a handled report followed by a deliberate later rethrow can update that occurrence rather than create duplicate tracker incidents. If a handler itself fails, do not first announce successful handling of the old error: report the new terminal outcome, with a cause only if deliberately preserved.

The hook observes outcomes. It cannot replace the error, retry the operation, change the program's return value, or override exit status. Start with a single best-effort callback, bounded delivery, and no recursive invocation: while the hook runs, its own errors do not call it again. Hook failures leave the original failure and normal diagnostics intact; an optional local hook-health notice may describe delivery failure without exposing the original payload. A tracker adapter owns any background queue/retry policy, not the language evaluator.

### What leaves the process

Nothing leaves the process by default. The application chooses an adapter and destination. The exported snapshot should begin with code, operation, and approved source-location information. Omit message text, full paths, details, and causes' payloads unless the application explicitly permits or sanitizes them. Source filenames may need normalization too. In particular, today's parse diagnostic includes the original input, and file diagnostics include paths: treating every existing message as safe to transmit would be wrong. Do not capture arguments, file contents, environment variables, or local values implicitly.

[OpenTelemetry's error-recording guidance](https://opentelemetry.io/docs/specs/semconv/general/recording-errors/) discourages repeated recording and recording internally handled exceptions. Its [exception conventions](https://opentelemetry.io/docs/specs/semconv/exceptions/exceptions-spans/) separate type, message, and stacktrace and identify message text as potentially sensitive. These are reporting design lessons, not a claim that a Zhex Result is an exception. A future adapter can map family/code to a classification and an approved message/path to suitable telemetry fields without making OpenTelemetry a language dependency.

## What remains before a final plan

Settle one recovery spelling, the fallible signature and success-return rules, top-level failure/exit behavior, the stored-Result obligation rule, and accessor syntax for the proposed error shape. Decide whether the first slice supports only equal error types or also explicit conversion. Give flows explicit rules rather than silently lifting their steps over Results. Specify the optional reporter's registration, metadata bounds, and delivery contract separately from ordinary error handling.

Then write the phased design plan required by #23, with complete accepted examples and checker/runtime acceptance cases. Until then, this is research and a reviewable set of examples. The current contract, website reference, roadmap feature status, and interpreter remain unchanged.
