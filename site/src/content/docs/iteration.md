---
title: Collection iteration
description: Traverse lists and arrays in a defined order with checked value and position bindings.
section: Language
order: 45
---

The existing `for` statement also traverses lists and arrays. The collection expression is evaluated once. An empty collection runs zero iterations, and each loop binding exists only inside the loop body. Traversal does not snapshot or copy the collection; element references keep their usual shared-reference behavior.

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

The collection cannot be changed while one of its traversals is active, including through an alias. Nested traversals keep the guard active until the last one finishes. Attempts to append or resize the collection during its loop fail with `collection-iterating` before the mutation takes effect. The failure is a `Result` and can be caught in the body:

```zhex
let names: List<String> = ["Ada", "Grace"]
let alias = names
for name in names
    list.append(alias, "new name") catch problem
        print(error.code(problem))
```

A different output collection can be built inside the loop. The traversal guard is released on every exit path, including word returns, `try` propagation, runtime faults, `console.exit`, and `console.fail`.

These loops yield `Unit`; use a list or array operation to keep results. See [lists and arrays](collections.html) for construction, indexing, resizing, and shared storage, and [control flow](control-flow.html) for ranges and file-line loops.
