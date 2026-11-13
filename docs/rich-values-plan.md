# Named records, lists, arrays, and collection loops

Approved first implementation for [#5](https://github.com/desireco/zhex/issues/5) and [#31](https://github.com/desireco/zhex/issues/31). The owner requested implementation, tests, runnable examples, and website documentation. This plan supplies the concrete contract; completion still requires executable verification and deployment. #23 must ship before this implementation starts.

## Scope

A named type is not synonymous with a record. This slice implements nominal records as its first representation, homogeneous growable List<T>, rectangular runtime-shaped Array<T>, and collection loops. Other named representations remain tracked in [#32](https://github.com/desireco/zhex/issues/32), maps/interchange in [#33](https://github.com/desireco/zhex/issues/33), and validated temporal values in [#34](https://github.com/desireco/zhex/issues/34). Sets remain deferred. No general user-defined generic declarations, iterator objects, yield, lazy protocols, or implicit conversions are introduced.

## Records and annotations

```text
type Customer = record
    name: String
    age: Int

let customer: Customer = Customer("Ada", 36)
print(customer.name)
```

`type Name = record` is top-level, followed by four-space-indented `field: Type` declarations. Every field has an explicit type and a unique name. The positional constructor uses declaration order and exact count/types. Fields are immutable; read through a bound value with `customer.name`, including nested field paths. A mutable binding can be reassigned to another value of the same named type. There is no field-update syntax in this slice.

Two declarations with identical fields remain different nominal types. A name cannot collide with a constructor/word in the same file or a built-in type. Collect declarations before checking words, resolve forward type references, and reject unknown, duplicate, or recursive definitions before execution. Recursion through collection fields is also excluded initially.

Types can appear in optional binding annotations, word signatures, fields, and nested List/Array type applications. Existing inferred locals continue to work. Imported types use module qualification, e.g. `model.Customer`; internal identity remains qualified even inside the defining file. Module-local type references normalize in that module's context. Imported files may declare types as well as words. A leading underscore makes the declared type private under the existing module visibility rule. Import cycles still fail before checking effects.

An imported module's public word parameters/results and public record fields must not expose its private record types, including through nested List/Array/Result applications. Private words/types may use them internally. Entry-only local helpers need no exported-interface check. This slice does not implement an opaque representation: returning a private instance publicly and then exposing all of its fields is not a substitute for one.

Record fields are immutable values, but a field containing a mutable collection refers to that collection. A record does not clone its collection fields. Record/collection equality and direct printing are rejected in this slice; print specific fields/elements or explicitly build text. No implicit formatting or deep copy is invented.

## Lists

```text
let numbers: List<Int> = [1, 2, 3]
let empty: List<String> = []
list.append(numbers, 4) try
let first = numbers[0] try
(numbers[1] = 7) try
```

List<T> is homogeneous and one-dimensional. Nonempty literals infer one checked element type; mixed element types fail. Empty literals and `list.new()` require an expected List<T>, supplied by an annotation or another declared type context. This is ordinary checked inference, not conversion or a dynamic heterogeneous table.

| Operation | Result |
| --- | --- |
| `list.new()` | Contextually typed empty List<T> |
| `list.length(values)` | Int |
| `list.append(values, value)` | Result<Unit, Error> |
| `values[index]` | Result<T, Error> |
| `(values[index] = value)` | Result<Unit, Error> |

Indices are zero-based signed Ints. Negative or out-of-range access/write fails with `list-range`. A receiver, each index, and the write value evaluate exactly once, left to right; validate before committing a mutation. Failed writes/append do not change data or length. All returned Results follow #23's explicit handling/propagation rules.

List assignment and parameter passing share mutable storage, as Buffer does. `let` fixes the name, not the collection. Reassigning a `var` changes that binding and does not redirect existing aliases. Successful writes/append are visible through every alias. No implicit copy or ownership-transfer protocol is added.

## Arrays

```text
let grid: Array<Int> = array.new([2, 3], 0) try
let cell = grid[1, 2] try
(grid[1, 2] = 7) try
array.resize(grid, [3, 4], 0) try
```

Array<T> is rectangular and has runtime rank at least two. Its type does not encode static capacity or extents. Dimensions are a List<Int>; annotations/context keep empty or nested values typed. Arrays do not use nested-list literals as an implicit conversion.

| Operation | Result |
| --- | --- |
| `array.new(dimensions, fill)` | Result<Array<T>, Error> |
| `array.shape(values)` | Fresh List<Int> of extents |
| `array.rank(values)` | Int |
| `array.length(values)` | Int total cell count |
| `array.resize(values, dimensions, fill)` | Result<Unit, Error> |
| `values[i, j, ...]` | Result<T, Error> |
| `(values[i, j, ...] = value)` | Result<Unit, Error> |

Rank must be 2..32. Extents are nonnegative; any zero extent makes the array empty. Check rank, dimensions, product overflow, and the runtime element limit before allocation or mutation. The initial maximum is 1,000,000 elements per list/array. These are explicit runtime safety limits, not type-level capacities or a promise to recover from process-level out-of-memory termination.

Use exactly one zero-based index per axis. Wrong rank fails with `array-rank`, invalid shape with `array-shape`, and invalid coordinates with `array-range`. Construction/growth beyond the element limit fails with `collection-limit`. Preserve operation/source and meaningful index/shape details in the common Error view; no new implicit error-family conversion is needed.

Resize preserves rank and mutates shared array storage atomically. Values in the overlap of old/new coordinates retain their logical positions. New coordinates use fill; removed coordinates are discarded, and later regrowth does not restore them. Shape lists returned by `array.shape` are independent: changing one cannot change the array's shape. Validate/copy incoming dimension values before committing, even if dimensions were obtained through an alias.

Fill repeats normal value/reference semantics. Scalars and immutable record fields retain their values; a mutable collection supplied as fill is the **same referenced collection** in each filled cell. There is no hidden deep clone or implicit per-cell factory. Document and test this rule; independent cells can be assigned separately when needed.

## Error-family and Result boundaries

Record fields and collection elements cannot hold Result values and hide unhandled failure obligations. Concrete caught errors cannot be laundered into common Error fields/elements without explicit `error.wrap`; apply the same constraint already used for Error-valued returns and success payloads in #23. Ordinary common Error values created or converted explicitly can be stored where Error is declared.

Checked collection operations return Result<T, Error> or Result<Unit, Error>. Type/name/field/constructor errors are checker diagnostics before effects. Index/rank/shape/limit and active-iteration failures are source-located Result failures; callers must use try/catch. There is no silent default value for failed access and no implicit result discard.

## Collection loops (#31)

```text
for value in numbers
    print(value)

for index, value in numbers
    print(int.to-string(index) + ": " + int.to-string(value))

for value in grid
    print(value)

for coordinates, value in grid
    let row = coordinates[0] try
    let column = coordinates[1] try
    print(value)
```

Evaluate the collection expression once. Bind immutable, inferred element/index/coordinate names in fresh per-iteration scope. Lists traverse increasing indices. Arrays traverse row-major coordinates with the last axis varying fastest. Empty lists and zero-sized arrays execute no bodies. No try marker is added to ordinary traversal; fallible body operations remain explicit.

Value-only traversal does not copy/snapshot the collection. Array coordinate traversal creates a fresh independent List<Int> each step. Retaining coordinates from one iteration cannot make them change at the next step. An element that itself contains a shared collection remains a reference to that collection.

### Mutation is rejected before the write

An iterator increments an active traversal count on the collection object's identity and releases it on every exit. All checked mutators of that same object return `collection-iterating` **before changing it**, even when reached through aliases. Failed mutation leaves the object unchanged and can be caught inside the body. Modifying a separate output collection is allowed. Nested traversals of one object retain the guard until all of them finish.

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
