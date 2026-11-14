# Named records, lists, arrays, and collection loops

Approved first implementation for [#5](https://github.com/desireco/zhex/issues/5) and [#31](https://github.com/desireco/zhex/issues/31). The owner requested implementation, tests, runnable examples, and website documentation. This plan supplies the concrete contract; the initial record/collection slice and later choice/map/effect slices have shipped. Remaining accepted work is tracked below; each new slice still requires review and deployment.

## Scope

A named type is not synonymous with a record. This slice implements nominal records as its first representation, homogeneous growable List<T>, rectangular runtime-shaped Array<T>, and collection loops. Other named representations remain tracked in [#32](https://github.com/desireco/zhex/issues/32), maps/interchange in [#33](https://github.com/desireco/zhex/issues/33), and validated temporal values in [#34](https://github.com/desireco/zhex/issues/34). Sets remain deferred. One-parameter generic choices are implemented; generic words/records, iterator objects, yield, lazy protocols and implicit conversions remain outside scope.

## Records and annotations

```text
type Customer = record
    name: String
    age: Int

let customer: Customer = Customer("Ada", 36)
print(customer.name)
```

`type Name = record` is top-level, followed by four-space-indented `field: Type` declarations. Every field has an explicit type and a unique name. The positional constructor uses declaration order and exact types. Required fields precede optional trailing field defaults, described below. Fields are immutable; read through a bound value with `customer.name`, including nested field paths. A mutable binding can be reassigned to another value of the same named type. There is no field-update syntax in this slice.

Two declarations with identical fields remain different nominal types. A name cannot collide with a constructor/word in the same file or a built-in type. Collect declarations before checking words, resolve forward type references, and reject unknown, duplicate, or recursive definitions before execution. Recursion through collection fields is also excluded initially.

Types can appear in optional binding annotations, word signatures, fields, and nested List/Array type applications. Existing inferred locals continue to work. Imported types use module qualification, e.g. `model.Customer`; internal identity remains qualified even inside the defining file. Module-local type references normalize in that module's context. Imported files may declare types as well as words. A leading underscore makes the declared type private under the existing module visibility rule. Import cycles still fail before checking effects.

An imported module's public word parameters/results and public record fields must not expose its private record types, including through nested List/Array/Result applications. Private words/types may use them internally. Entry-only local helpers need no exported-interface check.

Record fields are immutable values, but a field containing a mutable collection refers to that collection. A record does not clone its collection fields. Record/collection equality and direct printing are rejected in this slice; print specific fields/elements or explicitly build text. No implicit formatting or deep copy is invented.

## Lists

```text
let numbers: List<Int> = [1, 2, 3]
let empty: List<String> = []
list.append(numbers, 4)
let first = numbers[0]
numbers[1] = 7
```

List<T> is homogeneous and one-dimensional. Nonempty literals infer one checked element type; mixed element types fail. Empty literals and `list.new()` require an expected List<T>, supplied by an annotation or another declared type context. This is ordinary checked inference, not conversion or a dynamic heterogeneous table.

| Operation | Result |
| --- | --- |
| `list.new()` | Contextually typed empty List<T> |
| `list.length(values)` | Int |
| `list.append(values, value)` | Unit; direct try/catch selects Result<Unit, Error> |
| `values[index]` | T; direct try/catch selects Result<T, Error> |
| `values[index] = value` | Unit; failure is a runtime diagnostic |
| `(values[index] = value)` | Result<Unit, Error> |

Indices are zero-based signed Ints. Negative or out-of-range access/write fails with `list-range`. A receiver, each index, and the write value evaluate exactly once, left to right; validate before committing a mutation. Failed writes/append do not change data or length. All returned Results follow #23's explicit handling/propagation rules.

List assignment and parameter passing share mutable storage, as Buffer does. `let` fixes the name, not the collection. Reassigning a `var` changes that binding and does not redirect existing aliases. Successful writes/append are visible through every alias. No implicit copy or ownership-transfer protocol is added.

## Arrays

```text
let grid: Array<Int> = array.new([2, 3], 0)
let cell = grid[1, 2]
grid[1, 2] = 7
array.resize(grid, [3, 4], 0)
```

Array<T> is rectangular and has runtime rank at least two. Its type does not encode static capacity or extents. Dimensions are a List<Int>; annotations/context keep empty or nested values typed. Arrays do not use nested-list literals as an implicit conversion.

| Operation | Result |
| --- | --- |
| `array.new(dimensions, fill)` | Array<T>; direct try/catch selects Result<Array<T>, Error> |
| `array.shape(values)` | Fresh List<Int> of extents |
| `array.rank(values)` | Int |
| `array.length(values)` | Int total cell count |
| `array.resize(values, dimensions, fill)` | Unit; direct try/catch selects Result<Unit, Error> |
| `values[i, j, ...]` | T; direct try/catch selects Result<T, Error> |
| `values[i, j, ...] = value` | Unit; failure is a runtime diagnostic |
| `(values[i, j, ...] = value)` | Result<Unit, Error> |

Rank must be 2..32. Extents are nonnegative; any zero extent makes the array empty. Check rank, dimensions, product overflow, and the runtime element limit before allocation or mutation. The initial maximum is 1,000,000 elements per list/array. These are explicit runtime safety limits, not type-level capacities or a promise to recover from process-level out-of-memory termination.

Use exactly one zero-based index per axis. Wrong rank fails with `array-rank`, invalid shape with `array-shape`, and invalid coordinates with `array-range`. Construction/growth beyond the element limit fails with `collection-limit`. Preserve operation/source and meaningful index/shape details in the common Error view; no new implicit error-family conversion is needed.

Resize preserves rank and mutates shared array storage atomically. Values in the overlap of old/new coordinates retain their logical positions. New coordinates use fill; removed coordinates are discarded, and later regrowth does not restore them. Shape lists returned by `array.shape` are independent: changing one cannot change the array's shape. Validate/copy incoming dimension values before committing, even if dimensions were obtained through an alias.

Fill repeats normal value/reference semantics. Scalars and immutable record fields retain their values; a mutable collection supplied as fill is the **same referenced collection** in each filled cell. There is no hidden deep clone or implicit per-cell factory. Document and test this rule; independent cells can be assigned separately when needed.

## Error-family and Result boundaries

Record fields and collection elements cannot hold Result values and hide unhandled failure obligations. Concrete caught errors cannot be laundered into common Error fields/elements without explicit `error.wrap`; apply the same constraint already used for Error-valued returns and success payloads in #23. Ordinary common Error values created or converted explicitly can be stored where Error is declared.

Passing a caught concrete-family error to any user word also requires explicit `error.wrap`, including a word that only formats its argument today. A helper can return an aggregate or mutate shared storage, so checking only direct constructor/append/write expressions would allow implicit family conversion. This conservative boundary avoids adding effect analysis. Built-in error inspection, reporting, and wrapping operations still accept the caught error directly. Error-valued user flow steps are conservatively rejected even after wrapping; use a direct call, where argument provenance is checked.

Checked collection operations return Result<T, Error> or Result<Unit, Error>. Type/name/field/constructor errors are checker diagnostics before effects. Ordinary reads, writes, append, creation and resize return plain values and diagnose invalid operations at runtime. Direct try/catch selects recoverable Results only for the designated collection intrinsics; checked writes retain the parenthesized form. See #40/#43/#44 below. There is no silent default value for failed access and no implicit ordinary Result discard.

## Collection loops (#31)

```text
for value in numbers
    print(value)

for index, value in numbers
    print(int.to-string(index) + ": " + int.to-string(value))

for value in grid
    print(value)

for coordinates, value in grid
    let row = coordinates[0]
    let column = coordinates[1]
    print(value)
```

Evaluate the collection expression once. Bind immutable, inferred element/index/coordinate names in fresh per-iteration scope. Lists traverse increasing indices. Arrays traverse row-major coordinates with the last axis varying fastest. Empty lists and zero-sized arrays execute no bodies. No try marker is added to ordinary traversal; fallible body operations remain explicit.

Value-only traversal does not copy/snapshot the collection. Array coordinate traversal creates a fresh independent List<Int> each step. Retaining coordinates from one iteration cannot make them change at the next step. An element that itself contains a shared collection remains a reference to that collection.

### Mutation is rejected before the write

An iterator increments an active traversal count on the collection object's identity and releases it on every exit. Checked mutators of that same object return `collection-iterating` **before changing it**, even when reached through aliases; the checked failure can be caught inside the body. Plain indexed writes raise the corresponding runtime diagnostic instead. Failed mutation leaves the object unchanged. Modifying a separate output collection is allowed. Nested traversals of one object retain the guard until all of them finish.

Guard release must cover normal completion, early Result return, propagated Result failure, runtime fault, console.exit and console.fail. Deliberate control flow must not be replaced by a mutation/cleanup error. Existing guaranteed file.lines cleanup remains intact. This is a pre-mutation guard, not post-write invalidation or a hidden whole-collection snapshot.

## Incremental delivery and verification

Terra is sole staging/commit/push owner and implements in focused conventional commits on main: annotations/types; records/modules; lists/access; arrays/resize; #31 traversal/guards; integration. Luna owns executable corpus examples, public topic pages, repository contract/guides, AGENTS/ROADMAP/llms/test catalog. Sol owns this design and independent adversarial review.

Tests are part of each slice, before implementation where behavior changes:

- Correct/unknown/duplicate/nominal/recursive types, annotations, constructors, immutable fields, forward references, imported qualification/privacy/collisions, nested generic applications, no implicit conversion or Result/Error-family laundering.
- Homogeneous/empty lists, nested mutable references, record-held collections, bracket evaluation order, negative/range/rank/shape/product/limit failures and precise diagnostics.
- Every failed write/append/resize preserves all original observable state, overlap coordinates survive resizing, removed cells do not reappear, mutable fills share by the declared rule, shape snapshots do not alias array metadata.
- List/array value and dual-binding traversal order, ranks and coordinate independence, one evaluation, empties, name types/scope, nesting, aliases, pre-mutation guard failures, separate outputs, and every error/return/deliberate-exit release path.
- Real programs exercising try/catch/reporting and fresh built CLI execution, plus syntax support in every shipped editor and validated website snippets/downloads.

Run just test, go vet ./..., applicable editor checks, and a fresh just build with real binary smoke tests. Review/fix findings, push and wait for green website deployment, then record evidence before closing #5/#31. Complete this approved slice without claiming the staged #32/#33/#34 work has shipped.

## Approved continuation: choices and matching (#32)

The owner resumed the whole #5 epic after the first slice shipped. Choices, one-parameter generics and explicit transparent aliases are the selected named representations. Additional nominal scalar/list wrappers were not selected. Opaque types were explicitly excluded and are not active or future requirements for this epic.

```text
type Status = choice
    Pending
    Running
    Failed(message: String)

let status = Status.Failed("File not found")
let description = match status
    case Status.Failed(message)
        message
    case _
        "no failure"
```

Constructors retain their type qualifier. Inference removes redundant binding annotations, not the qualifier. A nullary variant is a value such as `Status.Pending`; payload variants take positional arguments. Imported constructors use `model.Status.Pending`. All variants share the parent type's visibility; public payload types cannot leak module-private types. Constructor collisions, including exact overlap with imported qualified words, diagnose before effects. A value binding with the same base spelling does not change constructor resolution.

Match accepts an arbitrary choice-valued expression evaluated once. It works as a statement, final word result, or whole binding RHS. Payload bindings are immutable and scoped to one arm; `_` ignores a payload. Without a default, every variant must appear. `case _` appears at most once, last, and covers remaining variants. Duplicate arms, missing coverage and unreachable defaults diagnose. Arm result types agree, with never-returning exits allowed. No guards, nested patterns, pinning or multi-clause words are introduced.

Choice payloads cannot hide Results or implicitly convert concrete Error families. Matching must preserve these obligations through branch results, nested matches/catches and final Result-returning words. No direct choice formatting/equality or recursive type graphs are added. One-parameter generic choices are implemented with payload inference and contextual nullary alternatives. Built-in Option provisioning is recorded below; generic words and records remain outside scope.

## Approved continuation: inferred fallibility (#38)

```text
word parse(raw: String) -> Int
    flow raw
        text.trim
        text.to-int try

word configure(raw: String) -> Int
    parse(raw) try

let count = configure("abc") catch
    3
```

These examples describe the approved #38 behavior, not the earlier interpreter contract. An ordinary return annotation names the success type. An escaping `try` infers one error family and exposes a checked `Result<T, E>` call signature; success values wrap automatically once. Pure words retain plain return types. Every caller still explicitly handles or propagates a Result, and raw Results cannot be silently discarded. Explicit Result declarations with `ok`/`err` remain supported without double wrapping.

Infer effects over the complete call graph to a fixed point, including recursion, forward calls and modules. A catch handles the protected returned failure; argument/seed propagation and handler failures may still escape. Multiple escaping families require explicit handling/conversion, never implicit widening. Runtime faults and deliberate process exits are not Result effects. Public/REPL signatures expose inferred effects, and incompatible REPL redefinitions must recheck dependents or reject transactionally.

Existing composition already avoids many temporary names: a flow seed can be a computed or unwrapped call, a word may end in a catch fallback, and a match arm may end in a flow. Before #39, flow steps were unary and multiple arguments required direct calls. The approved extension below adds explicit arguments while keeping the same visible composition. No implicit stack, placeholder, partial application or callback protocol is required.

## Approved maps and flow calls (#33, #39)

The owner has now approved both extensions. This supersedes earlier tentative fallible-map-mutation and fixed-map-limit recommendations; current List/Array rules remain unchanged.

```text
let scores = {"Ada": 10}
scores["Ada"] = 11
map.remove(scores, "absent")

match scores["Ada"]
    case Option.Some(value)
        print(value)
    case Option.None
        print("missing")

flow scores
    map.put("Nancy", 23)
    map.length
    print

flow {"Grace": 42}
    map.into(scores)
```

Nonempty literals infer exact homogeneous Map<K,V>; empty literals require an expected type. Constant String/Int/Bool keys include negative integers; escaped-string and signed-integer equality determines duplicate literal keys, which diagnose before effects. Value expressions evaluate once left to right. Map values cannot hide Results or implicitly erase concrete Error families.

Option<T> is a built-in reserved generic choice with qualified `Option.Some(value)` and nullary `Option.None`; the latter requires an expected type. Lookup returns Option<V> without try. `map.length` returns Int and `map.contains` returns Bool. Plain index assignment and `map.remove` return Unit; removing an absent key is a no-op. `map.put(target,key,value)` returns that same target. `map.into(source,target)` merges and returns that same target, using source-first order to fit the flow rule.

Maps share mutable identity and traverse in insertion order: one loop binding receives values, two receive key and value. Updates keep a key's existing position; removing then reinserting moves it last. Automatic growth has no visible fixed capacity or 1M map cap; process OOM is not promised recoverable. Reclaim deleted order entries rather than growing tombstone storage without bound.

All mutation attempts on an actively traversed target diagnose before changing it, including no-op removals/overwrites. This is a runtime programmer diagnostic, not a Result effect. Release guards on every exit. A different target can receive values from a currently traversed source. Merge preflights the target guard and preserves atomic observable state; source==target is a no-op after the same guard check. Existing target positions survive overwrites, and new keys append in source order.

Flow steps may now be named calls with explicit extra arguments. Prepend the prior flowing value as argument one; evaluate explicit arguments once left to right after producing that value. Bare unary steps stay compatible. Reuse ordinary argument/context checking, including generic constructors and empty collections, while preserving Error provenance. Both postfix step try and try inside explicit arguments contribute their normal escaping effects. No placeholder or partial function object is introduced.

Map/flow examples in this section are implemented, independently reviewed and shipped. The approved typed JSON continuation below is also implemented and independently reviewed; release gates and deployment remain required for each new slice. No Array/arbitrary-choice wire format is invented.

## Approved opt-in indexed-write guards (#40)

The owner subsequently requested ordinary indexed assignment without mandatory guard plumbing while retaining explicit recovery. Both forms are supported:

```text
grid[0, 1] = 7
(grid[0, 1] = 8) try
(grid[0, 1] = 9) catch problem
    print(error.code(problem))
```

For List/Array, a plain indexed assignment statement yields Unit and turns a bounds, rank or active-traversal failure into a source-located runtime diagnostic. A parenthesized indexed-write expression keeps Result<Unit,Error>; try propagates and catch handles that failure. The parser distinguishes these forms. This narrow intrinsic rule does not permit silently discarded ordinary Results.

Both forms evaluate receiver, indices and value once left to right, then validate before mutation. Neither leaks an unchecked Go indexing panic or changes storage on failure. The active-identity guard applies to aliases and no-op attempts in the selected failure mode, and releases on every exit. Plain assignment has no inferred Result effect; explicit checked try and tries in subexpressions retain their effects.

This historical indexed-write slice is extended by #43/#44 below: reads, list.append, array.new and array.resize are now plain by default with direct opt-in checked handling. Controlled/preflight failures must not be advertised as reliable recovery from Go process OOM. No bulk initializer API or arbitrary new capacity limit is introduced.

## Approved typed JSON with supplied defaults (#33)

```text
type Settings = record
    retries: Int
    verbose: Bool

let settings = json.decode("{\"verbose\":true}", Settings(3, false)) try
print(settings.retries)
let encoded = json.encode(settings) try
```

The checked builtins infer the target type from the supplied default instance. Record-field defaults are independently implemented in #42, allowing Settings() when all fields default; no generic-word syntax is needed. Missing fields inherit supplied defaults, while present values must have the exact expected type. Unknown and duplicate keys fail. Null is distinct from missing: only Option accepts null as None, and a non-null optional value decodes as Some.

Support Int, Bool, String, ordinary records, List<T>, Map<String,T> and Option<T> recursively. Reject unsupported resources, Error/Result, Array, arbitrary choices and non-string-key maps before execution. Do not invent a wire representation for unsupported types. Opaque types are excluded, not a codec dependency.

Construct a fresh result without mutating defaults. Inherited mutable lists/maps are deeply cloned for each decode, with a memo table preserving alias relationships within one inherited graph. A present collection replaces its field instead of merging into its default; newly introduced record elements require all fields when no record prototype is supplied. A nested supplied record or Some(record) can provide nested defaults.

Use exact JSON number tokens, never float64 rounding. Initial Int decoding requires an in-range signed-64 integer token; decimal/exponent forms fail explicitly. Encoding includes all fields, in declaration order, with maps in insertion order. None encodes null, Some its payload, empty lists/maps as []/{}. Consume one JSON value and reject trailing non-whitespace.

Failures use common Error with json-syntax, json-type, json-field, json-range or json-depth codes. The path detail is JSON Pointer: empty at the root, /items/0/name for nested data, with ~0 for ~ and ~1 for /. Preserve the Zhex call's source and operation. A nesting limit of 256 is recoverable; existing List limits apply and Map remains uncapped. See the [Go JSON documentation](https://pkg.go.dev/encoding/json#Decoder.UseNumber) for token-preserving decoding and [JSON Pointer](https://www.rfc-editor.org/rfc/rfc6901.html#section-3) for the path convention.

## Accepted automatic timezone interpretation (#34)

The owner selected automatic documented resolution. Repeated local times resolve to the earlier instant. Nonexistent local times shift forward by the actual transition gap, which is not assumed to be one hour. Preserve the named zone and resolved instant; converting zones preserves the instant.

This follows Temporal's compatible policy, described in its [official timezone documentation](https://tc39.es/proposal-temporal/docs/timezone.html). Go time.Date explicitly leaves ambiguous-zone selection unspecified, so it must not define Zhex's policy accidentally; see [Go time.Date](https://pkg.go.dev/time#Date). Validate invalid calendar/clock fields separately from legitimate timezone gaps. Date/time APIs and tests should be finalized as ordinary builtins; no opaque-type feature is required.

## Approved aliases and optional record-field defaults (#32, #42)

Transparent aliases use `type Names = alias List<String>` and normalize to the exact target type; there is no constructor or nominal wrapper. Forward references, cycles, private exposure, imported identity and expanded nesting are checked. Aliases cannot hide Result obligations or erase Error-family provenance. The owner selected aliases; additional nominal wrappers were not selected, and opaque declarations are explicitly excluded.

Record fields may opt into `field: Type = expression`. Required fields come before defaulted fields; constructors may omit only trailing defaulted arguments. Supplied arguments override defaults. No automatic zero/null defaults or named arguments are introduced. Defaults evaluate only when omitted, in order after supplied arguments, in the defining module's scope. They cannot capture caller/global bindings or earlier fields. Plain helper calls are permitted; escaping Result effects are rejected, while a helper may handle its own expected failure. Each construction evaluates mutable defaults afresh, with normal sharing for explicitly supplied values. Constructor-default recursion is bounded separately and unwinds safely. JSON can use `json.decode(raw, Settings()) try` for all-defaulted Settings.

## Plain collection computation and opt-in recovery (#43, #44)

Plain List/Array indexed reads yield the element. `array.new` yields Array<T>; `list.append` and `array.resize` yield Unit. Failures produce automatic safe runtime diagnostics. Apply try/catch directly to one of these intrinsics to select the checked Result form; marked collection flow steps behave the same. Parentheses do not prevent direct selection. Maps retain Option lookup and plain mutation; ordinary Result-returning user/builtin calls still require explicit handling.

Receiver/arguments/indices evaluate once left to right. Recovery handles only the selected intrinsic's own controlled failure, never nested argument/receiver faults. Explicit tries in nested expressions preserve their effects. Bounds, shape, limits and active traversal are validated before mutation; failed operations leave storage unchanged and all guards unwind. Real process OOM is not promised recoverable and no new artificial limits are introduced. `let item = values[i]` now stores T; later `item try` is invalid. Put the guard directly on the indexed read if deliberate recovery is needed.

## Released temporal API (#34)

The issue body is the exact executable contract. Date/Time numeric construction, Duration nanosecond construction, zoned composition, calendar add-days and elapsed add-duration are plain validated computation. Invalid numeric values or range overflow produce safe runtime diagnostics. Date/time/instant/duration parsing and timezone.load remain Result boundaries. The approved automatic gap/overlap policy above is final; parsing is not silently used as a substitute for numeric construction. Date, Time, Instant, Duration, TimeZone and ZonedDateTime remain distinct, with explicit formatting and conversion. No temporal JSON representation is silently invented.
