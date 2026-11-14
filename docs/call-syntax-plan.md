# Simple calls and explicit partial application

Accepted design for [#55](https://github.com/desireco/zhex/issues/55), November 13, 2026. **This is a historical implementation contract; the syntax shipped in [`0b21a0c`](https://github.com/desireco/zhex/commit/0b21a0c).** See the current [calls and defaults guide](../site/src/content/docs/calls.md) for user-facing behavior. It follows return inference [#51](https://github.com/desireco/zhex/issues/51), parameter inference [#52](https://github.com/desireco/zhex/issues/52), and word defaults [#53](https://github.com/desireco/zhex/issues/53). Existing parenthesized syntax remains supported. Automatic currying, anonymous functions, rank polymorphism, general named word arguments, and implicit bare function references are outside this feature.

## The intended everyday form

The following examples use the shipped syntax:

```zhex
word join left: String = "one", right: String = "two"
    left + " " + right

let message = join "hello", "world"
print message

let from-hello = join "hello", _
print(from-hello("world"))
```

`join` has success type `String`. Both outputs are `hello world`. Its parenthesized equivalent remains valid with word defaults:

```zhex
word join(left: String = "one", right: String = "two") -> String
    left + " " + right

print(join("hello", "world"))
```

A fully inferred header is also supported after the dependent inference/default milestones: `word join left = "one", right = "two"`. Annotations remain available wherever an intent or ambiguity needs to be stated; there is no fallback to `Any`, `Unit`, or a first caller's type.

## Declaration and layout rules

A simple word declaration may omit the parentheses around its comma-separated parameter list. Parameter annotations, optional return annotations, required-before-default ordering, default expression scope and type constraints retain their existing rules. A zero-parameter header may be `word answer -> Int` or `word answer` when its result is inferable.

Use the parenthesized header for a multiline parameter list. Use it for complex arrow-typed parameters where the parameter type's arrow could otherwise overlap the word's return arrow; do not guess an interpretation from whitespace. For example:

```zhex
word apply(
    operation: (Int) -> Int,
    value: Int
) -> Int
    operation(value)
```

A body still begins on the following line, indented four spaces. An unparenthesized header does not itself enable continuation. The existing `()`, `[]`, `{}` continuation rules apply; angle brackets alone and an unfinished binary expression do not. Preserve the currently rejected trailing-comma policy unless it is separately changed.

## Calls, boundaries, and precedence

Simple bare calls are accepted as an entire statement, the entire RHS of a binding/reassignment, a final word/block value, or a flow step. Commas separate arguments; whitespace alone does not separate several arguments.

```zhex
let size = distance "one", "two"
print 1 + 1

flow "one"
    distance "two"
    print
```

The flow still prepends its current value. The distance step is equivalent to `distance("one", "two")`. No currying is involved in that flow rule.

A bare call consumes its ordinary argument expressions: `print 1 + 1` is `print(1 + 1)`. Preserve explicit nesting where another call or result operator would be ambiguous:

```zhex
print distance("one", "two")
let adjusted = distance("one", "two") + 1
```

Do not interpret `print distance "one", "two"` by guessing which call owns the comma. Require the established parenthesized nested form. Do not reinterpret existing subtraction/index expressions through whitespace; ambiguous first arguments such as a negative number or a list literal may use `operation(-1)` or `operation([1, 2])`. In particular, existing `values [0]` indexing must remain indexing. This is shorthand for simple calls, not a replacement for every use of parentheses.

Trailing `try` or an indented `catch` at a whole bare-call boundary guards that call's result. For a guard on a captured/ordinary argument, use the existing unambiguous parenthesized argument/call form or place it before a separating comma. `let value = parse raw try` means `parse(raw) try`; `parse(read-line("N: ") try)` guards its argument separately. A guard on a partial function value itself is an error, not deferred handling.

Named record constructors retain their existing parentheses, including `Settings(verbose: true)`. Choice constructors retain their existing qualified syntax; nullary alternatives remain values. Constructors are not made curried/partial by this story. Ordinary words, callable values and suitable monomorphic builtins participate; loop-only `range`/`file.lines` and special named-reporter registration are not ordinary partial-call targets. Direct partials of `console.exit`/`console.fail` are also rejected initially: their internal non-returning result has no public function-type spelling. Their direct calls and ordinary explicitly typed wrapper words remain available; this feature does not introduce a public `Never` type.

## Identifier and callable resolution

A bare identifier first resolves as an ordinary value binding. Thus a value named `bye` remains a value. Otherwise an accessible word at a complete expression boundary is a zero-supplied-argument call: `bye` executes the ordinary REPL word, and a word with required arguments diagnoses missing arguments. A word with only defaulted parameters executes using those defaults. This is normal checked calling, not REPL command interception.

A bound function value can be called using its identifier, with `operation(value)` or `operation value`. A lexical function-valued binding is a callable target. Preserve existing word-call resolution for same-named non-function value bindings rather than breaking old valid parenthesized calls. A non-callable value without an accessible callable target gets a precise diagnostic.

A bare word name is never an implicit function reference. Use explicit holes to obtain a function value. Referring to a zero-parameter word as a function value is unsupported initially; do not invent `&word` or another reference operator in this feature. Existing named-word reporter registration remains its specifically checked interface.

## Only underscores create partials

```zhex
word add left: Int, right: Int -> Int
    left + right

let plus-five = add _, 5
let sum: (Int) -> Int = add 5, _
print plus-five(7)
print sum(7)
```

Both calls print `12`. Each standalone `_` argument slot becomes one required parameter of the resulting function, in left-to-right slot order. It is a contextual placeholder, not a new lambda body: `add (_ + 1), 5` does not create a lambda. Existing `_` pattern/discard uses are unchanged.

All required positions must have a supplied argument or an explicit hole. `add 5` is an arity error, not a function value. `join "hello"` uses its right default and immediately returns a string; `join "hello", _` explicitly defers that right argument. A hole must be filled when the resulting function is called, even if that original parameter had a default. Other omitted defaults stay deferred inside the partial.

At least one hole is required. Applying a function value with further explicit holes creates another checked partial; do not build an unbounded recursive dispatch chain that bypasses call-depth limits. There is no automatic underapplication or implicit callable conversion.

## Evaluation, defaults, effects, and captured values

Resolve and validate target, argument positions/types, missing required positions and placeholder shape before any runtime effects, just as the checker validates the rest of a program first.

At partial creation, evaluate the callee and every supplied non-hole argument exactly once, left to right. Store the evaluated values, not live binding cells. Reassigning a captured `var` later does not redirect the stored value. A captured list/array/map/buffer/file retains its normal reference identity, so mutations of that object remain visible. Do not deep-copy collections implicitly.

Do not execute the callee body or omitted defaults at creation. On full invocation, evaluate remaining supplied values once, put them into their hole positions, then evaluate only the still-omitted defaults in original parameter order and declaring-module scope, freshly for that invocation. Repeated calls do not share freshly constructed mutable defaults. Existing explicit supplied references continue to share.

A fallible callee produces its Result only on invocation. The inferred function signature exposes that Result, so callers still use `try`/`catch`. An explicit `try` inside an argument evaluated while creating a partial propagates at creation instead. No failure is silently discarded or captured as a hidden pending Result.

The opt-in checked collection intrinsics keep their narrow direct-call semantics. Capturing their ordinary form produces its ordinary plain-value function contract; do not make a later `try` change a stored function's type. An ordinary wrapper containing a deliberate intrinsic `try` can provide a fallible function contract when recovery is needed. Do not promise recovery from process OOM.

## Function types and safety

Use a monomorphic arrow type with an explicit parameter tuple: `(String) -> Int`, `(String, Int) -> String`, or `(String) -> Result<Int, ParseError>`. Function values normally infer this type, so annotations are not mandatory. Different calls to one function value must satisfy the same type. No rank polymorphism, generic function definitions, variance conversion, lambdas or automatic currying is implied.

A Result in the function's *return signature* is a checked effect contract, not a stored unhandled Result value. Type containment checks distinguish it from an actual Result parameter/capture. Function parameters cannot smuggle Results, and capture arguments must satisfy the immediate handling rules. The function type may appear in ordinary supported annotation/collection/record positions when these checks are preserved.

Concrete caught errors cannot be laundered through a capture or a function-valued call. Apply the same explicit `error.wrap` requirement as ordinary user-word arguments/storage; indirect callable values do not gain a blanket exception for builtin inspection. Direct builtin error inspection remains unchanged.

Preserve module-qualified nominal identities and private type/word boundaries through callable signatures and captures. Bind a word target when the partial is created, consistent with existing checked word pointers; REPL redefinition must not silently reinterpret existing captures. Publish only resolved function signatures and reject incompatible changes transactionally.

Go-managed closure storage may outlive its creating word. Captured files are shared resources, not transferred ownership: reading after explicit close returns the existing failure. No destructor/finalizer or asynchronous execution is introduced. Every word/function/partial dispatch respects existing recursion/depth limits and deliberate exit/stream/traversal cleanup. Cyclic value graphs through mutable collections must not recurse indefinitely when formatting or tracing.

Function values are excluded from direct equality and JSON; formatting uses a concise callable identity/remaining-arity summary rather than recursively dumping captures. Preserve the existing meaning of word-call/return events; partial creation must not pretend the target body already ran. Add only explicitly documented trace data needed to distinguish creation from invocation.

## Delivery, examples, and review

Terra owns implementation and main staging/commits/build/deploy. Sol owns this contract and independent grammar/type/effect/lifetime review. Luna owns public usage docs and runnable examples. The implementation followed #51/#52/#53 in focused slices: simple declaration/call sugar; callable types/storage/invocation; explicit partial creation/default/effect integration; editor/docs/example integration. The contract records the accepted scope; the calls guide describes the shipped behavior.

Keep the current inline Language basics/Algorithms example groups. Add a discoverable words/calls/flows/defaults/partials guide or thematic subsection within the language basics, with beginner examples before advanced capture/error cases. Include full runnable examples for normal calls/defaults, `plus-five`, left/right holes, a partial used in a flow, repeated invocation with fresh defaults, and meaningful fallible invocation. Do not replace beginner recovery with a large reporting demonstration.

**As soon as the reorganized examples are inspectable, notify the owner with a direct preview/live URL and a brief list of the new sections. Do not wait for the entire feature ticket to close to provide that milestone.** No automation is required; this is part of the active delivery.

Required verification: old corpus unchanged; bare/parenthesized equivalence; comma/operator/nesting/negative/index ambiguity cases; name/value shadowing and zero-arg calls; default/missing/extra/hole arity; source-order single-evaluation oracles; mutable alias vs var-reassignment capture; default freshness; capture-time vs invocation-time failures; hidden Result/Error-family rejection; monomorphic higher-order types; module privacy and REPL rollback; deep partial invocation/cyclic capture formatting; early returns/exits/resource cleanup; real PTY and multiline/paste behavior; all shipped editor grammars; source/download/output equality and mobile accessibility. Finish with just test, go vet, a fresh binary, independent review, push and green website deployment before closing #55.
