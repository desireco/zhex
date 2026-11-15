---
title: Lists and arrays
description: Create homogeneous lists and shaped arrays, index them safely, and mutate their shared storage.
section: Language
order: 35
---

Lists and arrays are mutable, homogeneous collections. Their element type is checked and their variables hold shared references: assigning a collection to another name does not copy its storage. `let` prevents rebinding that name, while collection operations can still change the shared collection. List and array size is limited to 1,000,000 elements; maps have no fixed entry-count limit (see [maps](maps.html)).

## Lists

A nonempty list literal infers its element type from its elements. Every element must have the same checked type:

```zhex
let names: List<String> = ["Ada", "Grace", "Lin"]
print(list.length(names))
```

An empty list needs an expected element type:

```zhex
let names: List<String> = []
list.append(names, "Ada")
print(list.length(names))
```

`list.new()` creates the same empty list when an expected `List<T>` type is available:

```zhex
let empty: List<String> = list.new()
```

Plain operations return ordinary values and raise runtime diagnostics when validation fails. Apply `try` or `catch` directly to a supported operation to handle its own failure as a Result. The list API is:

| Operation | Result | Behavior |
| --- | --- | --- |
| `list.new()` | `List<T>` | Create an empty list when an expected `List<T>` type is available. |
| `list.append(list, value)` | `Unit` | Append a value of the list's element type; fails with `collection-limit` if the size limit would be exceeded. |
| `list.length(list)` | `Int` | Return the number of elements. |
| `list.map(list, transform)` | `List<U>` | Call `transform` (`(T) -> U`) once per element, left to right; fresh outer list, shared elements, source unchanged. |
| `list.filter(list, predicate)` | `List<T>` | Keep the elements whose `(T) -> Bool` predicate holds; same freshness rules as `list.map`. |
| `list.select(list, predicate)` | `List<T>` | Keep the elements whose `(T) -> Bool` predicate holds, unmodified; `select` visits every element with no short-circuit; same freshness rules as `list.map`. |
| `list.fold(list, initial, step)` | `A` | Combine elements left to right with `step` (`(A, T) -> A`), starting from `initial`; return the final accumulator. |
| `list.find(list, predicate)` | `Option<T>` | Return `Option.Some` for the first matching element and `Option.None` when nothing matches, including an empty list. |
| `list.any(list, predicate)` | `Bool` | Stop at the first element whose predicate holds and return `true`; an empty list is `false`. |
| `list.all(list, predicate)` | `Bool` | Stop at the first element whose predicate fails and return `false`; an empty list is `true`. |
| `list.sort(list)` | `List<T>` | Return a stable ascending copy of a `List<Int>` or `List<String>`; `String` order is Unicode code-point order. |
| `list.sort-by(list, key)` | `List<T>` | Return a stable ascending copy ordered by `key` (`(T) -> Int` or `(T) -> String`), evaluated once per element in source order; same freshness rules as `list.map`. |

Read and write a list element with one zero-based signed `Int` index. A plain read returns the element and raises a `list-range` runtime diagnostic when the index is negative or outside the list. Write `items[i] try` or `items[i] catch` to handle that read as a Result. The error exposes `index` and `length` details. A plain indexed assignment returns `Unit`; an invalid index produces a runtime diagnostic. Parenthesize the assignment to handle that failure as a `Result<Unit, Error>` with `try` or `catch`:

```zhex
let items: List<Int> = [10, 20]
let second = items[1]
items[0] = second
(items[8] = 0) catch problem
    print(error.code(problem))
```

The seven list callback words take ordinary function values — explicit partials such as `text.trim(_)`, word-name partials such as `passing(_)`, or let-bound callables; there is no lambda syntax. Callbacks run once per element, left to right, and may print or mutate unrelated values. A callback whose type returns a `Result` is rejected at check time, as is a `list.fold` accumulator, with `a Result cannot be returned from a callback` (`unhandled-result`). While callbacks run, the source list holds its traversal guard: mutating it, including through aliases, raises the `collection-iterating` runtime diagnostic before the change. `list.any` and `list.all` stop calling the predicate once the answer is determined, so effects from later elements' callbacks do not happen; `list.select` and `array.select` visit every element or row — they never short-circuit.

`list.sort` and `list.sort-by` return stable ascending copies: a fresh outer list with shared elements, an unchanged source, and equal keys keeping source order. `list.sort` accepts `List<Int>` or `List<String>` values; `list.sort-by` orders any list by a key that is a callback under the same contract, evaluated exactly once per element in source order before any comparison while the traversal guard is held, and returning `Int` or `String`. `String` order is Unicode code-point order, without locale collation or normalization. Wrong types fail at check time before any key runs: `list.sort requires a List<Int> or List<String>, found ...`, `list.sort-by requires a key function, found ...`, and `list.sort-by key must return Int or String, found ...`.

