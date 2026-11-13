---
title: Lists and arrays
description: Create homogeneous lists and shaped arrays, index them safely, and mutate their shared storage.
section: Language
order: 35
---

Lists and arrays are mutable, homogeneous collections. Their element type is checked and their variables hold shared references: assigning a collection to another name does not copy its storage. `let` prevents rebinding that name, while collection operations can still change the shared collection. Collection size is limited to 1,000,000 elements.

## Lists

A nonempty list literal infers its element type from its elements. Every element must have the same checked type:

```zhex
let names: List<String> = ["Ada", "Grace", "Lin"]
print(list.length(names))
```

An empty list needs an expected element type:

```zhex
let names: List<String> = []
list.append(names, "Ada") try
print(list.length(names))
```

`list.new()` creates the same empty list when an expected `List<T>` type is available:

```zhex
let empty: List<String> = list.new()
```

`list.append` returns a Result and must be handled immediately. The list API is:

| Operation | Result | Behavior |
| --- | --- | --- |
| `list.new()` | `List<T>` | Create an empty list when an expected `List<T>` type is available. |
| `list.append(list, value)` | `Result<Unit, Error>` | Append a value of the list's element type; fails with `collection-limit` if the size limit would be exceeded. |
| `list.length(list)` | `Int` | Return the number of elements. |

Read and write a list element with one zero-based signed `Int` index. Each access returns a Result, failing with `list-range` when the index is negative or outside the list. The error exposes `index` and `length` details:

```zhex
let items: List<Int> = [10, 20]
let second = items[1] try
(items[0] = second) try
```

## Arrays

An array has a fixed rank and a shape: one nonnegative extent per dimension. `array.new` creates a filled array from a list of extents and a value whose type determines the element type:

```zhex
let shape: List<Int> = [2, 3]
let grid: Array<Int> = array.new(shape, 0) try
print(array.rank(grid))
print(array.length(grid))
```

Rank is the number of dimensions; length is the total number of elements. Arrays have rank 2 through 32. Each extent must be nonnegative; a zero extent makes an empty array. The product of the extents is checked against the 1,000,000-element limit. `array.shape` returns the extents as a fresh list.

| Operation | Result | Behavior |
| --- | --- | --- |
| `array.new(shape, fill)` | `Result<Array<T>, Error>` | Allocate a shaped array filled with `fill`; fails with `array-rank`, `array-shape`, or `collection-limit` for invalid dimensions or size. |
| `array.resize(array, shape, fill)` | `Result<Unit, Error>` | Change the extents while keeping the rank; failed resizes leave the original array unchanged. |
| `array.shape(array)` | `List<Int>` | Return the current extents in a fresh list. |
| `array.rank(array)` | `Int` | Return the number of dimensions. |
| `array.length(array)` | `Int` | Return the total element count. |

Array reads and writes use zero-based bracket indexing. Use exactly one index for each axis. Reads and writes can fail, so handle them with `try` or `catch`:

```zhex
let grid: Array<Int> = array.new([2, 3], 0) try
let first = grid[0, 0] try
(grid[0, 1] = first + 1) try
```

An index is checked against every dimension and fails with `array-range` when it is outside the shape; details include `axis`, `index`, and `extent`. List indexing uses `list-range` with `index` and `length` details. Receivers, indices, and assigned values evaluate once, left to right. Failed writes leave the collection unchanged. Array traversal order is row-major: the last coordinate changes fastest. `array.resize` preserves values at overlapping coordinates; removed values are discarded. Newly introduced cells receive the supplied fill value. When the fill value is itself a mutable collection, each new cell refers to that same collection; resize does not make deep copies.

`array.resize` cannot change the rank. Invalid rank fails with `array-rank` (`rank`, `expected_rank`); rank errors while creating an array may also include `min_rank` and `max_rank`. Negative extents or other invalid dimensions fail with `array-shape` (`axis`, `extent`); an element count over the limit fails with `collection-limit` (`limit`). A checked shape/limit failure leaves existing collections unchanged. Process-level out-of-memory termination is not recoverable. Array dimensions are explicit `List<Int>` values; nested lists are not converted to arrays.

List and array indexes, dimensions, sizes, and mutations return the common `Result<..., Error>` family where they can fail. These Results must be handled with `try` or `catch`. Collections cannot contain a `Result` value, so a fallible operation cannot hide an unhandled result obligation in an element. A caught concrete error cannot be hidden as an `Error` collection element or passed to a user word taking `Error` without first using `error.wrap`; built-in `error.*` operations accept it directly. Records and collections cannot be printed or compared directly; inspect fields/elements or construct text explicitly.

Lists and arrays are shared mutable storage. See [collection iteration](iteration.html) for traversal order, positions, and mutation restrictions while iterating.
