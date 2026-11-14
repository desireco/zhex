---
title: Collection iteration
description: Traverse lists and arrays in a defined order with checked value and position bindings.
section: Language
order: 45
---

The existing `for` statement traverses lists, arrays, and maps. The collection expression is evaluated once. An empty collection runs zero iterations, and each loop binding exists only inside the loop body. Traversal does not snapshot or copy the collection; element references keep their usual shared-reference behavior.

For a list, bind each value or both its zero-based index and value:

```zhex
let names: List<String> = ["Ada", "Grace", "Lin"]

for index, name in names
    print(int.to-string(index) + ": " + name)
```

`index` is an `Int`. For an array, bind each value or both its coordinate list and value:

```zhex
let shape: List<Int> = [2, 3]
let grid: Array<Int> = array.new(shape, 0) try
(grid[0, 0] = 10) try
(grid[0, 1] = 20) try
(grid[1, 0] = 30) try
(grid[1, 1] = 40) try
(grid[0, 2] = 50) try
(grid[1, 2] = 60) try

for coordinates, value in grid
    let row = coordinates[0] try
    let column = coordinates[1] try
    print(int.to-string(row) + "," + int.to-string(column) + ": " + int.to-string(value))
```

Array values are visited in row-major order: the last coordinate changes fastest. `coordinates` is a fresh `List<Int>` for the current element. A one-binding array loop omits coordinates and receives only each value.

Maps visit values in insertion order. Add a second binding to receive each key and value:

```zhex
let scores = {"Ada": 10, "Grace": 9}

for name, score in scores
    print(name + ": " + int.to-string(score))
```

Lists and arrays cannot be changed while one of their traversals is active, including through an alias. Nested traversals keep the guard active until the last one finishes. Attempts to append, resize, or write an indexed element during a loop fail with `collection-iterating` before the mutation takes effect. Append and resize return checked Results, and a parenthesized indexed assignment can be caught as a Result in the body:

```zhex
let names: List<String> = ["Ada", "Grace"]
let alias = names
for name in names
    (alias[0] = "changed") catch problem
        print(error.code(problem))
    list.append(alias, "new name") catch problem
        print(error.code(problem))
```

A plain indexed assignment during an active traversal raises a runtime `collection-iterating` diagnostic. Use the parenthesized checked form when the loop should recover instead of ending at that diagnostic.

A different output collection can be built inside the loop. The traversal guard is released on every exit path, including word returns, `try` propagation, runtime faults, `console.exit`, and `console.fail`.

An actively traversed map also rejects mutation through any alias, including a no-op removal or merge. Map mutation raises a runtime `collection-iterating` diagnostic before changing storage; it is not a `Result` and cannot be caught. Mutating a different map inside the loop is allowed.

These loops yield `Unit`; use collection operations to keep results. See [lists and arrays](collections.html) for construction, indexing, resizing, and shared storage, [maps](maps.html) for key lookup and insertion order, and [control flow](control-flow.html) for ranges and file-line loops.