```zhex
word passing score: Int -> Bool
    score > 10

let scores: List<Int> = [3, 12, 7]
print(list.length(list.filter(scores, passing(_))))
match list.find(scores, passing(_))
    case Option.Some(first)
        print("first passing: " + int.to-string(first))
    case Option.None
        print("nobody passing")
```

## Arrays

An array has a fixed rank and a shape: one nonnegative extent per dimension. `array.new` creates a filled array from a list of extents and a value whose type determines the element type:

```zhex
let shape: List<Int> = [2, 3]
let grid: Array<Int> = array.new(shape, 0)
print(array.rank(grid))
print(array.length(grid))
```

Rank is the number of dimensions; length is the total number of elements. Arrays have rank 2 through 32. Each extent must be nonnegative; a zero extent makes an empty array. The product of the extents is checked against the 1,000,000-element limit. `array.shape` returns the extents as a fresh list.

| Operation | Result | Behavior |
| --- | --- | --- |
| `array.new(shape, fill)` | `Array<T>` | Allocate a shaped array filled with `fill`; fails with `array-rank`, `array-shape`, or `collection-limit` for invalid dimensions or size. |
| `array.resize(array, shape, fill)` | `Unit` | Change the extents while keeping the rank; failed resizes leave the original array unchanged. |
| `array.shape(array)` | `List<Int>` | Return the current extents in a fresh list. |
| `array.rank(array)` | `Int` | Return the number of dimensions. |
| `array.length(array)` | `Int` | Return the total element count. |
| `array.select(array, predicate)` | `Array<T>` | Keep the rows of a rank-2 array whose `(List<T>) -> Bool` predicate holds: one row per call, left to right, each row a fresh `List<T>`; kept rows are unmodified and the rectangle (row width) is preserved in a fresh outer array. Rank 3+ raises the `array-rank` runtime diagnostic. |

Array reads and writes use zero-based bracket indexing. Use exactly one index for each axis. Plain reads return the element and raise a runtime diagnostic on invalid rank or coordinates. Apply `try` or `catch` directly to a read to handle its failure as a Result. A plain indexed assignment returns `Unit`; an invalid rank or coordinate produces a runtime diagnostic. Parenthesize the assignment when the caller needs a recoverable `Result<Unit, Error>`:

```zhex
let grid: Array<Int> = array.new([2, 3], 0)
let first = grid[0, 0]
grid[0, 1] = first + 1
(grid[0, 3] = 9) catch problem
    print(error.code(problem))
```

An index is checked against every dimension and fails with `array-range` when it is outside the shape; details include `axis`, `index`, and `extent`. List indexing uses `list-range` with `index` and `length` details. Receivers, indices, and assigned values evaluate once, left to right. Failed writes leave the collection unchanged. Array traversal order is row-major: the last coordinate changes fastest. `array.resize` preserves values at overlapping coordinates; removed values are discarded. Newly introduced cells receive the supplied fill value. When the fill value is itself a mutable collection, each new cell refers to that same collection; resize does not make deep copies.

`array.resize` cannot change the rank. Invalid rank fails with `array-rank` (`rank`, `expected_rank`); rank errors while creating an array may also include `min_rank` and `max_rank`. Negative extents or other invalid dimensions fail with `array-shape` (`axis`, `extent`); an element count over the limit fails with `collection-limit` (`limit`). A checked shape/limit failure leaves existing collections unchanged. Process-level out-of-memory termination is not recoverable. Array dimensions are explicit `List<Int>` values; nested lists are not converted to arrays.

Plain reads, append, array creation and resize return ordinary values and raise runtime diagnostics for their own validation failures. Directly applying `try` or `catch` selects the common `Result<..., Error>` family for these operations; a parenthesized indexed assignment has the same checked form. Guards cover that operation only, not failures in its receiver or arguments. A stored `let value = items[i]` is an element, so apply a guard directly to the read when recovery is needed. Collections cannot contain a `Result` value, so a fallible operation cannot hide an unhandled result obligation in an element. A caught concrete error cannot be hidden as an `Error` collection element or passed to a user word taking `Error` without first using `error.wrap`; built-in `error.*` operations accept it directly. Records and collections cannot be printed or compared directly; inspect fields/elements or construct text explicitly.

`array.select(values, predicate)` follows the same callback contract: the predicate is an explicit partial or word name receiving each row as a fresh `List<T>`, called once per row left to right while the source array holds its traversal guard; kept rows come back unmodified in a fresh outer array that preserves the row width, elements are shared with the source, and an empty source or no matching rows gives a zero-row array. The interpreter has no rank-1 arrays, and a rank above 2 cannot be represented as a row list, so it fails at runtime with `array.select requires a rank-2 array, found rank N` (`array-rank`).

Lists and arrays are shared mutable storage. See [collection iteration](iteration.html) for traversal order, positions, and mutation restrictions while iterating.
